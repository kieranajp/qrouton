package launch

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kieranajp/qrouton/internal/workbench"
)

func documentRoot(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for name, body := range files {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

var testDocumentEditor = EditorCommand{Argv: []string{"vi", "+{line}", "{path}"}, Template: true}

func TestDocumentWindowRendersMarkdownAndEditsEverythingElse(t *testing.T) {
	root := documentRoot(t, map[string]string{
		"thoughts/shared/plans/P007.md": "# Document panes\n\nThe pane is a tab.\n",
		"src/app/main.go":               "package main\n",
	})

	pane, err := DocumentWindow(root, "thoughts/shared/plans/P007.md", testDocumentEditor, workbench.LineSpan{Line: 12})
	if err != nil {
		t.Fatal(err)
	}
	if pane.Kind != workbench.KindDocument || pane.Format != workbench.FormatMarkdown {
		t.Fatalf("markdown opened as a %q/%q", pane.Kind, pane.Format)
	}
	if pane.Source != filepath.Join("thoughts", "shared", "plans", "P007.md") {
		t.Fatalf("pane source = %q", pane.Source)
	}
	if !strings.Contains(pane.Content, "The pane is a tab.") {
		t.Fatalf("pane content = %q", pane.Content)
	}
	if len(pane.Command) != 0 {
		t.Fatalf("a rendered pane carries a command: %v", pane.Command)
	}
	if pane.Span != (workbench.LineSpan{Line: 12, Through: 12}) {
		t.Fatalf("pane span = %+v, want line 12 alone", pane.Span)
	}

	editor, err := DocumentWindow(root, "src/app/main.go", testDocumentEditor, workbench.LineSpan{Line: 12})
	if err != nil {
		t.Fatal(err)
	}
	if editor.Kind != workbench.KindTerminal {
		t.Fatalf("a source file opened as a %q", editor.Kind)
	}
	real, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(editor.Command, " "); got != "vi +12 "+filepath.Join(real, "src/app/main.go") {
		t.Fatalf("editor command = %q, want the file at line 12", got)
	}
}

func TestDocumentWindowPassesAMarkedRangeToThePaneAndItsFirstLineToTheEditor(t *testing.T) {
	root := documentRoot(t, map[string]string{
		"plan.md": "# Plan\n\nA paragraph.\n",
		"main.go": "package main\n",
	})

	pane, err := DocumentWindow(root, "plan.md", testDocumentEditor, workbench.LineSpan{Line: 8, Through: 14})
	if err != nil {
		t.Fatal(err)
	}
	if pane.Span != (workbench.LineSpan{Line: 8, Through: 14}) {
		t.Fatalf("pane span = %+v, want lines 8 to 14", pane.Span)
	}

	editor, err := DocumentWindow(root, "main.go", testDocumentEditor, workbench.LineSpan{Line: 8, Through: 14})
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(editor.Command, " "); !strings.Contains(got, "+8 ") {
		t.Fatalf("editor command = %q, want it opened at line 8", got)
	}
}

// A span nobody asked for must not mark the top of the document, and one whose
// range runs backwards is the line it names rather than an empty range the pane
// would draw nothing for.
func TestDocumentWindowNormalisesASpanThePaneCannotUse(t *testing.T) {
	root := documentRoot(t, map[string]string{"plan.md": "# Plan\n"})

	for _, tc := range []struct {
		name string
		span workbench.LineSpan
		want workbench.LineSpan
	}{
		{"none", workbench.LineSpan{}, workbench.LineSpan{}},
		{"a through with no line", workbench.LineSpan{Through: 9}, workbench.LineSpan{}},
		{"a line below one", workbench.LineSpan{Line: -3}, workbench.LineSpan{}},
		{"a backwards range", workbench.LineSpan{Line: 9, Through: 4}, workbench.LineSpan{Line: 9, Through: 9}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			opts, err := DocumentWindow(root, "plan.md", testDocumentEditor, tc.span)
			if err != nil {
				t.Fatal(err)
			}
			if opts.Span != tc.want {
				t.Fatalf("pane span = %+v, want %+v", opts.Span, tc.want)
			}
		})
	}
}

// A session's thoughts/ is a symlink out of the session, so every document
// resolves outside the root and the relative name used to fall back to whatever
// the caller passed — an absolute path, when the agent had one to hand, which
// the pane then printed above the title.
func TestDocumentWindowNamesAParkedDocumentRelativeToTheSession(t *testing.T) {
	root := documentRoot(t, map[string]string{"qrouton.json": "{}"})
	parked := filepath.Join(t.TempDir(), "session-slug")
	if err := os.MkdirAll(filepath.Join(parked, "shared", "plans"), 0o755); err != nil {
		t.Fatal(err)
	}
	doc := filepath.Join(parked, "shared", "plans", "P007.md")
	if err := os.WriteFile(doc, []byte("# Document panes\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(parked, filepath.Join(root, "thoughts")); err != nil {
		t.Fatal(err)
	}

	want := filepath.Join("thoughts", "shared", "plans", "P007.md")
	for _, name := range []string{want, doc} {
		opts, err := DocumentWindow(root, name, testDocumentEditor, workbench.LineSpan{Line: 1})
		if err != nil {
			t.Fatal(err)
		}
		if opts.Source != want {
			t.Errorf("opening %q sourced the pane at %q, want %q", name, opts.Source, want)
		}
	}
}

// The tab has room for a title, not a path — and the heading a document opens
// with is the title it chose. Only the opening one: a `# ` further down is a
// later section or the inside of a code fence, and a tab named after either
// would be lying.
func TestDocumentWindowNamesThePaneAfterTheDocument(t *testing.T) {
	for name, body := range map[string]string{
		"◆ Document panes": "# Document panes\n\nbody\n",
		"◆ Behind the fence": "---\ndate: 2026-08-11\ntitle: not this one\n---\n\n" +
			"# Behind the fence\n",
		"◆ headless.md": "No heading here, just prose.\n",
		"◆ deeper.md":   "## Only a second-level heading\n",
		"◆ fenced.md":   "```sh\n# not a title, a shell comment\n```\n",
		"◆ sectioned.md": "Intro prose the document opens with.\n\n" +
			"# Section two\n",
	} {
		file := strings.TrimPrefix(name, "◆ ")
		if !strings.HasSuffix(file, ".md") {
			file = "doc.md"
		}
		root := documentRoot(t, map[string]string{file: body})
		opts, err := DocumentWindow(root, file, testDocumentEditor, workbench.LineSpan{Line: 1})
		if err != nil {
			t.Fatal(err)
		}
		if opts.Label != name {
			t.Errorf("%q opened a pane labelled %q, want %q", file, opts.Label, name)
		}
	}
}

// A file this size is a log, and a window holding a copy of it serves nobody.
func TestDocumentWindowSendsAHugeMarkdownFileToTheEditor(t *testing.T) {
	root := documentRoot(t, map[string]string{"huge.md": strings.Repeat("x", documentLimit+1)})
	opts, err := DocumentWindow(root, "huge.md", testDocumentEditor, workbench.LineSpan{Line: 1})
	if err != nil {
		t.Fatal(err)
	}
	if opts.Kind != workbench.KindTerminal {
		t.Fatalf("a %d-byte document opened as a %q", documentLimit+1, opts.Kind)
	}
}

func TestDocumentWindowRefusesToLeaveTheSession(t *testing.T) {
	root := documentRoot(t, map[string]string{"thoughts/shared/plans/P007.md": "# in\n"})
	if _, err := DocumentWindow(root, "../outside.md", testDocumentEditor, workbench.LineSpan{Line: 1}); err == nil {
		t.Fatal("a path outside the session opened a window")
	}
}

// A session with no editor still renders what it can, and says so otherwise.
func TestDocumentWindowNeedsNoEditorForAPane(t *testing.T) {
	root := documentRoot(t, map[string]string{"note.md": "# Note\n", "main.go": "package main\n"})
	if _, err := DocumentWindow(root, "note.md", EditorCommand{}, workbench.LineSpan{Line: 1}); err != nil {
		t.Fatalf("a pane needed an editor: %v", err)
	}
	if _, err := DocumentWindow(root, "main.go", EditorCommand{}, workbench.LineSpan{Line: 1}); !errors.Is(err, ErrNoEditor) {
		t.Fatalf("a source file with no editor configured = %v, want %v", err, ErrNoEditor)
	}
}
