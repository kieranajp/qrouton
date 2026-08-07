package main

import (
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/kieranajp/qrouton/internal/github"
	"github.com/kieranajp/qrouton/internal/launch"
	"github.com/kieranajp/qrouton/internal/session"
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

// Assembly happens before the workbench is handed over, so its progress has to
// reach the terminal the user is still watching.
func TestProgressReachesTheParentsTerminal(t *testing.T) {
	read, write, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	saved := os.Stderr
	os.Stderr = write
	printProgress(session.Progress{Status: session.ProgressCompleted, Step: "cloned",
		Repo: &github.Repo{Org: "kieranajp", Name: "qrouton"}})
	printProgress(session.Progress{Status: session.ProgressAdvanced, Step: "fetching",
		Repo: &github.Repo{Org: "kieranajp", Name: "qrouton"}})
	os.Stderr = saved
	_ = write.Close()

	out, err := io.ReadAll(read)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "kieranajp/qrouton cloned") {
		t.Fatalf("stderr = %q, want the completed step", out)
	}
	if strings.Contains(string(out), "fetching") {
		t.Fatalf("stderr = %q, want outcomes only", out)
	}
}

// The one line the user gets back has to name something they recognise and a log
// they can open.
func TestOpenedLineNamesTheSessionAndItsLog(t *testing.T) {
	chosen := launch.WorkbenchSpec{SessionRoot: "/sessions/api-web", Socket: "/tmp/qrouton-sock/501/ab.sock"}
	line := fmt.Sprintf(openedFormat, subject(chosen.SessionRoot), workbenchLog(chosen))
	if !strings.Contains(line, "api-web") || !strings.Contains(line, "/sessions/api-web/.qrouton/workbench.log") {
		t.Fatalf("line = %q, want the session name and its log", line)
	}

	landing := launch.WorkbenchSpec{Socket: "/tmp/qrouton-sock/501/ab.sock"}
	line = fmt.Sprintf(openedFormat, subject(landing.SessionRoot), workbenchLog(landing))
	if !strings.Contains(line, sessionListSubject) || !strings.Contains(line, "/tmp/qrouton-sock/501/ab.log") {
		t.Fatalf("line = %q, want the session list and a log beside its socket", line)
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
