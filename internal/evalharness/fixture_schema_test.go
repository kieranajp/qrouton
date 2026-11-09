package evalharness

// Fixture manifests must satisfy both the eval harness view and the session schema.

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/kieranajp/qrouton/internal/session"
)

func fixtureManifestPaths(t *testing.T) []string {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join("..", "..", "eval", "fixtures", "*", "qrouton.json"))
	if err != nil {
		t.Fatalf("glob fixture manifests: %v", err)
	}
	if len(paths) == 0 {
		t.Fatal("no fixture manifests found")
	}
	return paths
}

func TestFixtureManifestsMatchSessionSchema(t *testing.T) {
	for _, path := range fixtureManifestPaths(t) {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}

		decoder := json.NewDecoder(bytes.NewReader(content))
		decoder.DisallowUnknownFields()
		var manifest session.Manifest
		if err := decoder.Decode(&manifest); err != nil {
			t.Fatalf("%s does not decode as a session manifest: %v", path, err)
		}

		if manifest.Name == "" || manifest.Slug == "" || manifest.Description == "" {
			t.Errorf("%s: name, slug, and description carry the goal an agent reads; got %q/%q/%q",
				path, manifest.Name, manifest.Slug, manifest.Description)
		}
		if manifest.EffectiveMode() != session.ModeRPI {
			t.Errorf("%s: eval exercises the RPI workflow, got mode %q", path, manifest.EffectiveMode())
		}
		// A session records the agent it was assembled with, and every boot after
		// the first starts that one.
		if manifest.Runner == "" {
			t.Errorf("%s: fixture records no runner, so a boot would fall back to the workbench's", path)
		}

		var editing, reference int
		for _, repo := range manifest.Repos {
			if repo.WorktreePath == "" {
				t.Errorf("%s: repo %s records no worktree path", path, repo.Name)
			}
			switch repo.Role {
			case session.RepoRoleEditing:
				editing++
			case session.RepoRoleReference:
				reference++
			default:
				t.Errorf("%s: repo %s has role %q", path, repo.Name, repo.Role)
			}
		}
		if editing == 0 || reference == 0 {
			t.Errorf("%s: fixtures need an editing and a reference repo to grade both roles, got %d/%d",
				path, editing, reference)
		}
	}
}

// TestFixtureManifestsDecodeIntoHarnessView pins the other half of the contract:
// the narrow struct the graders actually use must see the same repositories.
func TestFixtureManifestsDecodeIntoHarnessView(t *testing.T) {
	for _, path := range fixtureManifestPaths(t) {
		manifest, err := readManifest(path)
		if err != nil {
			t.Fatalf("read %s through the harness view: %v", path, err)
		}
		if len(manifest.Repos) == 0 {
			t.Fatalf("%s: harness view found no repositories; the manifest key drifted", path)
		}
		for _, repo := range manifest.Repos {
			if repo.Name == "" || repo.Role == "" {
				t.Errorf("%s: harness view lost a repo's name or role: %+v", path, repo)
			}
		}
	}
}
