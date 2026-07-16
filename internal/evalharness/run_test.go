package evalharness

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRunContinuesAfterMalformedCaseAndProducesReport(t *testing.T) {
	repoRoot := t.TempDir()
	assets := filepath.Join(repoRoot, "assets")
	writeTestFile(t, filepath.Join(assets, "AGENTS.md"), "# Instructions\n")
	writeRunFixture(t, repoRoot, "bad", "malformed")
	writeRunFixture(t, repoRoot, "good", "healthy")

	bin := filepath.Join(repoRoot, "fake-claude")
	writeTestFile(t, bin, `#!/bin/sh
if [ "$1" = "--version" ]; then
  echo "fake 1.0"
  exit 0
fi
case "$*" in
  *malformed*) echo "not json" ;;
  *)
    echo '{"type":"system","session_id":"fake-session"}'
    echo '{"type":"result","result":"completed"}'
    ;;
esac
`)
	if err := os.Chmod(bin, 0o755); err != nil {
		t.Fatal(err)
	}

	output := filepath.Join(repoRoot, "results")
	report, err := Run(context.Background(), Config{
		RepoRoot:  repoRoot,
		Runner:    "claude",
		Scenario:  "all",
		Samples:   1,
		AssetsDir: assets,
		NoJudge:   true,
		Timeout:   time.Second,
		Output:    output,
		ClaudeBin: bin,
		SelfPath:  bin,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Cases) != 2 {
		t.Fatalf("cases = %d, want 2", len(report.Cases))
	}
	if report.Cases[0].InfrastructureError == "" {
		t.Fatal("malformed case did not record infrastructure error")
	}
	if report.Cases[1].InfrastructureError != "" {
		t.Fatalf("healthy case failed: %s", report.Cases[1].InfrastructureError)
	}
	for _, path := range []string{"run.json", "report.md"} {
		if _, err := os.Stat(filepath.Join(output, path)); err != nil {
			t.Fatal(err)
		}
	}
}

func writeRunFixture(t *testing.T, root, id, prompt string) {
	t.Helper()
	scenario := `{
  "id": "` + id + `",
  "fixture": "` + id + `",
  "turns": ["` + prompt + `"],
  "rubric": "respond"
}`
	writeTestFile(t, filepath.Join(root, "eval", "scenarios", id+".json"), scenario)
	fixture := filepath.Join(root, "eval", "fixtures", id)
	writeTestFile(t, filepath.Join(fixture, "qrouton.json"), `{
  "repositories": [{"name":"app","role":"active"}]
}`)
	writeTestFile(t, filepath.Join(fixture, "src", "app", "README.md"), "fixture")
}

func TestSelectedAdaptersRejectsUnknownRunner(t *testing.T) {
	_, err := selectedAdapters(Config{Runner: "other"})
	if err == nil || !strings.Contains(err.Error(), "claude, codex, or all") {
		t.Fatalf("unexpected error: %v", err)
	}
}
