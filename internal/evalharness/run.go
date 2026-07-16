package evalharness

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

func Run(ctx context.Context, config Config) (Report, error) {
	output, err := filepath.Abs(config.Output)
	if err != nil {
		return Report{}, fmt.Errorf("resolve output path: %w", err)
	}
	config.Output = output

	scenariosDir := filepath.Join(config.RepoRoot, "eval", "scenarios")
	scenarios, err := LoadScenarios(scenariosDir, config.Scenario)
	if err != nil {
		return Report{}, err
	}

	runners, err := selectedAdapters(config)
	if err != nil {
		return Report{}, err
	}
	if config.Samples < 1 {
		return Report{}, fmt.Errorf("samples must be at least 1")
	}
	if err := os.MkdirAll(config.Output, 0o755); err != nil {
		return Report{}, err
	}

	report := Report{
		Metadata: buildMetadata(ctx, config, runners),
	}
	for _, scenario := range scenarios {
		for _, adapter := range runners {
			for sample := 1; sample <= config.Samples; sample++ {
				result := runCase(ctx, config, scenario, adapter, sample)
				report.Cases = append(report.Cases, result)
			}
		}
	}
	if !config.NoJudge {
		if len(runners) != 2 {
			report.Warnings = append(report.Warnings, "pairwise judging requires --runner all")
		} else {
			report.Pairwise = JudgePairs(ctx, config, scenarios, report.Cases, runners)
		}
	}
	for _, result := range report.Cases {
		if result.Model != "" {
			report.Metadata.Models[result.Runner] = result.Model
		}
	}

	if err := WriteReport(config.Output, report); err != nil {
		return Report{}, err
	}
	return report, nil
}

func selectedAdapters(config Config) ([]Adapter, error) {
	adapters := map[string]Adapter{
		"claude": {
			Name:     "claude",
			Bin:      config.ClaudeBin,
			Model:    config.ClaudeModel,
			SelfPath: config.SelfPath,
		},
		"codex": {
			Name:     "codex",
			Bin:      config.CodexBin,
			Model:    config.CodexModel,
			SelfPath: config.SelfPath,
		},
	}

	var names []string
	switch config.Runner {
	case "", "all":
		names = []string{"claude", "codex"}
	case "claude", "codex":
		names = []string{config.Runner}
	default:
		return nil, fmt.Errorf("runner must be claude, codex, or all")
	}

	selected := make([]Adapter, 0, len(names))
	for _, name := range names {
		adapter := adapters[name]
		if adapter.Bin == "" {
			adapter.Bin = name
		}
		if _, err := exec.LookPath(adapter.Bin); err != nil {
			return nil, fmt.Errorf("runner %s is unavailable: %w", name, err)
		}
		selected = append(selected, adapter)
	}
	return selected, nil
}

func buildMetadata(ctx context.Context, config Config, adapters []Adapter) Metadata {
	versions := make(map[string]string, len(adapters))
	models := make(map[string]string, len(adapters))
	for _, adapter := range adapters {
		versions[adapter.Name] = adapter.Version(ctx)
		models[adapter.Name] = adapter.Model
	}

	assetHash, _ := HashTree(config.AssetsDir)
	judgeMode := "pairwise"
	if config.NoJudge {
		judgeMode = "none"
	}
	gitSHA, err := commandOutput(ctx, config.RepoRoot, "git", "rev-parse", "HEAD")
	if err != nil {
		gitSHA = "" // omitted from the report rather than recording git's error text
	}
	return Metadata{
		CreatedAt:       time.Now().UTC(),
		AssetHash:       assetHash,
		GitSHA:          gitSHA,
		CLIVersions:     versions,
		Models:          models,
		ScenarioVersion: ScenarioVersion,
		Invocation: map[string]any{
			"runner":     config.Runner,
			"scenario":   config.Scenario,
			"samples":    config.Samples,
			"assets_dir": config.AssetsDir,
			"no_judge":   config.NoJudge,
			"judge_mode": judgeMode,
			"timeout":    config.Timeout.String(),
		},
	}
}

func runCase(
	parent context.Context,
	config Config,
	scenario Scenario,
	adapter Adapter,
	sample int,
) (result CaseResult) {
	started := time.Now()
	caseID := fmt.Sprintf("%s-%s-%d", scenario.ID, adapter.Name, sample)
	result = CaseResult{
		ID:              caseID,
		ScenarioID:      scenario.ID,
		ScenarioVersion: scenario.Version,
		Runner:          adapter.Name,
		Model:           adapter.Model,
		Sample:          sample,
		StartedAt:       started.UTC(),
		Diffs:           make(map[string]string),
	}
	defer func() {
		result.DurationMS = time.Since(started).Milliseconds()
	}()

	caseDir := filepath.Join(config.Output, "cases", caseID)
	workspace := filepath.Join(caseDir, "workspace")
	fixture := filepath.Join(config.RepoRoot, "eval", "fixtures", scenario.Fixture)
	if err := CopyTree(fixture, workspace); err != nil {
		result.InfrastructureError = fmt.Sprintf("materialize fixture: %v", err)
		return result
	}

	snapshot := filepath.Join(caseDir, "prompt-snapshot")
	assetHash, err := MaterializeAssets(config.AssetsDir, workspace, snapshot)
	if err != nil {
		result.InfrastructureError = fmt.Sprintf("materialize assets: %v", err)
		return result
	}
	if err := makeTreeReadOnly(snapshot); err != nil {
		result.InfrastructureError = fmt.Sprintf("protect prompt snapshot: %v", err)
		return result
	}
	if err := os.WriteFile(filepath.Join(caseDir, "asset-sha256.txt"), []byte(assetHash+"\n"), 0o644); err != nil {
		result.InfrastructureError = fmt.Sprintf("write asset hash: %v", err)
		return result
	}

	baselines, err := initializeRepositories(workspace)
	if err != nil {
		result.InfrastructureError = fmt.Sprintf("initialize fixture repositories: %v", err)
		return result
	}

	caseCtx, cancel := context.WithTimeout(parent, config.Timeout)
	defer cancel()
	mcpLog := filepath.Join(caseDir, "mcp.jsonl")
	for turnIndex, prompt := range scenario.Turns {
		turn := turnIndex + 1
		result.Events = append(result.Events, Event{
			Time: time.Now().UTC(),
			Kind: "user",
			Turn: turn,
			Role: "user",
			Text: prompt,
		})

		mcpOffset := len(ReadMCPEvents(mcpLog, turn))
		events, final, session, runErr := adapter.RunTurn(
			caseCtx,
			workspace,
			mcpLog,
			prompt,
			result.SessionID,
			turn,
		)
		result.Events = append(result.Events, events...)
		if result.Model == "" {
			result.Model = detectedModel(events)
		}
		mcpEvents := ReadMCPEvents(mcpLog, turn)
		if mcpOffset < len(mcpEvents) {
			result.Events = append(result.Events, mcpEvents[mcpOffset:]...)
		}
		result.FinalResponse = final
		result.SessionID = session
		if runErr != nil {
			result.InfrastructureError = runErr.Error()
			break
		}
	}

	result.Artifacts, err = collectArtifacts(workspace)
	if err != nil && result.InfrastructureError == "" {
		result.InfrastructureError = fmt.Sprintf("collect artifacts: %v", err)
	}
	result.Diffs = collectDiffs(workspace, baselines)
	result.Assertions = Grade(scenario, result, workspace, baselines)

	result.DurationMS = time.Since(started).Milliseconds()
	if err := writeCaseFiles(caseDir, result); err != nil && result.InfrastructureError == "" {
		result.InfrastructureError = fmt.Sprintf("write case files: %v", err)
	}
	return result
}

func detectedModel(events []Event) string {
	for _, event := range events {
		if len(event.Arguments) == 0 {
			continue
		}
		var value any
		if err := json.Unmarshal(event.Arguments, &value); err != nil {
			continue
		}
		if model := findStringField(value, "model"); model != "" {
			return model
		}
	}
	return ""
}

func findStringField(value any, field string) string {
	switch typed := value.(type) {
	case map[string]any:
		if result, ok := typed[field].(string); ok {
			return result
		}
		for _, nested := range typed {
			if result := findStringField(nested, field); result != "" {
				return result
			}
		}
	case []any:
		for _, nested := range typed {
			if result := findStringField(nested, field); result != "" {
				return result
			}
		}
	}
	return ""
}

func initializeRepositories(workspace string) (map[string]string, error) {
	root := filepath.Join(workspace, "src")
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}

	baselines := make(map[string]string)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		repo := filepath.Join(root, entry.Name())
		commands := [][]string{
			{"git", "init", "-q"},
			{"git", "config", "user.email", "eval@qrouton.local"},
			{"git", "config", "user.name", "qrouton eval"},
			{"git", "add", "."},
			{"git", "commit", "-qm", "fixture baseline"},
		}
		for _, command := range commands {
			cmd := exec.Command(command[0], command[1:]...)
			cmd.Dir = repo
			if output, err := cmd.CombinedOutput(); err != nil {
				return nil, fmt.Errorf("%s in %s: %w: %s", strings.Join(command, " "), repo, err, output)
			}
		}
		head, err := commandOutput(context.Background(), repo, "git", "rev-parse", "HEAD")
		if err != nil {
			return nil, fmt.Errorf("resolve baseline in %s: %w: %s", repo, err, head)
		}
		baselines[entry.Name()] = head
	}
	return baselines, nil
}

func collectArtifacts(workspace string) ([]Artifact, error) {
	thoughts := filepath.Join(workspace, "thoughts", "shared")
	var artifacts []Artifact
	err := filepath.WalkDir(thoughts, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			if os.IsNotExist(walkErr) {
				return nil
			}
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(workspace, path)
		if err != nil {
			return err
		}
		digest := sha256.Sum256(content)
		artifacts = append(artifacts, Artifact{
			Path:   filepath.ToSlash(rel),
			SHA256: hex.EncodeToString(digest[:]),
			Text:   string(content),
		})
		return nil
	})
	sort.Slice(artifacts, func(i, j int) bool {
		return artifacts[i].Path < artifacts[j].Path
	})
	return artifacts, err
}

// collectDiffs deliberately ignores the case context: it runs after the turns,
// where the per-case timeout may already have expired, and a timed-out case is
// exactly the one whose diffs the report and judges must still see.
func collectDiffs(workspace string, baselines map[string]string) map[string]string {
	diffs := make(map[string]string, len(baselines))
	for repo := range baselines {
		repoDir := filepath.Join(workspace, "src", repo)
		ctx := context.Background()
		diff, err := commandOutput(ctx, repoDir, "git", "diff", "--no-ext-diff", "HEAD")
		if err != nil {
			// Fail loud: an error string trips repo_unchanged instead of
			// letting a broken diff pass as "no changes".
			diff = fmt.Sprintf("(git diff failed: %v)\n%s", err, diff)
		}
		diffs[repo] = diff
		untracked, err := commandOutput(ctx, repoDir, "git", "ls-files", "--others", "--exclude-standard")
		if err == nil && untracked != "" {
			diffs[repo] += "\nUntracked files:\n" + untracked + "\n"
		}
	}
	return diffs
}

func commandOutput(ctx context.Context, dir, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(output)), err
}

func writeCaseFiles(caseDir string, result CaseResult) error {
	tracePath := filepath.Join(caseDir, "trace.jsonl")
	for _, event := range result.Events {
		if err := AppendJSONL(tracePath, event); err != nil {
			return err
		}
	}
	if err := os.WriteFile(
		filepath.Join(caseDir, "final-response.md"),
		[]byte(result.FinalResponse+"\n"),
		0o644,
	); err != nil {
		return err
	}
	for repo, diff := range result.Diffs {
		path := filepath.Join(caseDir, "diffs", repo+".diff")
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(path, []byte(diff), 0o644); err != nil {
			return err
		}
	}
	assertions, err := json.MarshalIndent(result.Assertions, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(
		filepath.Join(caseDir, "assertions.json"),
		append(assertions, '\n'),
		0o644,
	); err != nil {
		return err
	}
	encoded, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(caseDir, "case.json"), append(encoded, '\n'), 0o644)
}

func makeTreeReadOnly(root string) error {
	return filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return os.Chmod(path, 0o755)
		}
		return os.Chmod(path, 0o444)
	})
}
