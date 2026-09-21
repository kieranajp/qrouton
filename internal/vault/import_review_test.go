package vault

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeImportDocument(t *testing.T, root, path string, d Document) []byte {
	t.Helper()
	b, err := Encode(d)
	if err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(root, filepath.FromSlash(path))
	if err = os.MkdirAll(filepath.Dir(target), 0700); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(target, b, 0600); err != nil {
		t.Fatal(err)
	}
	return b
}
func TestImportLineageIncludesExistingDestination(t *testing.T) {
	for _, arrival := range []string{"before-preview", "after-preview"} {
		t.Run(arrival, func(t *testing.T) {
			root := t.TempDir()
			s := importService(t, []Profile{{ID: "main", Name: "Main", Root: root}}, t.TempDir())
			existing := fixtureDocument()
			existing.Lineage = []Edge{{Relation: "derived_from", Target: "session/R2"}}
			candidate := fixtureDocument()
			candidate.ID = "session/R2"
			candidate.Lineage = []Edge{{Relation: "derived_from", Target: "session/R1"}}
			b, _ := Encode(candidate)
			request := ImportRequest{Sources: []ImportSource{{Key: "candidate", Name: "R2.md", Content: b}}}
			if arrival == "before-preview" {
				writeImportDocument(t, root, "R1.md", existing)
			}
			p, err := s.PreviewImport(context.Background(), request)
			if err != nil {
				t.Fatal(err)
			}
			if arrival == "before-preview" {
				if p.RepairRequired != 1 {
					t.Fatal("cycle accepted", p)
				}
				return
			}
			if p.Accepted != 1 {
				t.Fatal(p)
			}
			writeImportDocument(t, root, "R1.md", existing)
			confirmPreview(t, s, p, request)
			waitImport(t, s, ImportRepairRequired)
			if _, err = os.Stat(filepath.Join(root, "session", "R2.md")); !os.IsNotExist(err) {
				t.Fatal("cycle published")
			}
		})
	}
}
func TestImportUsesExistingTargetPathWithoutCopy(t *testing.T) {
	root := t.TempDir()
	s := importService(t, []Profile{{ID: "main", Name: "Main", Root: root}}, t.TempDir())
	target := fixtureDocument()
	target.ID = "session/R2"
	target.Body = "Target\n"
	b := writeImportDocument(t, root, "notes/old.md", target)
	before, _ := os.Stat(filepath.Join(root, "notes", "old.md"))
	request := importRequest(t, "[target](R2.md#target)\n")
	request.Sources = append(request.Sources, ImportSource{Key: "target", Name: "R2.md", Content: b})
	p := previewAccepted(t, s, request)
	if p.Entries[1].Path != "notes/old.md" || !strings.Contains(p.Entries[0].Body, "../notes/old.md#target") {
		t.Fatal(p)
	}
	confirmPreview(t, s, p, request)
	waitImport(t, s, ImportWritten)
	after, _ := os.Stat(filepath.Join(root, "notes", "old.md"))
	if !before.ModTime().Equal(after.ModTime()) {
		t.Fatal("idempotent target rewritten")
	}
	if _, err := os.Stat(filepath.Join(root, "session", "R2.md")); !os.IsNotExist(err) {
		t.Fatal("extra target copy")
	}
	result, fragment, err := s.ResolveLink(Scope{ReadProfiles: []string{"main"}}, Reference{Profile: "main", ID: "session/R1"}, "../notes/old.md#target")
	if err != nil || result.Document.ID != target.ID || fragment != "target" {
		t.Fatal(result, fragment, err)
	}
}
func TestImportMovedDependencyRequiresFreshReview(t *testing.T) {
	for _, selected := range []bool{false, true} {
		t.Run(map[bool]string{true: "selected", false: "existing"}[selected], func(t *testing.T) {
			root := t.TempDir()
			s := importService(t, []Profile{{ID: "main", Name: "Main", Root: root}}, t.TempDir())
			target := fixtureDocument()
			target.ID = "session/R2"
			b := writeImportDocument(t, root, "notes/old.md", target)
			request := importRequest(t, "[target](R2.md)\n")
			request.Sources = append(request.Sources, ImportSource{Key: "target", Name: "R2.md", Content: b})
			p := previewAccepted(t, s, request)
			if err := os.Rename(filepath.Join(root, "notes", "old.md"), filepath.Join(root, "notes", "moved.md")); err != nil {
				t.Fatal(err)
			}
			keys := []string{"source"}
			if selected {
				keys = append(keys, "target")
			}
			_, err := s.ConfirmImport(context.Background(), p.ID, keys, func(key string) ([]byte, error) {
				if key == "target" {
					return b, nil
				}
				return request.Sources[0].Content, nil
			})
			if !errors.Is(err, ErrImportRepair) {
				t.Fatal(err)
			}
			if _, err := os.Stat(filepath.Join(root, "session", "R1.md")); !os.IsNotExist(err) {
				t.Fatal("broken dependent written")
			}

		})
	}
}
func TestImportReciprocalMarkdownDependencies(t *testing.T) {
	s := importService(t, []Profile{{ID: "main", Name: "Main", Root: t.TempDir()}}, t.TempDir())
	request := importRequest(t, "[two](R2.md)\n")
	second := fixtureDocument()
	second.ID = "session/R2"
	second.Body = "[one](R1.md)\n"
	b, _ := Encode(second)
	request.Sources = append(request.Sources, ImportSource{Key: "two", Name: "R2.md", Content: b})
	p := previewAccepted(t, s, request)
	confirmPreview(t, s, p, request)
	waitImport(t, s, ImportWritten)
}
func TestQueuedDependencyMissingAndPausedStayPending(t *testing.T) {
	root, state := t.TempDir(), t.TempDir()
	s, err := NewService(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err = s.ConfigureImports(state); err != nil {
		t.Fatal(err)
	}
	profile := Profile{ID: "main", Name: "Main", Root: root}
	first := fixtureDocument()
	a, _ := Encode(first)
	second := fixtureDocument()
	second.ID = "session/R2"
	b, _ := Encode(second)
	store, err := os.OpenRoot(filepath.Join(state, importDirectoryName))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	one := queueEntry{ImportJob: ImportJob{ID: "queue-one", ArtifactID: first.ID, Profile: "main", State: ImportPending}, Version: 1, Path: first.ID + ".md", Root: canonicalProfileRoot(root), Canonical: a, Hash: contentHash(string(a)), Dependencies: []importDependency{{ArtifactID: second.ID, Path: second.ID + ".md", Hash: contentHash(string(b)), Job: "queue-two"}}}
	two := queueEntry{ImportJob: ImportJob{ID: "queue-two", ArtifactID: second.ID, Profile: "main", SessionKey: "second-session", State: ImportPending}, Version: 1, Path: second.ID + ".md", Root: one.Root, Canonical: b, Hash: contentHash(string(b))}
	ledger := identityLedger{}
	if err = s.validateImportDependencies(store, profile, one, inventory{}, ledger, map[string]bool{}); !errors.Is(err, ErrImportPending) {
		t.Fatal(err)
	}
	if err = storeJSON(store, "queue-two.json", two); err != nil {
		t.Fatal(err)
	}
	if err = s.SetPublicationDisabled(context.Background(), two.SessionKey, true); err != nil {
		t.Fatal(err)
	}
	if err = s.validateImportDependencies(store, profile, one, inventory{}, ledger, map[string]bool{}); !errors.Is(err, ErrImportPending) {
		t.Fatal(err)
	}
	if err = s.SetPublicationDisabled(context.Background(), two.SessionKey, false); err != nil {
		t.Fatal(err)
	}
	if err = s.validateImportDependencies(store, profile, one, inventory{}, ledger, map[string]bool{}); err != nil {
		t.Fatal(err)
	}
}
func TestImportSerializedPreviewLimit(t *testing.T) {
	record := importRecord{Version: 1, Sources: []ImportSource{{Content: []byte(strings.Repeat("<", 256))}}, Preview: ImportPreview{Entries: []ImportEntry{{Body: strings.Repeat("<", 256), Canonical: strings.Repeat("<", 256)}}}}
	encoded, err := encodeImportRecord(record, maxImportRecordBytes)
	if err != nil {
		t.Fatal(err)
	}
	if len(encoded) <= len(record.Sources[0].Content)*3 {
		t.Fatal("fixture does not exercise serialization expansion")
	}
	if _, err = encodeImportRecord(record, len(encoded)-1); !errors.Is(err, ErrImportSource) {
		t.Fatal("oversized preview accepted", err)
	}
	if _, err = encodeImportRecord(record, len(encoded)); err != nil {
		t.Fatal("exact limit rejected", err)
	}
}
func policyDisabled(t *testing.T, state, key string) bool {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(state, importDirectoryName, policyPrefix+contentHash(key)+".json"))
	if err != nil {
		t.Fatal(err)
	}
	var p publicationPolicy
	if err = json.Unmarshal(b, &p); err != nil {
		t.Fatal(err)
	}
	return p.Disabled
}
func TestPublicationPolicyPersistenceFailureStaysPaused(t *testing.T) {
	state := t.TempDir()
	s := importService(t, []Profile{{ID: "main", Name: "Main", Root: t.TempDir()}}, state)
	failure := errors.New("manifest write failed")
	for _, disabled := range []bool{true, false} {
		err := s.UpdatePublicationPolicy(context.Background(), "source-session", disabled, func() error { return failure })
		if !errors.Is(err, failure) {
			t.Fatal(err)
		}
		if !policyDisabled(t, state, "source-session") {
			t.Fatal("failure unpaused publication")
		}
	}
}
func TestPublicationTransactionAcrossProcesses(t *testing.T) {
	state := t.TempDir()
	s, err := NewService(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err = s.ConfigureImports(state); err != nil {
		t.Fatal(err)
	}
	command := exec.Command(os.Args[0], "-test.run=^TestPublicationTransactionProcessHelper$")
	command.Env = append(os.Environ(), "QROUTON_PUBLICATION_FIXTURE="+state)
	output := filepath.Join(state, "child-output")
	file, err := os.Create(output)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	command.Stdout = file
	command.Stderr = file
	if err = command.Start(); err != nil {
		t.Fatal(err)
	}
	defer command.Process.Kill()
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err = os.Stat(filepath.Join(state, "entered")); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("child did not enter transaction")
		}
		time.Sleep(5 * time.Millisecond)
	}
	result := make(chan error, 1)
	go func() {
		result <- s.UpdatePublicationPolicy(context.Background(), "session", true, func() error { return os.WriteFile(filepath.Join(state, "manifest"), []byte("true"), 0600) })
	}()
	select {
	case err := <-result:
		t.Fatalf("cross-process transaction interleaved: %v", err)
	case <-time.After(40 * time.Millisecond):
	}
	if err = os.WriteFile(filepath.Join(state, "release"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	if err = command.Wait(); err != nil {
		b, _ := os.ReadFile(output)
		t.Fatal(err, string(b))
	}
	if err = <-result; err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(filepath.Join(state, "manifest"))
	if string(b) != "true" || !policyDisabled(t, state, "session") {
		t.Fatal("manifest and durable policy disagree")
	}
}
func TestPublicationTransactionProcessHelper(t *testing.T) {
	state := os.Getenv("QROUTON_PUBLICATION_FIXTURE")
	if state == "" {
		t.Skip("subprocess fixture")
	}
	s, err := NewService(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err = s.ConfigureImports(state); err != nil {
		t.Fatal(err)
	}
	err = s.UpdatePublicationPolicy(context.Background(), "session", false, func() error {
		if err := os.WriteFile(filepath.Join(state, "entered"), nil, 0600); err != nil {
			return err
		}
		deadline := time.Now().Add(5 * time.Second)
		for {
			if _, err := os.Stat(filepath.Join(state, "release")); err == nil {
				break
			}
			if time.Now().After(deadline) {
				return context.DeadlineExceeded
			}
			time.Sleep(5 * time.Millisecond)
		}
		return os.WriteFile(filepath.Join(state, "manifest"), []byte("false"), 0600)
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestImportLongIdentityCollisionStillQuarantined(t *testing.T) {
	root, state := t.TempDir(), t.TempDir()
	s := importService(t, []Profile{{ID: "main", Name: "Main", Root: root}}, state)
	incoming := fixtureDocument()
	incoming.ID = "session/" + strings.Repeat("a", 240)
	incoming.Body = "incoming\n"
	b, _ := Encode(incoming)
	request := ImportRequest{Sources: []ImportSource{{Key: "long", Name: "source.md", Content: b}}}
	p := previewAccepted(t, s, request)
	existing := incoming
	existing.Body = "existing\n"
	writeImportDocument(t, root, existing.ID+".md", existing)
	confirmPreview(t, s, p, request)
	waitImport(t, s, ImportConflict)
	siblings, _ := filepath.Glob(filepath.Join(root, "session", "conflict-*.md"))
	if len(siblings) != 1 {
		t.Fatal(siblings)
	}
	if _, err := s.Read(Scope{ReadProfiles: []string{"main"}}, Reference{Profile: "main", ID: incoming.ID}); !errors.Is(err, ErrConflict) {
		t.Fatal(err)
	}
}

func TestPublicationRechecksDependencyPathUnderLock(t *testing.T) {
	root, state := t.TempDir(), t.TempDir()
	profile := Profile{ID: "main", Name: "Main", Root: root}
	s, err := NewService([]Profile{profile}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	s.importState = state
	storePath := filepath.Join(state, importDirectoryName)
	if err = os.Mkdir(storePath, 0700); err != nil {
		t.Fatal(err)
	}
	store, err := os.OpenRoot(storePath)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	target := fixtureDocument()
	target.ID = "session/R2"
	b := writeImportDocument(t, root, "notes/old.md", target)
	source := fixtureDocument()
	source.Body = "[target](../notes/old.md)\n"
	a, _ := Encode(source)
	job := queueEntry{ImportJob: ImportJob{ID: "queue-source", ArtifactID: source.ID, Profile: profile.ID}, Version: 1, Path: source.ID + ".md", Root: canonicalProfileRoot(root), Canonical: a, Hash: contentHash(string(a)), Dependencies: []importDependency{{ArtifactID: target.ID, Path: "notes/old.md", Hash: contentHash(string(b))}}}
	if err = os.Rename(filepath.Join(root, "notes", "old.md"), filepath.Join(root, "notes", "moved.md")); err != nil {
		t.Fatal(err)
	}
	if err = s.writeCanonical(context.Background(), profile, job, store); !errors.Is(err, ErrImportRepair) {
		t.Fatal(err)
	}
	if _, err = os.Stat(filepath.Join(root, "session", "R1.md")); !os.IsNotExist(err) {
		t.Fatal("broken dependent written")
	}
}
