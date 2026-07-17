package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/kieranajp/qrouton/internal/evalharness"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "qrouton-eval:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) > 0 && args[0] == "mock-mcp" {
		return runMockMCP(args[1:])
	}
	if len(args) > 0 && args[0] == "compare" {
		return runCompare(args[1:])
	}

	repoRoot, err := findRepoRoot()
	if err != nil {
		return err
	}
	selfPath, err := os.Executable()
	if err != nil {
		return err
	}

	flags := flag.NewFlagSet("qrouton-eval", flag.ContinueOnError)
	runner := flags.String("runner", "all", "runner: claude, codex, or all")
	scenario := flags.String("scenario", "all", "scenario ID or all")
	samples := flags.Int("samples", 1, "samples per runner and scenario")
	assetsDir := flags.String("assets-dir", filepath.Join(repoRoot, "prompts"), "prompt source directory")
	claudeModel := flags.String("claude-model", "", "Claude model override")
	codexModel := flags.String("codex-model", "", "Codex model override")
	noJudge := flags.Bool("no-judge", false, "skip blinded pairwise judging")
	timeout := flags.Duration("timeout", 10*time.Minute, "timeout per scenario run")
	output := flags.String("output", defaultOutput(repoRoot), "result directory")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}

	config := evalharness.Config{
		RepoRoot:    repoRoot,
		Runner:      *runner,
		Scenario:    *scenario,
		Samples:     *samples,
		AssetsDir:   *assetsDir,
		ClaudeModel: *claudeModel,
		CodexModel:  *codexModel,
		NoJudge:     *noJudge,
		Timeout:     *timeout,
		Output:      *output,
		ClaudeBin:   envOrDefault("QROUTON_EVAL_CLAUDE_BIN", "claude"),
		CodexBin:    envOrDefault("QROUTON_EVAL_CODEX_BIN", "codex"),
		SelfPath:    selfPath,
	}
	report, err := evalharness.Run(context.Background(), config)
	if err != nil {
		return err
	}
	fmt.Printf("Wrote %d cases to %s\n", len(report.Cases), *output)
	return nil
}

func runMockMCP(args []string) error {
	flags := flag.NewFlagSet("mock-mcp", flag.ContinueOnError)
	root := flags.String("root", "", "workspace root")
	logPath := flags.String("log", "", "JSONL event log")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if *root == "" || *logPath == "" {
		return fmt.Errorf("mock-mcp requires --root and --log")
	}
	return evalharness.RunMockMCP(context.Background(), *root, *logPath)
}

func runCompare(args []string) error {
	flags := flag.NewFlagSet("compare", flag.ContinueOnError)
	output := flags.String("output", "", "optional Markdown output path")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if flags.NArg() != 2 {
		return fmt.Errorf("usage: qrouton-eval compare [--output path] LEFT RIGHT")
	}
	markdown, err := evalharness.Compare(flags.Arg(0), flags.Arg(1), *output)
	if err != nil {
		return err
	}
	fmt.Print(markdown)
	return nil
}

func findRepoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		module, err := os.ReadFile(filepath.Join(dir, "go.mod"))
		if err == nil && strings.Contains(string(module), "github.com/kieranajp/qrouton") {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("run from inside the qrouton repository")
		}
		dir = parent
	}
}

func defaultOutput(repoRoot string) string {
	stamp := time.Now().Format("20060102-150405")
	return filepath.Join(repoRoot, "eval", "results", stamp)
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
