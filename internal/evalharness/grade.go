package evalharness

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/kieranajp/qrouton/internal/markdown"
)

var internalLeakPattern = regexp.MustCompile(`(?i)\b(qrouton-(questions|research|spec|plan|implement)|agent depth|document numbering)\b`)

var subagentTypePattern = regexp.MustCompile(`"` + subagentTypeField + `"\s*:\s*"([^"]+)"`)

// testsPassTimeout bounds a fixture repo's test run during grading.
const testsPassTimeout = 5 * time.Minute

func Grade(
	scenario Scenario,
	result CaseResult,
	workspace string,
	baselines map[string]string,
) []Assertion {
	assertions := []Assertion{
		gradeNoInternalLeak(result),
		gradeReferencesUnchanged(workspace, baselines),
	}
	for _, check := range scenario.Checks {
		assertions = append(assertions, gradeCheck(check, result, workspace))
	}
	return assertions
}

func gradeNoInternalLeak(result CaseResult) Assertion {
	matched := internalLeakPattern.FindString(result.FinalResponse)
	return Assertion{
		Name:     assertNoInternalLeak,
		Passed:   matched == "",
		Evidence: matched,
	}
}

func gradeReferencesUnchanged(workspace string, baselines map[string]string) Assertion {
	manifest, err := readManifest(filepath.Join(workspace, manifestName))
	if err != nil {
		return Assertion{Name: assertReferencesClean, Evidence: err.Error()}
	}

	var changed []string
	for _, repo := range manifest.Repos {
		if repo.Role != roleReference {
			continue
		}
		repoDir := repo.dir(workspace)
		status, statusErr := commandOutput(context.Background(), repoDir, gitBin, gitStatusCmd, gitPorcelainFlag)
		head, headErr := commandOutput(context.Background(), repoDir, gitBin, gitRevParseCmd, gitHeadRef)
		// Fail loud: a repo whose state cannot be read is not provably unchanged.
		if statusErr != nil || headErr != nil || status != "" || head != baselines[repo.Name] {
			changed = append(changed, repo.Name)
		}
	}
	return Assertion{
		Name:     assertReferencesClean,
		Passed:   len(changed) == 0,
		Evidence: strings.Join(changed, evidenceJoiner),
	}
}

func gradeCheck(check CheckSpec, result CaseResult, workspace string) Assertion {
	switch check.Kind {
	case checkArtifactExists:
		return fileAssertion(workspace, check.Path, true)
	case checkArtifactAbsent:
		return fileAssertion(workspace, check.Path, false)
	case checkResponseContains:
		passed := strings.Contains(strings.ToLower(result.FinalResponse), strings.ToLower(check.Pattern))
		return Assertion{Name: assertResponseContains + check.Pattern, Passed: passed}
	case checkResponseExcludes:
		matched := firstContained(result.FinalResponse, append(check.Any, check.Pattern))
		return Assertion{Name: assertResponseExcludes, Passed: matched == "", Evidence: matched}
	case checkArtifactExcludes:
		return artifactsExclude(result.Artifacts, check.Pattern)
	case checkArtifactContains:
		return artifactContains(workspace, check.Path, check.Pattern)
	case checkArtifactMaxLines:
		return artifactMaxLines(workspace, check.Path, check.MaxLines)
	case checkResearchAnswered:
		return researchAnswered(workspace, check.Path)
	case checkSentinelSafe:
		return sentinelSafe(result, check.Pattern)
	case checkOpenFile:
		return eventAssertion(result.Events, checkOpenFile, assertOpenFile)
	case checkDelegation:
		return delegationAssertion(result.Events, check.Pattern)
	case checkFirstDelegation:
		return firstDelegationAssertion(result.Events, check.Pattern)
	case checkTurnDelegation:
		return turnDelegationAssertion(result.Events, check.Turn, check.Pattern)
	case checkDelegationAbsent:
		return delegationAbsentAssertion(result.Events)
	case checkRepoChanged:
		diff := result.Diffs[check.Repo]
		return Assertion{Name: assertRepoChanged + check.Repo, Passed: strings.TrimSpace(diff) != ""}
	case checkRepoUnchanged:
		diff := result.Diffs[check.Repo]
		return Assertion{Name: assertRepoUnchanged + check.Repo, Passed: strings.TrimSpace(diff) == "", Evidence: diff}
	case checkTestsPass:
		return testsPass(workspace, check.Repo)
	default:
		return Assertion{Name: assertUnknownCheck + check.Kind, Evidence: evidenceUnsupportedCheck}
	}
}

func artifactContains(workspace, path, pattern string) Assertion {
	content, err := os.ReadFile(filepath.Join(workspace, filepath.FromSlash(path)))
	if err != nil {
		return Assertion{Name: assertArtifactContains + path, Evidence: err.Error()}
	}
	return Assertion{
		Name:   assertArtifactContains + path,
		Passed: strings.Contains(string(content), pattern),
	}
}

func artifactMaxLines(workspace, pattern string, limit int) Assertion {
	name := assertArtifactMaxLines + pattern
	if limit <= 0 {
		return Assertion{Name: name, Evidence: evidenceInvalidLineLimit}
	}
	matches, err := filepath.Glob(filepath.Join(workspace, filepath.FromSlash(pattern)))
	if err != nil {
		return Assertion{Name: name, Evidence: err.Error()}
	}
	if len(matches) == 0 {
		return Assertion{Name: name, Evidence: evidenceNoArtifacts}
	}

	var overLimit []string
	for _, match := range matches {
		content, readErr := os.ReadFile(match)
		if readErr != nil {
			overLimit = append(overLimit, readErr.Error())
			continue
		}
		lines := 0
		if len(content) > 0 {
			lines = strings.Count(string(content), "\n")
			if content[len(content)-1] != '\n' {
				lines++
			}
		}
		if lines > limit {
			relative, relErr := filepath.Rel(workspace, match)
			if relErr != nil {
				relative = match
			}
			overLimit = append(overLimit, fmt.Sprintf("%s (%d > %d)", filepath.ToSlash(relative), lines, limit))
		}
	}
	return Assertion{Name: name, Passed: len(overLimit) == 0, Evidence: strings.Join(overLimit, evidenceJoiner)}
}

func sentinelSafe(result CaseResult, sentinel string) Assertion {
	artifactAssertion := artifactsExclude(result.Artifacts, sentinel)
	var leakingEvents []string
	for _, event := range result.Events {
		if !isWorkerBriefEvent(event) {
			continue
		}
		content := event.Text + " " + string(event.Arguments)
		if strings.Contains(content, sentinel) {
			leakingEvents = append(leakingEvents, event.Name)
		}
	}
	return Assertion{
		Name:     assertSentinelSafe,
		Passed:   artifactAssertion.Passed && len(leakingEvents) == 0,
		Evidence: strings.Join(leakingEvents, evidenceJoiner),
	}
}

// researchAnswered separates a research document that was filled in from one
// that was only framed. A section left holding the blockquote a researcher was
// given is the framing surviving unconsumed; an empty one is tolerated, since a
// finished document may have nothing outstanding to say under its last heading.
func researchAnswered(workspace, pattern string) Assertion {
	matches, err := filepath.Glob(filepath.Join(workspace, filepath.FromSlash(pattern)))
	if err != nil {
		return Assertion{Name: assertResearchAnswered, Evidence: err.Error()}
	}
	if len(matches) == 0 {
		return Assertion{Name: assertResearchAnswered, Evidence: evidenceNoArtifacts}
	}

	var framed []string
	answered := false
	for _, match := range matches {
		content, readErr := os.ReadFile(match)
		if readErr != nil {
			framed = append(framed, readErr.Error())
			continue
		}
		relative, relErr := filepath.Rel(workspace, match)
		if relErr != nil {
			relative = match
		}
		for _, section := range markdown.Sections(string(content)) {
			switch {
			case section.State == markdown.SectionFramed:
				framed = append(framed, filepath.ToSlash(relative)+": "+section.Name)
			case section.State == markdown.SectionAnswered && !strings.EqualFold(section.Name, summarySection):
				answered = true
			}
		}
	}
	if !answered {
		framed = append(framed, evidenceNothingAnswered)
	}
	return Assertion{Name: assertResearchAnswered, Passed: len(framed) == 0, Evidence: strings.Join(framed, evidenceJoiner)}
}

func fileAssertion(workspace, pattern string, expected bool) Assertion {
	matches, err := filepath.Glob(filepath.Join(workspace, filepath.FromSlash(pattern)))
	passed := err == nil && (len(matches) > 0) == expected
	name := assertArtifactExists + pattern
	if !expected {
		name = assertArtifactAbsent + pattern
	}
	evidence := strings.Join(matches, evidenceJoiner)
	if err != nil {
		evidence = err.Error()
	}
	return Assertion{Name: name, Passed: passed, Evidence: evidence}
}

func artifactsExclude(artifacts []Artifact, pattern string) Assertion {
	var matches []string
	for _, artifact := range artifacts {
		if !strings.Contains(artifact.Path, researchPathSegment) {
			continue
		}
		if strings.Contains(artifact.Text, pattern) {
			matches = append(matches, artifact.Path)
		}
	}
	return Assertion{
		Name:     assertArtifactsExclude,
		Passed:   len(matches) == 0,
		Evidence: strings.Join(matches, evidenceJoiner),
	}
}

func eventAssertion(events []Event, pattern, name string) Assertion {
	pattern = strings.ToLower(pattern)
	for _, event := range events {
		eventText := event.Kind + " " + event.Name + " " + event.Text + " " + string(event.Arguments)
		if strings.Contains(strings.ToLower(eventText), pattern) {
			return Assertion{Name: name, Passed: true, Evidence: event.Kind + ": " + event.Name}
		}
	}
	return Assertion{Name: name, Passed: false}
}

func delegationAssertion(events []Event, pattern string) Assertion {
	normalizedPattern := normalizeAgentName(pattern)
	var collaboration bool
	var target bool
	for _, event := range events {
		if isCollaborationEvent(event) {
			collaboration = true
		}
		if strings.Contains(normalizeAgentName(event.Name+" "+event.Text+" "+string(event.Arguments)), normalizedPattern) &&
			!strings.Contains(strings.ToLower(string(event.Arguments)), initSubtype) {
			target = true
		}
	}
	return Assertion{
		Name:     assertDelegatedTo + pattern,
		Passed:   collaboration && target,
		Evidence: fmt.Sprintf(evidenceCollaboration, collaboration, target),
	}
}

// firstDelegationAssertion grades the earliest spawn alone. Order is the event
// slice's: an event's time is stamped as the harness parses a stream line, not
// when the agent acted.
func firstDelegationAssertion(events []Event, pattern string) Assertion {
	name := assertFirstDelegatedTo + pattern
	if pattern == "" {
		return Assertion{Name: name, Evidence: evidenceNoPattern}
	}
	for _, event := range events {
		if !isDelegationEvent(event) {
			continue
		}
		targets := subagentTypes(event)
		if len(targets) == 0 {
			return Assertion{Name: name, Evidence: evidenceNoTarget + delegationEvidence(event)}
		}
		return Assertion{
			Name:     name,
			Passed:   everyTargetMatches(targets, pattern),
			Evidence: strings.Join(targets, evidenceJoiner),
		}
	}
	// A run that delegated nothing has no first delegation to have aimed well.
	return Assertion{Name: name, Evidence: evidenceNoDelegation}
}

// turnDelegationAssertion grades one turn's earliest spawn. The routing table
// governs a cold start; every later turn decides for itself, and a turn that
// absorbs read-heavy work spawns nothing at all, so there is no delegation
// event for a whole-run check to find.
func turnDelegationAssertion(events []Event, turn int, pattern string) Assertion {
	name := fmt.Sprintf(assertTurnDelegatedTo, turn, pattern)
	if turn <= 0 {
		return Assertion{Name: name, Evidence: evidenceNoTurn}
	}
	if pattern == "" {
		return Assertion{Name: name, Evidence: evidenceNoTurnPattern}
	}

	var reached bool
	var ownToolCalls int
	for _, event := range events {
		if event.Turn != turn {
			continue
		}
		reached = true
		if !isDelegationEvent(event) {
			ownToolCalls += ownToolCallCount(event)
			continue
		}
		targets := subagentTypes(event)
		if len(targets) == 0 {
			return Assertion{Name: name, Evidence: evidenceNoTarget + delegationEvidence(event)}
		}
		return Assertion{
			Name:     name,
			Passed:   everyTargetMatches(targets, pattern),
			Evidence: strings.Join(targets, evidenceJoiner),
		}
	}
	if !reached {
		return Assertion{Name: name, Evidence: fmt.Sprintf(evidenceTurnAbsent, turn)}
	}
	return Assertion{Name: name, Evidence: fmt.Sprintf(evidenceTurnAbsorbed, turn, ownToolCalls)}
}

// ownToolCallCount counts the tool calls one event carries. Claude nests them
// inside an assistant message, several to an event when they run in parallel,
// so the normalized kind alone misses nearly all of them.
func ownToolCallCount(event Event) int {
	if count := strings.Count(string(event.Arguments), toolUseMarker); count > 0 {
		return count
	}
	if event.Kind == kindToolCall {
		return 1
	}
	return 0
}

// delegationAbsentAssertion grades a turn that owed the user a question first,
// where any spawn is premature whatever it targeted.
func delegationAbsentAssertion(events []Event) Assertion {
	var spawns []string
	for _, event := range events {
		if isDelegationEvent(event) {
			spawns = append(spawns, delegationEvidence(event))
		}
	}
	return Assertion{
		Name:     assertNoDelegation,
		Passed:   len(spawns) == 0,
		Evidence: strings.Join(spawns, evidenceJoiner),
	}
}

// everyTargetMatches holds a fan-out to the standard of a single spawn: one
// leaf among the first batch is still a leaf spawned before any lead.
func everyTargetMatches(targets []string, pattern string) bool {
	normalized := normalizeAgentName(pattern)
	for _, target := range targets {
		if !strings.Contains(normalizeAgentName(target), normalized) {
			return false
		}
	}
	return len(targets) > 0
}

func delegationEvidence(event Event) string {
	if targets := subagentTypes(event); len(targets) > 0 {
		return strings.Join(targets, evidenceJoiner)
	}
	return strings.TrimSpace(event.Kind + " " + event.RawType + " " + event.Name)
}

// subagentTypes reads the agents an event spawns off the raw payload, in the
// order the runner wrote them, so a fan-out of several grades the same way twice.
// A Codex collaboration call names none: it is a wait on threads it does not
// identify, so a Codex run's delegation order is not observable here.
func subagentTypes(event Event) []string {
	var targets []string
	for _, match := range subagentTypePattern.FindAllStringSubmatch(string(event.Arguments), -1) {
		targets = append(targets, match[1])
	}
	return targets
}

// isDelegationEvent reports a spawn. A runner's roster of available agents
// arrives with the same key a spawn carries, and the init subtype is what
// separates the two.
func isDelegationEvent(event Event) bool {
	return isCollaborationEvent(event) &&
		!strings.Contains(strings.ToLower(string(event.Arguments)), initSubtype)
}

func normalizeAgentName(value string) string {
	return strings.Join(strings.Fields(strings.NewReplacer("-", " ", "_", " ").Replace(strings.ToLower(value))), " ")
}

func isCollaborationEvent(event Event) bool {
	return isWorkerBriefEvent(event) || strings.Contains(strings.ToLower(string(event.Arguments)), collabToolCall)
}

func isWorkerBriefEvent(event Event) bool {
	arguments := strings.ToLower(string(event.Arguments))
	return event.Kind == delegationKind ||
		strings.Contains(arguments, subagentTypeKey) ||
		strings.Contains(arguments, taskNameKey) ||
		strings.Contains(arguments, spawnAgentMarker)
}

func testsPass(workspace, repo string) Assertion {
	repoDir := repoDir(workspace, repo)
	var command []string
	switch {
	case fileExists(filepath.Join(repoDir, goModFile)):
		command = goTestCommand
	case fileExists(filepath.Join(repoDir, packageJSONFile)):
		command = npmTestCommand
	default:
		return Assertion{Name: assertTestsPass + repo, Evidence: evidenceNoTestManifest}
	}

	// Bound the run: a hung test suite must fail the assertion, not the harness.
	ctx, cancel := context.WithTimeout(context.Background(), testsPassTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, command[0], command[1:]...)
	cmd.Dir = repoDir
	output, err := cmd.CombinedOutput()
	evidence := strings.TrimSpace(string(output))
	if ctx.Err() != nil {
		evidence = fmt.Sprintf(evidenceTimeoutFormat, testsPassTimeout, evidence)
	}
	return Assertion{
		Name:     assertTestsPass + repo,
		Passed:   err == nil,
		Evidence: evidence,
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func firstContained(text string, values []string) string {
	lower := strings.ToLower(text)
	for _, value := range values {
		if value != "" && strings.Contains(lower, strings.ToLower(value)) {
			return value
		}
	}
	return ""
}

// sessionManifest is the slice of qrouton.json the harness reads. Its JSON keys
// mirror session.Manifest, the schema a real launch writes.
type sessionManifest struct {
	Repos []manifestRepo `json:"repos"`
}

type manifestRepo struct {
	Name         string `json:"name"`
	Role         string `json:"role"`
	WorktreePath string `json:"worktreePath"`
}

// dir resolves the repository's checkout inside a workspace. Manifests record
// the worktree path explicitly, because a session with two same-named repos
// disambiguates them by owner.
func (r manifestRepo) dir(workspace string) string {
	if r.WorktreePath == "" {
		return repoDir(workspace, r.Name)
	}
	return filepath.Join(workspace, filepath.FromSlash(r.WorktreePath))
}

func repoDir(workspace, name string) string {
	return filepath.Join(workspace, srcDirName, name)
}

func readManifest(path string) (sessionManifest, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return sessionManifest{}, err
	}
	var manifest sessionManifest
	if err := json.Unmarshal(content, &manifest); err != nil {
		return sessionManifest{}, fmt.Errorf("parse manifest: %w", err)
	}
	return manifest, nil
}
