package evalharness

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMaterializeAssetsCreatesRunnerDiscoveryLayout(t *testing.T) {
	assets := filepath.Join(t.TempDir(), "assets")
	writeTestFile(t, filepath.Join(assets, "CLAUDE.md"), "# Orchestrator\n")
	writeTestFile(
		t,
		filepath.Join(assets, ".claude", "skills", "research", "SKILL.md"),
		"---\nname: research\n---\nInstructions\n",
	)
	writeTestFile(t, filepath.Join(assets, ".codex", "agents", "lead.toml"), "name = \"lead\"\n")

	root := t.TempDir()
	snapshot := filepath.Join(t.TempDir(), "snapshot")
	hash, err := MaterializeAssets(assets, root, snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if hash == "" {
		t.Fatal("asset hash is empty")
	}

	for _, path := range []string{
		filepath.Join(root, "CLAUDE.md"),
		filepath.Join(root, "AGENTS.md"),
		filepath.Join(root, ".claude", "skills", "research", "SKILL.md"),
		filepath.Join(root, ".agents", "skills", "research", "SKILL.md"),
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
	if !strings.Contains(string(skill), marker) {
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
