package mux

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHandleRoundTripsAcrossExecBoundary(t *testing.T) {
	h := Handle{Kind: "zellij", Session: "my-session", SocketDir: "/tmp/zellij"}
	got, err := ParseHandle(h.Marshal())
	if err != nil {
		t.Fatal(err)
	}
	if got != h {
		t.Fatalf("round-trip = %#v, want %#v", got, h)
	}
}

func TestParseHandleRejectsGarbageAndMissingIdentity(t *testing.T) {
	if _, err := ParseHandle("not json"); err == nil {
		t.Fatal("accepted malformed handle")
	}
	if _, err := ParseHandle(`{"kind":"zellij"}`); err == nil {
		t.Fatal("accepted handle without a session")
	}
	if _, err := ParseHandle(`{"session":"s"}`); err == nil {
		t.Fatal("accepted handle without a kind")
	}
}

// A Handle crosses the exec boundary as JSON, so it stays validated even though
// Zellij is the only backend New can hand back.
func TestUnknownBackendsAreRejectedByHandle(t *testing.T) {
	_, err := (Handle{Kind: "tmux", Session: "s"}).PaneHost()
	if err == nil || !strings.Contains(err.Error(), "tmux") {
		t.Fatalf("PaneHost should name the unsupported backend, got %v", err)
	}
}

func TestStageSubstitutesSessionDirIntoRunBlocks(t *testing.T) {
	dir := t.TempDir()
	bin := "/opt/build/qrouton"
	ws := Workspace{Slug: "demo", Dir: dir, Binary: bin, HelpScript: "/cfg/help.sh",
		Tiled: Node{Pane: &Pane{Name: "agent", Command: []string{"sh"}}}}
	if err := NewZellij("zellij", "/tmp/zellij").Stage(ws); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(dir, ".qrouton", "zellij-config.kdl"))
	if err != nil {
		t.Fatal(err)
	}
	config := string(b)
	if !strings.Contains(config, `bind "Alt e"`) {
		t.Fatal("staged config missing the Alt-e escalation binding")
	}
	if !strings.Contains(config, `"pick" "--session-root" "`+dir+`"`) {
		t.Fatalf("staged config does not target this session's directory:\n%s", config)
	}
	// The chords must run this binary, not whatever "qrouton" PATH resolves to
	// — a locally built one usually is not on PATH at all.
	for _, want := range []string{`Run "` + bin + `" "pick"`, `Run "` + bin + `" "mode"`} {
		if !strings.Contains(config, want) {
			t.Fatalf("staged config missing %q:\n%s", want, config)
		}
	}
	for _, placeholder := range []string{sessionDirPlaceholder, binaryPlaceholder, helpScriptPlaceholder} {
		if strings.Contains(config, placeholder) {
			t.Fatalf("%s survived staging", placeholder)
		}
	}
}

func TestRenderKDLShapesSplitsSizesAndFloats(t *testing.T) {
	ws := Workspace{
		Slug: "demo",
		Dir:  "/work/demo",
		Tiled: Node{Split: "vertical", Children: []Node{
			{Size: "65%", Pane: &Pane{Name: "agent", Command: []string{"claude", "--flag", `tricky "quote"`}}},
			{Split: "horizontal", Size: "35%", Children: []Node{
				{Pane: &Pane{Name: "shell", Command: []string{"sh", "-lc", "echo hi"}}},
				{Size: "6", Pane: &Pane{Name: "status", Command: []string{"qrouton", "repos"}}},
				{Size: "1", Pane: &Pane{Name: "strip", Borderless: true, Command: []string{"qrouton", "status"}}},
			}},
		}},
	}
	kdl := renderKDL(ws)
	for _, want := range []string{
		`plugin location="zellij:status-bar"`,
		`pane split_direction="vertical" {`,
		`pane split_direction="horizontal" size="35%" {`, // percentages stay quoted
		`pane size=6 name="status" {`,                    // row counts render bare
		`pane size=1 borderless=true name="strip" {`,     // borderless leaves render frameless
		`pane size="65%" name="agent" {`,
		"command \"claude\"\n",
		`args "--flag" "tricky \"quote\""`, // args survive KDL quoting
		"session_name \"demo\"\nattach_to_session true\n",
	} {
		if !strings.Contains(kdl, want) {
			t.Fatalf("rendered layout missing %q:\n%s", want, kdl)
		}
	}
	// A layout is applied to a clientless session, whose viewport is the
	// backend's ~50x50 default — anything floated from here is sized against
	// that and comes up squished in a real terminal. Runtime spawns only.
	if strings.Contains(kdl, "floating_panes") {
		t.Fatalf("rendered layout floats a pane; geometry would resolve pre-attach:\n%s", kdl)
	}
	// The bar goes on top: qrouton's own strip pane owns the bottom row, and two
	// bars stacked there would cost a row of the repos column for nothing.
	if !strings.HasPrefix(kdl, "layout {\n"+kdlIndent+"pane size=1 borderless=true {\n") {
		t.Fatalf("status bar is not the layout's first pane:\n%s", kdl)
	}
}

// Attached is what keeps a runtime-spawned floating pane from being sized
// against the clientless server's default viewport. list-clients prints its
// column header whether or not anyone is looking, so only a row past it counts.
func TestAttachedReadsClientRowsNotTheHeader(t *testing.T) {
	for _, tc := range []struct {
		name, out string
		want      bool
	}{
		{"detached", "CLIENT_ID ZELLIJ_PANE_ID RUNNING_COMMAND\n", false},
		{"blank", "", false},
		{"attached", "CLIENT_ID ZELLIJ_PANE_ID RUNNING_COMMAND\n1         terminal_0     claude\n", true},
		{"trailing blank lines", "CLIENT_ID PANE\n\n  \n", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			bin := filepath.Join(dir, "zellij")
			script := "#!/bin/sh\nprintf '%s' " + shellQuote(tc.out) + "\n"
			if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
				t.Fatal(err)
			}
			got, err := NewZellijHost(bin, "s").Attached(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Fatalf("Attached() = %v, want %v for output %q", got, tc.want, tc.out)
			}
		})
	}
}

// shellQuote wraps s for the /bin/sh stub above.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// Start must create the session detached before attaching: handing the layout to a
// session we are attaching to at the same time races with zellij's startup and can
// come up with zellij's default layout instead of ours.
func TestCreateArgvCreatesDetachedWithTheLayout(t *testing.T) {
	argv := strings.Join(createArgv("/s/.qrouton/zellij-config.kdl", "/s/.qrouton/layout.kdl", "slug"), " ")
	for _, want := range []string{
		"--config /s/.qrouton/zellij-config.kdl",
		"--layout /s/.qrouton/layout.kdl",
		"attach --create-background slug",
	} {
		if !strings.Contains(argv, want) {
			t.Errorf("create argv %q missing %q", argv, want)
		}
	}
}
