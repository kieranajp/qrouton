package vault

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestCorpusHistoricalPinPrecedence(t *testing.T) {
	a, b := strings.Repeat("a", 40), strings.Repeat("b", 40)
	cases := []struct {
		name, metadata string
		ready          bool
		pin            string
	}{
		{"artifact before manifest", "repo: qrouton\nrevision: " + a, true, a},
		{"conflicting artifact fields", "repo: qrouton\nrevision: " + a + "\ngit_commit: " + b, false, ""},
		{"object conflicts with artifact", "repos:\n  - path: github.com/kieranajp/qrouton\n    role: editing\n    revision: " + a + "\nrevision: " + b, false, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s, request := corpusFixture(t)
			source := corpusSource("one", "R1-2026-07-01-finding.md", tc.metadata, "# Finding\n")
			source.Source.Session = &ImportSession{Key: "manifest", Content: []byte("historical manifest"), Slug: "session", Repositories: []Repository{{Path: "github.com/kieranajp/qrouton", Role: "editing", Revision: &b}}}
			request.Sources = []CorpusSource{source}
			preview, err := s.PreviewCorpus(context.Background(), request)
			if err != nil {
				t.Fatal(err)
			}
			if tc.ready && preview.Ready != 1 {
				t.Fatalf("expected ready, got %+v", preview)
			}
			if !tc.ready && preview.RepairRequired != 1 {
				t.Fatalf("expected repair, got %+v", preview)
			}
			if tc.pin != "" {
				entry, err := s.CorpusEntry(preview.ID, "one")
				if err != nil {
					t.Fatal(err)
				}
				if len(entry.Document.Repos) != 1 || entry.Document.Repos[0].Revision == nil || *entry.Document.Repos[0].Revision != tc.pin {
					t.Fatalf("historical pin replaced: %+v", entry.Document.Repos)
				}
			}
		})
	}
}

func TestCorpusPinAmbiguityAndEquivalence(t *testing.T) {
	full := strings.Repeat("a", 40)
	for _, tc := range []struct {
		name, metadata string
		ready          bool
	}{
		{"equivalent", "repo: qrouton\nrevision: " + full + "\ncommit: aaaaaaa", true},
		{"multiple repositories", "repos: [qrouton, lsx]\nrevision: " + full, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, r := corpusFixture(t)
			r.ResolveRevision = func(repo, pin string) (string, error) {
				if repo != "github.com/kieranajp/qrouton" || pin != "aaaaaaa" {
					t.Fatalf("unexpected resolution %s %s", repo, pin)
				}
				return full, nil
			}
			r.Sources = []CorpusSource{corpusSource("one", "R1-2026-07-01-finding.md", tc.metadata, "# Finding\n")}
			p, err := s.PreviewCorpus(context.Background(), r)
			if err != nil {
				t.Fatal(err)
			}
			if (p.Ready == 1) != tc.ready || (!tc.ready && p.RepairRequired != 1) {
				t.Fatal(p)
			}
		})
	}
}

func TestCorpusSessionArtifactScope(t *testing.T) {
	pin := strings.Repeat("a", 40)
	for _, tc := range []struct {
		name, peer string
		manifest   []Repository
		ready      bool
	}{
		{name: "consistent", peer: "repo: qrouton\nrevision: " + pin, ready: true},
		{name: "different scope", peer: "repo: lsx"},
		{name: "mixed scope", peer: "repos: [qrouton, lsx]"},
		{name: "partial scope", peer: "repos: [qrouton, unknown]"},
		{name: "manifest conflict", peer: "repo: qrouton", manifest: []Repository{{Path: "github.com/lifesum/lsx", Role: "editing"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, r := corpusFixture(t)
			missing := corpusSource("missing", "R1-2026-07-01-finding.md", "", "# Finding\n[peer](R2-2026-07-01-peer.md)\n")
			peer := corpusSource("peer", "R2-2026-07-01-peer.md", tc.peer, "# Peer\n")
			if tc.manifest != nil {
				peer.Source.Session = &ImportSession{Key: "manifest", Content: []byte("manifest"), Repositories: tc.manifest}
			}
			anchor := corpusSource("anchor", "R3-2026-07-01-anchor.md", "repo: qrouton", "# Anchor\n")
			r.Sources = []CorpusSource{missing, peer, anchor}
			p, err := s.PreviewCorpus(context.Background(), r)
			if err != nil {
				t.Fatal(err)
			}
			e, err := s.CorpusEntry(p.ID, "missing")
			if err != nil {
				t.Fatal(err)
			}
			if tc.ready {
				if e.Disposition != "ready" || len(e.Document.Repos) != 1 || e.Document.Repos[0].Revision != nil || e.Document.Repos[0].Role != "reference" || e.Provenance["github.com/kieranajp/qrouton"].Source != "session_artifacts" || len(e.Dependencies) != 1 {
					t.Fatalf("%+v", e)
				}
			} else if e.Disposition != "repair" || len(e.Document.Repos) != 0 {
				t.Fatalf("unsafe inference %+v", e)
			}
		})
	}
}

func TestCorpusSessionEvidenceRevalidatedWithoutPublication(t *testing.T) {
	for _, mutation := range []string{"none", "peer", "absent", "manifest"} {
		t.Run(mutation, func(t *testing.T) {
			s, r := corpusFixture(t)
			a := corpusSource("a", "R1-2026-07-01-a.md", "", "# A\n")
			b := corpusSource("b", "R2-2026-07-01-b.md", "repo: qrouton", "# B\n")
			c := corpusSource("c", "R3-2026-07-01-c.md", "", "# C\n")
			b.Source.Session = &ImportSession{Key: "manifest", Content: []byte("original manifest"), Repositories: []Repository{{Path: "github.com/kieranajp/qrouton", Role: "editing"}}}
			r.Sources = []CorpusSource{a, b, c}
			preview, err := s.PreviewCorpus(context.Background(), r)
			if err != nil || preview.Ready != 3 {
				t.Fatal(preview, err)
			}
			current := map[string][]byte{"a": a.Source.Content, "b": b.Source.Content, "c": c.Source.Content, "manifest": b.Source.Session.Content}
			switch mutation {
			case "peer":
				current["b"] = []byte(strings.ReplaceAll(string(b.Source.Content), "qrouton", "lsx"))
			case "absent":
				current["c"] = corpusSource("c", "R3-2026-07-01-c.md", "repo: lsx", "# C\n").Source.Content
			case "manifest":
				current["manifest"] = []byte("changed manifest")
			}
			read := map[string]bool{}
			report, err := s.ConfirmImport(context.Background(), preview.ID, []string{"a"}, func(key string) ([]byte, error) { read[key] = true; return current[key], nil })
			if mutation != "none" {
				if !errors.Is(err, ErrImportStale) {
					t.Fatalf("expected stale: %+v %v", report, err)
				}
				return
			}
			if err != nil || len(report.Entries) != 1 {
				t.Fatal(report, err)
			}
			for _, key := range []string{"a", "b", "c", "manifest"} {
				if !read[key] {
					t.Fatalf("evidence not read: %s", key)
				}
			}
			waitImport(t, s, ImportWritten)
			for _, id := range []string{"session/R2", "session/R3"} {
				if _, err := s.Read(Scope{ReadProfiles: []string{"personal"}}, Reference{Profile: "personal", ID: id}); !errors.Is(err, ErrNotFound) {
					t.Fatalf("unselected source published: %s %v", id, err)
				}
			}
		})
	}
}
