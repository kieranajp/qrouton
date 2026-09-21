package vault

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestImportMarkdownLinksAndNamespaceMappings(t *testing.T) {
	s := importService(t, []Profile{{ID: "main", Name: "Main", Root: t.TempDir()}}, t.TempDir())
	sourceRoot := t.TempDir()
	first := fixtureDocument()
	first.Body = "[target](R2%20finding.md#evidence \"Title\")\n[[R2 finding#section|Alias]]\n[reference][proof]\n\n[proof]: <R2 finding.md#section> \"Proof\"\n\nNext paragraph.\n"
	first.Lineage = []Edge{{Relation: "derived_from", Target: "session/R2"}}
	second := fixtureDocument()
	second.ID = "session/R2"
	second.Body = "# Evidence\n\n## Section\n"
	a, _ := Encode(first)
	b, _ := Encode(second)
	request := ImportRequest{Sources: []ImportSource{{Key: "a", Name: "R1.md", Origin: filepath.Join(sourceRoot, "R1.md"), Content: a}, {Key: "b", Name: "R2 finding.md", Origin: filepath.Join(sourceRoot, "R2 finding.md"), Content: b}}, Namespaces: map[string]string{"session": "moved"}}
	p := previewAccepted(t, s, request)
	entry := p.Entries[0]
	for _, part := range []string{"[target](R2.md#evidence \"Title\")", "[Alias](R2.md#section)", "[proof]: <R2.md#section> \"Proof\"\n\nNext paragraph."} {
		if !strings.Contains(entry.Body, part) {
			t.Fatalf("missing %s in %s", part, entry.Body)
		}
	}
	if entry.Document.Lineage[0].Target != "moved/R2" {
		t.Fatal(entry.Document.Lineage)
	}
	request.Sources[0], request.Sources[1] = request.Sources[1], request.Sources[0]
	reordered := previewAccepted(t, s, request)
	for i := range p.Entries {
		if p.Entries[i].Canonical != reordered.Entries[i].Canonical {
			t.Fatal("source order changed canonical mappings")
		}
	}
	_, err := s.ConfirmImport(context.Background(), p.ID, []string{"a"}, func(key string) ([]byte, error) {
		if key == "a" {
			return a, nil
		}
		return b, nil
	})
	if !errors.Is(err, ErrImportRepair) {
		t.Fatal("missing linked dependency admitted", err)
	}
}
func TestImportCodeAndExternalBytesUnchanged(t *testing.T) {
	body := "```md\n[attachment](missing.png)\n```\n~~~\n[[missing]]\n~~~\n`[attachment](missing.png)`\n    [attachment](missing.png)\n[x](https://example.test/a\\(b\\)?q=1 \"(title)\")\n[x](https://example.test:8080)\n"
	d := fixtureDocument()
	entry := ImportEntry{Key: "a", Document: d, Destination: "main"}
	source := ImportSource{Key: "a"}
	normalized, repairs, _ := normalizeImportLinks(body, source, entry, []ImportEntry{entry}, []ImportSource{source}, nil)
	if normalized != body || len(repairs) != 0 {
		t.Fatalf("bytes changed: %q %+v", normalized, repairs)
	}
	for _, body := range []string{"unmatched ` [attachment](missing.png)\n", "unmatched ` [[missing]]\n", "![[session/R1]]\n", "![[private.png]]\n", "![image][attachment]\n\n[attachment]: private.png\n\nFollowing.\n"} {
		_, repairs, _ := normalizeImportLinks(body, source, entry, []ImportEntry{entry}, []ImportSource{source}, nil)
		if len(repairs) == 0 {
			t.Fatalf("unresolved link hidden: %q", body)
		}
	}
}
func TestImportPinnedCodeCitationsRequireUniqueRepository(t *testing.T) {
	root := t.TempDir()
	source := ImportSource{Key: "a", Origin: filepath.Join(root, "thoughts", "R1.md"), Session: &ImportSession{Origin: filepath.Join(root, "qrouton.json")}}
	d := fixtureDocument()
	entry := ImportEntry{Key: "a", Document: d, Destination: "main"}
	body := "[code](../src/repo/internal/main.go:12-19)\n"
	normalized, repairs, _ := normalizeImportLinks(body, source, entry, []ImportEntry{entry}, []ImportSource{source}, nil)
	if len(repairs) != 0 || !strings.Contains(normalized, "https://github.com/team/repo/blob/"+*d.Repos[0].Revision+"/internal/main.go#L12-L19") {
		t.Fatal(normalized, repairs)
	}
	second := d.Repos[0]
	second.Path = "github.com/other/repo"
	entry.Document.Repos = append(entry.Document.Repos, second)
	_, repairs, _ = normalizeImportLinks(body, source, entry, []ImportEntry{entry}, []ImportSource{source}, nil)
	if len(repairs) == 0 {
		t.Fatal("ambiguous basename silently resolved")
	}
	entry.Document.Repos = []Repository{d.Repos[0]}
	entry.Document.Repos[0].Revision = nil
	_, repairs, _ = normalizeImportLinks(body, source, entry, []ImportEntry{entry}, []ImportSource{source}, nil)
	if len(repairs) == 0 {
		t.Fatal("missing historical pin guessed")
	}
}
func TestImportLinkRepairAndExactDependency(t *testing.T) {
	root := t.TempDir()
	s := importService(t, []Profile{{ID: "main", Name: "Main", Root: root}}, t.TempDir())
	request := importRequest(t, "![attachment](private.png)\n")
	p, err := s.PreviewImport(context.Background(), request)
	if err != nil || p.RepairRequired != 1 {
		t.Fatal(p, err)
	}
	request.LinkMappings = map[string]string{"private.png": "https://example.test/public.png"}
	p = previewAccepted(t, s, request)
	if !strings.Contains(p.Entries[0].Body, "https://example.test/public.png") {
		t.Fatal(p)
	}
	request = importRequest(t, "[target](R2.md)\n")
	second := fixtureDocument()
	second.ID = "session/R2"
	second.Body = "Reviewed target\n"
	b, _ := Encode(second)
	request.Sources = append(request.Sources, ImportSource{Key: "target", Name: "R2.md", Content: b})
	p = previewAccepted(t, s, request)
	second.Body = "Different target\n"
	other, _ := Encode(second)
	if err = os.Mkdir(filepath.Join(root, "session"), 0700); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(root, "session", "R2.md"), other, 0600); err != nil {
		t.Fatal(err)
	}
	_, err = s.ConfirmImport(context.Background(), p.ID, []string{"source"}, func(string) ([]byte, error) { return request.Sources[0].Content, nil })
	if !errors.Is(err, ErrImportRepair) {
		t.Fatal("different dependency bytes admitted", err)
	}
}
func TestVaultRelativeNavigationRevalidatesScopeAndContainment(t *testing.T) {
	root := t.TempDir()
	s := importService(t, []Profile{{ID: "main", Name: "Main", Root: root}}, t.TempDir())
	if err := os.Mkdir(filepath.Join(root, "session"), 0700); err != nil {
		t.Fatal(err)
	}
	d := fixtureDocument()
	a, _ := Encode(d)
	d.ID = "session/R2"
	b, _ := Encode(d)
	for name, content := range map[string][]byte{"R1.md": a, "R2.md": b} {
		if err := os.WriteFile(filepath.Join(root, "session", name), content, 0600); err != nil {
			t.Fatal(err)
		}
	}
	scope := Scope{ReadProfiles: []string{"main"}}
	from := Reference{Profile: "main", ID: "session/R1"}
	result, fragment, err := s.ResolveLink(scope, from, "R2.md#Evidence")
	if err != nil || fragment != "Evidence" || result.Document.ID != "session/R2" {
		t.Fatal(result, fragment, err)
	}
	for _, target := range []string{"../../escape.md", "%2fetc/passwd", "https://example.test/file.md", "R2.md?query=1"} {
		if _, _, err := s.ResolveLink(scope, from, target); err == nil {
			t.Fatal("escaped:", target)
		}
	}
	if _, _, err := s.ResolveLink(Scope{}, from, "R2.md"); err == nil {
		t.Fatal("revoked scope admitted")
	}
	outside := filepath.Join(t.TempDir(), "R3.md")
	if err := os.WriteFile(outside, b, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "session", "R3.md")); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.ResolveLink(scope, from, "R3.md"); err == nil {
		t.Fatal("symlink escaped")
	}
}
