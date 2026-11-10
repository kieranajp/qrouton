package core

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/kieranajp/qrouton/internal/config"
	"github.com/kieranajp/qrouton/internal/vault"
)

type offlineEmbedder struct{}

var errOffline = errors.New("ollama is not running")

func (offlineEmbedder) Model(context.Context) (vault.Model, error) { return vault.Model{}, errOffline }
func (offlineEmbedder) Embed(context.Context, []string) ([][]float32, error) {
	return nil, errOffline
}

func TestSessionThoughtsRefusesAFolderWithNoIndexInsideAWiderOne(t *testing.T) {
	base := t.TempDir()
	def, shared := filepath.Join(base, "thoughts"), filepath.Join(base, "thoughts", "shared")
	if err := os.MkdirAll(filepath.Join(shared, "s1"), 0o755); err != nil {
		t.Fatal(err)
	}
	set := vault.NewSet(offlineEmbedder{}, t.TempDir())
	if err := set.Configure([]vault.Profile{{ID: config.DefaultThoughtsID, Root: def}}); err != nil {
		t.Fatal(err)
	}
	roots := []config.ThoughtsRoot{{ID: config.DefaultThoughtsID, Path: def}, {ID: "work", Path: shared, Orgs: []string{"acme"}}}
	sessionDir := t.TempDir()
	if err := os.Symlink(filepath.Join(shared, "s1"), filepath.Join(sessionDir, "thoughts")); err != nil {
		t.Fatal(err)
	}
	if _, err := SessionThoughts(set, roots, sessionDir); !errors.Is(err, ErrNoThoughtsRoot) {
		t.Fatalf("SessionThoughts = %v, want %v", err, ErrNoThoughtsRoot)
	}
}
