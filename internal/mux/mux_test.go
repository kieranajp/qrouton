package mux

import (
	"context"
	"errors"
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
				{Stacked: true, Children: []Node{
					{Pane: &Pane{Name: "shell", Command: []string{"sh", "-lc", "echo hi"}}},
				}},
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
		`pane stacked=true {`,
		`pane size=6 name="status" {`,                // row counts render bare
		`pane size=1 borderless=true name="strip" {`, // borderless leaves render frameless
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

func TestShellStackJoinsCurrentPaneToExistingNumberedShells(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "zellij")
	log := filepath.Join(dir, "calls")
	panes := `[
		{"id":0,"is_plugin":true,"title":"status"},
		{"id":2,"title":"shell 1 · keys"},
		{"id":5,"title":"shell 3 · keys"},
		{"id":7,"title":"shell 8 · keys","is_floating":true},
		{"id":8,"title":"shell 9 · keys","exited":true},
		{"id":9,"title":"shell"}
	]`
	script := "#!/bin/sh\n" +
		"printf '%s\\n' \"$*\" >> \"$CALL_LOG\"\n" +
		"if [ \"$4\" = list-panes ]; then printf '%s' \"$PANES_JSON\"; fi\n"
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CALL_LOG", log)
	t.Setenv("PANES_JSON", panes)
	stack := &zellijShellStack{
		zellijHost: zellijHost{bin: bin, session: "s"},
		currentID:  "terminal_9",
	}

	number, err := stack.JoinCurrent(context.Background(), "shell", " · keys")
	if err != nil {
		t.Fatal(err)
	}
	if number != 4 {
		t.Fatalf("new shell number = %d, want 4", number)
	}
	count, err := stack.Count(context.Background(), "shell")
	if err != nil {
		t.Fatal(err)
	}
	if count != 3 {
		t.Fatalf("shell count = %d, want 3 active tiled managed shells", count)
	}
	calls, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	got := string(calls)
	for _, want := range []string{
		"action rename-pane --pane-id terminal_9 shell 4 · keys",
		"action stack-panes -- terminal_2 terminal_5 terminal_9",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("calls missing %q:\n%s", want, got)
		}
	}
}

func TestShellStackUsesThePreRefactorShellAsItsLiveSessionAnchor(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "zellij")
	log := filepath.Join(dir, "calls")
	panes := `[
		{"id":0,"title":"agent"},
		{"id":2,"title":"shell · Alt-g"},
		{"id":13,"title":"shell"}
	]`
	script := "#!/bin/sh\n" +
		"printf '%s\\n' \"$*\" >> \"$CALL_LOG\"\n" +
		"if [ \"$4\" = list-panes ]; then printf '%s' \"$PANES_JSON\"; fi\n"
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CALL_LOG", log)
	t.Setenv("PANES_JSON", panes)
	stack := &zellijShellStack{
		zellijHost: zellijHost{bin: bin, session: "multipane"},
		currentID:  "terminal_13",
	}

	number, err := stack.JoinCurrent(context.Background(), "shell", " · keys")
	if err != nil {
		t.Fatal(err)
	}
	if number != 2 {
		t.Fatalf("new shell number = %d, want 2 after the legacy shell", number)
	}
	calls, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(calls); !strings.Contains(got, "action stack-panes -- terminal_2 terminal_13") {
		t.Fatalf("legacy shell was not used as the stack anchor:\n%s", got)
	}
}

func TestShellNumberRecognizesManagedAndLegacyTitles(t *testing.T) {
	for _, tc := range []struct {
		title string
		want  int
		ok    bool
	}{
		{"shell 1 · keys", 1, true},
		{"shell 12", 12, true},
		{"shell", 1, true},
		{"shell · Alt-g", 1, true},
		{"shellfish 2", 0, false},
		{"shell nope", 0, false},
		{"shell 0", 0, false},
	} {
		got, ok := shellNumber(tc.title, "shell")
		if got != tc.want || ok != tc.ok {
			t.Errorf("shellNumber(%q) = (%d,%v), want (%d,%v)", tc.title, got, ok, tc.want, tc.ok)
		}
	}
}

func TestCurrentShellStackRequiresZellijPaneIdentity(t *testing.T) {
	t.Setenv(zellijSessionEnvVar, "")
	t.Setenv(zellijPaneIDEnvVar, "")
	if _, err := CurrentShellStack(); !errors.Is(err, ErrShellContext) {
		t.Fatalf("CurrentShellStack() error = %v, want ErrShellContext", err)
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

// loggingZellij writes a stub that logs every invocation to $CALL_LOG, echoes
// $PANES_JSON for list-panes, and fails any action named in $FAIL_ACTION.
func loggingZellij(t *testing.T, dir string) (bin, log string) {
	t.Helper()
	bin, log = filepath.Join(dir, "zellij"), filepath.Join(dir, "calls")
	script := "#!/bin/sh\n" +
		"printf '%s\\n' \"$*\" >> \"$CALL_LOG\"\n" +
		"if [ -n \"$FAIL_ACTION\" ] && [ \"$4\" = \"$FAIL_ACTION\" ]; then exit 1; fi\n" +
		"case \"$4\" in\n" +
		"  new-pane) echo terminal_4;;\n" +
		"  list-panes) printf '%s' \"$PANES_JSON\";;\n" +
		"esac\n"
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CALL_LOG", log)
	return bin, log
}

// Reposition is the repair for a floating pane whose percentages resolved
// against the wrong viewport; it must carry the same geometry vocabulary Spawn
// uses, and must not restate pinned or borderless.
func TestRepositionReappliesGeometryByPaneID(t *testing.T) {
	dir := t.TempDir()
	bin, log := loggingZellij(t, dir)
	geom := Geometry{X: "15%", Y: "8%", Width: "70%", Height: "80%"}

	if err := NewZellijHost(bin, "s").Reposition(context.Background(), "terminal_4", geom); err != nil {
		t.Fatal(err)
	}
	got := readCalls(t, log)
	want := "action change-floating-pane-coordinates --pane-id terminal_4 --x 15% --y 8% --width 70% --height 80%"
	if !strings.Contains(got, want) {
		t.Fatalf("calls missing %q:\n%s", want, got)
	}
	for _, forbidden := range []string{"--pinned", "--borderless"} {
		if strings.Contains(got, forbidden) {
			t.Fatalf("Reposition restated %s; the pane was created with the one it wants:\n%s", forbidden, got)
		}
	}
}

func TestExistsDistinguishesLiveExitedAndPluginPanes(t *testing.T) {
	panes := `[
		{"id":4,"title":"diff"},
		{"id":6,"title":"gone","exited":true},
		{"id":7,"is_plugin":true,"title":"status-bar"}
	]`
	for _, tc := range []struct {
		id   string
		want bool
	}{
		{"terminal_4", true},
		{"terminal_6", false}, // exited: the id is still listed, the pane is not usable
		{"terminal_7", false}, // plugin 7 is not terminal 7
		{"plugin_7", true},
		{"terminal_99", false},
	} {
		t.Run(tc.id, func(t *testing.T) {
			dir := t.TempDir()
			bin, _ := loggingZellij(t, dir)
			t.Setenv("PANES_JSON", panes)
			got, err := NewZellijHost(bin, "s").Exists(context.Background(), tc.id)
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Fatalf("Exists(%q) = %v, want %v", tc.id, got, tc.want)
			}
		})
	}
}

func TestFindStackAndFloatPaneByID(t *testing.T) {
	dir := t.TempDir()
	bin, log := loggingZellij(t, dir)
	t.Setenv("PANES_JSON", `[
		{"id":3,"title":"dock"},
		{"id":4,"title":"dock","exited":true},
		{"id":5,"title":"dock","is_plugin":true}
	]`)
	host := NewZellijHost(bin, "s")
	dockID, err := host.FindPane(context.Background(), "dock")
	if err != nil {
		t.Fatal(err)
	}
	if dockID != "terminal_3" {
		t.Fatalf("dock id = %q, want terminal_3", dockID)
	}
	if err := host.Stack(context.Background(), dockID, "terminal_9"); err != nil {
		t.Fatal(err)
	}
	if err := host.Float(context.Background(), "terminal_9"); err != nil {
		t.Fatal(err)
	}
	got := readCalls(t, log)
	for _, want := range []string{
		"action stack-panes -- terminal_3 terminal_9",
		"action toggle-pane-embed-or-floating --pane-id terminal_9",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("calls missing %q:\n%s", want, got)
		}
	}
}

func TestFindPaneReportsMissingTitle(t *testing.T) {
	dir := t.TempDir()
	bin, _ := loggingZellij(t, dir)
	t.Setenv("PANES_JSON", `[{"id":3,"title":"agents"}]`)
	if _, err := NewZellijHost(bin, "s").FindPane(context.Background(), "dock"); !errors.Is(err, ErrPaneNotFound) {
		t.Fatalf("FindPane error = %v, want ErrPaneNotFound", err)
	}
}

// Returning focus by naming the owner pane, not by flipping the floating layer:
// the flip depends on the layer's current state, so a user who had hidden it
// with Alt-f got it shown again by the agent opening a pane.
func TestSpawnReturnsFocusToTheOwningPaneByID(t *testing.T) {
	dir := t.TempDir()
	bin, log := loggingZellij(t, dir)
	t.Setenv(zellijPaneIDEnvVar, "1")

	if _, err := NewZellijHost(bin, "s").Spawn(context.Background(), SpawnOptions{Command: []string{"ls"}}); err != nil {
		t.Fatal(err)
	}
	got := readCalls(t, log)
	if !strings.Contains(got, "action focus-pane-id terminal_1") {
		t.Fatalf("spawn did not focus the owning pane by id:\n%s", got)
	}
	if strings.Contains(got, "toggle-floating-panes") {
		t.Fatalf("spawn still flipped the floating layer:\n%s", got)
	}
}

func TestSpawnRejectsEmptyPaneID(t *testing.T) {
	dir := t.TempDir()
	bin, log := filepath.Join(dir, "zellij"), filepath.Join(dir, "calls")
	script := "#!/bin/sh\nprintf '%s\\n' \"$*\" >> \"$CALL_LOG\"\n"
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CALL_LOG", log)

	if _, err := NewZellijHost(bin, "s").Spawn(context.Background(), SpawnOptions{Focus: true, Command: []string{"sleep", "30"}}); !errors.Is(err, ErrPaneIDUnavailable) {
		t.Fatalf("Spawn error = %v, want ErrPaneIDUnavailable", err)
	}
	if got := readCalls(t, log); !strings.Contains(got, "action new-pane") || !strings.Contains(got, "sleep 30") {
		t.Fatalf("new-pane action missing from log:\n%s", got)
	}
}

// Two fallbacks to the layer toggle: a driver built outside a pane has no owner
// to name, and a focus that fails should still hand the keyboard back.
func TestSpawnFallsBackToTheLayerToggleWithoutAUsableOwner(t *testing.T) {
	t.Run("no owning pane", func(t *testing.T) {
		dir := t.TempDir()
		bin, log := loggingZellij(t, dir)
		t.Setenv(zellijPaneIDEnvVar, "")
		if _, err := NewZellijHost(bin, "s").Spawn(context.Background(), SpawnOptions{Command: []string{"ls"}}); err != nil {
			t.Fatal(err)
		}
		got := readCalls(t, log)
		if !strings.Contains(got, "toggle-floating-panes") || strings.Contains(got, "focus-pane-id") {
			t.Fatalf("no-owner spawn should toggle the layer:\n%s", got)
		}
	})
	t.Run("focus fails", func(t *testing.T) {
		dir := t.TempDir()
		bin, log := loggingZellij(t, dir)
		t.Setenv(zellijPaneIDEnvVar, "1")
		t.Setenv("FAIL_ACTION", "focus-pane-id")
		if _, err := NewZellijHost(bin, "s").Spawn(context.Background(), SpawnOptions{Command: []string{"ls"}}); err != nil {
			t.Fatal(err)
		}
		got := readCalls(t, log)
		if !strings.Contains(got, "focus-pane-id") || !strings.Contains(got, "toggle-floating-panes") {
			t.Fatalf("a failed focus should fall back to the layer toggle:\n%s", got)
		}
	})
}

// Spawn must keep focus on the new pane when asked, touching neither route.
func TestSpawnWithFocusLeavesTheKeyboardOnTheNewPane(t *testing.T) {
	dir := t.TempDir()
	bin, log := loggingZellij(t, dir)
	t.Setenv(zellijPaneIDEnvVar, "1")

	if _, err := NewZellijHost(bin, "s").Spawn(context.Background(), SpawnOptions{Focus: true, Command: []string{"ls"}}); err != nil {
		t.Fatal(err)
	}
	got := readCalls(t, log)
	if strings.Contains(got, "focus-pane-id") || strings.Contains(got, "toggle-floating-panes") {
		t.Fatalf("focus=true spawn moved focus away from the new pane:\n%s", got)
	}
}

func readCalls(t *testing.T, log string) string {
	t.Helper()
	b, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
