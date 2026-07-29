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

func TestDismissModeFollowsTheOwningClientsFocusedPane(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "zellij")
	log := filepath.Join(dir, "calls")
	focus := filepath.Join(dir, "focus")
	script := "#!/bin/sh\n" +
		"printf '%s\\n' \"$*\" >> \"$CALL_LOG\"\n" +
		"if [ \"$4\" = list-clients ]; then\n" +
		"  printf 'CLIENT_ID ZELLIJ_PANE_ID RUNNING_COMMAND\\n'\n" +
		"  cat \"$FOCUS_FILE\"\n" +
		"fi\n"
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CALL_LOG", log)
	t.Setenv("FOCUS_FILE", focus)
	t.Setenv(zellijPaneIDEnvVar, "0")
	if err := os.WriteFile(focus, []byte("2 terminal_7 less\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	host := NewZellijHost(bin, "s").(*zellijHost)
	host.dismissible["terminal_7"] = struct{}{}
	if err := host.syncDismissMode(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(focus, []byte("2 terminal_0 codex\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := host.syncDismissMode(context.Background()); err != nil {
		t.Fatal(err)
	}
	// A second observation of the same pane must not overwrite a mode the user
	// deliberately entered with Ctrl-g.
	if err := host.syncDismissMode(context.Background()); err != nil {
		t.Fatal(err)
	}

	calls, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	got := string(calls)
	for _, want := range []string{"action switch-mode normal", "action switch-mode locked"} {
		if !strings.Contains(got, want) {
			t.Fatalf("calls missing %q:\n%s", want, got)
		}
	}
	if strings.Count(got, "action switch-mode locked") != 1 {
		t.Fatalf("unchanged focus redundantly reset locked mode:\n%s", got)
	}
}

func TestParseClientFocusIgnoresHeaderAndCommands(t *testing.T) {
	got := parseClientFocus("CLIENT_ID ZELLIJ_PANE_ID RUNNING_COMMAND\n2 terminal_0 codex --flag words\n7 terminal_9 vim\n")
	want := []clientFocus{{id: "2", paneID: "terminal_0"}, {id: "7", paneID: "terminal_9"}}
	if len(got) != len(want) {
		t.Fatalf("clients = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("client %d = %#v, want %#v", i, got[i], want[i])
		}
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
