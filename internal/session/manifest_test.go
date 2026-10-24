package session

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kieranajp/qrouton/internal/sessionpaths"
)

func TestManifestRepoJSONFieldOrderIncludesBaseBranch(t *testing.T) {
	repo := ManifestRepo{
		Name: "svc", Org: "org", Role: RepoRoleEditing, Branch: "feat/svc",
		DefaultBranch: "main", BaseBranch: "develop", Revision: "deadbeef",
		WorktreePath: "src/svc", SSHURL: "git@github.com:org/svc.git",
	}
	got, err := json.Marshal(repo)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"name":"svc","org":"org","role":"editing","branch":"feat/svc","defaultBranch":"main",` +
		`"baseBranch":"develop","revision":"deadbeef","worktreePath":"src/svc","sshUrl":"git@github.com:org/svc.git"}`
	if string(got) != want {
		t.Fatalf("ManifestRepo JSON = %s, want %s", got, want)
	}
}

// A repository cut from its default branch carries no base at all, which is
// what keeps an untouched session's manifest identical to what it was.
func TestManifestRepoWithNoBaseOmitsTheKeyEntirely(t *testing.T) {
	repo := ManifestRepo{Name: "svc", Org: "org", Role: RepoRoleEditing, Branch: "feat/svc",
		DefaultBranch: "main", WorktreePath: "src/svc"}
	got, err := json.Marshal(repo)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(got), "baseBranch") {
		t.Fatalf("manifest repo with no base carries a baseBranch key: %s", got)
	}
}

func TestManifestWithNoSummaryOmitsTheKeyEntirely(t *testing.T) {
	got, err := json.Marshal(Manifest{Slug: "s", Description: "d"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(got), `"summary"`) {
		t.Fatalf("manifest with no summary carries a summary key: %s", got)
	}
}

func TestManifestRepoWithoutBaseBranchDecodesEmpty(t *testing.T) {
	doc := `{"name":"svc","org":"org","role":"editing","branch":"feat/svc","defaultBranch":"main","worktreePath":"src/svc"}`
	var repo ManifestRepo
	if err := json.Unmarshal([]byte(doc), &repo); err != nil {
		t.Fatal(err)
	}
	if repo.BaseBranch != "" {
		t.Fatalf("BaseBranch = %q, want empty for a pre-change document", repo.BaseBranch)
	}
}

// Several processes write the manifest at once, and a poller reads it between
// them: none of those writers may let Load observe a torn file, and none may
// leave a staging file behind.
func TestWriteManifestConcurrentWritesNeverTearOrLeaveTempFiles(t *testing.T) {
	dir := t.TempDir()
	if err := WriteManifest(dir, Manifest{SchemaVersion: manifestSchemaVersion, Slug: "concurrent"}); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	var readErr error
	var readers sync.WaitGroup
	readers.Add(1)
	go func() {
		defer readers.Done()
		for ctx.Err() == nil {
			if _, err := Load(dir); err != nil {
				readErr = err
				return
			}
		}
	}()

	var writers sync.WaitGroup
	for i := range 20 {
		writers.Add(1)
		go func(i int) {
			defer writers.Done()
			m := Manifest{SchemaVersion: manifestSchemaVersion, Slug: "concurrent", Name: strings.Repeat("x", i*97)}
			if err := WriteManifest(dir, m); err != nil {
				t.Errorf("WriteManifest: %v", err)
			}
		}(i)
	}
	writers.Wait()
	cancel()
	readers.Wait()

	if readErr != nil {
		t.Fatalf("a concurrent reader saw a torn manifest: %v", readErr)
	}
	if _, err := Load(dir); err != nil {
		t.Fatalf("final manifest does not parse: %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != sessionpaths.ManifestName {
		t.Fatalf("directory holds %v, want only %s", entries, sessionpaths.ManifestName)
	}
}

// Three processes edit a live session's manifest, each loading it, changing one
// field and replacing it. Two doing that at once must both leave their change
// behind: the later one has to see the earlier one's, not the snapshot it would
// have loaded before it.
func TestUpdateManifestConcurrentMutationsBothSurvive(t *testing.T) {
	dir := t.TempDir()
	if err := WriteManifest(dir, Manifest{SchemaVersion: manifestSchemaVersion, Slug: "concurrent"}); err != nil {
		t.Fatal(err)
	}

	mutations := []func(Manifest) (Manifest, error){
		func(m Manifest) (Manifest, error) {
			time.Sleep(50 * time.Millisecond)
			m.Mode = ModeAssistant
			return m, nil
		},
		func(m Manifest) (Manifest, error) {
			time.Sleep(50 * time.Millisecond)
			m.Runner = "codex"
			return m, nil
		},
	}
	var wg sync.WaitGroup
	for _, mutate := range mutations {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := UpdateManifest(dir, mutate); err != nil {
				t.Errorf("UpdateManifest: %v", err)
			}
		}()
	}
	wg.Wait()

	got, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got.Mode != ModeAssistant || got.Runner != "codex" {
		t.Fatalf("one update clobbered the other: mode = %q, runner = %q", got.Mode, got.Runner)
	}
}

func TestCycleStickerPersistsTheCompleteCycleAndClearsTheManifestKey(t *testing.T) {
	dir := t.TempDir()
	if err := WriteManifest(dir, Manifest{SchemaVersion: manifestSchemaVersion, Slug: "stickers"}); err != nil {
		t.Fatal(err)
	}
	order := []Sticker{"star", "bookmark", "question", "exclamation"}
	want := append(slices.Clone(order), "")
	for _, expected := range want {
		got, err := CycleSticker(dir, order)
		if err != nil {
			t.Fatal(err)
		}
		if got != expected {
			t.Fatalf("CycleSticker() = %q, want %q", got, expected)
		}
		loaded, err := Load(dir)
		if err != nil {
			t.Fatal(err)
		}
		if loaded.Sticker != expected {
			t.Fatalf("reloaded sticker = %q, want %q", loaded.Sticker, expected)
		}
	}
	b, err := os.ReadFile(sessionpaths.Manifest(dir))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), `"sticker"`) {
		t.Fatalf("cleared manifest retained sticker: %s", b)
	}
}

func TestNextWalksTheConfiguredOrder(t *testing.T) {
	order := []Sticker{"a1", "b2", "c3"}
	for _, tc := range []struct{ from, want Sticker }{
		{"", "a1"}, {"retired", "a1"}, {"a1", "b2"}, {"b2", "c3"}, {"c3", ""},
	} {
		if got := tc.from.Next(order); got != tc.want {
			t.Fatalf("Sticker(%q).Next() = %q, want %q", tc.from, got, tc.want)
		}
	}
	for _, from := range []Sticker{"", "a1"} {
		if got := from.Next(nil); got != "" {
			t.Fatalf("Sticker(%q).Next(nil) = %q, want none", from, got)
		}
	}

	dir := t.TempDir()
	if err := WriteManifest(dir, Manifest{SchemaVersion: manifestSchemaVersion, Slug: "legacy", Sticker: "retired"}); err != nil {
		t.Fatal(err)
	}
	got, err := CycleSticker(dir, order)
	if err != nil {
		t.Fatal(err)
	}
	if got != "a1" {
		t.Fatalf("CycleSticker() from unknown = %q, want a1", got)
	}
}

func TestCycleStickerAndAnotherManifestMutationBothSurvive(t *testing.T) {
	dir := t.TempDir()
	if err := WriteManifest(dir, Manifest{SchemaVersion: manifestSchemaVersion, Slug: "concurrent"}); err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		if _, err := CycleSticker(dir, []Sticker{"star"}); err != nil {
			t.Errorf("CycleSticker: %v", err)
		}
	}()
	go func() {
		defer wg.Done()
		if err := UpdateManifest(dir, func(m Manifest) (Manifest, error) {
			time.Sleep(50 * time.Millisecond)
			m.Runner = "codex"
			return m, nil
		}); err != nil {
			t.Errorf("UpdateManifest: %v", err)
		}
	}()
	wg.Wait()

	got, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got.Sticker != "star" || got.Runner != "codex" {
		t.Fatalf("concurrent result = %+v", got)
	}
}

func TestAManifestNamingACheckoutOutsideSrcFailsToLoad(t *testing.T) {
	for _, tc := range []struct {
		path string
		ok   bool
	}{
		{"src/api", true},
		{"src/acme-api", true},
		{"", true},
		{"../../victim", false},
		{"src/../../victim", false},
		{"/elsewhere/api", false},
		{"thoughts/api", false},
		{"src", false},
	} {
		t.Run(tc.path, func(t *testing.T) {
			dir := t.TempDir()
			m := Manifest{Slug: filepath.Base(dir), Repos: []ManifestRepo{{Org: "acme", Name: "api", Role: RepoRoleEditing, WorktreePath: tc.path}}}
			if err := WriteManifest(dir, m); err != nil {
				t.Fatal(err)
			}
			_, err := Load(dir)
			if tc.ok && err != nil {
				t.Fatalf("Load = %v", err)
			}
			if !tc.ok && !errors.Is(err, ErrCheckoutOutsideSrc) {
				t.Fatalf("Load = %v, want ErrCheckoutOutsideSrc", err)
			}
		})
	}
}
