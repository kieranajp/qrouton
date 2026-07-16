package evalharness

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

var judgeDimensions = []string{
	"responsiveness",
	"rpi_abstraction",
	"delegation_hygiene",
	"artifact_correctness",
	"ceremony_and_gates",
	"verification_quality",
}

func Judge(
	ctx context.Context,
	adapter Adapter,
	scenario Scenario,
	result CaseResult,
	workspace string,
	mcpLog string,
) *JudgeResult {
	judge := &JudgeResult{Judge: adapter.Name}
	prompt, err := judgePrompt(scenario, result)
	if err != nil {
		judge.Error = err.Error()
		return judge
	}

	_, final, _, err := adapter.RunTurn(ctx, workspace, mcpLog, prompt, "", 1)
	judge.Raw = final
	if err != nil {
		judge.Error = err.Error()
		return judge
	}

	structured := strings.TrimSpace(final)
	structured = strings.TrimPrefix(structured, "```json")
	structured = strings.TrimPrefix(structured, "```")
	structured = strings.TrimSuffix(structured, "```")
	if err := json.Unmarshal([]byte(strings.TrimSpace(structured)), judge); err != nil {
		judge.Error = fmt.Sprintf("parse judge response: %v", err)
		return judge
	}
	judge.Judge = adapter.Name
	return judge
}

func judgePrompt(scenario Scenario, result CaseResult) (string, error) {
	observable := struct {
		Rubric        string            `json:"rubric"`
		Transcript    []Event           `json:"transcript"`
		FinalResponse string            `json:"final_response"`
		Artifacts     []Artifact        `json:"artifacts"`
		Diffs         map[string]string `json:"diffs"`
	}{
		Rubric:        scenario.Rubric,
		Transcript:    result.Events,
		FinalResponse: result.FinalResponse,
		Artifacts:     result.Artifacts,
		Diffs:         result.Diffs,
	}
	encoded, err := json.Marshal(observable)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf(`You are grading an agent run. Use only the observable material below; do not infer hidden reasoning.
Return JSON only with this schema:
{"scores":{"responsiveness":1,"rpi_abstraction":1,"delegation_hygiene":1,"artifact_correctness":1,"ceremony_and_gates":1,"verification_quality":1},"evidence":{"responsiveness":"brief evidence"}}
Each score is an integer from 1 (poor) to 5 (excellent). Include every dimension: %s.

Observable material:
%s`, strings.Join(judgeDimensions, ", "), encoded), nil
}
