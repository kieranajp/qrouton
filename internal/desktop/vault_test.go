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

func TestTheControlSocketCarriesVaultSearchAndRead(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "note.md"), []byte("# Wombats\n\nWombats dig burrows.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	index, err := vault.New(vault.Profile{ID: "personal", Root: root}, offlineEmbedder{}, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	windows, _ := testWindows(t)
	socket, err := workbench.NewSocketPath()
	if err != nil {
		t.Fatal(err)
	}
	server, err := serveControl(socket, windows, windows.shown(), controlHooks{vault: index})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = server.Close() })
	host := (workbench.Handle{Socket: socket, SessionRoot: t.TempDir()}).WindowHost().(workbench.VaultHost)
	ctx := context.Background()

	res, err := host.SearchVault(ctx, vault.Query{Text: "wombat burrows"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Status.Semantic != vault.StatusUnavailable || len(res.Hits) != 1 || res.Hits[0].Ref != "personal:note.md" {
		t.Fatalf("search = %+v", res)
	}
	ex, err := host.ReadVault(ctx, vault.ReadRequest{Ref: res.Hits[0].Ref, Line: 3})
	if err != nil || ex.Content != "Wombats dig burrows.\n" {
		t.Fatalf("read = %+v, %v", ex, err)
	}
	if _, err := host.ReadVault(ctx, vault.ReadRequest{Ref: "personal:../escape.md"}); err == nil || !strings.Contains(err.Error(), vault.ErrUnknownDocument.Error()) {
		t.Fatalf("escaping read = %v", err)
	}
}
