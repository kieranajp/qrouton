package prompts

import (
	"context"
	"errors"
	"fmt"
	"path"
	"slices"
	"strconv"
	"strings"
	"testing"
)

const (
	taskTool       = "Task"
	leadNamePrefix = "qrouton-"
	leadNameSuffix = "-lead"
	toolsKey       = "tools"
	sandboxModeKey = "sandbox_mode"
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

// Depth stays bounded at three levels: a lead omits tools so it inherits Task
// and can delegate, and every specialist declares a tools set without Task so
// it cannot spawn a fourth.
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
				t.Error("a specialist declares no tools, so it inherits Task and can spawn")
			}
			for _, tool := range strings.Split(tools, ",") {
				if strings.TrimSpace(tool) == taskTool {
					t.Errorf("a specialist holds %s, so the tree can grow a fourth level", taskTool)
				}
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
	for _, key := range []string{frontmatterNameKey, frontmatterDescriptionKey} {
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
		if name, value, ok := strings.Cut(line, frontmatterKeySep); ok && strings.TrimSpace(name) == key {
			return strings.TrimSpace(value), true
		}
	}
	return "", false
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
	} {
		t.Run(name, func(t *testing.T) {
			_, err := Render(Prompt{ID: ID(agentIDPrefix + "probe"), Content: []byte(probe.source)})
			if !errors.Is(err, probe.want) {
				t.Fatalf("Render = %v, want %v", err, probe.want)
			}
		})
	}
}

// Only the orchestrator searches the vault during Research. Claude enforces the
// lead's refusal from its frontmatter; Codex and agy read only the prose.
func TestResearchPromptsCarryVaultIsolation(t *testing.T) {
	loader := NewEmbeddedLoader()
	lead, err := loader.Load(context.Background(), ID(agentIDPrefix+"qrouton-research-lead"))
	if err != nil {
		t.Fatal(err)
	}
	if denied, _ := frontmatterEntry(string(lead.Content), "disallowedTools"); denied != "mcp__qrouton__search_vault" {
		t.Errorf("research lead disallowedTools = %q", denied)
	}
	assets, err := Render(lead)
	if err != nil {
		t.Fatal(err)
	}
	claude := renderedAsset(t, assets, claudeAgentsDir+"qrouton-research-lead"+promptFileExt)
	if denied, _ := frontmatterEntry(claude, "disallowedTools"); denied != "mcp__qrouton__search_vault" {
		t.Errorf("the Claude rendering drops disallowedTools:\n%s", claude)
	}
	for _, asset := range assets {
		body := string(asset.Content)
		for _, phrase := range []string{"Do not call `search_vault`", "`read_vault`"} {
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
		"read them with `read_vault` (the lead has it; do not hedge on its availability)",
		"let the lead investigate fresh",
	} {
		if !strings.Contains(string(skill.Content), phrase) {
			t.Errorf("research skill is missing %q", phrase)
		}
	}
}
