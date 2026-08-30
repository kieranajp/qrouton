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

// Agents left to Codex's default sandbox. Naming them keeps a new agent from
// inheriting the default by nobody's decision.
var defaultSandboxAgents = map[string]bool{
	"qrouton-implementation-lead": true,
	"qrouton-planning-lead":       true,
}

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

func codexAsset(t *testing.T, assets []Rendered, name string) string {
	t.Helper()
	want := codexAgentsDir + name + tomlExtension
	for _, asset := range assets {
		if asset.Path == want {
			return string(asset.Content)
		}
	}
	t.Fatalf("no %s among %#v", want, assets)
	return ""
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
