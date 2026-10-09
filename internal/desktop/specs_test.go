package desktop

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/kieranajp/qrouton/internal/workbench"
)

const specSource = "thoughts/shared/specs/S1-retry.md"

type specFixture struct {
	w      *Windows
	id     string
	path   string
	mu     sync.Mutex
	pushed []document
}

func (f *specFixture) pushes() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.pushed)
}

func openSpec(t *testing.T, source, text string, mode os.FileMode) *specFixture {
	t.Helper()
	root := t.TempDir()
	f := &specFixture{path: filepath.Join(root, source)}
	f.w = newWindows(func(event string, payload any) {
		if !strings.HasPrefix(event, windowContentEvent) {
			return
		}
		f.mu.Lock()
		defer f.mu.Unlock()
		f.pushed = append(f.pushed, payload.(document))
	}, testRegistry(t, root))
	t.Cleanup(f.w.stopAll)
	if err := os.MkdirAll(filepath.Dir(f.path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(f.path, []byte(text), mode); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(f.path, mode); err != nil {
		t.Fatal(err)
	}
	id, err := f.w.openWindow(f.w.shown(), workbench.WindowOptions{
		Kind: workbench.KindDocument, Format: workbench.FormatMarkdown,
		Label: "S1", Source: source, Content: text,
	})
	if err != nil {
		t.Fatal(err)
	}
	f.id = id
	return f
}

func TestSaveSpecReplacesTheFileAndReturnsItsNewHash(t *testing.T) {
	const before, after = "# Spec\n\nAnswer:\n", "# Spec\n\nAnswer: B\n"
	f := openSpec(t, specSource, before, 0o644)
	page, err := f.w.Content(f.id)
	if err != nil {
		t.Fatal(err)
	}
	if page.Hash != contentHash(before) {
		t.Fatalf("Content hash = %q, want the hash of the text it carries", page.Hash)
	}

	hash, err := f.w.SaveSpec(f.id, page.Hash, after)
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(f.path); string(got) != after {
		t.Fatalf("file = %q, want %q", got, after)
	}
	if hash != contentHash(after) {
		t.Fatalf("SaveSpec hash = %q, want the hash of what it wrote", hash)
	}
	if page, _ = f.w.Content(f.id); page.Text != after || page.Hash != hash {
		t.Fatalf("Content after a save = %q/%q, want the saved text and hash", page.Text, page.Hash)
	}

	f.w.documents.rescan()
	if n := f.pushes(); n != 0 {
		t.Fatalf("rescan pushed %d documents after the pane's own save", n)
	}
}

func TestSaveSpecRefusesAStaleHashAndLeavesTheFileAlone(t *testing.T) {
	const before = "# Spec\n\nAnswer:\n"
	f := openSpec(t, specSource, before, 0o644)
	const edited = "# Spec\n\nAn agent's edit.\n\nAnswer:\n"
	if err := os.WriteFile(f.path, []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := f.w.SaveSpec(f.id, contentHash(before), "# Spec\n\nAnswer: A\n")
	if !errors.Is(err, ErrDocumentChanged) {
		t.Fatalf("SaveSpec over an edited file returned %v, want ErrDocumentChanged", err)
	}
	if got, _ := os.ReadFile(f.path); string(got) != edited {
		t.Fatalf("file = %q, want the edit left byte for byte", got)
	}
	if page, _ := f.w.Content(f.id); page.Text != edited || page.Hash != contentHash(edited) {
		t.Fatalf("Content after a refusal = %q, want the text on disk", page.Text)
	}
}

func TestSaveSpecRefusesAnythingButASpecFile(t *testing.T) {
	const text = "# Plan\n"
	plan := openSpec(t, "thoughts/shared/plans/P1-retry.md", text, 0o644)
	if _, err := plan.w.SaveSpec(plan.id, contentHash(text), "# Plan\n\nhijacked\n"); !errors.Is(err, ErrNotASpec) {
		t.Fatalf("SaveSpec on a plan returned %v, want ErrNotASpec", err)
	}
	if got, _ := os.ReadFile(plan.path); string(got) != text {
		t.Fatalf("plan = %q after a refused save", got)
	}

	w, _ := testWindows(t)
	loose, err := w.openWindow(w.shown(), workbench.WindowOptions{
		Kind: workbench.KindDocument, Format: workbench.FormatMarkdown, Label: "loose", Content: text,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.SaveSpec(loose, contentHash(text), "x"); !errors.Is(err, ErrNotASpec) {
		t.Fatalf("SaveSpec on a window with no source returned %v, want ErrNotASpec", err)
	}
}

func TestSaveSpecKeepsTheFileMode(t *testing.T) {
	const before = "# Spec\n"
	f := openSpec(t, specSource, before, 0o600)
	if _, err := f.w.SaveSpec(f.id, contentHash(before), "# Spec\n\nAnswer: A\n"); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(f.path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %o after a save, want 600", info.Mode().Perm())
	}
}

func TestSendSpecAnswersTypesTheFixedLineIntoTheConversation(t *testing.T) {
	rec := &recorder{}
	reg, term, w := testWorkbench(t, newFakeRenderer(), rec.emit)
	root := t.TempDir()
	state := reg.add(root, []string{"/bin/cat"}, withTerminalEnv(os.Environ()))
	reg.reveal(state)
	spec, err := w.openWindow(state, workbench.WindowOptions{
		Kind: workbench.KindDocument, Format: workbench.FormatMarkdown, Label: "S1", Source: specSource, Content: "# Spec\n",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := w.SendSpecAnswers(spec); !errors.Is(err, ErrTerminalNotStarted) {
		t.Fatalf("SendSpecAnswers before the conversation started returned %v, want ErrTerminalNotStarted", err)
	}

	if err := term.Start(state.terminal, 80, 24); err != nil {
		t.Fatal(err)
	}
	if err := w.SendSpecAnswers(spec); err != nil {
		t.Fatal(err)
	}
	line := fmt.Sprintf(specAnswersInFormat, specSource)
	if got := string(specAnswersLine(specSource)); got != line+"\r" {
		t.Fatalf("line = %q, want the fixed sentence and a carriage return", got)
	}
	waitFor(t, "the typed line", func() bool { return strings.Contains(rec.output(), line) })

	plan, err := w.openWindow(state, workbench.WindowOptions{
		Kind: workbench.KindDocument, Format: workbench.FormatMarkdown, Label: "P1", Source: "thoughts/shared/plans/P1.md", Content: "# Plan\n",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := w.SendSpecAnswers(plan); !errors.Is(err, ErrNotASpec) {
		t.Fatalf("SendSpecAnswers on a plan returned %v, want ErrNotASpec", err)
	}
}
