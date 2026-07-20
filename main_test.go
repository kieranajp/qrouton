package main

import (
	"testing"

	"github.com/kieranajp/qrouton/internal/github"
)

func TestParseRepoSpec(t *testing.T) {
	cases := []struct {
		in          string
		owner, name string
		wantErr     bool
	}{
		{in: "kieranajp/qrouton", owner: "kieranajp", name: "qrouton"},
		{in: "  lifesum/lifesum-ios  ", owner: "lifesum", name: "lifesum-ios"},
		{in: "kieranajp/qrouton.git", owner: "kieranajp", name: "qrouton"},
		{in: "kieranajp/qrouton/", owner: "kieranajp", name: "qrouton"},
		{in: "qrouton", wantErr: true},
		{in: "a/b/c", wantErr: true},
		{in: "/qrouton", wantErr: true},
		{in: "kieranajp/", wantErr: true},
		{in: "", wantErr: true},
	}
	for _, tc := range cases {
		owner, name, err := parseRepoSpec(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Errorf("parseRepoSpec(%q) = (%q,%q), want error", tc.in, owner, name)
			}
			continue
		}
		if err != nil || owner != tc.owner || name != tc.name {
			t.Errorf("parseRepoSpec(%q) = (%q,%q,%v), want (%q,%q,nil)", tc.in, owner, name, err, tc.owner, tc.name)
		}
	}
}

func TestAdhocName(t *testing.T) {
	single := adhocName([]github.Repo{{Name: "qrouton"}})
	if single != "qrouton" {
		t.Fatalf("single repo name = %q, want qrouton", single)
	}
	multi := adhocName([]github.Repo{{Name: "api"}, {Name: "web"}})
	if multi != "api-web" {
		t.Fatalf("multi repo name = %q, want api-web", multi)
	}
}
