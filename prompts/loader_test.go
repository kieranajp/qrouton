package prompts

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

func TestEmbeddedLoaderAndAgentRendering(t *testing.T) {
	loader := NewEmbeddedLoader()
	prompt, err := loader.Load(context.Background(), ID(agentIDPrefix+"qrouton-research-lead"))
	if err != nil {
		t.Fatal(err)
	}
	assets, err := Render(prompt)
	if err != nil {
		t.Fatal(err)
	}
	if len(assets) != 2 || assets[0].Path != ".claude/agents/qrouton-research-lead.md" || assets[1].Path != ".codex/agents/qrouton-research-lead.toml" {
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
		ID(agentIDPrefix + "qrouton-implementation-lead"),
		ID(agentIDPrefix + "qrouton-planning-lead"),
		ID(agentIDPrefix + "qrouton-research-lead"),
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

// A skill is a folder: a short entry file and the references it defers detail
// to. Nothing forces a skill that needs only one file to grow a second.
func TestASkillFolderShipsItsReferencesAndASoloSkillStaysSolo(t *testing.T) {
	loader := NewFSLoader(fstest.MapFS{
		"skills/solo/SKILL.md":                 {Data: []byte("---\nname: solo\n---\n\nAll of it, here.\n")},
		"skills/folder/SKILL.md":               {Data: []byte("---\nname: folder\n---\n\nSee references/detail.md.\n")},
		"skills/folder/references/detail.md":   {Data: []byte("# Detail\n")},
		"skills/folder/references/_partial.md": {Data: []byte("# Partial\n")},
		"skills/folder/scripts/run.py":         {Data: []byte("print(\"hi\")\n")},
		"skills/folder/.DS_Store":              {Data: []byte("junk")},
		"skills/folder/.git/config":            {Data: []byte("junk")},
	})
	ctx := context.Background()

	solo, err := loader.Load(ctx, ID("skills/solo"))
	if err != nil {
		t.Fatal(err)
	}
	if len(solo.Files) != 0 {
		t.Fatalf("a single-file skill carries %#v", solo.Files)
	}
	assets, err := Render(solo)
	if err != nil {
		t.Fatal(err)
	}
	if len(assets) != 1 || assets[0].Path != "skills/solo/SKILL.md" {
		t.Fatalf("rendered assets = %#v", assets)
	}

	folder, err := loader.Load(ctx, ID("skills/folder"))
	if err != nil {
		t.Fatal(err)
	}
	assets, err = Render(folder)
	if err != nil {
		t.Fatal(err)
	}
	var paths []string
	for _, asset := range assets {
		paths = append(paths, asset.Path)
	}
	want := []string{
		"skills/folder/SKILL.md",
		"skills/folder/references/_partial.md",
		"skills/folder/references/detail.md",
		"skills/folder/scripts/run.py",
	}
	if strings.Join(paths, ",") != strings.Join(want, ",") {
		t.Fatalf("rendered paths = %v, want %v", paths, want)
	}

	// The whole folder reaches the runner discovery tree, and only the formats
	// whose comment syntax we know carry the generated-by marker. Nothing hidden
	// goes with it.
	dir := t.TempDir()
	if err := Stamp(ctx, dir, loader, OrchestratorAsset); err != nil {
		t.Fatal(err)
	}
	for _, root := range []string{claudeSkillsDir, agentsSkillsDir} {
		reference := filepath.Join(dir, root, skillsDirName, "folder", "references", "detail.md")
		info, err := os.Lstat(reference)
		if err != nil || info.Mode()&os.ModeSymlink == 0 {
			t.Fatalf("%s is not a symlink: %v", reference, err)
		}
		content, err := os.ReadFile(reference)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(string(content), MarkerText) {
			t.Fatalf("%s missing the generated-by marker:\n%s", reference, content)
		}
		script, err := os.ReadFile(filepath.Join(dir, root, skillsDirName, "folder", "scripts", "run.py"))
		if err != nil {
			t.Fatal(err)
		}
		if string(script) != "print(\"hi\")\n" {
			t.Fatalf("a script was marked as if it were markdown:\n%s", script)
		}
		if _, err := os.Stat(filepath.Join(dir, root, skillsDirName, "solo", "references")); !os.IsNotExist(err) {
			t.Fatalf("a single-file skill grew a references directory: %v", err)
		}
		if _, err := os.Stat(filepath.Join(dir, root, skillsDirName, "folder", ".DS_Store")); !os.IsNotExist(err) {
			t.Fatalf("a hidden file was stamped as part of the skill: %v", err)
		}
	}
}

// The plan template lives in the plan skill's own reference file, so SKILL.md
// stays short enough to skim.
func TestPlanSkillDefersItsTemplateToAReference(t *testing.T) {
	prompt, err := NewEmbeddedLoader().Load(context.Background(), ID(skillIDPrefix+"qrspi-plan"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(prompt.Content), "### Verify") {
		t.Error("SKILL.md still holds the plan template")
	}
	if !strings.Contains(string(prompt.Content), "references/plan-shape.md") {
		t.Error("SKILL.md does not point at its reference")
	}
	var found bool
	for _, file := range prompt.Files {
		if file.Path == "references/plan-shape.md" && strings.Contains(string(file.Content), "### Verify") {
			found = true
		}
	}
	if !found {
		t.Errorf("the plan skill ships %#v, none of them the template", prompt.Files)
	}
}

// With whole folders embedded, a SKILL.md deeper inside a skill is one of that
// skill's files rather than a skill the loader lists twice.
func TestOnlyAFolderDirectlyUnderSkillsIsASkill(t *testing.T) {
	loader := NewFSLoader(fstest.MapFS{
		"skills/demo/SKILL.md":            {Data: []byte("---\nname: demo\n---\n")},
		"skills/demo/references/SKILL.md": {Data: []byte("# Not a skill\n")},
	})
	listed, err := loader.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 1 || listed[0].ID != ID("skills/demo") {
		t.Fatalf("listed %#v", listed)
	}
	if len(listed[0].Files) != 1 || listed[0].Files[0].Path != "references/SKILL.md" {
		t.Fatalf("the nested file is not the skill's own: %#v", listed[0].Files)
	}
}
