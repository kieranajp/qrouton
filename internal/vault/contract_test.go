package vault

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func fixtureDocument() Document {
	pin := strings.Repeat("a", 40)
	return Document{SchemaVersion: 1, ID: "session/R1", Session: "session", Kind: "research", Title: "Recorded finding", Date: "2026-09-21", Author: "Engineer", State: "active", Repos: []Repository{{Path: "github.com/team/repo", Role: "editing", Revision: &pin}}, Workstream: "work", Body: "# Evidence\n\nFound here.\n"}
}
func fixtureBytes(t *testing.T) []byte {
	t.Helper()
	b, err := Encode(fixtureDocument())
	if err != nil {
		t.Fatal(err)
	}
	return b
}
func TestCanonicalRoundTripAndClosedSchema(t *testing.T) {
	b := fixtureBytes(t)
	d, err := Parse(b)
	if err != nil {
		t.Fatal(err)
	}
	again, err := Encode(d)
	if err != nil {
		t.Fatal(err)
	}
	if string(again) != string(b) {
		t.Fatalf("unstable encoding:\n%s", again)
	}
	cases := map[string]string{
		"unknown":             strings.Replace(string(b), "schema_version: 1", "schema_version: 1\nextra: x", 1),
		"duplicate":           strings.Replace(string(b), "schema_version: 1", "schema_version: 1\nschema_version: 1", 1),
		"nested unknown":      strings.Replace(string(b), "role: editing", "role: editing\n      extra: x", 1),
		"numeric attribution": strings.Replace(string(b), "author: Engineer", "author: 23", 1),
		"invalid date":        strings.Replace(string(b), "2026-09-21", "2026-02-30", 1),
		"status prose":        strings.Replace(string(b), "state: active", "state: ready for review", 1),
		"short pin":           strings.Replace(string(b), strings.Repeat("a", 40), "abcdef", 1),
		"missing revision":    strings.Replace(string(b), "revision: "+strings.Repeat("a", 40), "unrecognized: true", 1),
		"null pin":            strings.Replace(string(b), strings.Repeat("a", 40), "null", 1),
		"absolute repo":       strings.Replace(string(b), "github.com/team/repo", "/tmp/repo", 1),
		"namespace":           strings.Replace(string(b), "id: session/R1", "id: other/R1", 1),
		"envelope":            strings.TrimPrefix(string(b), "---\n"),
		"alias":               strings.Replace(string(b), "author: Engineer", "author: &author Engineer\nticket: *author", 1),
		"wrong list":          strings.Replace(string(b), "repos:", "repos: {}\nignored:", 1),
	}
	for name, input := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := Parse([]byte(input)); !errors.Is(err, ErrInvalidDocument) {
				t.Fatalf("got %v for %s", err, input)
			}
		})
	}
	note := fixtureDocument()
	note.Kind = "note"
	note.Repos[0].Revision = nil
	nb, err := Encode(note)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Parse(nb); err != nil {
		t.Fatal(err)
	}
	note.Repos = nil
	nb, err = Encode(note)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Parse(nb); err != nil {
		t.Fatal(err)
	}
}
func TestLineageAndUnsupportedSchema(t *testing.T) {
	d := fixtureDocument()
	d.Lineage = []Edge{{Relation: "supersedes", Target: d.ID}}
	if _, err := Encode(d); !errors.Is(err, ErrInvalidDocument) {
		t.Fatal(err)
	}
	d.Lineage = []Edge{{Relation: "derived_from", Target: "earlier/R1"}}
	if _, err := Encode(d); err != nil {
		t.Fatal(err)
	}
	b := strings.Replace(string(fixtureBytes(t)), "schema_version: 1", "schema_version: 42\nfuture: true", 1)
	unknown, err := Parse([]byte(b))
	if !errors.Is(err, ErrUnsupportedSchema) || unknown.ID != "session/R1" || !strings.Contains(unknown.Body, "Evidence") {
		t.Fatalf("%+v: %v", unknown, err)
	}
}
func TestRoutingAndExplicitSelection(t *testing.T) {
	profiles := []Profile{{ID: "shared", Name: "Shared", Root: t.TempDir()}, {ID: "private", Name: "Private", Root: t.TempDir()}}
	mappings := map[string]string{"team": "shared", "other": "shared", "person": "private"}
	cases := []struct {
		name        string
		scope       Scope
		destination string
		wantErr     error
	}{
		{"converged", Scope{Repositories: []string{"github.com/team/repo", "github.com/other/repo"}}, "shared", nil},
		{"mixed", Scope{Repositories: []string{"github.com/team/repo", "github.com/person/repo"}}, "", ErrDestinationRequired},
		{"unmapped", Scope{Repositories: []string{"github.com/team/repo", "github.com/missing/repo"}}, "", ErrDestinationRequired},
		{"empty", Scope{}, "", ErrDestinationRequired},
		{"explicit", Scope{Destination: "private"}, "private", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ResolveDestination(profiles, mappings, tc.scope)
			if got != tc.destination || !errors.Is(err, tc.wantErr) {
				t.Fatalf("%s %v", got, err)
			}
		})
	}
	got, err := ResolveReadProfiles(profiles, mappings, Scope{Repositories: []string{"github.com/team/repo"}, ReadProfiles: []string{"private"}})
	if !errors.Is(err, ErrScope) {
		t.Fatalf("%+v %v", got, err)
	}
	got, err = ResolveReadProfiles(profiles, mappings, Scope{})
	if err != nil || !got.SelectionRequired || len(got.Profiles) != 0 {
		t.Fatalf("%+v %v", got, err)
	}
	got, err = ResolveReadProfiles(profiles, mappings, Scope{ReadProfiles: []string{"private"}})
	if err != nil || len(got.Profiles) != 1 || got.Profiles[0] != "private" {
		t.Fatalf("%+v %v", got, err)
	}
}
func put(t *testing.T, root, path string, b []byte) {
	t.Helper()
	target := filepath.Join(root, path)
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, b, 0644); err != nil {
		t.Fatal(err)
	}
}
func TestScopedReadsConflictsAndSymlinkIsolation(t *testing.T) {
	shared, private := t.TempDir(), t.TempDir()
	profiles := []Profile{{ID: "shared", Name: "Shared", Root: shared}, {ID: "private", Name: "Private", Root: private}}
	service, err := NewService(profiles, map[string]string{"team": "shared", "person": "private"})
	if err != nil {
		t.Fatal(err)
	}
	b := fixtureBytes(t)
	put(t, shared, "session/R1.md", b)
	put(t, private, "session/R1.md", b)
	scope := Scope{Repositories: []string{"github.com/team/repo"}}
	result, err := service.Read(scope, Reference{ID: "session/R1"})
	if err != nil || result.Reference.Profile != "shared" {
		t.Fatalf("%+v %v", result, err)
	}
	if _, err := service.Read(scope, Reference{Profile: "private", ID: "session/R1"}); !errors.Is(err, ErrScope) {
		t.Fatal(err)
	}
	mixed := Scope{Repositories: []string{"github.com/team/repo", "github.com/person/repo"}}
	if _, err := service.Read(mixed, Reference{ID: "session/R1"}); !errors.Is(err, ErrAmbiguous) {
		t.Fatal(err)
	}
	put(t, shared, "copy.md", b)
	if _, err := service.Read(scope, Reference{ID: "session/R1"}); err != nil {
		t.Fatal(err)
	}
	put(t, shared, "copy.md", append(append([]byte{}, b...), []byte("Different bytes\n")...))
	if _, err := service.Read(scope, Reference{ID: "session/R1"}); !errors.Is(err, ErrConflict) {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(shared, "copy.md")); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(shared, "session/R1.md")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(private, "session/R1.md"), filepath.Join(shared, "session/R1.md")); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Resolve(scope, result.Reference); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	if _, err := service.Read(scope, Reference{Profile: "shared", Path: "../private/session/R1.md"}); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	if got := service.Status(scope); got.Profiles[0].Invalid != 1 {
		t.Fatalf("%+v", got)
	}
}
func TestStatusAndUnsupportedReadsWithoutIndex(t *testing.T) {
	root := t.TempDir()
	service, err := NewService([]Profile{{ID: "shared", Name: "Shared", Root: root}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	status := service.Status(Scope{})
	if !status.Enabled || !status.Scope.SelectionRequired || len(status.Profiles) != 0 {
		t.Fatalf("%+v", status)
	}
	unknown := []byte("---\nschema_version: 99\nfuture: true\n---\nReadable future document\n")
	put(t, root, "future.md", unknown)
	scope := Scope{ReadProfiles: []string{"shared"}}
	result, err := service.Read(scope, Reference{Profile: "shared", Path: "future.md"})
	if err != nil || result.Supported || result.Content != string(unknown) {
		t.Fatalf("%+v %v", result, err)
	}
	if err := os.RemoveAll(root); err != nil {
		t.Fatal(err)
	}
	status = service.Status(scope)
	if !status.Enabled || status.State != StateUnavailable {
		t.Fatalf("%+v", status)
	}
	disabled, err := NewService(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if status := disabled.Status(Scope{}); status.Enabled || status.State != StateDisabled {
		t.Fatalf("%+v", status)
	}
}

func TestProfilesRejectCanonicalOverlap(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "private")
	if err := os.Mkdir(nested, 0755); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(root, alias); err != nil {
		t.Fatal(err)
	}
	for _, other := range []string{nested, alias} {
		if _, err := NewService([]Profile{{ID: "shared", Name: "Shared", Root: root}, {ID: "private", Name: "Private", Root: other}}, nil); !errors.Is(err, ErrInvalidConfig) {
			t.Fatalf("overlap accepted: %s %v", other, err)
		}
	}
}

func TestInventoryRejectsNamedPipesWithoutBlocking(t *testing.T) {
	root := t.TempDir()
	if err := syscall.Mkfifo(filepath.Join(root, "blocked.md"), 0600); err != nil {
		t.Fatal(err)
	}
	service, err := NewService([]Profile{{ID: "shared", Name: "Shared", Root: root}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan Status, 1)
	go func() { done <- service.Status(Scope{ReadProfiles: []string{"shared"}}) }()
	select {
	case status := <-done:
		if status.Profiles[0].Invalid != 1 {
			t.Fatalf("%+v", status)
		}
	case <-time.After(time.Second):
		t.Fatal("FIFO blocked inventory")
	}
}
