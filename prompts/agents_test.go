package prompts

import (
	"context"
	"fmt"
	"path"
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

// agentTargets are the runners that read a roster off disk: claude, codex and
// agy each get one file per agent prompt.
var agentTargets = []string{claudeAgentsDir, codexAgentsDir, agyAgentsDir}

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
			if len(assets) != len(agentTargets) {
				t.Errorf("agent renders %d files, want one per runner that reads a roster", len(assets))
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

// assertAgyAgent checks the third rendering carries the identity agy resolves an
// agent by, and the source body under the heading that introduces it.
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
	heading := fmt.Sprintf(agyPromptHeadingFormat, name)
	if !strings.HasPrefix(body, heading) {
		t.Errorf("agy body does not open on its own H1:\n%s", body)
	}
	if _, want, _ := strings.Cut(source, frontmatterClose); !strings.Contains(body, strings.TrimSpace(want)) {
		t.Error("agy body drops the source prompt")
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
