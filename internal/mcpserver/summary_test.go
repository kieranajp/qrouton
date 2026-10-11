package mcpserver

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/kieranajp/qrouton/internal/session"
	"github.com/kieranajp/qrouton/internal/sessionpaths"
	"github.com/kieranajp/qrouton/internal/status"
)

func summarySession(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	err := session.WriteManifest(dir, session.Manifest{
		SchemaVersion: 2, Name: "Retry", Slug: "retry", Description: "Back off on webhook failures.",
		TicketURL: "https://github.com/o/r/issues/1", Mode: session.ModeRPI,
		CreatedAt: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
		Repos:     []session.ManifestRepo{{Name: "svc", Org: "org", Role: session.RepoRoleEditing, WorktreePath: "src/svc"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

func rawManifest(t *testing.T, dir string) map[string]json.RawMessage {
	t.Helper()
	b, err := os.ReadFile(sessionpaths.Manifest(dir))
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestSetSummaryWritesOnlyTheSummary(t *testing.T) {
	dir := summarySession(t)
	before := rawManifest(t, dir)
	start := time.Now().UTC()

	if _, err := setSummary(dir, setSummaryInput{Summary: "  Retrying webhooks with backoff in org/svc.  "}); err != nil {
		t.Fatalf("setSummary: %v", err)
	}

	after := rawManifest(t, dir)
	if _, ok := before["summary"]; ok {
		t.Fatal("fixture already carries a summary")
	}
	summary := after["summary"]
	delete(after, "summary")
	if !reflect.DeepEqual(before, after) {
		t.Errorf("fields other than summary changed:\nbefore %s\nafter  %s", before, after)
	}

	m, err := session.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if m.Summary.Text != "Retrying webhooks with backoff in org/svc." {
		t.Errorf("text = %q", m.Summary.Text)
	}
	if m.Summary.Stage != status.StageResearch {
		t.Errorf("stage = %q, want %q", m.Summary.Stage, status.StageResearch)
	}
	if m.Summary.At.Before(start.Truncate(time.Second)) || m.Summary.At.Location() != time.UTC {
		t.Errorf("at = %v, want UTC at or after %v (raw %s)", m.Summary.At, start, summary)
	}
}

func TestSetSummaryTakesTheStageFromDisk(t *testing.T) {
	dir := summarySession(t)
	plans := filepath.Join(sessionpaths.Thoughts(dir), "plans")
	if err := os.MkdirAll(plans, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(plans, "P1-plan.md"), []byte("# Plan\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := setSummary(dir, setSummaryInput{Summary: "Building it."}); err != nil {
		t.Fatal(err)
	}
	m, err := session.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if m.Summary.Stage != status.StageImplement {
		t.Errorf("stage = %q, want %q", m.Summary.Stage, status.StageImplement)
	}
}

func TestSetSummaryHoldsTheLimitInRunes(t *testing.T) {
	for _, tc := range []struct {
		name string
		text string
		ok   bool
	}{
		{"exactly the limit", strings.Repeat("a", summaryLimit), true},
		{"one over", strings.Repeat("a", summaryLimit+1), false},
		{"multibyte at the limit", strings.Repeat("é", summaryLimit), true},
		{"multibyte one over", strings.Repeat("é", summaryLimit+1), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := summarySession(t)
			_, err := setSummary(dir, setSummaryInput{Summary: tc.text})
			if tc.ok {
				if err != nil {
					t.Fatalf("setSummary: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("accepted text over the limit")
			}
			for _, want := range []string{strconv.Itoa(summaryLimit + 1), strconv.Itoa(summaryLimit)} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error %q does not name %s", err, want)
				}
			}
			if _, ok := rawManifest(t, dir)["summary"]; ok {
				t.Error("a rejected summary was written")
			}
		})
	}
}

func TestSetSummaryRejectsBlankText(t *testing.T) {
	dir := summarySession(t)
	if _, err := setSummary(dir, setSummaryInput{Summary: " \n\t "}); !errors.Is(err, ErrSummaryRequired) {
		t.Fatalf("err = %v, want %v", err, ErrSummaryRequired)
	}
}

func TestSummarySchemaNamesTheLimit(t *testing.T) {
	field, _ := reflect.TypeOf(setSummaryInput{}).FieldByName("Summary")
	if got := field.Tag.Get("jsonschema"); !strings.Contains(got, strconv.Itoa(summaryLimit)) {
		t.Errorf("summary schema = %q, want it to name %d", got, summaryLimit)
	}
}

func TestStageSkillsStateTheSummaryLimit(t *testing.T) {
	want := "at most " + strconv.Itoa(summaryLimit) + " characters"
	corpus := promptCorpus(t)
	for _, id := range []string{"skills/qrouton-research", "skills/qrouton-spec", "skills/qrouton-plan", "skills/qrouton-implement"} {
		if !strings.Contains(corpus[id], want) {
			t.Errorf("%s does not say %q", id, want)
		}
	}
}
