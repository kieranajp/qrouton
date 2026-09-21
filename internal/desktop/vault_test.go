package desktop

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kieranajp/qrouton/internal/config"
	"github.com/kieranajp/qrouton/internal/session"
	"github.com/kieranajp/qrouton/internal/vault"
	"github.com/kieranajp/qrouton/internal/workbench"
)

func vaultFixture(t *testing.T) (*Windows, *config.Config, *vaults, string, string) {
	t.Helper()
	w, _ := testWindows(t)
	shared, private := t.TempDir(), t.TempDir()
	cfg := &config.Config{VaultProfiles: []config.VaultProfile{{ID: "shared", Name: "Shared", Root: shared}, {ID: "private", Name: "Private", Root: private}}, VaultMappings: map[string]string{"team": "shared", "person": "private"}}
	v := newVaults(cfg, w.sessions)
	w.setVaults(v)
	t.Cleanup(v.close)
	m := session.Manifest{Slug: w.shown().slug(), Repos: []session.ManifestRepo{{Org: "team", Name: "repo"}}}
	if err := session.WriteManifest(w.shown().root(), m); err != nil {
		t.Fatal(err)
	}
	pin := strings.Repeat("a", 40)
	doc := vault.Document{SchemaVersion: 1, ID: "example/R1", Session: "example", Kind: "research", Title: "Canonical finding", Date: "2026-09-21", Author: "Engineer", State: "active", Workstream: "vault", Repos: []vault.Repository{{Path: "github.com/team/repo", Role: "editing", Revision: &pin}}, Body: "# Canonical finding\n\nShared evidence.\n"}
	body, err := vault.Encode(doc)
	if err != nil {
		t.Fatal(err)
	}
	for _, root := range []string{shared, private} {
		if err := os.WriteFile(filepath.Join(root, "R1.md"), body, 0644); err != nil {
			t.Fatal(err)
		}
	}
	return w, cfg, v, shared, private
}
func TestVaultSocketUsesOwnerAndKeepsReadsIndependent(t *testing.T) {
	w, _, _, _, private := vaultFixture(t)
	socket, err := workbench.NewSocketPath()
	if err != nil {
		t.Fatal(err)
	}
	server, err := serveControl(socket, w, w.shown(), controlHooks{})
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	host := (workbench.Handle{Socket: socket, SessionRoot: private}).WindowHost()
	port := host.(workbench.VaultHost)
	status, err := port.VaultStatus(context.Background())
	if err != nil || len(status.Scope.Profiles) != 1 || status.Scope.Profiles[0] != "shared" {
		t.Fatalf("%+v %v", status, err)
	}
	read, err := port.ReadVault(context.Background(), vault.Reference{ID: "example/R1"})
	if err != nil || !strings.Contains(read.Content, "Shared evidence") {
		t.Fatalf("%+v %v", read, err)
	}
	if read.Path != "" {
		t.Fatal("machine path crossed socket")
	}
	if _, err := port.ReadVault(context.Background(), vault.Reference{Profile: "private", ID: "example/R1"}); err == nil {
		t.Fatal("private root admitted")
	}
	out, err := port.SearchVault(context.Background(), workbench.VaultSearchRequest{Query: "finding"})
	if err != nil || out.Outcome != vault.StateUnavailable || len(out.Results) != 0 {
		t.Fatalf("%+v %v", out, err)
	}
	id, err := host.Open(context.Background(), workbench.WindowOptions{Vault: &read.Reference, Content: "forged", Command: []string{"false"}, Kind: workbench.KindTerminal})
	if err != nil {
		t.Fatal(err)
	}
	page, err := w.Content(id)
	if err != nil || page.Vault == nil || page.Path != "" || page.Text != read.Content || page.Deck {
		t.Fatalf("%+v %v", page, err)
	}
	if _, err := host.Open(context.Background(), workbench.WindowOptions{Vault: &vault.Reference{Profile: "private", ID: "example/R1"}}); err == nil {
		t.Fatal("private pane admitted")
	}
	if err := w.Close(id); err != nil {
		t.Fatal(err)
	}
}
func TestVaultPaneRevocationAndSymlinkReplacement(t *testing.T) {
	w, cfg, _, shared, private := vaultFixture(t)
	id, err := w.openWindow(w.shown(), workbench.WindowOptions{Vault: &vault.Reference{Profile: "shared", ID: "example/R1"}})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(shared, "R1.md")
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(private, "R1.md"), path); err != nil {
		t.Fatal(err)
	}
	if _, err := w.readWindow(id, false); err == nil {
		t.Fatal("replaced symlink read succeeded")
	}
	w.documents.rescan()
	_ = w.with(id, func(window *agentWindow) error {
		if window.opts.Content != "" {
			t.Fatal("revoked content retained")
		}
		return nil
	})
	next := cfg.Snapshot()
	next.VaultProfiles = nil
	next.VaultMappings = nil
	cfg.Replace(next)
	if _, err := w.Content(id); err == nil {
		t.Fatal("revoked profile content succeeded")
	}
	if _, err := w.RenderDiagrams(id, 1); err == nil {
		t.Fatal("revoked diagrams succeeded")
	}
}
func TestVaultSessionSelectionIsPortableAndCannotExpandRepos(t *testing.T) {
	w, cfg, v, _, _ := vaultFixture(t)
	settings := testSettings(t, cfg, nil, nil, nil)
	settings.vaults = v
	if _, err := settings.SaveVaultSession(VaultSessionInput{Session: w.shown().slug(), Selection: session.VaultSelection{ReadProfiles: []string{"private"}}}); !errors.Is(err, vault.ErrScope) {
		t.Fatal(err)
	}
	if _, err := settings.SaveVaultSession(VaultSessionInput{Session: w.shown().slug(), Selection: session.VaultSelection{Repositories: []string{"github.com/person/repo"}}}); !errors.Is(err, vault.ErrScope) {
		t.Fatal(err)
	}
	if err := session.UpdateManifest(w.shown().root(), func(m session.Manifest) (session.Manifest, error) { m.Repos = nil; return m, nil }); err != nil {
		t.Fatal(err)
	}
	view, err := settings.VaultSession()
	if err != nil || !view.Status.Scope.SelectionRequired {
		t.Fatalf("%+v %v", view, err)
	}
	view, err = settings.SaveVaultSession(VaultSessionInput{Session: w.shown().slug(), Workstream: "explicit", Selection: session.VaultSelection{ReadProfiles: []string{"private"}, PublicationDisabled: true}})
	if err != nil || len(view.Status.Scope.Profiles) != 1 || view.Status.Scope.Profiles[0] != "private" {
		t.Fatalf("%+v %v", view, err)
	}
	manifest, err := session.Load(w.shown().root())
	if err != nil || manifest.Workstream != "explicit" || !manifest.Vault.PublicationDisabled {
		t.Fatalf("%+v %v", manifest, err)
	}
	v.close()
	if _, err := v.read(w.shown(), vault.Reference{ID: "example/R1"}); !errors.Is(err, vault.ErrUnavailable) {
		t.Fatal(err)
	}
}
func TestVaultSettingsRejectMappingBeforeSaving(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfg := &config.Config{Orgs: []string{"team"}, Root: t.TempDir()}
	settings := testSettings(t, cfg, nil, nil, nil)
	loaded := settings.Load()
	_, err := settings.Save(SettingsInput{Orgs: loaded.Orgs, Root: loaded.Root, StickerLabels: loaded.StickerLabels, VaultMappings: map[string]string{"team": "missing"}})
	if !errors.Is(err, vault.ErrInvalidConfig) {
		t.Fatal(err)
	}
	if len(cfg.Snapshot().VaultMappings) != 0 {
		t.Fatal("invalid configuration partially saved")
	}
}

func TestVaultReadPublishesRevocationBeforePoll(t *testing.T) {
	w, _, _, shared, _ := vaultFixture(t)
	w.stopFollow()
	id, err := w.openWindow(w.shown(), workbench.WindowOptions{Vault: &vault.Reference{ID: "example/R1"}})
	if err != nil {
		t.Fatal(err)
	}
	events := make(chan document, 4)
	w.registry.emit = func(name string, payload any) {
		if name == windowContentEvent+id {
			events <- payload.(document)
		}
	}
	if err := os.Remove(filepath.Join(shared, "R1.md")); err != nil {
		t.Fatal(err)
	}
	if _, err := w.readWindow(id, false); err == nil {
		t.Fatal("removed artifact readable")
	}
	select {
	case doc := <-events:
		if doc.Text != "" {
			t.Fatal("revocation event retained text")
		}
	default:
		t.Fatal("revocation did not reach pane")
	}
}
func TestVaultPublicationOrdersRefreshBeforeRevocation(t *testing.T) {
	w, cfg, _, _, _ := vaultFixture(t)
	w.stopFollow()
	id, err := w.openWindow(w.shown(), workbench.WindowOptions{Vault: &vault.Reference{ID: "example/R1"}})
	if err != nil {
		t.Fatal(err)
	}
	_ = w.with(id, func(window *agentWindow) error { window.opts.Content = ""; return nil })
	entered, release := make(chan struct{}), make(chan struct{})
	events := make(chan document, 4)
	emit := func(name string, payload any) {
		if name != windowContentEvent+id {
			return
		}
		doc := payload.(document)
		if doc.Text != "" {
			close(entered)
			<-release
		}
		events <- doc
	}
	w.registry.emit = emit
	w.documents.emit = emit
	refreshed := make(chan struct{})
	go func() { w.documents.rescan(); close(refreshed) }()
	<-entered
	next := cfg.Snapshot()
	next.VaultProfiles = nil
	next.VaultMappings = nil
	cfg.Replace(next)
	revoked := make(chan struct{})
	go func() { w.documents.invalidateVaults(); close(revoked) }()
	close(release)
	<-refreshed
	<-revoked
	first, last := <-events, <-events
	if first.Text == "" || last.Text != "" {
		t.Fatalf("refresh/revocation order: %q, %q", first.Text, last.Text)
	}
	select {
	case doc := <-events:
		if doc.Text != "" {
			t.Fatal("revoked content restored")
		}
	default:
	}
}
