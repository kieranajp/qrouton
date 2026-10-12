package mcpserver

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kieranajp/qrouton/internal/artefact"
)

func TestCreateArtefactReturnsTheNewPath(t *testing.T) {
	dir := summarySession(t)
	message, err := createArtefact(dir, createArtefactInput{Kind: " Research ", Title: "Team sync"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(message, filepath.Join("thoughts", "shared", "research", "R1-")) {
		t.Fatalf("message = %q", message)
	}
	entries, err := os.ReadDir(filepath.Join(dir, "thoughts", "shared", "research"))
	if err != nil || len(entries) != 1 {
		t.Fatalf("entries = %v, err %v", entries, err)
	}
}

func TestCreateArtefactRejectsABlankTitleAndAnUnknownKind(t *testing.T) {
	dir := summarySession(t)
	if _, err := createArtefact(dir, createArtefactInput{Kind: "plan", Title: "  "}); !errors.Is(err, ErrArtefactTitle) {
		t.Fatalf("blank title err = %v", err)
	}
	if _, err := createArtefact(dir, createArtefactInput{Kind: "notes", Title: "x"}); !errors.Is(err, artefact.ErrUnknownKind) {
		t.Fatalf("unknown kind err = %v", err)
	}
}
