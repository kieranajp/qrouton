package desktop

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/kieranajp/qrouton/internal/config"
	"github.com/kieranajp/qrouton/internal/session"
	"github.com/kieranajp/qrouton/internal/vault"
	"github.com/kieranajp/qrouton/internal/workbench"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func TestVaultCorpusScaleAndAdmission(t *testing.T) {
	s, source, _ := importFixture(t)
	root := t.TempDir()
	dir := filepath.Join(root, "example", "shared", "research")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	for i := 10; i < 310; i++ {
		if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("R%d.md", i)), []byte(strings.ReplaceAll(string(data), "example/R2", fmt.Sprintf("example/R%d", i))), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(t.TempDir(), filepath.Join(root, "escape")); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(filepath.Join(root, "fifo.md"), 0600); err != nil {
		t.Fatal(err)
	}
	s.chooseImportCorpus = func() (string, error) { return root, nil }
	corpus, err := s.SelectVaultImportCorpus()
	if err != nil || corpus.Documents != 300 || corpus.Skipped != 2 {
		t.Fatalf("%+v %v", corpus, err)
	}
	if _, err = s.PreviewVaultCorpus(VaultCorpusInput{Token: root, TargetProfile: "shared"}); err == nil {
		t.Fatal("path admitted")
	}
	preview, err := s.PreviewVaultCorpus(VaultCorpusInput{Token: corpus.Token, TargetProfile: "shared"})
	if err != nil || preview.Ready != 300 {
		t.Fatalf("%+v %v", preview, err)
	}
	encoded, _ := json.Marshal(preview)
	if strings.Contains(string(encoded), "Shared evidence") || strings.Contains(string(encoded), "canonical:") {
		t.Fatal("body in summary")
	}
	detail, err := s.VaultCorpusEntry(preview.ID, preview.Entries[0].Key)
	if err != nil || detail.Body == "" {
		t.Fatal(detail, err)
	}
	moved := filepath.Join(root, "old")
	if err := os.Rename(filepath.Join(root, "example"), moved); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(moved, filepath.Join(root, "example")); err != nil {
		t.Fatal(err)
	}
	if _, err = s.PreviewVaultCorpus(VaultCorpusInput{Token: corpus.Token, TargetProfile: "shared"}); err == nil {
		t.Fatal("intermediate symlink admitted")
	}
	if err = s.ClearVaultImportSources(); err != nil {
		t.Fatal(err)
	}
	if _, err = s.VaultCorpusEntry(preview.ID, preview.Entries[0].Key); err == nil {
		t.Fatal("cleared capability admitted")
	}
}

func TestVaultCorpusRejectsIntermediateSpecialAndAggregateBudget(t *testing.T) {
	s, _, _ := importFixture(t)
	root := t.TempDir()
	if err := syscall.Mkfifo(filepath.Join(root, "blocked"), 0600); err != nil {
		t.Fatal(err)
	}
	admitted, err := os.OpenRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	defer admitted.Close()
	if _, _, release, err := importParent(admitted, "blocked/document.md"); err == nil {
		release()
		t.Fatal("FIFO directory admitted")
	}
	content := []byte(strings.Repeat("a", 8<<20))
	for n := 0; n < 9; n++ {
		if err := os.WriteFile(filepath.Join(root, fmt.Sprintf("R%d.md", n)), content, 0600); err != nil {
			t.Fatal(err)
		}
	}
	s.chooseImportCorpus = func() (string, error) { return root, nil }
	corpus, err := s.SelectVaultImportCorpus()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.PreviewVaultCorpus(VaultCorpusInput{Token: corpus.Token, TargetProfile: "shared"}); err == nil {
		t.Fatal("oversized aggregate admitted")
	}
}

func TestVaultCorpusAliasesRequireUniqueMirrorIdentity(t *testing.T) {
	root := t.TempDir()
	for _, identity := range []string{"personal/repo", "company/repo", "personal/unique", "other/mismatch"} {
		dir := filepath.Join(root, ".mirrors", identity+".git")
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
		for _, name := range []string{"objects", "refs"} {
			if err := os.MkdirAll(filepath.Join(dir, name), 0700); err != nil {
				t.Fatal(err)
			}
		}
		if err := os.WriteFile(filepath.Join(dir, "HEAD"), []byte("ref: refs/heads/main\n"), 0600); err != nil {
			t.Fatal(err)
		}
		remote := identity
		if identity == "other/mismatch" {
			remote = "elsewhere/different"
		}
		if err := os.WriteFile(filepath.Join(dir, "config"), []byte("[remote \"origin\"]\nurl = https://github.com/"+remote+".git\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	aliases := corpusAliases(context.Background(), &config.Config{Root: root, VaultMappings: map[string]string{"personal": "private"}})
	if aliases["repo"] != "" || aliases["unique"] != "github.com/personal/unique" || aliases["company/repo"] != "github.com/company/repo" || aliases["mismatch"] != "" {
		t.Fatal(aliases)
	}
}
func TestVaultDocumentUnknownHistoryClears(t *testing.T) {
	_, source, _ := importFixture(t)
	bytes, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := vault.Parse(bytes)
	if err != nil {
		t.Fatal(err)
	}
	parsed.LegacyImport = &vault.LegacyImport{SourceHash: strings.Repeat("a", 64)}
	parsed.Repos[0].Revision = nil
	legacy, err := vault.Encode(parsed)
	if err != nil {
		t.Fatal(err)
	}
	window := &agentWindow{opts: workbench.WindowOptions{Vault: &vault.Reference{Profile: "shared", ID: parsed.ID}, Content: string(legacy), Format: workbench.FormatMarkdown}}
	if documentFor(window).Staleness != "unknown" {
		t.Fatal("missing unknown history")
	}
	window.opts.Content = string(bytes)
	if documentFor(window).Staleness != "" {
		t.Fatal("stale metadata after navigation")
	}
	window.opts.Content = ""
	if documentFor(window).Staleness != "" {
		t.Fatal("stale metadata after revocation")
	}
}

func TestVaultCorpusAdmitsOnlyMatchingManifestAndRechecksIt(t *testing.T) {
	s, source, _ := importFixture(t)
	root := t.TempDir()
	dir := filepath.Join(root, "example", "shared", "research")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(dir, "R2.md"), data, 0600); err != nil {
		t.Fatal(err)
	}
	sessions := t.TempDir()
	s.cfg.Root = sessions
	manifestRoot := filepath.Join(sessions, "example")
	if err = os.MkdirAll(manifestRoot, 0700); err != nil {
		t.Fatal(err)
	}
	manifest := session.Manifest{Slug: "example", Repos: []session.ManifestRepo{{Org: "team", Name: "repo", Revision: strings.Repeat("a", 40)}}}
	if err = session.WriteManifest(manifestRoot, manifest); err != nil {
		t.Fatal(err)
	}
	s.chooseImportCorpus = func() (string, error) { return root, nil }
	corpus, err := s.SelectVaultImportCorpus()
	if err != nil {
		t.Fatal(err)
	}
	if len(s.vaults.importSelections.associations) != 1 {
		t.Fatal("missing manifest association")
	}
	preview, err := s.PreviewVaultCorpus(VaultCorpusInput{Token: corpus.Token, TargetProfile: "shared"})
	if err != nil || preview.Ready != 1 {
		t.Fatal(preview, err)
	}
	manifest.Workstream = "changed"
	if err = session.WriteManifest(manifestRoot, manifest); err != nil {
		t.Fatal(err)
	}
	if _, err = s.ConfirmVaultImport(preview.ID, []string{preview.Entries[0].Key}); err == nil {
		t.Fatal("changed manifest accepted")
	}
}
