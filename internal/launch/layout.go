package launch

import (
	"bufio"
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/charmbracelet/huh"
)

// Panels are an opinionated Zellij workspace rather than a bespoke TUI. The shell
// scripts that drive its panes live under scripts/ and are embedded here so they read
// and edit as real scripts; each is written into .qrouton at launch (or, for shellIntro
// and codexDepthWarning, spliced into the generated layout).

// statusScript renders the live per-repo branch/status pane.
//
//go:embed scripts/status.sh
var statusScript string

// shellIntro greets the shell pane with a shallow tree, then execs an interactive login shell.
//
//go:embed scripts/shell-intro.sh
var shellIntro string

// notifyScript plays a short cross-platform attention sound; it backs both the notify
// MCP tool and the runner's Notification hook. See scripts/notify.sh for the fallbacks.
//
//go:embed scripts/notify.sh
var notifyScript string

// helpScript is the quick-start panel; @@WARNING@@ is replaced with codexDepthWarning or "".
//
//go:embed scripts/help.sh
var helpScript string

// codexDepthWarning warns when Codex's subagent nesting is too shallow.
//
//go:embed scripts/codex-warning.sh
var codexDepthWarning string

// writeSupport writes .qrouton/{status.sh,layout.kdl,zellij-config.kdl} at launch time,
// so old sessions pick up template changes on resume.
func writeSupport(dir, slug string, argv []string) (string, error) {
	cd := filepath.Join(dir, ".qrouton")
	if err := os.MkdirAll(cd, 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(cd, "status.sh"), []byte(statusScript), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(cd, "notify.sh"), []byte(notifyScript), 0o755); err != nil {
		return "", err
	}
	warning := ""
	if filepath.Base(argv[0]) == "codex" && codexMaxDepth(argv) < 2 {
		warning = strings.TrimRight(codexDepthWarning, "\n")
	}
	help := strings.ReplaceAll(helpScript, "@@WARNING@@", warning)
	if err := os.WriteFile(filepath.Join(cd, "help.sh"), []byte(help), 0o755); err != nil {
		return "", err
	}
	qroutonBin, err := os.Executable()
	if err != nil {
		return "", err
	}
	config, err := assetsFS.ReadFile("assets/zellij-config.kdl")
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(cd, "zellij-config.kdl"), config, 0o644); err != nil {
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
	runner := filepath.Base(argv[0])
	kdl := fmt.Sprintf(`layout {
    pane size=1 borderless=true {
        plugin location="zellij:tab-bar"
    }
    pane split_direction="vertical" {
        pane size="65%%" name="agent" {
            command %q%s
        }
		pane split_direction="horizontal" size="35%%" {
			pane name="shell" command="sh" {
				args "-lc" %q
			}
			pane split_direction="vertical" size=6 {
				pane name="repos" command="sh" {
					args %q
				}
				pane name="agents" command=%q {
					args "agents" "--session-root" %q "--runner" %q
				}
			}
        }
    }
    pane size=2 borderless=true {
        plugin location="zellij:status-bar"
    }
    floating_panes {
        pane x="27%%" y="25%%" width="46%%" height="35%%" name="qrouton · quick start" command="sh" close_on_exit=true {
            args %q
        }
    }
}
session_name %q
attach_to_session true
`, argv[0], args, strings.TrimSpace(shellIntro), filepath.Join(cd, "status.sh"), qroutonBin, dir, runner, filepath.Join(cd, "help.sh"), slug)
	lp := filepath.Join(cd, "layout.kdl")
	return lp, os.WriteFile(lp, []byte(kdl), 0o644)
}

// codexMaxDepth returns the configured nesting depth, or Codex's default of one.
// Command-line overrides win over the base config, matching Codex's precedence.
func codexMaxDepth(argv []string) int {
	depth := 1
	home := os.Getenv("CODEX_HOME")
	if home == "" {
		if userHome, err := os.UserHomeDir(); err == nil {
			home = filepath.Join(userHome, ".codex")
		}
	}
	if f, err := os.Open(filepath.Join(home, "config.toml")); err == nil {
		defer f.Close()
		section := ""
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := strings.TrimSpace(strings.SplitN(scanner.Text(), "#", 2)[0])
			if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
				section = strings.TrimSpace(line[1 : len(line)-1])
				continue
			}
			key, value, ok := strings.Cut(line, "=")
			if !ok {
				continue
			}
			key = strings.TrimSpace(key)
			if (section == "agents" && key == "max_depth") || (section == "" && key == "agents.max_depth") {
				if n, err := strconv.Atoi(strings.TrimSpace(value)); err == nil {
					depth = n
				}
			}
		}
	}
	for i := 1; i < len(argv); i++ {
		var override string
		switch {
		case argv[i] == "-c" || argv[i] == "--config":
			if i+1 < len(argv) {
				i++
				override = argv[i]
			}
		case strings.HasPrefix(argv[i], "--config="):
			override = strings.TrimPrefix(argv[i], "--config=")
		}
		if value, ok := strings.CutPrefix(override, "agents.max_depth="); ok {
			if n, err := strconv.Atoi(strings.TrimSpace(value)); err == nil {
				depth = n
			}
		}
	}
	return depth
}

func execv(bin string, argv []string, dir string) error {
	return execvEnv(bin, argv, dir, os.Environ())
}

func execvEnv(bin string, argv []string, dir string, env []string) error {
	if err := os.Chdir(dir); err != nil {
		return err
	}
	return syscall.Exec(bin, argv, env)
}

func Zellij(dir string, runner Runner, qroutonBin string, editor EditorCommand, resume bool) error {
	bin, err := exec.LookPath("zellij")
	if err != nil {
		return fmt.Errorf("zellij 0.44 or newer is required; install Zellij and try again")
	}
	if err := requireZellij044(bin); err != nil {
		return err
	}
	socketDir := os.Getenv("ZELLIJ_SOCKET_DIR")
	if socketDir == "" {
		socketDir = "/tmp/zellij"
	}
	argv, env, err := runnerLaunch(runner, qroutonBin, dir, editor, socketDir, resume)
	if err != nil {
		return err
	}
	env = withEnv(env, "QROUTON_EDITOR_JSON", editor.Marshal())
	slug := filepath.Base(dir)
	lp, err := writeSupport(dir, slug, argv)
	if err != nil {
		return err
	}
	// macOS $TMPDIR is long enough that zellij's socket path ($TMPDIR/zellij-<uid>/…/<session>)
	// exceeds the 104-byte unix-socket cap for real session names. Pin it somewhere short.
	env = withEnv(env, "ZELLIJ_SOCKET_DIR", socketDir)
	os.Setenv("ZELLIJ_SOCKET_DIR", socketDir) // discovery commands below use this socket too
	if out, err := exec.Command(bin, "list-sessions", "-n").Output(); err == nil {
		for _, l := range strings.Split(string(out), "\n") {
			if f := strings.Fields(l); len(f) > 0 && f[0] == slug {
				if !strings.Contains(l, "EXITED") {
					attach, err := chooseExistingSession(filepath.Base(argv[0]))
					if err != nil {
						return err
					}
					if attach {
						config := filepath.Join(dir, ".qrouton", "zellij-config.kdl")
						return execvEnv(bin, []string{"zellij", "--config", config, "attach", slug}, dir, env)
					}
					if err := exec.Command(bin, "delete-session", "--force", slug).Run(); err != nil {
						return err
					}
					break
				}
				// dead session: attach would resurrect zellij's *recorded* state (stale layout,
				// old paths) instead of applying the freshly-stamped one — delete and recreate
				exec.Command(bin, "delete-session", slug).Run()
				break
			}
		}
	}
	// 0.44: -s + -n conflict; the session is named via session_name in the layout itself
	config := filepath.Join(dir, ".qrouton", "zellij-config.kdl")
	return execvEnv(bin, []string{"zellij", "--config", config, "--new-session-with-layout", lp}, dir, env)
}

func requireZellij044(bin string) error {
	out, err := exec.Command(bin, "--version").Output()
	if err != nil {
		return fmt.Errorf("check zellij version: %w", err)
	}
	parts := strings.Split(strings.TrimSpace(string(out)), " ")
	if len(parts) != 2 {
		return fmt.Errorf("unrecognized zellij version %q", strings.TrimSpace(string(out)))
	}
	nums := strings.Split(parts[1], ".")
	major, _ := strconv.Atoi(nums[0])
	minor := 0
	if len(nums) > 1 {
		minor, _ = strconv.Atoi(nums[1])
	}
	if major == 0 && minor < 44 {
		return fmt.Errorf("zellij 0.44 or newer is required (found %s)", parts[1])
	}
	return nil
}

func chooseExistingSession(runner string) (bool, error) {
	action := "attach"
	err := huh.NewSelect[string]().Title("Workspace is already running").
		Description("Attach to its current agent, or restart all workspace panes with "+runner+".").
		Options(
			huh.NewOption("Attach existing workspace", "attach"),
			huh.NewOption("Restart workspace", "restart"),
		).Value(&action).Run()
	return action == "attach", err
}
