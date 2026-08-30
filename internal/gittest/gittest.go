// Package gittest builds the throwaway repositories qrouton's tests assemble
// sessions from.
//
// Not a _test.go file, so the packages under test can import it.
package gittest

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// identity is the committer every test repository commits as. Passed per
// invocation rather than written into a config, so a repository built here
// commits the same way whatever the machine's global git config says — and
// gpgsign off, or a developer with a signing key cannot run the suite.
var identity = []string{
	"-c", "user.name=qrouton test",
	"-c", "user.email=test@qrouton.invalid",
	"-c", "commit.gpgsign=false",
}

// Run runs one git command in dir, failing the test with git's own output.
func Run(t *testing.T, dir string, args ...string) {
	t.Helper()
	if out, err := run(dir, args...); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func run(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", append(append([]string(nil), identity...), args...)...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// output is one git command's trimmed stdout.
func output(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := run(dir, args...)
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(out)
}

// options is how a repository differs from the bare minimum.
type options struct {
	files   map[string][]string
	message string
	fileURL bool
}

type Option func(*options)

// WithFile commits one file of the given lines. Repeatable.
func WithFile(name string, lines ...string) Option {
	return func(o *options) { o.files[name] = lines }
}

// WithMessage names the first commit, for a test that reads the log.
func WithMessage(message string) Option {
	return func(o *options) { o.message = message }
}

// AsFileURL answers a file:// URL rather than a path, so cloning it is a real
// transfer over a transport instead of a local shortcut.
func AsFileURL() Option {
	return func(o *options) { o.fileURL = true }
}

// Origin is a repository a session can mirror from: one commit on main, in a
// directory of its own under the test's temporary root. With no options the
// commit is empty, which is all a test needs to clone and branch.
func Origin(t *testing.T, name string, opts ...Option) string {
	t.Helper()
	o := options{files: map[string][]string{}, message: "initial"}
	for _, apply := range opts {
		apply(&o)
	}
	dir := filepath.Join(t.TempDir(), name)
	Run(t, "", "init", "-b", "main", dir)
	if len(o.files) == 0 {
		Run(t, dir, "commit", "--allow-empty", "-m", o.message)
	} else {
		for file, lines := range o.files {
			WriteFile(t, dir, file, lines...)
		}
		Run(t, dir, "add", ".")
		Run(t, dir, "commit", "-m", o.message)
	}
	if o.fileURL {
		return "file://" + dir
	}
	return dir
}

// Worktree is a checkout as one of a session's own src/<repo> directories looks:
// on main, with its base commit reachable as origin/main. That ref is what
// repository stats measure a session's work against, so a fixture without it
// reads as unmeasured rather than unchanged.
func Worktree(t *testing.T, path string, opts ...Option) string {
	t.Helper()
	o := options{files: map[string][]string{"README.md": {"base"}}, message: "base"}
	for _, apply := range opts {
		apply(&o)
	}
	if err := mkdirAll(path); err != nil {
		t.Fatal(err)
	}
	Run(t, "", "init", "-b", "main", path)
	for file, lines := range o.files {
		WriteFile(t, path, file, lines...)
	}
	Run(t, path, "add", ".")
	Run(t, path, "commit", "-m", o.message)
	Run(t, path, "update-ref", "refs/remotes/origin/main", "HEAD")
	return path
}

// Head is dir's current commit, in full.
func Head(t *testing.T, dir string) string {
	t.Helper()
	return output(t, dir, "rev-parse", "HEAD")
}
