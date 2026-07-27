package mux

import (
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
	ws := Workspace{Slug: "demo", Dir: dir, Tiled: Node{Pane: &Pane{Name: "agent", Command: []string{"sh"}}}}
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
	if strings.Contains(config, sessionDirPlaceholder) {
		t.Fatal("session-dir placeholder survived staging")
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
		Floating: []Floating{{
			Pane:     Pane{Name: "quick start", Command: []string{"sh", "/work/demo/.qrouton/help.sh"}, CloseOnExit: true, Focus: true},
			Geometry: Geometry{X: "27%", Y: "25%", Width: "46%", Height: "35%"},
		}},
	}
	kdl := renderKDL(ws)
	for _, want := range []string{
		`plugin location="zellij:tab-bar"`,
		`pane split_direction="vertical" {`,
		`pane split_direction="horizontal" size="35%" {`, // percentages stay quoted
		`pane size=6 name="status" {`,                    // row counts render bare
		`pane size=1 borderless=true name="strip" {`,     // borderless leaves render frameless
		`pane size="65%" name="agent" {`,
		"command \"claude\"\n",
		`args "--flag" "tricky \"quote\""`, // args survive KDL quoting
		`pane x="27%" y="25%" width="46%" height="35%" name="quick start" close_on_exit=true focus=true {`,
		"session_name \"demo\"\nattach_to_session true\n",
	} {
		if !strings.Contains(kdl, want) {
			t.Fatalf("rendered layout missing %q:\n%s", want, kdl)
		}
	}
	// Zellij's own status-bar advertises modes the vendored config deleted;
	// qrouton's strip pane owns the bottom row instead.
	if strings.Contains(kdl, "zellij:status-bar") {
		t.Fatalf("rendered layout still carries zellij:status-bar:\n%s", kdl)
	}
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
