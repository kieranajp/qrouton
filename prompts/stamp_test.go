package prompts

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

func agentLoader(body string) PromptLoader {
	return NewFSLoader(fstest.MapFS{
		"agents/code-reviewer.md": {Data: []byte("---\nname: code-reviewer\ndescription: Reviews changes.\ntools: Read, Grep\n---\n\n" + body + "\n")},
	})
}

// Every launch re-stamps, so an agent file the user authored under a name
// qrouton also ships would otherwise be destroyed on every launch.
func TestStampRefusesToReplaceAUserOwnedAgentFile(t *testing.T) {
	dir := t.TempDir()
	agent := filepath.Join(dir, claudeAgentsDir, "code-reviewer.md")
	if err := os.MkdirAll(filepath.Dir(agent), dirMode); err != nil {
		t.Fatal(err)
	}
	mine := "---\nname: code-reviewer\n---\n\nMy own reviewer.\n"
	if err := os.WriteFile(agent, []byte(mine), fileMode); err != nil {
		t.Fatal(err)
	}

	err := Stamp(context.Background(), dir, agentLoader("Ours."), OrchestratorAsset)
	if !errors.Is(err, ErrUserOwnedAsset) {
		t.Fatalf("stamping over a user's agent file returned %v", err)
	}
	content, err := os.ReadFile(agent)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != mine {
		t.Fatalf("the user's agent file was rewritten:\n%s", content)
	}
}

func TestStampReplacesItsOwnAgentFiles(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	if err := Stamp(ctx, dir, agentLoader("First."), OrchestratorAsset); err != nil {
		t.Fatal(err)
	}
	if err := Stamp(ctx, dir, agentLoader("Second."), OrchestratorAsset); err != nil {
		t.Fatalf("re-stamping our own agent file: %v", err)
	}
	for _, agent := range []string{
		filepath.Join(dir, claudeAgentsDir, "code-reviewer.md"),
		filepath.Join(dir, codexAgentsDir, "code-reviewer.toml"),
	} {
		content, err := os.ReadFile(agent)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(content), "Second.") || strings.Contains(string(content), "First.") {
			t.Fatalf("%s was not replaced:\n%s", agent, content)
		}
	}
}
