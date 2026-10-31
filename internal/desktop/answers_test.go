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

const (
	specSource  = "thoughts/shared/specs/S1-retry.md"
	auditSource = "thoughts/shared/audits/A1-tells.md"
)

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

func TestSaveAnswersReplacesTheFileAndReturnsItsNewHash(t *testing.T) {
	const before, after = "# Spec\n\nAnswer:\n", "# Spec\n\nAnswer: B\n"
	f := openSpec(t, specSource, before, 0o644)
	page, err := f.w.Content(f.id)
	if err != nil {
		t.Fatal(err)
	}
	if page.Hash != contentHash(before) {
		t.Fatalf("Content hash = %q, want the hash of the text it carries", page.Hash)
	}

	hash, err := f.w.SaveAnswers(f.id, page.Hash, after)
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(f.path); string(got) != after {
		t.Fatalf("file = %q, want %q", got, after)
	}
	if hash != contentHash(after) {
		t.Fatalf("SaveAnswers hash = %q, want the hash of what it wrote", hash)
	}
	if page, _ = f.w.Content(f.id); page.Text != after || page.Hash != hash {
		t.Fatalf("Content after a save = %q/%q, want the saved text and hash", page.Text, page.Hash)
	}

	f.w.documents.rescan()
	if n := f.pushes(); n != 0 {
		t.Fatalf("rescan pushed %d documents after the pane's own save", n)
	}
}

func TestSaveAnswersRefusesAStaleHashAndLeavesTheFileAlone(t *testing.T) {
	const before = "# Spec\n\nAnswer:\n"
	f := openSpec(t, specSource, before, 0o644)
	const edited = "# Spec\n\nAn agent's edit.\n\nAnswer:\n"
	if err := os.WriteFile(f.path, []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := f.w.SaveAnswers(f.id, contentHash(before), "# Spec\n\nAnswer: A\n")
	if !errors.Is(err, ErrDocumentChanged) {
		t.Fatalf("SaveAnswers over an edited file returned %v, want ErrDocumentChanged", err)
	}
	if got, _ := os.ReadFile(f.path); string(got) != edited {
		t.Fatalf("file = %q, want the edit left byte for byte", got)
	}
	if page, _ := f.w.Content(f.id); page.Text != edited || page.Hash != contentHash(edited) {
		t.Fatalf("Content after a refusal = %q, want the text on disk", page.Text)
	}
}

func TestSaveAnswersRefusesAPlanOrResearchOrALooseWindow(t *testing.T) {
	const text = "# Plan\n"
	plan := openSpec(t, "thoughts/shared/plans/P1-retry.md", text, 0o644)
	if _, err := plan.w.SaveAnswers(plan.id, contentHash(text), "# Plan\n\nhijacked\n"); !errors.Is(err, ErrNotAnswerable) {
		t.Fatalf("SaveAnswers on a plan returned %v, want ErrNotAnswerable", err)
	}
	if got, _ := os.ReadFile(plan.path); string(got) != text {
		t.Fatalf("plan = %q after a refused save", got)
	}
	research := openSpec(t, "thoughts/shared/research/R1-retry.md", text, 0o644)
	if _, err := research.w.SaveAnswers(research.id, contentHash(text), "x"); !errors.Is(err, ErrNotAnswerable) {
		t.Fatalf("SaveAnswers on research returned %v, want ErrNotAnswerable", err)
	}

	w, _ := testWindows(t)
	loose, err := w.openWindow(w.shown(), workbench.WindowOptions{
		Kind: workbench.KindDocument, Format: workbench.FormatMarkdown, Label: "loose", Content: text,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.SaveAnswers(loose, contentHash(text), "x"); !errors.Is(err, ErrNotAnswerable) {
		t.Fatalf("SaveAnswers on a window with no source returned %v, want ErrNotAnswerable", err)
	}
}

func TestSaveAnswersKeepsTheFileMode(t *testing.T) {
	const before = "# Spec\n"
	f := openSpec(t, specSource, before, 0o600)
	if _, err := f.w.SaveAnswers(f.id, contentHash(before), "# Spec\n\nAnswer: A\n"); err != nil {
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

func TestSendSpecAnswersPastesTheFixedLineIntoTheConversation(t *testing.T) {
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
	if strings.ContainsAny(line, "\r\n\x1b") {
		t.Fatalf("line = %q, want no newline or paste marker inside it", line)
	}
	if got := string(fixedLine(specAnswersInFormat, specSource)); got != "\x1b[200~"+line+"\x1b[201~\r" {
		t.Fatalf("line = %q, want the fixed sentence as a bracketed paste and a carriage return outside it", got)
	}
	waitFor(t, "the typed line", func() bool { return strings.Contains(rec.output(), line) })

	plan, err := w.openWindow(state, workbench.WindowOptions{
		Kind: workbench.KindDocument, Format: workbench.FormatMarkdown, Label: "P1", Source: "thoughts/shared/plans/P1.md", Content: "# Plan\n",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := w.SendSpecAnswers(plan); !errors.Is(err, ErrNotAnswerable) {
		t.Fatalf("SendSpecAnswers on a plan returned %v, want ErrNotAnswerable", err)
	}
}

func TestSaveAnswersWritesAnAuditsTriage(t *testing.T) {
	const before, after = "### F1 — x\n\nTriage:\n", "### F1 — x\n\nTriage: accept\n"
	f := openSpec(t, auditSource, before, 0o644)
	hash, err := f.w.SaveAnswers(f.id, contentHash(before), after)
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(f.path); string(got) != after || hash != contentHash(after) {
		t.Fatalf("file = %q, hash %q", got, hash)
	}
	if _, err := f.w.SaveAnswers(f.id, contentHash(before), before); !errors.Is(err, ErrDocumentChanged) {
		t.Fatalf("SaveAnswers with a stale hash returned %v, want ErrDocumentChanged", err)
	}
}

func TestSendAuditTriagePastesItsOwnLineAndEachSendKeepsToItsKind(t *testing.T) {
	rec := &recorder{}
	reg, term, w := testWorkbench(t, newFakeRenderer(), rec.emit)
	state := reg.add(t.TempDir(), []string{"/bin/cat"}, withTerminalEnv(os.Environ()))
	reg.reveal(state)
	open := func(label, source string) string {
		id, err := w.openWindow(state, workbench.WindowOptions{
			Kind: workbench.KindDocument, Format: workbench.FormatMarkdown, Label: label, Source: source, Content: "# x\n",
		})
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	audit, spec := open("A1", auditSource), open("S1", specSource)
	if err := term.Start(state.terminal, 80, 24); err != nil {
		t.Fatal(err)
	}
	if err := w.SendAuditTriage(spec); !errors.Is(err, ErrWrongSend) {
		t.Fatalf("SendAuditTriage on a spec returned %v, want ErrWrongSend", err)
	}
	if err := w.SendSpecAnswers(audit); !errors.Is(err, ErrWrongSend) {
		t.Fatalf("SendSpecAnswers on an audit returned %v, want ErrWrongSend", err)
	}
	if err := w.SendAuditTriage(audit); err != nil {
		t.Fatal(err)
	}
	line := fmt.Sprintf(auditTriageInFormat, auditSource)
	waitFor(t, "the typed line", func() bool { return strings.Contains(rec.output(), line) })
}
