package launch

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestResolveConfiguredEditorAndSubstitute(t *testing.T) {
	e, err := ResolveEditor([]string{"sh", "-c", "edit {path} at {line}"})
	if err != nil {
		t.Fatal(err)
	}
	got := e.Args("/tmp/a file.md", 12)
	want := []string{"sh", "-c", "edit /tmp/a file.md at 12"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("args = %#v, want %#v", got, want)
	}
}

func TestResolveEditorFromEnvironment(t *testing.T) {
	t.Setenv("VISUAL", `sh -x`)
	t.Setenv("EDITOR", "")
	e, err := ResolveEditor(nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := e.Args("doc.md", 9); !reflect.DeepEqual(got, []string{"sh", "-x", "doc.md"}) {
		t.Fatalf("args = %#v", got)
	}
}

func TestResolveSessionFileRejectsEscapes(t *testing.T) {
	root := t.TempDir()
	inside := filepath.Join(root, "doc.md")
	outside := filepath.Join(t.TempDir(), "secret.md")
	os.WriteFile(inside, []byte("ok"), 0o644)
	os.WriteFile(outside, []byte("no"), 0o644)
	realInside, _ := filepath.EvalSymlinks(inside)
	if got, err := ResolveSessionFile(root, "doc.md"); err != nil || got != realInside {
		t.Fatalf("inside = %q, %v", got, err)
	}
	link := filepath.Join(root, "escape.md")
	os.Symlink(outside, link)
	if _, err := ResolveSessionFile(root, link); err == nil {
		t.Fatal("accepted symlink escape")
	}
}

func TestResolveSessionFileFollowsThoughtsLink(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "session")
	home := filepath.Join(parent, "thoughts", "session", "shared", "plans")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join("..", "thoughts", "session"),
		filepath.Join(root, "thoughts")); err != nil {
		t.Fatal(err)
	}
	plan := filepath.Join(home, "plan.md")
	if err := os.WriteFile(plan, []byte("# plan"), 0o644); err != nil {
		t.Fatal(err)
	}
	realPlan, _ := filepath.EvalSymlinks(plan)

	if got, err := ResolveSessionFile(root, "thoughts/shared/plans/plan.md"); err != nil || got != realPlan {
		t.Fatalf("relative = %q, %v", got, err)
	}
	if got, err := ResolveSessionFile(root, filepath.Join(root, "thoughts/shared/plans/plan.md")); err != nil || got != realPlan {
		t.Fatalf("absolute = %q, %v", got, err)
	}
	if _, err := ResolveSessionDir(root, "thoughts/shared/plans"); err != nil {
		t.Fatalf("dir: %v", err)
	}

	sibling := filepath.Join(parent, "thoughts", "other-session")
	if err := os.MkdirAll(sibling, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := ResolveSessionDir(root, sibling); err == nil {
		t.Fatal("accepted another session's thoughts")
	}
}
