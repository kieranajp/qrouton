package launch

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

const shellHelperEnv = "QROUTON_TEST_SHELL_HELPER"

// Catches a shell started without -l, without the /bin/sh fallback, or outside
// the session root.
func TestShellExecsTheLoginShellInTheSessionRoot(t *testing.T) {
	originalTree, originalExec := showShellTree, execve
	t.Cleanup(func() { showShellTree, execve = originalTree, originalExec })
	showShellTree = func(string) {}

	for _, tc := range []struct{ name, env, wantShell string }{
		{name: "bare name from SHELL", env: "sh", wantShell: "sh"},
		{name: "fallback", env: "", wantShell: defaultShell},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Chdir(t.TempDir())
			t.Setenv(shellEnvVar, tc.env)
			root, err := filepath.EvalSymlinks(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			var gotPath, gotDir string
			var gotArgv []string
			execve = func(path string, argv, _ []string) error {
				gotPath, gotArgv = path, argv
				gotDir, _ = os.Getwd()
				return nil
			}

			if err := Shell(root); err != nil {
				t.Fatalf("Shell returned %v", err)
			}
			wantPath, _ := exec.LookPath(tc.wantShell)
			if gotPath != wantPath {
				t.Errorf("exec path = %q, want %q", gotPath, wantPath)
			}
			if want := []string{tc.wantShell, loginShellFlag}; !slices.Equal(gotArgv, want) {
				t.Errorf("argv = %q, want %q", gotArgv, want)
			}
			if gotDir != root {
				t.Errorf("working directory = %q, want %q", gotDir, root)
			}
		})
	}
}

// Catches a return to running the login shell as a child, which would make the
// tab's pid differ from the shell's and every foreground read look busy.
func TestShellReplacesItselfWithTheLoginShell(t *testing.T) {
	if dir := os.Getenv(shellHelperEnv); dir != "" {
		showShellTree = func(string) {}
		err := Shell(dir)
		t.Fatalf("Shell returned %v", err)
	}

	cmd := exec.Command(os.Args[0], "-test.run=^TestShellReplacesItselfWithTheLoginShell$")
	cmd.Env = append(os.Environ(), shellHelperEnv+"="+t.TempDir(), shellEnvVar+"=/bin/sh")
	cmd.Stdin = strings.NewReader("echo pid=$$; exit 3\n")
	out, err := cmd.Output()
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 3 {
		t.Fatalf("helper ended with %v, want exit status 3; output %q", err, out)
	}
	if want := "pid=" + strconv.Itoa(cmd.Process.Pid) + "\n"; !strings.Contains(string(out), want) {
		t.Fatalf("output %q does not contain %q", out, want)
	}
}
