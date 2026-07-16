package evalharness

import (
	"strings"
	"testing"
)

func TestPairwisePromptIsBlinded(t *testing.T) {
	scenario := Scenario{Rubric: "Prefer the safer handoff."}
	a := CaseResult{
		Runner:        "claude",
		FinalResponse: "Research is ready.",
		Assertions:    []Assertion{{Name: "repository unchanged", Passed: true, Evidence: "/secret/path"}},
	}
	b := CaseResult{
		Runner:        "codex",
		FinalResponse: "Research is ready.",
		Assertions:    []Assertion{{Name: "repository unchanged", Passed: true}},
	}
	prompt, err := pairwisePrompt(scenario, a, b)
	if err != nil {
		t.Fatal(err)
	}
	for _, leaked := range []string{`"runner"`, "claude", "codex", "/secret/path"} {
		if strings.Contains(strings.ToLower(prompt), strings.ToLower(leaked)) {
			t.Fatalf("blinded prompt leaked %q:\n%s", leaked, prompt)
		}
	}
}

func TestPairwiseOutcome(t *testing.T) {
	tests := []struct {
		name      string
		judgments []PairwiseJudgment
		outcome   string
		agreement bool
	}{
		{"agreement", []PairwiseJudgment{{Winner: "claude"}, {Winner: "claude"}}, "claude", true},
		{"split", []PairwiseJudgment{{Winner: "claude"}, {Winner: "codex"}}, "tie", false},
		{"ties", []PairwiseJudgment{{Winner: "tie"}, {Winner: "tie"}}, "tie", true},
		{"errors", []PairwiseJudgment{{Error: "failed"}, {Error: "failed"}}, "error", false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			outcome, agreement := pairwiseOutcome(test.judgments)
			if outcome != test.outcome || agreement != test.agreement {
				t.Fatalf("outcome=%q agreement=%t, want %q %t", outcome, agreement, test.outcome, test.agreement)
			}
		})
	}
}
