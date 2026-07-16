package evalharness

// One evaluation case: materialize the fixture and prompt assets, drive the
// runner turn by turn, then collect artifacts, diffs, and assertions, and
// persist the per-case output files.

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

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
