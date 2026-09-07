package evalharness

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// writeSessionManifest writes a qrouton.json in the schema a real launch
// produces, so grading exercises the same keys production writes.
func writeSessionManifest(t *testing.T, workspace string, roles map[string]string) {
	t.Helper()
	var repos []string
	for _, name := range sortedKeys(roles) {
		repos = append(repos, fmt.Sprintf(
			`{"name":%q,"org":"qrouton-eval","role":%q,"defaultBranch":"main","worktreePath":"src/%s"}`,
			name, roles[name], name))
	}
	manifest := fmt.Sprintf(
		`{"schemaVersion":2,"name":"Grade test","slug":"grade-test","description":"Grade test",`+
			`"mode":"rpi","createdAt":"2026-07-16T00:00:00Z","repos":[%s]}`,
		strings.Join(repos, ","))
	writeTestFile(t, filepath.Join(workspace, "qrouton.json"), manifest)
}

func sortedKeys(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func TestGradeChecksSentinelLeakAndObservableEvents(t *testing.T) {
	workspace := t.TempDir()
	writeSessionManifest(t, workspace, map[string]string{"active": "editing", "reference": "reference"})
	for _, repo := range []string{"active", "reference"} {
		repoDir := filepath.Join(workspace, "src", repo)
		writeTestFile(t, filepath.Join(repoDir, "README.md"), repo)
		initializeTestRepo(t, repoDir)
	}
	writeTestFile(t, filepath.Join(workspace, "thoughts", "shared", "research", "01-research.md"), "safe findings")

	scenario := Scenario{Checks: []CheckSpec{
		{Kind: "artifact_exists", Path: "thoughts/shared/research/*.md"},
		{Kind: "artifact_excludes", Pattern: "SECRET"},
		{Kind: "open_file"},
		{Kind: "delegation", Pattern: "research-lead"},
	}}
	result := CaseResult{
		FinalResponse: "Research is ready.",
		Artifacts: []Artifact{{
			Path: "thoughts/shared/research/01-research.md",
			Text: "safe findings",
		}},
		Events: []Event{
			{Kind: "tool_call", Name: "open_file"},
			{Kind: "delegation", Name: "research-lead"},
		},
	}
	baselines := map[string]string{
		"active":    gitHead(t, filepath.Join(workspace, "src", "active")),
		"reference": gitHead(t, filepath.Join(workspace, "src", "reference")),
	}

	assertions := Grade(scenario, result, workspace, baselines)
	for _, assertion := range assertions {
		if !assertion.Passed {
			t.Errorf("assertion failed: %s (%s)", assertion.Name, assertion.Evidence)
		}
	}
}

func TestGradeDetectsReferenceModification(t *testing.T) {
	workspace := t.TempDir()
	writeSessionManifest(t, workspace, map[string]string{"reference": "reference"})
	repoDir := filepath.Join(workspace, "src", "reference")
	writeTestFile(t, filepath.Join(repoDir, "README.md"), "baseline")
	initializeTestRepo(t, repoDir)
	baseline := gitHead(t, repoDir)
	writeTestFile(t, filepath.Join(repoDir, "README.md"), "changed")

	assertion := gradeReferencesUnchanged(workspace, map[string]string{"reference": baseline})
	if assertion.Passed {
		t.Fatal("modified reference repository passed")
	}
}

func TestGradeNoInternalLeakCatchesSkillNamesButNotTheBareProductName(t *testing.T) {
	leaked := gradeNoInternalLeak(CaseResult{FinalResponse: "Delegating to qrouton-plan next."})
	if leaked.Passed {
		t.Fatal("response naming a skill passed no_internal_leak")
	}

	clean := gradeNoInternalLeak(CaseResult{FinalResponse: "This is a qrouton session."})
	if !clean.Passed {
		t.Fatalf("response naming only the product failed no_internal_leak: %s", clean.Evidence)
	}
}

func TestGradeChecksEditingRepositoryRemainsUnchanged(t *testing.T) {
	unchanged := gradeCheck(CheckSpec{Kind: "repo_unchanged", Repo: "app"}, CaseResult{
		Diffs: map[string]string{"app": ""},
	}, t.TempDir())
	if !unchanged.Passed {
		t.Fatal("unchanged editing repository failed")
	}

	changed := gradeCheck(CheckSpec{Kind: "repo_unchanged", Repo: "app"}, CaseResult{
		Diffs: map[string]string{"app": "diff --git a/file b/file"},
	}, t.TempDir())
	if changed.Passed {
		t.Fatal("changed editing repository passed as unchanged")
	}
}

// A framed document and a finished one are the same file; what separates them
// is whether anything sits under the headings.
func TestResearchAnsweredSeparatesFramingFromFindings(t *testing.T) {
	workspace := t.TempDir()
	document := filepath.Join(workspace, "thoughts", "shared", "research", "R1-2026-07-16-retry.md")
	pattern := "thoughts/shared/research/*.md"

	writeTestFile(t, document, strings.Join([]string{
		"# Retry",
		"",
		"## Summary",
		"",
		"What is being investigated, and where to look.",
		"",
		"## How does the client retry?",
		"",
		"> Start in internal/retry.",
		"",
		"## Open Questions",
		"",
	}, "\n"))

	framed := researchAnswered(workspace, pattern)
	if framed.Passed {
		t.Fatal("a document nobody has answered passed as research")
	}
	if !strings.Contains(framed.Evidence, "How does the client retry?") {
		t.Errorf("evidence %q does not name the unanswered question", framed.Evidence)
	}

	writeTestFile(t, document, strings.Join([]string{
		"# Retry",
		"",
		"## Summary",
		"",
		"It backs off three times.",
		"",
		"## How does the client retry?",
		"",
		"Three attempts, doubling the wait (`internal/retry/client.go:41`).",
		"",
		"```go",
		"## not a heading",
		"```",
		"",
		"## Open Questions",
		"",
	}, "\n"))

	// Nothing outstanding is a legitimate answer, and leaving that heading blank
	// is not the framing surviving.
	if answered := researchAnswered(workspace, pattern); !answered.Passed {
		t.Fatalf("a filled-in document failed: %s", answered.Evidence)
	}
}

// A summary is written at framing time, so it cannot be the evidence that the
// research happened.
func TestResearchAnsweredNeedsMoreThanASummary(t *testing.T) {
	workspace := t.TempDir()
	writeTestFile(t, filepath.Join(workspace, "thoughts", "shared", "research", "R1-retry.md"),
		"# Retry\n\n## Summary\n\nWhat is being investigated.\n\n## How does the client retry?\n")

	assertion := researchAnswered(workspace, "thoughts/shared/research/*.md")
	if assertion.Passed {
		t.Fatalf("a document with only its summary written passed: %#v", assertion)
	}
}

func TestArtifactMaxLines(t *testing.T) {
	workspace := t.TempDir()
	writeTestFile(t, filepath.Join(workspace, "thoughts", "shared", "research", "short.md"), "one\ntwo\n")
	writeTestFile(t, filepath.Join(workspace, "thoughts", "shared", "research", "long.md"), "one\ntwo\nthree\n")

	if assertion := artifactMaxLines(workspace, "thoughts/shared/research/short.md", 2); !assertion.Passed {
		t.Fatalf("artifact at line limit failed: %s", assertion.Evidence)
	}
	assertion := artifactMaxLines(workspace, "thoughts/shared/research/*.md", 2)
	if assertion.Passed || !strings.Contains(assertion.Evidence, "long.md (3 > 2)") {
		t.Fatalf("overlong artifact passed: %#v", assertion)
	}
}

func TestDelegationRequiresAnActualSpawnEvent(t *testing.T) {
	available := Event{Kind: "provider_event", Arguments: []byte(`{"agents":["qrouton-research-lead"]}`)}
	spawned := Event{Kind: "provider_event", Arguments: []byte(`{"subtype":"task_started","subagent_type":"qrouton-research-lead"}`)}

	if assertion := delegationAssertion([]Event{available}, "research-lead"); assertion.Passed {
		t.Fatal("available agent list was mistaken for delegation")
	}
	if assertion := delegationAssertion([]Event{spawned}, "research-lead"); !assertion.Passed {
		t.Fatal("actual task start was not recognized as delegation")
	}
}

func TestDelegationAcceptsCodexCollaborationStream(t *testing.T) {
	events := []Event{
		{Kind: "assistant", Text: "Handing this to the research-lead."},
		{Kind: "provider_event", Arguments: []byte(`{"item":{"type":"collab_tool_call","tool":"wait"}}`)},
	}
	if assertion := delegationAssertion(events, "research-lead"); !assertion.Passed {
		t.Fatalf("Codex collaboration was not recognized: %s", assertion.Evidence)
	}
}

func TestDelegationNormalizesAgentNameSeparators(t *testing.T) {
	events := []Event{
		{Kind: "assistant", Text: "Handing this to the planning lead."},
		{Kind: "provider_event", Arguments: []byte(`{"item":{"type":"collab_tool_call","tool":"wait"}}`)},
	}
	if assertion := delegationAssertion(events, "planning-lead"); !assertion.Passed {
		t.Fatalf("agent name separator was not normalized: %s", assertion.Evidence)
	}
}

// spawnEvent is a Claude Task call as the normalizer records one: the tool name
// on the event, the target inside the provider payload.
func spawnEvent(agent string) Event {
	return Event{
		Kind:      "tool_call",
		Name:      "Task",
		Arguments: []byte(`{"tool_name":"Task","subagent_type":"` + agent + `"}`),
	}
}

// The roster arrives with the same key a spawn carries, so the init subtype is
// the only thing separating "these agents exist" from "go".
func agentRosterEvent() Event {
	return Event{
		Kind:      "provider_event",
		Arguments: []byte(`{"subtype":"init","agents":[{"subagent_type":"qrouton-researcher"}]}`),
	}
}

func TestDelegationAbsentSeesEverySpawn(t *testing.T) {
	clarified := CaseResult{Events: []Event{
		{Kind: "user", Text: "Can you improve the account service?"},
		agentRosterEvent(),
		{Kind: "assistant", Text: "What outcome are you after?"},
	}}
	absent := gradeCheck(CheckSpec{Kind: "delegation_absent"}, clarified, t.TempDir())
	if !absent.Passed {
		t.Fatalf("a run that only asked a question failed: %s", absent.Evidence)
	}

	clarified.Events = append(clarified.Events, spawnEvent("qrouton-researcher"))
	spawned := gradeCheck(CheckSpec{Kind: "delegation_absent"}, clarified, t.TempDir())
	if spawned.Passed {
		t.Fatal("a run that spawned a specialist passed as having delegated nothing")
	}
	if spawned.Evidence != "qrouton-researcher" {
		t.Errorf("evidence %q does not name the premature spawn", spawned.Evidence)
	}
}

// The failure the order-blind delegation check cannot see: a leaf specialist
// first, the lead only after a correction.
func TestFirstDelegationJudgesTheEarliestSpawn(t *testing.T) {
	leafFirst := []Event{
		spawnEvent("qrouton-researcher"),
		{Kind: "user", Text: "yo no researchers. qrouton flow please."},
		spawnEvent("qrouton-research-lead"),
	}
	if assertion := firstDelegationAssertion(leafFirst, "research-lead"); assertion.Passed {
		t.Fatalf("a leaf spawned before the lead passed: %s", assertion.Evidence)
	}
	if orderBlind := delegationAssertion(leafFirst, "research-lead"); !orderBlind.Passed {
		t.Fatal("the order-blind check no longer passes this stream, so the new kind is redundant")
	}

	leadFirst := []Event{
		spawnEvent("qrouton-research-lead"),
		spawnEvent("qrouton-researcher"),
		spawnEvent("qrouton-researcher"),
	}
	assertion := firstDelegationAssertion(leadFirst, "research-lead")
	if !assertion.Passed {
		t.Fatalf("a lead spawned before its specialists failed: %s", assertion.Evidence)
	}
	if assertion.Evidence != "qrouton-research-lead" {
		t.Errorf("evidence %q does not name the graded spawn", assertion.Evidence)
	}
}

// Several agents can be spawned in one event, and the stream does not order
// them, so a lead alongside a leaf is still a leaf nobody routed.
func TestFirstDelegationRejectsAFanOutCarryingALeaf(t *testing.T) {
	batch := Event{
		Kind: "tool_call",
		Name: "Task",
		Arguments: []byte(`{"content":[` +
			`{"name":"Task","input":{"subagent_type":"qrouton-research-lead"}},` +
			`{"name":"Task","input":{"subagent_type":"qrouton-researcher"}}]}`),
	}
	assertion := firstDelegationAssertion([]Event{batch}, "research-lead")
	if assertion.Passed {
		t.Fatalf("a batch spawning a leaf beside the lead passed: %s", assertion.Evidence)
	}
	if assertion.Evidence != "qrouton-research-lead, qrouton-researcher" {
		t.Errorf("evidence %q does not name both spawns in stream order", assertion.Evidence)
	}
}

func TestFirstDelegationFailsWhenNothingWasDelegated(t *testing.T) {
	assertion := firstDelegationAssertion([]Event{{Kind: "assistant", Text: "Handing this to the research lead."}}, "research-lead")
	if assertion.Passed {
		t.Fatal("a run that delegated nothing satisfied a delegation-order check")
	}
	if assertion.Evidence != evidenceNoDelegation {
		t.Errorf("evidence = %q, want %q", assertion.Evidence, evidenceNoDelegation)
	}
}

func TestFirstDelegationIgnoresTheAgentRoster(t *testing.T) {
	events := []Event{agentRosterEvent(), spawnEvent("qrouton-research-lead")}
	if assertion := firstDelegationAssertion(events, "research-lead"); !assertion.Passed {
		t.Fatalf("the agent roster was graded as the first delegation: %s", assertion.Evidence)
	}
	if assertion := firstDelegationAssertion([]Event{agentRosterEvent()}, "researcher"); assertion.Passed {
		t.Fatal("the agent roster alone passed as a delegation")
	}
}

// A Codex collaboration call is a wait on threads it does not name, so no
// prose around it can stand in for a target the stream never recorded.
func TestFirstDelegationFailsWhenTheStreamNamesNoTarget(t *testing.T) {
	collaboration := Event{
		Kind:    "provider_event",
		RawType: "item.started",
		Arguments: []byte(`{"item":{"agents_states":{},"id":"item_4","prompt":null,` +
			`"receiver_thread_ids":[],"status":"in_progress","tool":"wait",` +
			`"type":"collab_tool_call"},"type":"item.started"}`),
	}
	events := []Event{
		{Kind: "user", Text: "Get the research lead onto the ticket."},
		{Kind: "assistant", Text: "Handing this to the research lead."},
		collaboration,
	}
	assertion := firstDelegationAssertion(events, "research-lead")
	if assertion.Passed {
		t.Fatalf("prose around an unnamed spawn passed as its target: %s", assertion.Evidence)
	}
	if !strings.Contains(assertion.Evidence, "provider_event item.started") {
		t.Errorf("evidence %q does not identify the unnamed spawn", assertion.Evidence)
	}
}

// An absent pattern matches everything, so the check has to refuse rather than
// tick for any spawn at all.
func TestFirstDelegationRefusesAnEmptyPattern(t *testing.T) {
	result := CaseResult{Events: []Event{spawnEvent("qrouton-researcher")}}
	assertion := gradeCheck(CheckSpec{Kind: "first_delegation"}, result, t.TempDir())
	if assertion.Passed {
		t.Fatal("a check naming no agent passed on a leaf spawn")
	}
	if assertion.Evidence != evidenceNoPattern {
		t.Errorf("evidence = %q, want %q", assertion.Evidence, evidenceNoPattern)
	}
}

func TestSentinelSafetyChecksWorkerBriefsNotOrchestratorReads(t *testing.T) {
	result := CaseResult{Events: []Event{
		{Kind: "tool_call", Text: "read TICKET-SENTINEL"},
	}}
	if assertion := sentinelSafe(result, "TICKET-SENTINEL"); !assertion.Passed {
		t.Fatal("orchestrator ticket read was mistaken for a worker leak")
	}

	result.Events = append(result.Events, Event{
		Kind:      "provider_event",
		Arguments: []byte(`{"subagent_type":"qrouton-research-lead","prompt":"TICKET-SENTINEL"}`),
	})
	if assertion := sentinelSafe(result, "TICKET-SENTINEL"); assertion.Passed {
		t.Fatal("sentinel in worker brief was not detected")
	}
}

func initializeTestRepo(t *testing.T, dir string) {
	t.Helper()
	commands := [][]string{
		{"git", "init", "-q"},
		{"git", "config", "user.email", "eval@example.test"},
		{"git", "config", "user.name", "eval"},
		{"git", "add", "."},
		{"git", "commit", "-qm", "initial"},
	}
	for _, args := range commands {
		command := exec.Command(args[0], args[1:]...)
		command.Dir = dir
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("%v: %v: %s", args, err, output)
		}
	}
}

func gitHead(t *testing.T, dir string) string {
	t.Helper()
	command := exec.Command("git", "rev-parse", "HEAD")
	command.Dir = dir
	output, err := command.Output()
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(output))
}

// atTurn stamps a spawn with the turn it belongs to, the way the harness stamps
// every event it records.
func atTurn(turn int, event Event) Event {
	event.Turn = turn
	return event
}

// mainThreadToolCall is a tool call as Claude records one: nested inside an
// assistant message, so nothing about the normalized kind says "tool".
func mainThreadToolCall(turn int, tool string) Event {
	return Event{
		Kind:      "assistant",
		Turn:      turn,
		Arguments: []byte(`{"message":{"content":[{"name":"` + tool + `","type":"tool_use"}]},"type":"assistant"}`),
	}
}

// The failure no whole-run delegation check can see: turn one routes correctly,
// then turn two reads the codebase itself and hands nothing off.
func TestTurnDelegationFailsWhenALaterTurnAbsorbsTheWork(t *testing.T) {
	absorbed := []Event{
		{Kind: "user", Turn: 1, Text: "Where does the retry work stand?"},
		atTurn(1, spawnEvent("qrouton-research-lead")),
		{Kind: "user", Turn: 2, Text: "Work out how the deadline propagates."},
		mainThreadToolCall(2, "Grep"),
		mainThreadToolCall(2, "Read"),
		mainThreadToolCall(2, "Read"),
		{Kind: "assistant", Turn: 2, Text: "The deadline is read in three places."},
	}

	assertion := turnDelegationAssertion(absorbed, 2, "lead")
	if assertion.Passed {
		t.Fatalf("a turn that delegated nothing passed: %s", assertion.Evidence)
	}
	if want := fmt.Sprintf(evidenceTurnAbsorbed, 2, 3); assertion.Evidence != want {
		t.Errorf("evidence = %q, want %q", assertion.Evidence, want)
	}

	if orderBlind := delegationAssertion(absorbed, "lead"); !orderBlind.Passed {
		t.Error("the whole-run check no longer passes this stream, so the new kind is redundant")
	}
	if earliest := firstDelegationAssertion(absorbed, "lead"); !earliest.Passed {
		t.Error("the first-spawn check no longer passes this stream, so the new kind is redundant")
	}
}

func TestTurnDelegationFailsWhenTheTurnSpawnsASpecialistDirectly(t *testing.T) {
	events := []Event{
		atTurn(1, spawnEvent("qrouton-research-lead")),
		atTurn(2, spawnEvent("codebase-researcher")),
		atTurn(2, spawnEvent("qrouton-research-lead")),
	}
	assertion := turnDelegationAssertion(events, 2, "lead")
	if assertion.Passed {
		t.Fatalf("a leaf spawned before the turn's lead passed: %s", assertion.Evidence)
	}
	if assertion.Evidence != "codebase-researcher" {
		t.Errorf("evidence %q does not name the graded spawn", assertion.Evidence)
	}
}

// A turn the run never got to is a truncated case, not an absorbed one, and the
// two failures need different evidence to be told apart.
func TestTurnDelegationSeparatesATruncatedRunFromAnAbsorbedTurn(t *testing.T) {
	events := []Event{atTurn(1, spawnEvent("qrouton-research-lead"))}
	assertion := turnDelegationAssertion(events, 2, "lead")
	if assertion.Passed {
		t.Fatal("a turn that never ran satisfied a delegation check")
	}
	if want := fmt.Sprintf(evidenceTurnAbsent, 2); assertion.Evidence != want {
		t.Errorf("evidence = %q, want %q", assertion.Evidence, want)
	}
}

func TestTurnDelegationRefusesAnUnusableCheck(t *testing.T) {
	result := CaseResult{Events: []Event{atTurn(2, spawnEvent("qrouton-researcher"))}}

	noTurn := gradeCheck(CheckSpec{Kind: checkTurnDelegation, Pattern: "lead"}, result, t.TempDir())
	if noTurn.Passed || noTurn.Evidence != evidenceNoTurn {
		t.Errorf("a check naming no turn: %#v", noTurn)
	}

	noPattern := gradeCheck(CheckSpec{Kind: checkTurnDelegation, Turn: 2}, result, t.TempDir())
	if noPattern.Passed || noPattern.Evidence != evidenceNoTurnPattern {
		t.Errorf("a check naming no agent: %#v", noPattern)
	}
}

// Scoping cuts both ways: an earlier turn that absorbed its work must not fail
// a later turn that routed properly.
func TestTurnDelegationGradesOnlyItsOwnTurn(t *testing.T) {
	events := []Event{
		mainThreadToolCall(1, "Read"),
		mainThreadToolCall(1, "Grep"),
		atTurn(2, spawnEvent("qrouton-planning-lead")),
		atTurn(2, spawnEvent("pattern-finder")),
	}
	assertion := turnDelegationAssertion(events, 2, "planning-lead")
	if !assertion.Passed {
		t.Fatalf("a turn routed through its lead failed: %s", assertion.Evidence)
	}
	if assertion.Evidence != "qrouton-planning-lead" {
		t.Errorf("evidence %q does not name the graded spawn", assertion.Evidence)
	}
	if first := turnDelegationAssertion(events, 1, "planning-lead"); first.Passed {
		t.Error("turn one's absorbed reading passed on turn two's spawn")
	}
}

// Parallel tool calls arrive as one event carrying several blocks, and evidence
// that counted events would understate the reading by most of it.
func TestTurnDelegationCountsEveryToolCallInABatchedEvent(t *testing.T) {
	batched := Event{
		Kind: "assistant",
		Turn: 2,
		Arguments: []byte(`{"message":{"content":[` +
			`{"name":"Grep","type":"tool_use"},` +
			`{"name":"Grep","type":"tool_use"},` +
			`{"name":"Read","type":"tool_use"}]}}`),
	}
	assertion := turnDelegationAssertion([]Event{batched}, 2, "lead")
	if want := fmt.Sprintf(evidenceTurnAbsorbed, 2, 3); assertion.Evidence != want {
		t.Errorf("evidence = %q, want %q", assertion.Evidence, want)
	}
}

// The shape a recorded Claude run actually carries: a spawn is an assistant
// message with no tool name of its own, and the target sits in the tool input.
func TestTurnDelegationPassesOnARecordedClaudeSpawn(t *testing.T) {
	spawn := Event{
		Kind:    "assistant",
		RawType: "assistant",
		Turn:    2,
		Arguments: []byte(`{"message":{"content":[{"id":"toolu_01MY5q6c7VBXbrok5B5ZiX6P",` +
			`"input":{"description":"Inventory the deadline reads","subagent_type":"qrouton-research-lead"},` +
			`"name":"Agent","type":"tool_use"}],"role":"assistant","type":"message"},` +
			`"session_id":"17cbb42d-1a58-43c2-b86a-fe87374693a3","type":"assistant"}`),
	}
	events := []Event{
		{Kind: "user", Turn: 1, Text: "Where does the retry deadline work stand?"},
		{Kind: "user", Turn: 2, Text: "Now build the inventory."},
		spawn,
	}
	assertion := turnDelegationAssertion(events, 2, "lead")
	if !assertion.Passed {
		t.Fatalf("a recorded spawn of the research lead failed: %s", assertion.Evidence)
	}
	if assertion.Evidence != "qrouton-research-lead" {
		t.Errorf("evidence %q does not name the graded spawn", assertion.Evidence)
	}
}
