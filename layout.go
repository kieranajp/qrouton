package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

// Panels (S001 #8): the multi-panel workspace is a generated multiplexer layout, not a bespoke TUI.
// zellij preferred, tmux fallback, plain exec if neither. QROUTON_PLAIN=1 forces plain.

const statusScript = `#!/bin/sh
# qrouton: live per-repo branch + status (generated; regenerated at every launch)
cd "$(dirname "$0")/.." || exit 1
while :; do
  clear
  for g in */.git; do
    [ -e "$g" ] || continue
    r=${g%/.git}
    printf '\033[1m%s\033[0m  (%s)\n' "$r" "$(git -C "$r" branch --show-current)"
    git -C "$r" status -s | head -6
    echo
  done
  sleep 3
done
`

// writeSupport writes .qrouton/{status.sh,layout.kdl} into the session dir at launch time,
// so old sessions pick up template changes on resume.
func writeSupport(dir, slug string, argv []string) (string, error) {
	cd := filepath.Join(dir, ".qrouton")
	if err := os.MkdirAll(cd, 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(cd, "status.sh"), []byte(statusScript), 0o755); err != nil {
		return "", err
	}

	var args string
	if len(argv) > 1 {
		quoted := make([]string, len(argv)-1)
		for i, a := range argv[1:] {
			quoted[i] = fmt.Sprintf("%q", a)
		}
		args = "\n            args " + strings.Join(quoted, " ")
	}
	kdl := fmt.Sprintf(`layout {
    pane size=1 borderless=true {
        plugin location="zellij:tab-bar"
    }
    pane split_direction="vertical" {
        pane size="65%%" name="agent" {
            command %q%s
        }
        pane split_direction="horizontal" size="35%%" {
            pane name="shell"
            pane name="repos" command="sh" {
                args ".qrouton/status.sh"
            }
        }
    }
    pane size=2 borderless=true {
        plugin location="zellij:status-bar"
    }
}
session_name %q
attach_to_session true
`, argv[0], args, slug)
	lp := filepath.Join(cd, "layout.kdl")
	return lp, os.WriteFile(lp, []byte(kdl), 0o644)
}

func execv(bin string, argv []string, dir string) error {
	if err := os.Chdir(dir); err != nil {
		return err
	}
	return syscall.Exec(bin, argv, os.Environ())
}

func launchZellij(bin, dir string, argv []string) error {
	slug := filepath.Base(dir)
	// macOS $TMPDIR is long enough that zellij's socket path ($TMPDIR/zellij-<uid>/…/<session>)
	// exceeds the 104-byte unix-socket cap for real session names. Pin it somewhere short.
	if os.Getenv("ZELLIJ_SOCKET_DIR") == "" {
		os.Setenv("ZELLIJ_SOCKET_DIR", "/tmp/zellij")
	}
	if out, err := exec.Command(bin, "list-sessions", "-s").Output(); err == nil {
		for _, l := range strings.Split(string(out), "\n") {
			if strings.TrimSpace(l) == slug {
				return execv(bin, []string{"zellij", "attach", slug}, dir)
			}
		}
	}
	lp, err := writeSupport(dir, slug, argv)
	if err != nil {
		return err
	}
	// 0.44: -s + -n conflict; the session is named via session_name in the layout itself
	return execv(bin, []string{"zellij", "--new-session-with-layout", lp}, dir)
}

func launchTmux(bin, dir string, argv []string) error {
	slug := filepath.Base(dir)
	if exec.Command(bin, "has-session", "-t", "="+slug).Run() == nil {
		return execv(bin, []string{"tmux", "attach", "-t", "=" + slug}, dir)
	}
	if _, err := writeSupport(dir, slug, argv); err != nil {
		return err
	}
	// argv becomes a shell command string — quote each word or a spaced arg (the initial prompt) splits
	quoted := make([]string, len(argv))
	for i, a := range argv {
		quoted[i] = fmt.Sprintf("%q", a)
	}
	return execv(bin, []string{"tmux",
		"new-session", "-s", slug, "-c", dir, strings.Join(quoted, " "),
		";", "split-window", "-h", "-l", "35%", "-c", dir,
		";", "split-window", "-v", "-c", dir, "sh .qrouton/status.sh",
		";", "select-pane", "-L"}, dir)
}
