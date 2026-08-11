package onboard

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"net"
	"os"
	"strings"
	"testing"

	"github.com/kieranajp/qrouton/internal/config"
	"github.com/kieranajp/qrouton/internal/launch"
	"github.com/kieranajp/qrouton/internal/tui"
	"github.com/kieranajp/qrouton/internal/workbench"
)

// adoptions serves the control socket, recording the session roots it is told
// about. The real server is the desktop process, which cannot run headlessly.
func adoptions(t *testing.T) (socket string, roots func() []string) {
	t.Helper()
	socket, err := workbench.NewSocketPath()
	if err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close(); _ = os.Remove(socket) })

	seen := make(chan string, 4)
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			line, err := bufio.NewReader(conn).ReadBytes('\n')
			if err == nil {
				var req workbench.Request
				if json.Unmarshal(line, &req) == nil && req.Op == workbench.OpAdopt {
					seen <- req.Root
				}
				_, _ = conn.Write([]byte("{}\n"))
			}
			_ = conn.Close()
		}
	}()
	return socket, func() []string {
		var out []string
		for {
			select {
			case root := <-seen:
				out = append(out, root)
			default:
				return out
			}
		}
	}
}

// The handover keeps the terminal onboarding drew in: the supervisor argv it
// execs is the same one a directory or owner/repo launch would have run, and the
// workbench is told which session it is on before the process is gone.
func TestHandOverAdoptsTheSessionThenExecsTheSupervisor(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("CODEX_HOME", t.TempDir())
	t.Setenv("VISUAL", "/bin/cat")
	socket, roots := adoptions(t)
	dir := t.TempDir()

	var argv []string
	var adoptedFirst bool
	handover = func(execArgv, env []string) error {
		argv, adoptedFirst = execArgv, len(roots()) == 1
		if !containsPrefix(env, launch.EditorEnvVar+"=") {
			t.Errorf("the exec'd environment does not carry the resolved editor")
		}
		return nil
	}
	t.Cleanup(func() { handover = execHandover })

	request := tui.LaunchRequest{Dir: dir, Runner: launch.Runner{ID: "codex", Command: []string{"codex"}}, Resume: true}
	if err := handOver(context.Background(), &config.Config{}, socket, request); err != nil {
		t.Fatal(err)
	}

	if !adoptedFirst {
		t.Fatal("the supervisor replaced onboarding before the workbench adopted the session")
	}
	joined := strings.Join(argv, " ")
	for _, want := range []string{"agent", "--session-root " + dir, "--runner codex", "--workbench-json", "--resume"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("handover argv %q missing %q", joined, want)
		}
	}
}

// A workbench that never heard which session this is has no chrome and no user
// shell, so the handover stops rather than leaving half a session behind.
func TestHandOverStopsWhenTheWorkbenchCannotBeReached(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("CODEX_HOME", t.TempDir())
	t.Setenv("VISUAL", "/bin/cat")
	handover = func([]string, []string) error {
		t.Error("onboarding handed over without the workbench adopting the session")
		return nil
	}
	t.Cleanup(func() { handover = execHandover })

	request := tui.LaunchRequest{Dir: t.TempDir(), Runner: launch.Runner{ID: "codex", Command: []string{"codex"}}}
	err := handOver(context.Background(), &config.Config{}, "/tmp/qrouton-sock/nothing-listens.sock", request)
	if !errors.Is(err, errNotAdopted) {
		t.Fatalf("handOver error = %v, want it to report the session unadopted", err)
	}
}

func containsPrefix(items []string, prefix string) bool {
	for _, item := range items {
		if strings.HasPrefix(item, prefix) {
			return true
		}
	}
	return false
}
