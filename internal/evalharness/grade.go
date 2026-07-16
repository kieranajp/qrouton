package evalharness

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

var internalLeakPattern = regexp.MustCompile(`(?i)\b(QRSPI|qrspi-[a-z-]+|agent depth|document numbering)\b`)

func Grade(
	scenario Scenario,
	result CaseResult,
	workspace string,
	baselines map[string]string,
) []Assertion {
	assertions := []Assertion{
		gradeNoInternalLeak(result),
		gradeReferencesUnchanged(workspace, baselines),
	}
	for _, check := range scenario.Checks {
		assertions = append(assertions, gradeCheck(check, result, workspace))
	}
	return assertions
}

func gradeNoInternalLeak(result CaseResult) Assertion {
	matched := internalLeakPattern.FindString(result.FinalResponse)
	return Assertion{
		Name:     "no internal workflow terminology in final response",
		Passed:   matched == "",
		Evidence: matched,
	}
}

func gradeReferencesUnchanged(workspace string, baselines map[string]string) Assertion {
	manifest, err := readManifest(filepath.Join(workspace, "qrouton.json"))
	if err != nil {
		return Assertion{Name: "reference repositories unchanged", Evidence: err.Error()}
	}

	var changed []string
	for _, repo := range manifest.Repositories {
		if repo.Role != "reference" {
			continue
		}
		repoDir := filepath.Join(workspace, "src", repo.Name)
		status := commandOutput(context.Background(), repoDir, "git", "status", "--porcelain")
		head := commandOutput(context.Background(), repoDir, "git", "rev-parse", "HEAD")
		if status != "" || head != baselines[repo.Name] {
			changed = append(changed, repo.Name)
		}
	}
	return Assertion{
		Name:     "reference repositories unchanged",
		Passed:   len(changed) == 0,
		Evidence: strings.Join(changed, ", "),
	}
}

func gradeCheck(check CheckSpec, result CaseResult, workspace string) Assertion {
	switch check.Kind {
	case "artifact_exists":
		return fileAssertion(workspace, check.Path, true)
	case "artifact_absent":
		return fileAssertion(workspace, check.Path, false)
	case "response_contains":
		passed := strings.Contains(strings.ToLower(result.FinalResponse), strings.ToLower(check.Pattern))
		return Assertion{Name: "response contains " + check.Pattern, Passed: passed}
	case "response_excludes":
		matched := firstContained(result.FinalResponse, append(check.Any, check.Pattern))
		return Assertion{Name: "response excludes internal terms", Passed: matched == "", Evidence: matched}
	case "artifact_excludes":
		return artifactsExclude(result.Artifacts, check.Pattern)
	case "artifact_contains":
		return artifactContains(workspace, check.Path, check.Pattern)
	case "sentinel_safe":
		return sentinelSafe(result, check.Pattern)
	case "open_file":
		return eventAssertion(result.Events, "open_file", "completed document presented with open_file")
	case "delegation":
		return eventAssertion(result.Events, check.Pattern, "delegated to "+check.Pattern)
	case "repo_changed":
		diff := result.Diffs[check.Repo]
		return Assertion{Name: "repository changed: " + check.Repo, Passed: strings.TrimSpace(diff) != ""}
	case "tests_pass":
		return testsPass(workspace, check.Repo)
	default:
		return Assertion{Name: "unknown check: " + check.Kind, Evidence: "unsupported check kind"}
	}
}

func artifactContains(workspace, path, pattern string) Assertion {
	content, err := os.ReadFile(filepath.Join(workspace, filepath.FromSlash(path)))
	if err != nil {
		return Assertion{Name: "artifact contains progress: " + path, Evidence: err.Error()}
	}
	return Assertion{
		Name:   "artifact contains progress: " + path,
		Passed: strings.Contains(string(content), pattern),
	}
}

func sentinelSafe(result CaseResult, sentinel string) Assertion {
	artifactAssertion := artifactsExclude(result.Artifacts, sentinel)
	var leakingEvents []string
	for _, event := range result.Events {
		if event.Kind == "user" {
			continue
		}
		content := event.Text + " " + string(event.Arguments)
		if strings.Contains(content, sentinel) {
			leakingEvents = append(leakingEvents, event.Name)
		}
	}
	return Assertion{
		Name:     "ticket sentinel absent from research briefs and artifacts",
		Passed:   artifactAssertion.Passed && len(leakingEvents) == 0,
		Evidence: strings.Join(leakingEvents, ", "),
	}
}

func fileAssertion(workspace, pattern string, expected bool) Assertion {
	matches, err := filepath.Glob(filepath.Join(workspace, filepath.FromSlash(pattern)))
	passed := err == nil && (len(matches) > 0) == expected
	name := "artifact exists: " + pattern
	if !expected {
		name = "artifact absent: " + pattern
	}
	evidence := strings.Join(matches, ", ")
	if err != nil {
		evidence = err.Error()
	}
	return Assertion{Name: name, Passed: passed, Evidence: evidence}
}

func artifactsExclude(artifacts []Artifact, pattern string) Assertion {
	var matches []string
	for _, artifact := range artifacts {
		if !strings.Contains(artifact.Path, "/research/") {
			continue
		}
		if strings.Contains(artifact.Text, pattern) {
			matches = append(matches, artifact.Path)
		}
	}
	return Assertion{
		Name:     "artifacts exclude sentinel",
		Passed:   len(matches) == 0,
		Evidence: strings.Join(matches, ", "),
	}
}

func eventAssertion(events []Event, pattern, name string) Assertion {
	pattern = strings.ToLower(pattern)
	for _, event := range events {
		eventText := event.Kind + " " + event.Name + " " + event.Text + " " + string(event.Arguments)
		if strings.Contains(strings.ToLower(eventText), pattern) {
			return Assertion{Name: name, Passed: true, Evidence: event.Kind + ": " + event.Name}
		}
	}
	return Assertion{Name: name, Passed: false}
}

func testsPass(workspace, repo string) Assertion {
	repoDir := filepath.Join(workspace, "src", repo)
	var command []string
	switch {
	case fileExists(filepath.Join(repoDir, "go.mod")):
		command = []string{"go", "test", "./..."}
	case fileExists(filepath.Join(repoDir, "package.json")):
		command = []string{"npm", "test", "--", "--runInBand"}
	default:
		return Assertion{Name: "tests pass: " + repo, Evidence: "no supported test manifest"}
	}

	cmd := exec.Command(command[0], command[1:]...)
	cmd.Dir = repoDir
	output, err := cmd.CombinedOutput()
	return Assertion{
		Name:     "tests pass: " + repo,
		Passed:   err == nil,
		Evidence: strings.TrimSpace(string(output)),
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func firstContained(text string, values []string) string {
	lower := strings.ToLower(text)
	for _, value := range values {
		if value != "" && strings.Contains(lower, strings.ToLower(value)) {
			return value
		}
	}
	return ""
}

type fixtureManifest struct {
	Repositories []struct {
		Name string `json:"name"`
		Role string `json:"role"`
	} `json:"repositories"`
}

func readManifest(path string) (fixtureManifest, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return fixtureManifest{}, err
	}
	var manifest fixtureManifest
	if err := jsonUnmarshal(content, &manifest); err != nil {
		return fixtureManifest{}, fmt.Errorf("parse manifest: %w", err)
	}
	return manifest, nil
}

var jsonUnmarshal = func(content []byte, destination any) error {
	return json.Unmarshal(content, destination)
}
