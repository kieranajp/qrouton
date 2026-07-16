package evalharness

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func WriteReport(output string, report Report) error {
	encoded, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(output, "run.json"), append(encoded, '\n'), 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(output, "report.md"), []byte(renderMarkdown(report)), 0o644)
}

func renderMarkdown(report Report) string {
	var builder strings.Builder
	builder.WriteString("# qrouton prompt evaluation\n\n")
	fmt.Fprintf(&builder, "Generated: %s  \n", report.Metadata.CreatedAt.Format("2006-01-02 15:04:05Z"))
	fmt.Fprintf(&builder, "Asset hash: `%s`  \n", report.Metadata.AssetHash)
	fmt.Fprintf(&builder, "Git SHA: `%s`\n\n", report.Metadata.GitSHA)

	if len(report.Warnings) > 0 {
		builder.WriteString("## Warnings\n\n")
		for _, warning := range report.Warnings {
			fmt.Fprintf(&builder, "- %s\n", warning)
		}
		builder.WriteString("\n")
	}

	builder.WriteString("## Results\n\n")
	builder.WriteString("| Case | Assertions | Judge | Duration | Infrastructure |\n")
	builder.WriteString("| --- | ---: | ---: | ---: | --- |\n")
	for _, result := range report.Cases {
		passed, total := assertionCount(result.Assertions)
		judge := "—"
		if result.Judge != nil {
			judge = judgeSummary(*result.Judge)
		}
		infra := result.InfrastructureError
		if infra == "" {
			infra = "ok"
		}
		fmt.Fprintf(
			&builder,
			"| `%s` | %d/%d | %s | %s | %s |\n",
			result.ID,
			passed,
			total,
			judge,
			formatDuration(result.DurationMS),
			escapeTable(infra),
		)
	}

	for _, result := range report.Cases {
		fmt.Fprintf(&builder, "\n### %s\n\n", result.ID)
		for _, assertion := range result.Assertions {
			mark := "✗"
			if assertion.Passed {
				mark = "✓"
			}
			fmt.Fprintf(&builder, "- %s %s", mark, assertion.Name)
			if assertion.Evidence != "" {
				fmt.Fprintf(&builder, ": %s", strings.ReplaceAll(assertion.Evidence, "\n", " "))
			}
			builder.WriteString("\n")
		}
	}

	if len(report.Pairwise) > 0 {
		builder.WriteString("\n## Pairwise judging\n\n")
		wins := map[string]int{"claude": 0, "codex": 0, "tie": 0, "error": 0}
		for _, pair := range report.Pairwise {
			wins[pair.Outcome]++
		}
		fmt.Fprintf(&builder, "Claude wins: %d · Codex wins: %d · Ties/mixed: %d · Errors: %d\n\n", wins["claude"], wins["codex"], wins["tie"], wins["error"])
		builder.WriteString("| Pair | Outcome | Agreement | Claude judge | Codex judge |\n")
		builder.WriteString("| --- | --- | ---: | --- | --- |\n")
		for _, pair := range report.Pairwise {
			judgments := map[string]string{"claude": "—", "codex": "—"}
			for _, judgment := range pair.Judgments {
				judgments[judgment.Judge] = pairwiseJudgmentSummary(judgment)
			}
			fmt.Fprintf(&builder, "| `%s` | %s | %t | %s | %s |\n",
				pair.ID, pair.Outcome, pair.Agreement,
				escapeTable(judgments["claude"]), escapeTable(judgments["codex"]),
			)
		}
	}
	return builder.String()
}

func Compare(leftDir, rightDir, output string) (string, error) {
	left, err := readReport(leftDir)
	if err != nil {
		return "", err
	}
	right, err := readReport(rightDir)
	if err != nil {
		return "", err
	}

	warnings := comparisonWarnings(left, right)
	leftCases := indexCases(left.Cases)
	rightCases := indexCases(right.Cases)
	keys := caseKeys(leftCases, rightCases)

	var builder strings.Builder
	builder.WriteString("# qrouton evaluation comparison\n\n")
	for _, warning := range warnings {
		fmt.Fprintf(&builder, "> Warning: %s\n\n", warning)
	}
	builder.WriteString("| Case | Assertions | Judge | Artifacts | Transcript |\n")
	builder.WriteString("| --- | --- | --- | --- | --- |\n")
	for _, key := range keys {
		before, beforeOK := leftCases[key]
		after, afterOK := rightCases[key]
		if !beforeOK || !afterOK {
			fmt.Fprintf(&builder, "| `%s` | case added/removed | — | — | — |\n", key)
			continue
		}
		fmt.Fprintf(
			&builder,
			"| `%s` | %s | %s | %s | %s |\n",
			key,
			assertionDelta(before, after),
			judgeDelta(before.Judge, after.Judge),
			artifactDelta(before.Artifacts, after.Artifacts),
			textDelta(before.FinalResponse, after.FinalResponse),
		)
	}

	markdown := builder.String()
	if output != "" {
		if err := os.WriteFile(output, []byte(markdown), 0o644); err != nil {
			return "", err
		}
	}
	return markdown, nil
}

func readReport(dir string) (Report, error) {
	content, err := os.ReadFile(filepath.Join(dir, "run.json"))
	if err != nil {
		return Report{}, err
	}
	var report Report
	if err := json.Unmarshal(content, &report); err != nil {
		return Report{}, err
	}
	return report, nil
}

func comparisonWarnings(left, right Report) []string {
	var warnings []string
	if left.Metadata.ScenarioVersion != right.Metadata.ScenarioVersion {
		warnings = append(warnings, "scenario versions differ")
	}
	if !mapsEqual(left.Metadata.Models, right.Metadata.Models) {
		warnings = append(warnings, "model identifiers differ")
	}
	if !mapsEqual(left.Metadata.CLIVersions, right.Metadata.CLIVersions) {
		warnings = append(warnings, "CLI versions differ")
	}
	if len(left.Cases) != len(right.Cases) {
		warnings = append(warnings, "sample or case counts differ")
	}
	leftSamples, _ := left.Metadata.Invocation["samples"].(float64)
	rightSamples, _ := right.Metadata.Invocation["samples"].(float64)
	if leftSamples != rightSamples {
		warnings = append(warnings, "configured sample counts differ")
	}
	if left.Metadata.AssetHash != right.Metadata.AssetHash {
		warnings = append(warnings, "prompt asset hashes differ")
	}
	return warnings
}

func caseKeys(left, right map[string]CaseResult) []string {
	keys := make(map[string]struct{}, len(left)+len(right))
	for key := range left {
		keys[key] = struct{}{}
	}
	for key := range right {
		keys[key] = struct{}{}
	}
	ordered := make([]string, 0, len(keys))
	for key := range keys {
		ordered = append(ordered, key)
	}
	sort.Strings(ordered)
	return ordered
}

func mapsEqual(left, right map[string]string) bool {
	if len(left) != len(right) {
		return false
	}
	for key, value := range left {
		if right[key] != value {
			return false
		}
	}
	return true
}

func indexCases(cases []CaseResult) map[string]CaseResult {
	indexed := make(map[string]CaseResult, len(cases))
	for _, result := range cases {
		indexed[result.ID] = result
	}
	return indexed
}

func assertionCount(assertions []Assertion) (int, int) {
	passed := 0
	for _, assertion := range assertions {
		if assertion.Passed {
			passed++
		}
	}
	return passed, len(assertions)
}

func judgeSummary(judge JudgeResult) string {
	if judge.Error != "" {
		return "error"
	}
	if len(judge.Scores) == 0 {
		return "—"
	}
	total := 0
	for _, score := range judge.Scores {
		total += score
	}
	return fmt.Sprintf("%.1f/5", float64(total)/float64(len(judge.Scores)))
}

func pairwiseJudgmentSummary(judgment PairwiseJudgment) string {
	if judgment.Error != "" {
		return "error: " + judgment.Error
	}
	return fmt.Sprintf("%s→%s: %s", judgment.Choice, judgment.Winner, judgment.Evidence)
}

func assertionDelta(before, after CaseResult) string {
	beforePassed, beforeTotal := assertionCount(before.Assertions)
	afterPassed, afterTotal := assertionCount(after.Assertions)
	return fmt.Sprintf("%d/%d → %d/%d", beforePassed, beforeTotal, afterPassed, afterTotal)
}

func judgeDelta(before, after *JudgeResult) string {
	if before == nil || after == nil {
		return "—"
	}
	return judgeSummary(*before) + " → " + judgeSummary(*after)
}

func artifactDelta(before, after []Artifact) string {
	beforeHashes := make(map[string]string, len(before))
	for _, artifact := range before {
		beforeHashes[artifact.Path] = artifact.SHA256
	}
	changed := 0
	for _, artifact := range after {
		if beforeHashes[artifact.Path] != artifact.SHA256 {
			changed++
		}
		delete(beforeHashes, artifact.Path)
	}
	return fmt.Sprintf("%d changed", changed+len(beforeHashes))
}

func textDelta(before, after string) string {
	if before == after {
		return "unchanged"
	}
	return "changed"
}

func formatDuration(milliseconds int64) string {
	return fmt.Sprintf("%.1fs", float64(milliseconds)/1000)
}

func escapeTable(value string) string {
	value = strings.ReplaceAll(value, "|", "\\|")
	return strings.ReplaceAll(value, "\n", " ")
}
