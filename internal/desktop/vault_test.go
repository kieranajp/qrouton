package desktop

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kieranajp/qrouton/internal/vault"
	"github.com/kieranajp/qrouton/internal/workbench"
)

type offlineEmbedder struct{}

var errOffline = errors.New("ollama is not running")

func (offlineEmbedder) Model(context.Context) (vault.Model, error) { return vault.Model{}, errOffline }
func (offlineEmbedder) Embed(context.Context, []string) ([][]float32, error) {
	return nil, errOffline
}

// thoughtsHost serves a session whose thoughts link points at target, with
// two roots configured.
func thoughtsHost(t *testing.T, own, other, target string) workbench.VaultHost {
	t.Helper()
	set := vault.NewSet(offlineEmbedder{}, t.TempDir())
	if err := set.Configure([]vault.Profile{{ID: "personal", Root: own}, {ID: "team", Root: other}}); err != nil {
		t.Fatal(err)
	}
	sessionDir := t.TempDir()
	if err := os.Symlink(target, filepath.Join(sessionDir, "thoughts")); err != nil {
		t.Fatal(err)
	}
	windows, _ := testWindows(t)
	socket, err := workbench.NewSocketPath()
	if err != nil {
		t.Fatal(err)
	}
	server, err := serveControl(socket, windows, &sessionState{sessionRoot: sessionDir}, controlHooks{vault: set})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = server.Close() })
	return (workbench.Handle{Socket: socket, SessionRoot: sessionDir}).WindowHost().(workbench.VaultHost)
}

func TestTheControlSocketSearchesOnlyTheSessionsThoughtsRoot(t *testing.T) {
	own, other := t.TempDir(), t.TempDir()
	write := func(path, text string) {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(own, "s1", "note.md"), "# Wombats\n\nWombats dig burrows.\n")
	write(filepath.Join(other, "s2", "team.md"), "# Team wombats\n\nWombats dig burrows together.\n")
	host := thoughtsHost(t, own, other, filepath.Join(own, "s1"))
	ctx := context.Background()

	res, err := host.SearchVault(ctx, vault.Query{Text: "wombat burrows"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Status.Semantic != vault.StatusUnavailable || len(res.Hits) != 1 || res.Hits[0].Ref != "personal:s1/note.md" {
		t.Fatalf("search = %+v", res)
	}
	ex, err := host.ReadVault(ctx, vault.ReadRequest{Ref: res.Hits[0].Ref, Line: 3})
	if err != nil || ex.Content != "Wombats dig burrows.\n" {
		t.Fatalf("read = %+v, %v", ex, err)
	}
	for _, ref := range []string{"personal:../escape.md", "team:s2/team.md", "s2/team"} {
		if _, err := host.ReadVault(ctx, vault.ReadRequest{Ref: ref}); err == nil || !strings.Contains(err.Error(), vault.ErrUnknownDocument.Error()) {
			t.Errorf("read %q = %v", ref, err)
		}
	}
}

func TestAThoughtsLinkOutsideEveryRootFindsNoThoughtsRoot(t *testing.T) {
	host := thoughtsHost(t, t.TempDir(), t.TempDir(), t.TempDir())
	if _, err := host.SearchVault(context.Background(), vault.Query{Text: "wombat"}); err == nil || !strings.Contains(err.Error(), ErrNoThoughtsRoot.Error()) {
		t.Fatalf("search = %v", err)
	}
}
