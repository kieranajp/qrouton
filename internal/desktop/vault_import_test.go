package desktop

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/kieranajp/qrouton/internal/session"
	"github.com/kieranajp/qrouton/internal/sessionpaths"
	"github.com/kieranajp/qrouton/internal/vault"
	"github.com/kieranajp/qrouton/internal/workbench"
)

func importFixture(t *testing.T) (*Settings, string, string) {
	t.Helper()
	_, cfg, v, shared, _ := vaultFixture(t)
	state := t.TempDir()
	v.importDirectory = func() (string, error) { return state, nil }
	settings := testSettings(t, cfg, nil, nil, nil)
	settings.vaults = v
	bytes, err := os.ReadFile(filepath.Join(shared, "R1.md"))
	if err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(t.TempDir(), "R2.md")
	bytes = []byte(strings.ReplaceAll(string(bytes), "example/R1", "example/R2"))
	if err = os.WriteFile(source, bytes, 0600); err != nil {
		t.Fatal(err)
	}
	settings.chooseImportFiles = func(string) ([]string, error) { return []string{source}, nil }
	return settings, source, shared
}
func TestVaultImportRequiresNativeAdmissionAndFreshSource(t *testing.T) {
	s, source, _ := importFixture(t)
	if _, err := s.PreviewVaultImport(VaultImportInput{Sources: []string{source}}); !errors.Is(err, vault.ErrImportSource) {
		t.Fatal(err)
	}
	files, err := s.SelectVaultImportSources(importMarkdown)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0].Token == source || files[0].Name != "R2.md" {
		t.Fatal(files)
	}
	preview, err := s.PreviewVaultImport(VaultImportInput{Sources: []string{files[0].Token}})
	if err != nil || preview.Accepted != 1 {
		t.Fatalf("%+v %v", preview, err)
	}
	if _, err = s.ConfirmVaultImport("forged", []string{files[0].Token}); !errors.Is(err, vault.ErrImportSource) {
		t.Fatal(err)
	}
	if err = os.WriteFile(source, []byte("changed"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = s.ConfirmVaultImport(preview.ID, []string{files[0].Token}); err == nil {
		t.Fatal("stale source accepted")
	}
	if err = s.ClearVaultImportSources(); err != nil {
		t.Fatal(err)
	}
	if len(s.vaults.importSelections.files) != 0 || len(s.vaults.importSelections.previews) != 0 {
		t.Fatal("selection retained")
	}
}
func TestVaultImportConfirmLeavesOriginalAndSurvivesProfileRemoval(t *testing.T) {
	s, source, shared := importFixture(t)
	original, _ := os.ReadFile(source)
	files, err := s.SelectVaultImportSources(importMarkdown)
	if err != nil {
		t.Fatal(err)
	}
	preview, err := s.PreviewVaultImport(VaultImportInput{Sources: []string{files[0].Token}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(filepath.Join(shared, "example", "R2.md")); !os.IsNotExist(err) {
		t.Fatal("preview wrote canonical")
	}
	for range 2 {
		if _, err = s.ConfirmVaultImport(preview.ID, []string{files[0].Token}); err != nil {
			t.Fatal(err)
		}
	}
	deadline := time.Now().Add(3 * time.Second)
	written := false
	for time.Now().Before(deadline) {
		report, err := s.VaultImportStatus()
		if err != nil {
			t.Fatal(err)
		}
		if len(report.Entries) == 1 && report.Entries[0].State == vault.ImportWritten {
			written = true
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !written {
		t.Fatal("import did not write")
	}
	after, _ := os.ReadFile(source)
	if string(after) != string(original) {
		t.Fatal("source modified")
	}
	next := s.cfg.Snapshot()
	next.VaultProfiles = nil
	next.VaultMappings = nil
	s.cfg.Replace(next)
	report, err := s.VaultImportStatus()
	if err != nil || len(report.Entries) != 1 {
		t.Fatalf("%+v %v", report, err)
	}
}
func TestVaultPickerRollbackAndSymlinkReplacement(t *testing.T) {
	s, source, _ := importFixture(t)
	s.chooseImportFiles = func(string) ([]string, error) { return []string{source, filepath.Join(t.TempDir(), "missing.md")}, nil }
	if _, err := s.SelectVaultImportSources(importMarkdown); err == nil {
		t.Fatal("invalid batch accepted")
	}
	if len(s.vaults.importSelections.files) != 0 {
		t.Fatal("failed batch retained root")
	}
	s.chooseImportFiles = func(string) ([]string, error) { return []string{source}, nil }
	files, err := s.SelectVaultImportSources(importMarkdown)
	if err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "other.md")
	os.WriteFile(target, []byte("private"), 0600)
	os.Remove(source)
	os.Symlink(target, source)
	if _, err = s.PreviewVaultImport(VaultImportInput{Sources: []string{files[0].Token}}); !errors.Is(err, vault.ErrImportSource) {
		t.Fatal(err)
	}
}
func TestVaultImportSelectedManifestAndPublicationPause(t *testing.T) {
	s, _, shared := importFixture(t)
	files, err := s.SelectVaultImportSources(importMarkdown)
	if err != nil {
		t.Fatal(err)
	}
	owner := s.vaults.sessions.current()
	manifestPath := filepath.Join(owner.root(), sessionpaths.ManifestName)
	s.chooseImportFiles = func(string) ([]string, error) { return []string{manifestPath}, nil }
	manifests, err := s.SelectVaultImportSources(importManifest)
	if err != nil {
		t.Fatal(err)
	}
	input := VaultImportInput{Sources: []string{files[0].Token}, Associations: map[string]string{files[0].Token: manifests[0].Token}}
	preview, err := s.PreviewVaultImport(input)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.RemoveAll(shared); err != nil {
		t.Fatal(err)
	}
	if _, err = s.ConfirmVaultImport(preview.ID, []string{files[0].Token}); err != nil {
		t.Fatal(err)
	}
	if _, err = s.SaveVaultSession(VaultSessionInput{Session: owner.slug(), Selection: session.VaultSelection{PublicationDisabled: true}}); err != nil {
		t.Fatal(err)
	}
	report, err := s.VaultImportStatus()
	if err != nil || report.Entries[0].State != vault.ImportPaused {
		t.Fatalf("%+v %v", report, err)
	}
	if _, err = s.SaveVaultSession(VaultSessionInput{Session: owner.slug()}); err != nil {
		t.Fatal(err)
	}
}
func TestDocumentLinksUseAdmittedPaneAndPreserveFragment(t *testing.T) {
	w, _, _, shared, private := vaultFixture(t)
	bytes, _ := os.ReadFile(filepath.Join(shared, "R1.md"))
	bytes = []byte(strings.ReplaceAll(string(bytes), "example/R1", "example/R2"))
	os.WriteFile(filepath.Join(shared, "R2.md"), bytes, 0600)
	id, err := w.openWindow(w.shown(), workbench.WindowOptions{Vault: &vault.Reference{ID: "example/R1"}})
	if err != nil {
		t.Fatal(err)
	}
	target, err := w.OpenDocumentLink(DocumentLink{Source: id, Href: "R2.md#Heading%20With%20Spaces"})
	if err != nil {
		t.Fatal(err)
	}
	doc, err := w.Content(target)
	if err != nil || doc.Fragment != "Heading With Spaces" || doc.Vault.Profile != "shared" {
		t.Fatalf("%+v %v", doc, err)
	}
	for _, href := range []string{"../../R1.md", private + "/R1.md", "%2Fetc/passwd.md", "//remote/R1.md", "file:///etc/passwd", "R2.md?profile=private"} {
		if _, err = w.OpenDocumentLink(DocumentLink{Source: id, Href: href}); err == nil {
			t.Fatalf("accepted %s", href)
		}
	}
	if _, err = w.OpenDocumentLink(DocumentLink{Source: "forged", Href: "R2.md"}); err == nil {
		t.Fatal("forged pane accepted")
	}
}

func TestDocumentLinkPublishesBeforeRevocation(t *testing.T) {
	w, cfg, _, _, _ := vaultFixture(t)
	w.stopFollow()
	id, err := w.openWindow(w.shown(), workbench.WindowOptions{Vault: &vault.Reference{ID: "example/R1"}})
	if err != nil {
		t.Fatal(err)
	}
	entered, release := make(chan struct{}), make(chan struct{})
	events := make(chan document, 8)
	emit := func(name string, payload any) {
		if name != windowContentEvent+id {
			return
		}
		doc := payload.(document)
		if doc.Fragment == "section" && doc.Text != "" {
			close(entered)
			<-release
		}
		events <- doc
	}
	w.registry.emit, w.documents.emit = emit, emit
	done := make(chan error, 1)
	go func() { _, err := w.OpenDocumentLink(DocumentLink{Source: id, Href: "#section"}); done <- err }()
	<-entered
	next := cfg.Snapshot()
	next.VaultProfiles = nil
	next.VaultMappings = nil
	cfg.Replace(next)
	revoked := make(chan struct{})
	go func() { w.documents.invalidateVaults(); close(revoked) }()
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	<-revoked
	first, last := <-events, <-events
	if first.Text == "" || last.Text != "" {
		t.Fatal("fragment restored revoked content")
	}
}
func TestDocumentLinkKeepsOrdinarySourceOwnerAndFragment(t *testing.T) {
	w, _ := testWindows(t)
	owner := w.shown()
	id, err := w.openStructural(owner, workbench.WindowOptions{Kind: workbench.KindDocument, Format: workbench.FormatMarkdown, Source: "notes/source.md", Content: "source"})
	if err != nil {
		t.Fatal(err)
	}
	w.newDocumentFor = func(actual *sessionState, name string) (string, error) {
		if actual != owner || name != "notes/target.md" {
			t.Fatalf("%p %s", actual, name)
		}
		return w.openStructural(actual, workbench.WindowOptions{Kind: workbench.KindDocument, Format: workbench.FormatMarkdown, Source: name, Content: "# Target"})
	}
	target, err := w.OpenDocumentLink(DocumentLink{Source: id, Href: "target.md#target"})
	if err != nil {
		t.Fatal(err)
	}
	var fragment string
	w.with(target, func(window *agentWindow) error { fragment = documentFor(window).Fragment; return nil })
	if fragment != "target" {
		t.Fatal(fragment)
	}
}
func TestInvalidVaultScopeDoesNotPausePublication(t *testing.T) {
	s, _, _ := importFixture(t)
	service, err := s.vaults.manager()
	if err != nil {
		t.Fatal(err)
	}
	owner := s.vaults.sessions.current()
	key := vault.SourceSessionKey(filepath.Join(owner.root(), sessionpaths.ManifestName))
	if err = service.SetPublicationDisabled(s.vaults.ctx, key, false); err != nil {
		t.Fatal(err)
	}
	if _, err = s.SaveVaultSession(VaultSessionInput{Session: owner.slug(), Selection: session.VaultSelection{ReadProfiles: []string{"private"}, PublicationDisabled: true}}); !errors.Is(err, vault.ErrScope) {
		t.Fatal(err)
	}
	directory, _ := s.vaults.importDirectory()
	files, _ := filepath.Glob(filepath.Join(directory, "imports", "policy-*.json"))
	for _, file := range files {
		bytes, _ := os.ReadFile(file)
		if strings.Contains(string(bytes), `"disabled":true`) {
			t.Fatal("invalid save paused publication")
		}
	}
	manifest, err := session.Load(owner.root())
	if err != nil || manifest.Vault != nil && manifest.Vault.PublicationDisabled {
		t.Fatal("invalid save changed manifest")
	}
}

func TestVaultCloseCancelsConfirmationWaitingForPublicationLock(t *testing.T) {
	s, _, _ := importFixture(t)
	files, err := s.SelectVaultImportSources(importMarkdown)
	if err != nil {
		t.Fatal(err)
	}
	owner := s.vaults.sessions.current()
	manifestPath := filepath.Join(owner.root(), sessionpaths.ManifestName)
	s.chooseImportFiles = func(string) ([]string, error) { return []string{manifestPath}, nil }
	manifests, err := s.SelectVaultImportSources(importManifest)
	if err != nil {
		t.Fatal(err)
	}
	preview, err := s.PreviewVaultImport(VaultImportInput{Sources: []string{files[0].Token}, Associations: map[string]string{files[0].Token: manifests[0].Token}})
	if err != nil {
		t.Fatal(err)
	}
	service, err := s.vaults.manager()
	if err != nil {
		t.Fatal(err)
	}
	if err = service.SetPublicationDisabled(s.vaults.ctx, vault.SourceSessionKey(manifestPath), false); err != nil {
		t.Fatal(err)
	}
	state, _ := s.vaults.importDirectory()
	locks, _ := filepath.Glob(filepath.Join(state, "imports", "policy-*.lock"))
	if len(locks) != 1 {
		t.Fatal(locks)
	}
	lock, err := os.OpenFile(locks[0], os.O_RDWR, 0600)
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Close()
	if err = syscall.Flock(int(lock.Fd()), syscall.LOCK_EX); err != nil {
		t.Fatal(err)
	}
	defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
	confirmed := make(chan error, 1)
	go func() { _, err := s.ConfirmVaultImport(preview.ID, []string{files[0].Token}); confirmed <- err }()
	deadline := time.Now().Add(time.Second)
	for s.vaults.importSelections.mu.TryLock() {
		s.vaults.importSelections.mu.Unlock()
		if time.Now().After(deadline) {
			t.Fatal("confirmation did not start")
		}
		time.Sleep(time.Millisecond)
	}
	closed := make(chan struct{})
	go func() { s.vaults.close(); close(closed) }()
	select {
	case <-closed:
	case <-time.After(2 * time.Second):
		t.Fatal("close blocked behind confirmation")
	}
	if err := <-confirmed; err == nil {
		t.Fatal("cancelled confirmation succeeded")
	}
}

func TestVaultValidSessionWriteFailureKeepsPublicationPaused(t *testing.T) {
	s, _, _ := importFixture(t)
	owner := s.vaults.sessions.current()
	service, err := s.vaults.manager()
	if err != nil {
		t.Fatal(err)
	}
	key := vault.SourceSessionKey(filepath.Join(owner.root(), sessionpaths.ManifestName))
	if err = service.SetPublicationDisabled(s.vaults.ctx, key, false); err != nil {
		t.Fatal(err)
	}
	lockPath := sessionpaths.ManifestLock(owner.root())
	if err = os.Remove(lockPath); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if err = os.MkdirAll(lockPath, 0700); err != nil {
		t.Fatal(err)
	}
	if _, err = s.SaveVaultSession(VaultSessionInput{Session: owner.slug(), Selection: session.VaultSelection{PublicationDisabled: true}}); err == nil {
		t.Fatal("manifest lock failure ignored")
	}
	directory, _ := s.vaults.importDirectory()
	policies, _ := filepath.Glob(filepath.Join(directory, "imports", "policy-*.json"))
	paused := false
	for _, file := range policies {
		b, _ := os.ReadFile(file)
		paused = paused || strings.Contains(string(b), `"disabled":true`)
	}
	if !paused {
		t.Fatal("failed manifest save left publication enabled")
	}
	manifest, err := session.Load(owner.root())
	if err != nil {
		t.Fatal(err)
	}
	if manifest.Vault != nil && manifest.Vault.PublicationDisabled {
		t.Fatal("failed save modified manifest")
	}
}
