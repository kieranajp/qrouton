package launch

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func stubCommand(t *testing.T, dir, name, body string) {
	t.Helper()
	script := "#!/bin/sh\n" + body + "\n"
	if err := os.WriteFile(filepath.Join(dir, name), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
}

// A capture named by the agent can carry a space or a quote, and a name the
// script interpolated would be two arguments or a broken one by the time it
// reached the converter.
func TestCopyImageArgvKeepsAnAwkwardNameWhole(t *testing.T) {
	bin := t.TempDir()
	log := filepath.Join(t.TempDir(), "log")
	stubCommand(t, bin, "sips", `printf '%s\n' "$4" >>`+log+`
: >"$6"`)
	stubCommand(t, bin, "osascript", `printf '%s\n' "$7" >>`+log)

	source := filepath.Join(t.TempDir(), `a "loud" shot.png`)
	if err := os.WriteFile(source, []byte("not really a png"), 0o644); err != nil {
		t.Fatal(err)
	}
	argv := CopyImageArgv(source)
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Env = append(os.Environ(), "PATH="+bin+":"+os.Getenv("PATH"))
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("copy failed: %v\n%s", err, out)
	}

	seen, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(seen)), "\n")
	if len(lines) != 2 {
		t.Fatalf("the two commands logged %q, want one path each", seen)
	}
	if lines[0] != source {
		t.Fatalf("converted %q, want the file as named %q", lines[0], source)
	}
}

// Nothing is left behind for the next copy to find, and nothing that ran is
// holding the file open by the time the clipboard is read.
func TestCopyImageRemovesWhatItConverted(t *testing.T) {
	bin := t.TempDir()
	log := filepath.Join(t.TempDir(), "log")
	stubCommand(t, bin, "sips", `: >"$6"`)
	stubCommand(t, bin, "osascript", `printf '%s\n' "$7" >>`+log)

	source := filepath.Join(t.TempDir(), "shot.png")
	if err := os.WriteFile(source, []byte("not really a png"), 0o644); err != nil {
		t.Fatal(err)
	}
	argv := CopyImageArgv(source)
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Env = append(os.Environ(), "PATH="+bin+":"+os.Getenv("PATH"))
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("copy failed: %v\n%s", err, out)
	}

	converted, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	png := strings.TrimSpace(string(converted))
	if png == "" {
		t.Fatal("the clipboard was set from nothing")
	}
	if _, err := os.Stat(filepath.Dir(png)); !os.IsNotExist(err) {
		t.Fatalf("the temporary directory %q outlived the copy", filepath.Dir(png))
	}
}

// A converter that failed leaves the clipboard alone rather than putting an
// empty picture on it, and the page is told.
func TestCopyImageStopsWhenTheConversionFails(t *testing.T) {
	bin := t.TempDir()
	log := filepath.Join(t.TempDir(), "log")
	stubCommand(t, bin, "sips", "exit 1")
	stubCommand(t, bin, "osascript", `printf 'ran\n' >>`+log)

	argv := CopyImageArgv(filepath.Join(t.TempDir(), "shot.png"))
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Env = append(os.Environ(), "PATH="+bin+":"+os.Getenv("PATH"))
	if err := cmd.Run(); err == nil {
		t.Fatal("a failed conversion reported success")
	}
	if _, err := os.Stat(log); !os.IsNotExist(err) {
		t.Fatal("the clipboard was set from a conversion that failed")
	}
}
