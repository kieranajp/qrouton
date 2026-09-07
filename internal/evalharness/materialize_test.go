package evalharness

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kieranajp/qrouton/prompts"
)

func TestMaterializeAssetsCreatesRunnerDiscoveryLayout(t *testing.T) {
	assets := filepath.Join(t.TempDir(), "prompts")
	writeTestFile(t, filepath.Join(assets, "orchestrator.md"), "# Orchestrator\n")
	writeTestFile(
		t,
		filepath.Join(assets, "skills", "research", "SKILL.md"),
		"---\nname: research\n---\nInstructions\n",
	)
	writeTestFile(t, filepath.Join(assets, "agents", "lead.md"), "---\nname: lead\ndescription: Leads work.\n---\nLead instructions.\n")

	root := t.TempDir()
	snapshot := filepath.Join(t.TempDir(), "snapshot")
	hash, err := MaterializeAssets(assets, root, snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if hash == "" {
		t.Fatal("asset hash is empty")
	}

	// A skill links at its folder, not at the files inside it: Codex follows a
	// symlinked skill directory and will not follow a symlinked SKILL.md.
	for _, path := range []string{
		filepath.Join(root, "CLAUDE.md"),
		filepath.Join(root, "AGENTS.md"),
		filepath.Join(root, ".claude", "skills", "research"),
		filepath.Join(root, ".agents", "skills", "research"),
	} {
		info, err := os.Lstat(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode()&os.ModeSymlink == 0 {
			t.Fatalf("%s is not a symlink", path)
		}
	}

	skill, err := os.ReadFile(filepath.Join(root, ".agents", "skills", "research", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(skill), "---\n") {
		t.Fatal("generated marker broke YAML frontmatter")
	}
	// The marker is the launch-time one: eval stamps through prompts.Stamp, so a
	// graded workspace is byte-identical to a real session's.
	if !strings.Contains(string(skill), prompts.MarkerText) {
		t.Fatal("skill is missing generated marker")
	}
}

func TestCopyTreeProducesIndependentFixture(t *testing.T) {
	source := t.TempDir()
	writeTestFile(t, filepath.Join(source, "file.txt"), "original")
	destination := filepath.Join(t.TempDir(), "copy")
	if err := CopyTree(source, destination); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(destination, "file.txt"), "changed")

	content, err := os.ReadFile(filepath.Join(source, "file.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "original" {
		t.Fatalf("source fixture changed: %q", content)
	}
}
