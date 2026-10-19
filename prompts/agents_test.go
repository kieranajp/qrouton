package prompts

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/kieranajp/qrouton/internal/agentevent"
)

const (
	leadNamePrefix = "qrouton-"
	leadNameSuffix = "-lead"
	toolsKey       = "tools"
	sandboxModeKey = "sandbox_mode"
	auditVerifier  = "audit-verifier"
)

// agentAssetPaths are the files one agent prompt renders to, one per runner
// that resolves a roster off disk. agy's is a directory holding agent.md.
func agentAssetPaths(name string) []string {
	return []string{
		claudeAgentsDir + name + promptFileExt,
		codexAgentsDir + name + tomlExtension,
		agyAgentsDir + name + "/" + agyAgentFileName,
	}
}

// Agents left to Codex's default sandbox. Naming them keeps a new agent from
// inheriting the default by nobody's decision.
var defaultSandboxAgents = map[string]bool{}

// A lead omits tools so it can spawn; a specialist declares tools without Agent or Task so it cannot.
func TestAgentDepthAndSandboxAreDeclared(t *testing.T) {
	loaded, err := NewEmbeddedLoader().List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var agents int
	for _, prompt := range loaded {
		id := string(prompt.ID)
		if !strings.HasPrefix(id, agentIDPrefix) {
			continue
		}
		agents++
		name := path.Base(id)
		t.Run(name, func(t *testing.T) {
			tools, declared := frontmatterEntry(string(prompt.Content), toolsKey)
			lead := strings.HasPrefix(name, leadNamePrefix) && strings.HasSuffix(name, leadNameSuffix)
			switch {
			case lead && declared:
				t.Errorf("a lead declares tools %q, so it can no longer delegate", tools)
			case !lead && !declared:
				t.Error("a specialist declares no tools, so it inherits the spawn tool")
			}
			if tool := spawnTool(tools); tool != "" {
				t.Errorf("a specialist holds %s, so the tree can grow a fourth level", tool)
			}

			assets, err := Render(prompt)
			if err != nil {
				t.Fatal(err)
			}
			paths := make([]string, len(assets))
			for i, asset := range assets {
				paths[i] = asset.Path
			}
			if want := agentAssetPaths(name); !slices.Equal(paths, want) {
				t.Errorf("rendered %v, want %v", paths, want)
			}
			assertAgyAgent(t, assets, name, string(prompt.Content))

			codex := codexAsset(t, assets, name)
			switch want := wantSandbox(t, name); want {
			case "":
				if strings.Contains(codex, sandboxModeKey) {
					t.Errorf("agent in no sandbox set renders one:\n%s", codex)
				}
			default:
				if !strings.Contains(codex, want) {
					t.Errorf("Codex rendering wants %q:\n%s", want, codex)
				}
			}
		})
	}
	if agents == 0 {
		t.Fatal("no agent prompts are embedded")
	}
}

// spawnTool returns the first tool in a tools: list that can start a subagent.
func spawnTool(tools string) string {
	for _, tool := range strings.Split(tools, ",") {
		if tool = strings.TrimSpace(tool); tool == agentevent.ToolAgent || tool == agentevent.ToolTaskLegacy {
			return tool
		}
	}
	return ""
}

// wantSandbox is the sandbox line the maps the renderer consults call for. An
// agent in none of them is an undecided grant, not a default.
func wantSandbox(t *testing.T, name string) string {
	t.Helper()
	switch {
	case readOnlyAgents[name]:
		return fmt.Sprintf(codexSandboxFormat, strconv.Quote(sandboxReadOnly))
	case workspaceWriteAgents[name]:
		return fmt.Sprintf(codexSandboxFormat, strconv.Quote(sandboxWorkspaceWrite))
	case defaultSandboxAgents[name]:
		return ""
	default:
		t.Fatalf("%s belongs to no sandbox set: choose its Codex sandbox", name)
		return ""
	}
}

func assertAgyAgent(t *testing.T, assets []Rendered, name, source string) {
	t.Helper()
	agy := renderedAsset(t, assets, agyAgentsDir+name+"/"+agyAgentFileName)
	for _, key := range []string{"name", "description"} {
		rendered, declared := frontmatterEntry(agy, key)
		if !declared {
			t.Errorf("agy rendering declares no %s", key)
			continue
		}
		unquoted, err := strconv.Unquote(rendered)
		if err != nil {
			t.Errorf("agy %s is not a quoted scalar: %s", key, rendered)
			continue
		}
		if want, _ := frontmatterEntry(source, key); unquoted != want {
			t.Errorf("agy %s = %q, want %q", key, unquoted, want)
		}
	}
	if subagent, _ := frontmatterEntry(agy, agySubagentKey); subagent != "true" {
		t.Errorf("agy rendering is not declared a subagent: %s = %q", agySubagentKey, subagent)
	}

	_, body, split := strings.Cut(agy, frontmatterClose)
	if !split {
		t.Fatalf("agy rendering has no terminated frontmatter:\n%s", agy)
	}
	if strings.Contains(body, "\n# ") || strings.HasPrefix(strings.TrimLeft(body, "\n"), "# ") {
		t.Errorf("an H1 splits the prompt out of the section agy reads as the system prompt:\n%s", body)
	}
	if _, want, _ := strings.Cut(source, frontmatterClose); strings.TrimSpace(body) != strings.TrimSpace(want) {
		t.Errorf("agy body is not the source prompt:\n%s", body)
	}
}

func renderedAsset(t *testing.T, assets []Rendered, want string) string {
	t.Helper()
	for _, asset := range assets {
		if asset.Path == want {
			return string(asset.Content)
		}
	}
	t.Fatalf("no %s among %#v", want, assets)
	return ""
}

func codexAsset(t *testing.T, assets []Rendered, name string) string {
	t.Helper()
	return renderedAsset(t, assets, codexAgentsDir+name+tomlExtension)
}

func frontmatterEntry(text, key string) (string, bool) {
	if !strings.HasPrefix(text, frontmatterFence) {
		return "", false
	}
	end := strings.Index(text[len(frontmatterFence):], frontmatterClose)
	if end < 0 {
		return "", false
	}
	for _, line := range strings.Split(text[len(frontmatterFence):len(frontmatterFence)+end], "\n") {
		if name, value, ok := strings.Cut(line, ":"); ok && strings.TrimSpace(name) == key {
			return strings.TrimSpace(value), true
		}
	}
	return "", false
}

func TestEveryAgentPromptParsesToItsFileNameAndAPlainDescription(t *testing.T) {
	loaded, err := NewEmbeddedLoader().List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, prompt := range loaded {
		id := string(prompt.ID)
		if !strings.HasPrefix(id, agentIDPrefix) {
			continue
		}
		t.Run(path.Base(id), func(t *testing.T) {
			agent, err := parseAgentPrompt(prompt.Content)
			if err != nil {
				t.Fatal(err)
			}
			if agent.name != path.Base(id) {
				t.Errorf("name = %q, want %q", agent.name, path.Base(id))
			}
			if agent.description == "" || strings.ContainsAny(agent.description[:1], `"'>`) {
				t.Errorf("description = %q, want plain text", agent.description)
			}
		})
	}
}

func TestQuotedAndFoldedDescriptionsDecodeToPlainText(t *testing.T) {
	for name, source := range map[string]string{
		"quoted": "---\nname: probe\ndescription: \"Probes: things.\"\n---\n\nBody.\n",
		"folded": "---\nname: probe\ndescription: >\n  Probes:\n  things.\n---\n\nBody.\n",
	} {
		t.Run(name, func(t *testing.T) {
			agent, err := parseAgentPrompt([]byte(source))
			if err != nil {
				t.Fatal(err)
			}
			if agent.description != "Probes: things." {
				t.Fatalf("description = %q, want %q", agent.description, "Probes: things.")
			}
		})
	}
}

// Render is the only guard on a malformed agent source, and a source that
// cannot be parsed must stop a launch rather than stamp a half-built roster.
func TestRenderRejectsMalformedAgentPrompts(t *testing.T) {
	for name, probe := range map[string]struct {
		source string
		want   error
	}{
		"no frontmatter":    {"Body with no frontmatter.\n", ErrNoFrontmatter},
		"unterminated":      {"---\nname: probe\ndescription: Probes.\n", ErrUnterminatedFrontmatter},
		"no name":           {"---\ndescription: Probes.\n---\n\nBody.\n", ErrIncompleteAgentPrompt},
		"no description":    {"---\nname: probe\n---\n\nBody.\n", ErrIncompleteAgentPrompt},
		"empty description": {"---\nname: probe\ndescription:\n---\n\nBody.\n", ErrIncompleteAgentPrompt},
		"invalid yaml":      {"---\nname: probe\ndescription: [Probes.\n---\n\nBody.\n", ErrInvalidAgentFrontmatter},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := Render(Prompt{ID: ID(agentIDPrefix + "probe"), Content: []byte(probe.source)})
			if !errors.Is(err, probe.want) {
				t.Fatalf("Render = %v, want %v", err, probe.want)
			}
		})
	}
}

// Only the orchestrator searches the thoughts root during Research. Claude enforces the
// lead's refusal from its frontmatter; Codex and agy read only the prose.
func TestResearchPromptsCarryVaultIsolation(t *testing.T) {
	loader := NewEmbeddedLoader()
	lead, err := loader.Load(context.Background(), ID(agentIDPrefix+"qrouton-research-lead"))
	if err != nil {
		t.Fatal(err)
	}
	if denied, _ := frontmatterEntry(string(lead.Content), "disallowedTools"); denied != "mcp__qrouton__search_thoughts" {
		t.Errorf("research lead disallowedTools = %q", denied)
	}
	assets, err := Render(lead)
	if err != nil {
		t.Fatal(err)
	}
	claude := renderedAsset(t, assets, claudeAgentsDir+"qrouton-research-lead"+promptFileExt)
	if denied, _ := frontmatterEntry(claude, "disallowedTools"); denied != "mcp__qrouton__search_thoughts" {
		t.Errorf("the Claude rendering drops disallowedTools:\n%s", claude)
	}
	for _, asset := range assets {
		body := string(asset.Content)
		for _, phrase := range []string{"Do not call `search_thoughts`", "`read_thoughts`"} {
			if !strings.Contains(body, phrase) {
				t.Errorf("%s is missing %q", asset.Path, phrase)
			}
		}
	}

	skill, err := loader.Load(context.Background(), ID(skillIDPrefix+"qrouton-research"))
	if err != nil {
		t.Fatal(err)
	}
	for _, phrase := range []string{
		"`kinds: [\"research\"]`",
		"Only research artifacts may reach the lead: specs, plans and notes carry intended solutions",
		"Pass a reference, not content: the artifact ID, heading breadcrumb, line range, and the approved question it bears on",
		"Add nothing else: no summary, no staleness note, no comment on its claims",
		"read them with `read_thoughts` (the lead has it; do not hedge on its availability)",
		"let the lead investigate fresh",
	} {
		if !strings.Contains(string(skill.Content), phrase) {
			t.Errorf("research skill is missing %q", phrase)
		}
	}
}

// The lead names are what the workbench reads a stage's lead off, so each must
// still name an agent prompt.
func TestLeadNamesNameAgentPrompts(t *testing.T) {
	for _, lead := range []string{ResearchLead, AuditLead, PlanningLead, ImplementationLead} {
		if _, err := fs.Stat(embedded, agentIDPrefix+lead+promptFileExt); err != nil {
			t.Errorf("lead %q names no agent prompt: %v", lead, err)
		}
	}
}

// The re-read must come from an agent that cannot edit the draft it checks, so
// it gets history from the lead's reads and never from a shell of its own.
func TestAuditLeadSpawnsReadOnlyVerifier(t *testing.T) {
	loader := NewEmbeddedLoader()
	lead, err := loader.Load(context.Background(), ID(agentIDPrefix+AuditLead))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(lead.Content), "`"+auditVerifier+"`") {
		t.Errorf("audit lead does not spawn %s", auditVerifier)
	}
	checkDraft := markdownSection(string(lead.Content), "## Check the draft")
	for _, phrase := range []string{"history directory", "`git log -L "} {
		if !strings.Contains(checkDraft, phrase) {
			t.Errorf("audit lead's Check the draft does not carry %q", phrase)
		}
	}
	verifier, err := loader.Load(context.Background(), ID(agentIDPrefix+auditVerifier))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(verifier.Content), "supplied history") {
		t.Errorf("%s does not read the supplied history", auditVerifier)
	}
	if strings.Contains(string(verifier.Content), "`git") {
		t.Errorf("%s names a git command it cannot run", auditVerifier)
	}
	if tools, _ := frontmatterEntry(string(verifier.Content), toolsKey); tools != "Read, Grep, Glob, LS" {
		t.Errorf("%s tools = %q, want read-only", auditVerifier, tools)
	}
	if !readOnlyAgents[auditVerifier] {
		t.Errorf("%s is not in readOnlyAgents", auditVerifier)
	}
	assets, err := Render(verifier)
	if err != nil {
		t.Fatal(err)
	}
	want := fmt.Sprintf(codexSandboxFormat, strconv.Quote(sandboxReadOnly))
	if codex := codexAsset(t, assets, auditVerifier); !strings.Contains(codex, want) {
		t.Errorf("Codex rendering wants %q:\n%s", want, codex)
	}
}

// An audit's verdicts come from the code alone. Claude enforces the lead's
// refusal from its frontmatter; Codex and agy read only the prose.
func TestAuditPromptsJudgeLiveCode(t *testing.T) {
	const denied = "mcp__qrouton__search_thoughts, mcp__qrouton__read_thoughts"
	const liveCodeOnly = "Judge the live code only."
	loader := NewEmbeddedLoader()
	lead, err := loader.Load(context.Background(), ID(agentIDPrefix+AuditLead))
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := frontmatterEntry(string(lead.Content), "disallowedTools"); got != denied {
		t.Errorf("audit lead disallowedTools = %q", got)
	}
	assets, err := Render(lead)
	if err != nil {
		t.Fatal(err)
	}
	claude := renderedAsset(t, assets, claudeAgentsDir+AuditLead+promptFileExt)
	if got, _ := frontmatterEntry(claude, "disallowedTools"); got != denied {
		t.Errorf("the Claude rendering drops disallowedTools:\n%s", claude)
	}

	for _, id := range []ID{ID(agentIDPrefix + AuditLead), ID(agentIDPrefix + "audit-checker"), ID(agentIDPrefix + auditVerifier), ID(skillIDPrefix + "qrouton-audit")} {
		prompt, err := loader.Load(context.Background(), id)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(prompt.Content), liveCodeOnly) {
			t.Errorf("%s does not carry %q", id, liveCodeOnly)
		}
		if !strings.HasPrefix(string(id), agentIDPrefix) {
			continue
		}
		rendered, err := Render(prompt)
		if err != nil {
			t.Fatal(err)
		}
		for _, asset := range rendered {
			for _, phrase := range []string{liveCodeOnly, "`search_thoughts`", "`read_thoughts`"} {
				if !strings.Contains(string(asset.Content), phrase) {
					t.Errorf("%s does not carry %q", asset.Path, phrase)
				}
			}
		}
	}
}

// The worked examples are examples. A tool chosen for any other ecosystem is
// an equal choice, so no line of the audit prompts may rank a tool.
func TestAuditSkillRanksNoTool(t *testing.T) {
	ranking := regexp.MustCompile(`(?i)\b(approved|gaps?|fallbacks?)\b`)
	tool := regexp.MustCompile(`(?i)\b(tools?|detectors?|knip|jscpd|eslint|deadcode|dupl|staticcheck|vet)\b`)
	loader := NewEmbeddedLoader()
	var files []PromptFile
	for _, id := range []ID{ID(skillIDPrefix + "qrouton-audit"), ID(agentIDPrefix + AuditLead), ID(agentIDPrefix + "audit-checker"), ID(agentIDPrefix + auditVerifier)} {
		prompt, err := loader.Load(context.Background(), id)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, PromptFile{Path: string(id), Content: prompt.Content})
		files = append(files, prompt.Files...)
	}
	for _, file := range files {
		for i, line := range strings.Split(string(file.Content), "\n") {
			if word := ranking.FindString(line); word != "" && tool.MatchString(line) {
				t.Errorf("%s:%d says %q: %s", file.Path, i+1, word, line)
			}
		}
	}
}

// Each anchor is a rule of the audit method. The test checks that the rule is
// stated, not that it is stated well.
func TestAuditPromptsCarryMethod(t *testing.T) {
	const (
		skill    = "skill"
		lead     = "lead"
		checker  = "checker"
		verifier = "verifier"
		shape    = "references/audit-shape.md"
		template = "references/audit-template.md"
		goNotes  = "references/go.md"
		tsNotes  = "references/typescript.md"
		areas    = "references/areas.md"
	)
	loader := NewEmbeddedLoader()
	texts := map[string]string{}
	for name, id := range map[string]ID{
		skill:    ID(skillIDPrefix + "qrouton-audit"),
		lead:     ID(agentIDPrefix + AuditLead),
		checker:  ID(agentIDPrefix + "audit-checker"),
		verifier: ID(agentIDPrefix + auditVerifier),
	} {
		prompt, err := loader.Load(context.Background(), id)
		if err != nil {
			t.Fatal(err)
		}
		texts[name] = string(prompt.Content)
		for _, file := range prompt.Files {
			texts[file.Path] = string(file.Content)
		}
	}
	leadPrompt, err := loader.Load(context.Background(), ID(agentIDPrefix+AuditLead))
	if err != nil {
		t.Fatal(err)
	}
	rendered, err := Render(leadPrompt)
	if err != nil {
		t.Fatal(err)
	}

	for _, rule := range []struct {
		name    string
		phrases []string
		in      []string
	}{
		{"tell 3 is wider", []string{"installed or not"}, []string{lead, checker, template}},
		{"tell 3 cost", []string{"install cost"}, []string{lead, checker}},
		{"written reasons", []string{"written at the site"}, []string{lead, checker, shape}},
		{"runtime claims", []string{"runtime claim"}, []string{lead, checker, shape}},
		{"no bulk dismissal", []string{"one line per candidate"}, []string{lead, shape}},
		{"no grab-bags", []string{"one finding per defect"}, []string{lead, shape}},
		{"unreachable step", []string{"live caller in shipped code"}, []string{checker, shape}},
		{"tell-1 and tell-2 method", []string{"bypasses a shared", "two sources", "called only from tests"}, []string{lead}},
		{"low confidence", []string{"Confidence: low", "No other section holds candidates"}, []string{shape}},
		{"job inventory", []string{"job inventory", "## Tools"}, []string{lead, shape}},
		{"whole-family cost", []string{"whole family", "size and installed status"}, []string{lead, checker, shape}},
		{"category sweep", []string{"category the repo hand-rolls"}, []string{lead, checker}},
		{"no contradicting spare", []string{"contradicts another finding"}, []string{lead, shape}},
		{"conformance pass", []string{"conformance pass"}, []string{lead, shape}},
		{"inferred is not checked", []string{"is not a runtime check"}, []string{lead, checker, shape}},
		{"impact field", []string{"Impact:", "by impact"}, []string{shape}},
		{"impact order", []string{"by impact"}, []string{lead, template}},
		{"cosmetic nits", []string{"one shared root cause"}, []string{lead, shape}},
		{"full inventory", []string{"full inventory", "stated skip", "routes searched"}, []string{lead, shape}},
		{"judgement skip", []string{"name what was compared"}, []string{lead, shape}},
		{"shared machinery", []string{"kinds of shared machinery"}, []string{lead}},
		{"independent re-read", []string{"independent re-read"}, []string{lead, shape, verifier}},
		{"what the re-read verifies", []string{"agree or differ", "one route", "proposes to grow", "serve another purpose"}, []string{shape, verifier}},
		{"category outcomes", []string{"outcome per category", "none hand-rolled"}, []string{lead, checker, shape}},
		{"platform comments", []string{"platform or runtime claims"}, []string{lead, checker}},
		{"root-cause rule", []string{"not a shared root cause"}, []string{lead, shape}},
		{"test-preserving deletions", []string{"serve another purpose"}, []string{checker}},
		{"impact", []string{"data loss"}, []string{shape}},
		{"summary", []string{"one sentence per high and medium finding"}, []string{lead, shape, template}},
		{"scope and tells question", []string{"one structured question", "`AskUserQuestion`", "ask in chat"}, []string{skill}},
		{"chosen tells key", []string{"`tells:`"}, []string{lead, shape}},
		{"template key", []string{"tells:"}, []string{template}},
		{"gated inventory", []string{"only when tell 1 or tell 3 is chosen"}, []string{lead, shape}},
		{"chosen only", []string{"only the chosen tells"}, []string{lead, shape}},
		{"tell 4 heading", []string{"Tests that pass or fail for the wrong reason"}, []string{template}},
		{"test check", []string{"Would the test fail if the code it names were broken?"}, []string{checker, shape}},
		{"weakened tests", []string{"needs history"}, []string{lead, checker, shape}},
		{"shipped code under test", []string{"behaves differently under test"}, []string{lead, checker, verifier}},
		{"never runs tests", []string{"never runs the target's tests"}, []string{lead, shape}},
		{"no snapshot rewrites", []string{"snapshots or golden files"}, []string{lead, tsNotes, goNotes}},
		{"verifier test checks", []string{"cannot fail", "never runs"}, []string{verifier}},
		{"go tell-4 notes", []string{"`t.Skip`", "`testing.Testing()`"}, []string{goNotes}},
		{"typescript tell-4 notes", []string{"`testMatch`", "`.only`"}, []string{tsNotes}},
		{"no-tie check", []string{"search for a tie"}, []string{verifier}},
		{"history check", []string{"supplied history", "unchecked"}, []string{verifier}},
		{"history handoff", []string{"history directory"}, []string{lead, shape}},
		{"inventory claims", []string{"none hand-rolled", "stated skip"}, []string{verifier}},
		{"behaviour risk", []string{"persisted key", "user-visible string"}, []string{verifier}},
		{"migrations", []string{"states a migration"}, []string{verifier}},
		{"file references", []string{"allowlists"}, []string{verifier}},
		{"impact check", []string{"wrong behaviour today"}, []string{verifier}},
		{"complete family", []string{"family is complete"}, []string{verifier}},
		{"repo checks first", []string{"the repo's own checks first"}, []string{lead, tsNotes, goNotes}},
		{"file-type skip", []string{"names the file type"}, []string{lead, shape}},
		{"no dropped category", []string{"never dropped without a stated skip"}, []string{lead}},
		{"data tables", []string{"another module mirrors"}, []string{lead, shape}},
		{"own fixture", []string{"a literal copy of itself"}, []string{lead, checker}},
		{"dead-code skip", []string{"skipped only because the code it names is dead"}, []string{lead}},
		{"defect is not a spare", []string{"a behaviour defect"}, []string{checker, shape, verifier}},
		{"stated intent", []string{"its own comment"}, []string{checker, verifier}},
		{"tell-4 deliberate re-check", []string{"tell-4 line under `## Checked and deliberate`"}, []string{verifier}},
		{"category outcomes verified", []string{"every category outcome"}, []string{verifier}},
		{"impact reconciliation", []string{"cosmetic difference"}, []string{shape, verifier}},
		{"impact reconciliation today", []string{"differ or disagree today"}, []string{verifier}},
		{"derived-list gate", []string{"keeps every gate"}, []string{checker, shape, verifier}},
		{"deliberate subset", []string{"stays a subset"}, []string{shape, verifier}},
		{"coarse rows", []string{"call sites searched"}, []string{lead, shape}},
		{"row sampling", []string{"five of the entry's call sites"}, []string{verifier}},
		{"conformance home", []string{"`## Before returning`"}, []string{lead}},
		{"area split", []string{"`areas.md`", "seam pass"}, []string{lead}},
		{"split threshold", []string{"300 shipped source files", "at most four areas"}, []string{areas}},
		{"seam pass", []string{"seam pass", "cross areas"}, []string{areas}},
		{"merge", []string{"areas table", "`Area` column"}, []string{areas, shape}},
		{"ids after merge", []string{"after the merge"}, []string{lead, areas}},
		{"verifier once", []string{"once over the merged draft"}, []string{lead, areas}},
		{"fan-out cap", []string{"20 checker spawns"}, []string{areas}},
	} {
		for _, where := range rule.in {
			for _, phrase := range rule.phrases {
				if !strings.Contains(texts[where], phrase) {
					t.Errorf("%s: %s does not carry %q", rule.name, where, phrase)
				}
			}
		}
	}

	for _, phrase := range []string{"agree or differ", "search for a tie", "persisted key", "300 shipped source files", "its own comment", "stays a subset", "five of the entry's call sites"} {
		if strings.Contains(texts[lead], phrase) {
			t.Errorf("the lead restates the verifier's check %q; it briefs check numbers instead", phrase)
		}
	}

	for _, phrase := range []string{"job inventory", "independent re-read", "conformance pass", "only when tell 1 or tell 3", "supplied history", "the repo's own checks first", "seam pass", "`Area` column"} {
		if strings.Contains(texts[skill], phrase) {
			t.Errorf("the skill restates the lead's rule %q; its fallback reads the lead's definition instead", phrase)
		}
	}

	for _, asset := range rendered {
		for _, phrase := range []string{"bypasses a shared", "two sources", "called only from tests", "job inventory", "## Tools", "full inventory", "stated skip", "routes searched", "seam pass"} {
			if !strings.Contains(string(asset.Content), phrase) {
				t.Errorf("%s does not carry %q", asset.Path, phrase)
			}
		}
	}

	for where, text := range texts {
		for _, gone := range []string{"installed library", "four-step", "the one most files import", "Add any concern", "Every library category has an outcome"} {
			if strings.Contains(text, gone) {
				t.Errorf("%s still says %q", where, gone)
			}
		}
	}
}

// The verifier's prompt is the one home for its checks, and the lead briefs
// them by number, so neither prompt may grow back the other's text.
func TestAuditPromptsStayWithinBudget(t *testing.T) {
	for name, limit := range map[string]int{AuditLead: 3450, auditVerifier: 1000} {
		raw, err := fs.ReadFile(embedded, agentIDPrefix+name+promptFileExt)
		if err != nil {
			t.Fatal(err)
		}
		if words := len(strings.Fields(string(raw))); words > limit {
			t.Errorf("%s has %d words, over its budget of %d", name, words, limit)
		}
	}
}

func markdownSection(text, heading string) string {
	start := strings.Index(text, heading)
	if start < 0 {
		return ""
	}
	rest := text[start+len(heading):]
	if end := strings.Index(rest, "\n## "); end >= 0 {
		return rest[:end]
	}
	return rest
}
