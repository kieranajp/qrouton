package share

import (
	"bytes"
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTitleReadsTheOpeningHeading(t *testing.T) {
	cases := map[string]struct {
		markdown string
		want     string
	}{
		"opening heading":   {"# Sharing an artefact\n\nprose\n", "Sharing an artefact"},
		"after frontmatter": {"---\nstatus: draft\n---\n\n# Real title\n", "Real title"},
		"deeper heading":    {"## Not the title\n", "notes.md"},
		"prose first":       {"Just prose.\n\n# Late heading\n", "notes.md"},
		"blank lines first": {"\n\n# Spaced\n", "Spaced"},
	}
	for name, test := range cases {
		t.Run(name, func(t *testing.T) {
			if got := Title("thoughts/shared/notes.md", []byte(test.markdown)); got != test.want {
				t.Errorf("Title = %q, want %q", got, test.want)
			}
		})
	}
}

// The publisher wraps the fragment in a document of its own and reads the title
// out of the opening kilobytes, which the inlined fonts would otherwise fill.
func TestPageIsAFragmentTitledFirst(t *testing.T) {
	page, err := Page("thoughts/shared/plans/thing.md", []byte("# Thing\n\ntext\n"))
	if err != nil {
		t.Fatalf("Page: %v", err)
	}
	if !strings.HasPrefix(string(page), "<title>Thing</title>") {
		t.Errorf("page does not open with its title: %.60q", page)
	}
	for _, tag := range []string{"<html", "<head", "<body", "<!doctype"} {
		if strings.Contains(strings.ToLower(string(page)), tag) {
			t.Errorf("page carries a %s tag of its own", tag)
		}
	}
}

// A page that fetched anything would render unstyled under a strict policy.
func TestPageFetchesNothing(t *testing.T) {
	page, err := Page("notes.md", []byte("# Notes\n\ntext\n"))
	if err != nil {
		t.Fatalf("Page: %v", err)
	}
	for _, external := range []string{"url(http", "url(//", "url(/", "src=\"http", "href=\"http"} {
		if strings.Contains(string(page), external) {
			t.Errorf("page reaches for %q", external)
		}
	}
}

// Publishing a page means uploading it, and a validator that meets a
// replacement character takes it for content that arrived broken.
func TestPageCarriesNoReplacementCharacter(t *testing.T) {
	page, err := Page("notes.md", []byte("# Notes\n\ntext\n"))
	if err != nil {
		t.Fatalf("Page: %v", err)
	}
	if bytes.ContainsRune(page, '\uFFFD') {
		t.Error("page carries U+FFFD")
	}
}

func TestPageCarriesTheDocumentAndItsSource(t *testing.T) {
	source, markdown := "thoughts/shared/notes.md", "# Notes\n\n</script> is fine here\n"
	page, err := Page(source, []byte(markdown))
	if err != nil {
		t.Fatalf("Page: %v", err)
	}
	encoded := base64.StdEncoding.EncodeToString([]byte("NOTE\n" + source + "\n" + markdown))
	if !strings.Contains(string(page), encoded) {
		t.Error("page does not carry the encoded document")
	}
	if strings.Contains(string(page), markdown) {
		t.Error("page carries the document unencoded, so its markup can close the script tag")
	}
}

func TestPageRefusesAnEmptyDocument(t *testing.T) {
	if _, err := Page("empty.md", []byte("   \n\n")); !errors.Is(err, ErrNoDocument) {
		t.Errorf("err = %v, want ErrNoDocument", err)
	}
}

// Two plans of the same name in different directories are two documents.
func TestWriteKeepsTheWholePathInTheName(t *testing.T) {
	dir := t.TempDir()
	first, err := Write(dir, "thoughts/shared/plans/thing.md", []byte("# One\n"))
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	second, err := Write(dir, "thoughts/shared/research/thing.md", []byte("# Two\n"))
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if first == second {
		t.Fatalf("both documents wrote to %s", first)
	}
	if got := filepath.Base(first); got != "thoughts-shared-plans-thing.html" {
		t.Errorf("name = %q", got)
	}
	body, err := os.ReadFile(first)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if !strings.HasPrefix(string(body), "<title>One</title>") {
		t.Errorf("wrote the wrong document: %.40q", body)
	}
}
