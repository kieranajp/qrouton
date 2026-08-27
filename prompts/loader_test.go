package prompts

import (
	"context"
	"strings"
	"testing"
)

func TestEmbeddedLoaderAndAgentRendering(t *testing.T) {
	loader := NewEmbeddedLoader()
	prompt, err := loader.Load(context.Background(), ID(agentIDPrefix+"qrspi-research-lead"))
	if err != nil {
		t.Fatal(err)
	}
	assets, err := Render(prompt)
	if err != nil {
		t.Fatal(err)
	}
	if len(assets) != 2 || assets[0].Path != ".claude/agents/qrspi-research-lead.md" || assets[1].Path != ".codex/agents/qrspi-research-lead.toml" {
		t.Fatalf("rendered assets = %#v", assets)
	}
	if !strings.Contains(string(assets[1].Content), `sandbox_mode = "workspace-write"`) || !strings.Contains(string(assets[1].Content), "Lead a bounded") {
		t.Fatalf("Codex rendering missing metadata or instructions:\n%s", assets[1].Content)
	}
	all, err := loader.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 17 {
		t.Fatalf("embedded prompt count = %d, want 17", len(all))
	}
}

func TestSubagentChoiceExpandedForDelegatingPrompts(t *testing.T) {
	loader := NewEmbeddedLoader()
	ids := []ID{
		Orchestrator,
		Assistant,
		ID(agentIDPrefix + "qrspi-implementation-lead"),
		ID(agentIDPrefix + "qrspi-planning-lead"),
		ID(agentIDPrefix + "qrspi-research-lead"),
	}
	for _, id := range ids {
		prompt, err := loader.Load(context.Background(), id)
		if err != nil {
			t.Fatal(err)
		}
		content := string(prompt.Content)
		if !strings.Contains(content, "Default to the efficient tier (to keep fan-out costs from ballooning)") {
			t.Errorf("prompt %q missing subagent choice guidance", id)
		}
		if strings.Contains(content, subagentChoicePlaceholder) {
			t.Errorf("prompt %q retained subagent choice placeholder", id)
		}
	}
}
