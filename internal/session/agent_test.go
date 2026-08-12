package session

import (
	"os"
	"os/exec"
	"strconv"
	"testing"

	"github.com/kieranajp/qrouton/internal/sessionpaths"
)

func TestAgentAliveIgnoresAnUnusablePIDFile(t *testing.T) {
	root := t.TempDir()
	if pid, alive := AgentAlive(root); pid != 0 || alive {
		t.Fatalf("AgentAlive with no pid file = %d, %v; want 0, false", pid, alive)
	}
	for _, body := range []string{"not a pid\n", "0\n", "-1\n", ""} {
		writeAgentPIDTest(t, root, body)
		if pid, alive := AgentAlive(root); pid != 0 || alive {
			t.Fatalf("AgentAlive on pid file %q = %d, %v; want 0, false", body, pid, alive)
		}
	}
}

func TestAgentAliveSeesThisProcess(t *testing.T) {
	root := t.TempDir()
	writeAgentPIDTest(t, root, strconv.Itoa(os.Getpid())+"\n")
	pid, alive := AgentAlive(root)
	if pid != os.Getpid() || !alive {
		t.Fatalf("AgentAlive on its own pid = %d, %v; want %d, true — a running agent read as dead lets a second one launch over it", pid, alive, os.Getpid())
	}
}

func TestAgentAliveReportsAReapedPIDDead(t *testing.T) {
	root := t.TempDir()
	cmd := exec.Command("/bin/sh", "-c", "exit 0")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Wait(); err != nil {
		t.Fatal(err)
	}
	reaped := cmd.Process.Pid
	writeAgentPIDTest(t, root, strconv.Itoa(reaped))
	pid, alive := AgentAlive(root)
	if pid != reaped || alive {
		t.Fatalf("AgentAlive on reaped pid %d = %d, %v; want %d, false — a stale pid read as alive strands the session, because nothing is left to signal and it can never be booted again", reaped, pid, alive, reaped)
	}
}

func writeAgentPIDTest(t *testing.T, root, body string) {
	t.Helper()
	if err := os.MkdirAll(sessionpaths.Dir(root), dirMode); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sessionpaths.AgentPID(root), []byte(body), fileMode); err != nil {
		t.Fatal(err)
	}
}
