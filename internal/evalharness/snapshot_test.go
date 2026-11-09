package evalharness

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/kieranajp/qrouton/prompts"
)

func TestSnapshotFileSymlinkKeepsRecordedInstructionsAndHash(t *testing.T) {
	source, snapshot, workspace := t.TempDir(), t.TempDir(), t.TempDir()
	target := filepath.Join(t.TempDir(), "target.md")
	writeTestFile(t, target, "# Original captured instructions\n")
	if err := os.Symlink(target, filepath.Join(source, "orchestrator.md")); err != nil {
		t.Fatal(err)
	}
	loader, recordedHash, err := snapshotPrompts(source, snapshot)
	if err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, target, "# Changed external target authority\n")
	if err := prompts.Stamp(context.Background(), workspace, loader, prompts.OrchestratorAsset); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(filepath.Join(workspace, "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "Original captured instructions") || strings.Contains(string(content), "Changed external target authority") {
		t.Errorf("recorded snapshot followed mutable file symlink: %q", content)
	}
	currentHash, err := HashTree(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if currentHash != recordedHash {
		t.Errorf("recorded snapshot hash changed after external target edit: recorded=%s current=%s", recordedHash, currentHash)
	}
}

func TestSnapshotDirectorySymlinkIsRejected(t *testing.T) {
	source, snapshot := t.TempDir(), t.TempDir()
	writeTestFile(t, filepath.Join(source, "orchestrator.md"), "# Instructions\n")
	agents := t.TempDir()
	writeTestFile(t, filepath.Join(agents, "lead.md"), "---\nname: lead\ndescription: Leads work.\n---\nLead instructions.\n")
	if err := os.Symlink(agents, filepath.Join(source, "agents")); err != nil {
		t.Fatal(err)
	}
	_, _, err := snapshotPrompts(source, snapshot)
	if !errors.Is(err, ErrPromptLink) {
		t.Fatalf("directory symlink error = %v, want %v", err, ErrPromptLink)
	}
}

func TestSnapshotRejectsCyclicPromptSymlinks(t *testing.T) {
	source := t.TempDir()
	if err := os.Symlink("orchestrator.md", filepath.Join(source, "orchestrator.md")); err != nil {
		t.Fatal(err)
	}
	_, _, err := snapshotPrompts(source, t.TempDir())
	if !errors.Is(err, ErrPromptLink) || !errors.Is(err, syscall.ELOOP) {
		t.Fatalf("cyclic prompt symlink error = %v", err)
	}
}

func TestCopyTreePreservesFixtureFileAndDirectorySymlinks(t *testing.T) {
	source, destination := t.TempDir(), t.TempDir()
	writeTestFile(t, filepath.Join(source, "target.md"), "fixture target")
	writeTestFile(t, filepath.Join(source, "real", "README.md"), "fixture reference")
	links := map[string]string{"note.md": "target.md", "references": "real"}
	for name, target := range links {
		if err := os.Symlink(target, filepath.Join(source, name)); err != nil {
			t.Fatal(err)
		}
	}
	if err := CopyTree(source, destination); err != nil {
		t.Fatal(err)
	}
	for name, target := range links {
		got, err := os.Readlink(filepath.Join(destination, name))
		if err != nil || got != target {
			t.Errorf("fixture link %q = %q, %v, want %q", name, got, err, target)
		}
	}
}
