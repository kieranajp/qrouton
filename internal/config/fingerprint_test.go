package config

import "testing"

func fingerprintOf(roots ...ThoughtsRoot) string {
	return (&Config{Root: "/sessions", Thoughts: Thoughts{Roots: roots}}).ThoughtsFingerprint()
}

func TestTheThoughtsFingerprintIgnoresOrderCaseAndSpacing(t *testing.T) {
	a := fingerprintOf(
		ThoughtsRoot{ID: "work", Path: "/sync/work", Orgs: []string{"acme", "Club"}},
		ThoughtsRoot{ID: "oss", Path: "/sync/oss", Orgs: []string{"gnu"}},
	)
	b := fingerprintOf(
		ThoughtsRoot{ID: "oss", Path: "/sync/oss", Orgs: []string{" GNU "}},
		ThoughtsRoot{ID: "work", Path: "/sync/work", Orgs: []string{"club", "acme"}},
	)
	if a != b {
		t.Fatalf("equivalent configs differ: %s %s", a, b)
	}
}

func TestTheThoughtsFingerprintChangesWithAnyRoutingEdit(t *testing.T) {
	base := fingerprintOf(ThoughtsRoot{ID: "work", Path: "/sync/work", Orgs: []string{"acme"}})
	for name, other := range map[string]string{
		"no folders":   fingerprintOf(),
		"new path":     fingerprintOf(ThoughtsRoot{ID: "work", Path: "/sync/other", Orgs: []string{"acme"}}),
		"new org":      fingerprintOf(ThoughtsRoot{ID: "work", Path: "/sync/work", Orgs: []string{"acme", "club"}}),
		"new id":       fingerprintOf(ThoughtsRoot{ID: "team", Path: "/sync/work", Orgs: []string{"acme"}}),
		"added folder": fingerprintOf(ThoughtsRoot{ID: "work", Path: "/sync/work", Orgs: []string{"acme"}}, ThoughtsRoot{ID: "oss", Path: "/sync/oss", Orgs: []string{"gnu"}}),
	} {
		if other == base {
			t.Errorf("%s did not change the fingerprint", name)
		}
	}
	moved := &Config{Root: "/elsewhere"}
	if moved.ThoughtsFingerprint() == (&Config{Root: "/sessions"}).ThoughtsFingerprint() {
		t.Error("a different derived default did not change the fingerprint")
	}
	custom := &Config{Root: "/sessions", Thoughts: Thoughts{Default: "/notes"}}
	if custom.ThoughtsFingerprint() == (&Config{Root: "/sessions"}).ThoughtsFingerprint() {
		t.Error("a different default path did not change the fingerprint")
	}
}
