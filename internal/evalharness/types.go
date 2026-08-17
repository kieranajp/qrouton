package evalharness

import (
	"encoding/json"
	"time"
)

const ScenarioVersion = 1

type Scenario struct {
	ID          string      `json:"id"`
	Version     int         `json:"version"`
	Description string      `json:"description"`
	Fixture     string      `json:"fixture"`
	Turns       []string    `json:"turns"`
	Rubric      string      `json:"rubric"`
	Checks      []CheckSpec `json:"checks"`
}

type CheckSpec struct {
	Kind     string   `json:"kind"`
	Path     string   `json:"path,omitempty"`
	Repo     string   `json:"repo,omitempty"`
	Pattern  string   `json:"pattern,omitempty"`
	Any      []string `json:"any,omitempty"`
	MaxLines int      `json:"max_lines,omitempty"`
}

type Config struct {
	RepoRoot    string
	Runner      string
	Scenario    string
	Samples     int
	AssetsDir   string
	ClaudeModel string
	CodexModel  string
	NoJudge     bool
	Timeout     time.Duration
	Output      string
	ClaudeBin   string
	CodexBin    string
	SelfPath    string
}

type Event struct {
	Time      time.Time       `json:"time"`
	Kind      string          `json:"kind"`
	Turn      int             `json:"turn,omitempty"`
	Role      string          `json:"role,omitempty"`
	Name      string          `json:"name,omitempty"`
	Text      string          `json:"text,omitempty"`
	Path      string          `json:"path,omitempty"`
	Arguments json.RawMessage `json:"arguments,omitempty"`
	RawType   string          `json:"raw_type,omitempty"`
}

type Assertion struct {
	Name     string `json:"name"`
	Passed   bool   `json:"passed"`
	Evidence string `json:"evidence,omitempty"`
}

type Artifact struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Text   string `json:"text,omitempty"`
}

type PairwiseJudgment struct {
	Judge    string `json:"judge"`
	ARunner  string `json:"a_runner"`
	BRunner  string `json:"b_runner"`
	Choice   string `json:"choice,omitempty"`
	Winner   string `json:"winner,omitempty"`
	Evidence string `json:"evidence,omitempty"`
	Raw      string `json:"raw,omitempty"`
	Error    string `json:"error,omitempty"`
}

type PairwiseResult struct {
	ID        string             `json:"id"`
	Scenario  string             `json:"scenario"`
	Sample    int                `json:"sample"`
	Outcome   string             `json:"outcome"`
	Agreement bool               `json:"agreement"`
	Judgments []PairwiseJudgment `json:"judgments"`
}

type CaseResult struct {
	ID                  string            `json:"id"`
	ScenarioID          string            `json:"scenario_id"`
	ScenarioVersion     int               `json:"scenario_version"`
	Runner              string            `json:"runner"`
	Model               string            `json:"model,omitempty"`
	Sample              int               `json:"sample"`
	SessionID           string            `json:"session_id,omitempty"`
	StartedAt           time.Time         `json:"started_at"`
	DurationMS          int64             `json:"duration_ms"`
	Events              []Event           `json:"events"`
	FinalResponse       string            `json:"final_response,omitempty"`
	Artifacts           []Artifact        `json:"artifacts,omitempty"`
	Diffs               map[string]string `json:"diffs,omitempty"`
	Assertions          []Assertion       `json:"assertions"`
	InfrastructureError string            `json:"infrastructure_error,omitempty"`
}

type Metadata struct {
	CreatedAt       time.Time         `json:"created_at"`
	AssetHash       string            `json:"asset_hash"`
	GitSHA          string            `json:"git_sha,omitempty"`
	CLIVersions     map[string]string `json:"cli_versions"`
	Models          map[string]string `json:"models"`
	ScenarioVersion int               `json:"scenario_version"`
	Invocation      map[string]any    `json:"invocation"`
}

type Report struct {
	Metadata Metadata         `json:"metadata"`
	Cases    []CaseResult     `json:"cases"`
	Pairwise []PairwiseResult `json:"pairwise,omitempty"`
	Warnings []string         `json:"warnings,omitempty"`
}
