package userscope

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

// A dropped --expand-environment=no would let systemd rewrite $ in run_command
// text; a dropped --quiet would print into every terminal.
func TestWrapPrefixIsExact(t *testing.T) {
	got := Wrap([]string{"sh", "-c", "echo $HOME"}, "qrouton-tab-1.scope", "A=1")
	want := []string{"systemd-run", "--user", "--scope", "--quiet", "--expand-environment=no", "--collect",
		"--unit=qrouton-tab-1.scope", "-p", "A=1", "--", "sh", "-c", "echo $HOME"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Wrap = %q, want %q", got, want)
	}
}

// A machine without a user manager must keep starting children unwrapped.
func TestAvailableIsFalseWhenProbeFails(t *testing.T) {
	saved := run
	t.Cleanup(func() { run = saved })
	var probed []string
	run = func(_ context.Context, argv []string) error {
		probed = argv
		return errors.New("no user bus")
	}
	if Available() {
		t.Fatal("Available() = true after a failed probe")
	}
	if !strings.Contains(strings.Join(probed, " "), "--expand-environment=no --collect -- true") {
		t.Fatalf("probe argv = %q, want Wrap's flags", probed)
	}
}
