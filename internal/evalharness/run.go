package evalharness

// Suite orchestration: resolve configuration, run every scenario × runner ×
// sample (caserun.go), pairwise-judge the results, and write the report.

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

func Run(ctx context.Context, config Config) (Report, error) {
	output, err := filepath.Abs(config.Output)
	if err != nil {
		return Report{}, fmt.Errorf("resolve output path: %w", err)
	}
	config.Output = output

	scenariosDir := filepath.Join(config.RepoRoot, evalDirName, scenariosDirName)
	scenarios, err := LoadScenarios(scenariosDir, config.Scenario)
	if err != nil {
		return Report{}, err
	}

	runners, err := selectedAdapters(config)
	if err != nil {
		return Report{}, err
	}
	if config.Samples < 1 {
		return Report{}, ErrSamplesTooFew
	}
	if err := os.MkdirAll(config.Output, outputDirMode); err != nil {
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
			report.Warnings = append(report.Warnings, warnPairwiseNeedsBoth)
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
		runnerClaude: {
			Name:     runnerClaude,
			Bin:      config.ClaudeBin,
			Model:    config.ClaudeModel,
			SelfPath: config.SelfPath,
		},
		runnerCodex: {
			Name:     runnerCodex,
			Bin:      config.CodexBin,
			Model:    config.CodexModel,
			SelfPath: config.SelfPath,
		},
	}

	var names []string
	switch config.Runner {
	case "", runnerAll:
		names = []string{runnerClaude, runnerCodex}
	case runnerClaude, runnerCodex:
		names = []string{config.Runner}
	default:
		return nil, ErrUnknownRunner
	}

	selected := make([]Adapter, 0, len(names))
	for _, name := range names {
		adapter := adapters[name]
		if adapter.Bin == "" {
			adapter.Bin = name
		}
		if _, err := exec.LookPath(adapter.Bin); err != nil {
			return nil, fmt.Errorf("%w: %s: %w", ErrRunnerUnavailable, name, err)
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
	judgeMode := judgeModePairwise
	if config.NoJudge {
		judgeMode = judgeModeNone
	}
	gitSHA, err := commandOutput(ctx, config.RepoRoot, gitBin, gitRevParseCmd, gitHeadRef)
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
			metaRunnerKey:    config.Runner,
			metaScenarioKey:  config.Scenario,
			metaSamplesKey:   config.Samples,
			metaAssetsDirKey: config.AssetsDir,
			metaNoJudgeKey:   config.NoJudge,
			metaJudgeModeKey: judgeMode,
			metaTimeoutKey:   config.Timeout.String(),
		},
	}
}
