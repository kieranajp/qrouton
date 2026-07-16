package evalharness

import (
	"context"
	"os"
	"os/exec"
	"testing"
	"time"
)

func TestClaudeSmoke(t *testing.T) {
	runnerSmokeTest(t, "claude")
}

func TestCodexSmoke(t *testing.T) {
	runnerSmokeTest(t, "codex")
}

func runnerSmokeTest(t *testing.T, runner string) {
	t.Helper()
	if os.Getenv("QROUTON_EVAL_SMOKE") != "1" {
		t.Skip("set QROUTON_EVAL_SMOKE=1 to run authenticated runner smoke tests")
	}
	bin, err := exec.LookPath(runner)
	if err != nil {
		t.Skipf("%s is not installed", runner)
	}
	self := buildEvalCommand(t)
	adapter := Adapter{Name: runner, Bin: bin, SelfPath: self}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	_, final, _, err := adapter.RunTurn(ctx, t.TempDir(), t.TempDir()+"/mcp.jsonl", "Reply with the word ok.", "", 1)
	if err != nil {
		t.Fatal(err)
	}
	if final == "" {
		t.Fatal("runner returned no final response")
	}
}

func buildEvalCommand(t *testing.T) string {
	t.Helper()
	output := t.TempDir() + "/qrouton-eval"
	command := exec.Command("go", "build", "-o", output, "../../cmd/qrouton-eval")
	if buildOutput, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build qrouton-eval: %v: %s", err, buildOutput)
	}
	return output
}
