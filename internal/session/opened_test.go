package session

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kieranajp/qrouton/internal/sessionpaths"
)

func TestMarkOpenedRoundTripsThroughLastOpened(t *testing.T) {
	root := t.TempDir()
	at := time.Now().UTC()
	if err := MarkOpened(root, at); err != nil {
		t.Fatal(err)
	}
	got, ok := LastOpened(root)
	if !ok {
		t.Fatal("a session just stamped reads as one this workbench never showed")
	}
	if !got.Equal(at) {
		t.Fatalf("stamp read back as %v, want %v", got, at)
	}
}

func TestLastOpenedReadsAnUnusableStampAsNeverShown(t *testing.T) {
	if at, ok := LastOpened(t.TempDir()); ok || !at.IsZero() {
		t.Fatalf("LastOpened with no stamp = %v, %v; want the zero time and false", at, ok)
	}
	root := t.TempDir()
	if err := os.MkdirAll(sessionpaths.Dir(root), dirMode); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sessionpaths.Opened(root), []byte("last tuesday"), fileMode); err != nil {
		t.Fatal(err)
	}
	if at, ok := LastOpened(root); ok || !at.IsZero() {
		t.Fatalf("LastOpened on an unparseable stamp = %v, %v; want the zero time and false", at, ok)
	}
}

func TestMarkOpenedLeavesTheManifestByteForByte(t *testing.T) {
	root := t.TempDir()
	if err := WriteManifest(root, Manifest{Slug: "webhook", Name: "webhook retry", Mode: ModeRPI}); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(sessionpaths.Manifest(root))
	if err != nil {
		t.Fatal(err)
	}
	if err := MarkOpened(root, time.Now()); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(sessionpaths.Manifest(root))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatalf("stamping rewrote the manifest, so a stamp holding a stale copy can revert an escalation\nbefore: %s\nafter:  %s", before, after)
	}
}

func TestPreferredUsesTheLatestStampAndFallsBackToCreationTime(t *testing.T) {
	root := t.TempDir()
	sessions := []Manifest{
		{Slug: "octopus", CreatedAt: time.Now().Add(-48 * time.Hour)},
		{Slug: "kraken", CreatedAt: time.Now()},
	}
	if got, ok := Preferred(root, sessions); !ok || got.Slug != "kraken" {
		t.Fatalf("Preferred without stamps = %q, %v; want kraken", got.Slug, ok)
	}
	if err := MarkOpened(filepath.Join(root, "octopus"), time.Now()); err != nil {
		t.Fatal(err)
	}
	if got, ok := Preferred(root, sessions); !ok || got.Slug != "octopus" {
		t.Fatalf("Preferred with a stamp = %q, %v; want octopus", got.Slug, ok)
	}
	if got, ok := Preferred(root, nil); ok || got.Slug != "" {
		t.Fatalf("Preferred without sessions = %q, %v", got.Slug, ok)
	}
}
