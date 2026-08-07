package agents

import (
	"bufio"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/kieranajp/qrouton/internal/codex"
	"github.com/kieranajp/qrouton/internal/sessionpaths"
)

type agentStatus struct {
	Name      string
	Path      string
	State     string
	UpdatedAt time.Time
}

type rolloutRecord struct {
	Timestamp time.Time `json:"timestamp"`
	Type      string    `json:"type"`
	Payload   struct {
		Type           string `json:"type"`
		CWD            string `json:"cwd"`
		ParentThreadID string `json:"parent_thread_id"`
		AgentNickname  string `json:"agent_nickname"`
		AgentPath      string `json:"agent_path"`
	} `json:"payload"`
}

type claudeAgentEvent struct {
	HookEventName string `json:"hook_event_name"`
	AgentID       string `json:"agent_id"`
	AgentType     string `json:"agent_type"`
	Timestamp     string `json:"timestamp,omitempty"`
}

// RecordEvent appends one Claude subagent hook event read from input to the session's log.
func RecordEvent(root string, input io.Reader) error {
	var event claudeAgentEvent
	if err := json.NewDecoder(input).Decode(&event); err != nil {
		return err
	}
	if event.AgentID == "" || (event.HookEventName != hookSubagentStart && event.HookEventName != hookSubagentStop) {
		return nil
	}
	event.Timestamp = time.Now().UTC().Format(time.RFC3339Nano)
	b, err := json.Marshal(event)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(sessionpaths.ClaudeAgentLog(root), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(append(b, '\n'))
	return err
}

func scanClaudeAgentStatuses(root string) ([]agentStatus, error) {
	byID := make(map[string]agentStatus)
	f, err := os.Open(sessionpaths.ClaudeAgentLog(root))
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	if err == nil {
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			var event claudeAgentEvent
			if json.Unmarshal(scanner.Bytes(), &event) != nil || event.AgentID == "" {
				continue
			}
			updated, _ := time.Parse(time.RFC3339Nano, event.Timestamp)
			state := stateRunning
			if event.HookEventName == hookSubagentStop {
				state = stateDone
			}
			name := event.AgentType
			if name == "" {
				name = event.AgentID
			}
			byID[event.AgentID] = agentStatus{Name: name, Path: event.AgentID, State: state, UpdatedAt: updated}
		}
		closeErr := f.Close()
		if scanner.Err() != nil {
			return nil, scanner.Err()
		}
		if closeErr != nil {
			return nil, closeErr
		}
	}
	mergeClaudeBackgroundAgents(byID, root)
	statuses := make([]agentStatus, 0, len(byID))
	for _, status := range byID {
		statuses = append(statuses, status)
	}
	sortAgentStatuses(statuses)
	return statuses, nil
}

func mergeClaudeBackgroundAgents(byID map[string]agentStatus, root string) {
	out, err := exec.Command("claude", "agents", "--json", "--all", "--cwd", root).Output()
	if err != nil {
		return
	}
	var agents []map[string]any
	if json.Unmarshal(out, &agents) != nil {
		return
	}
	for _, agent := range agents {
		id := firstString(agent, "sessionId", "session_id", "id")
		if id == "" {
			continue
		}
		name := firstString(agent, "name", "title", "agent")
		if name == "" {
			name = id
		}
		state := normalizeClaudeState(strings.ToLower(firstString(agent, "status", "state")))
		byID[id] = agentStatus{Name: name, Path: id, State: state, UpdatedAt: time.Now()}
	}
}

// normalizeClaudeState maps the `claude agents` vocabulary onto the three states
// this package reports.
func normalizeClaudeState(state string) string {
	switch state {
	case "", "active", "working":
		return stateRunning
	case "completed", "stopped":
		return stateDone
	default:
		return state
	}
}

// firstString returns the first key holding a non-empty string. The eval
// harness keeps an identical copy, deliberately: it does not import qrouton's
// packages.
func firstString(values map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := values[key].(string); ok && value != "" {
			return value
		}
	}
	return ""
}

func scanAgentStatuses(sessionsDir, sessionRoot string) ([]agentStatus, error) {
	wantRoot, err := filepath.Abs(sessionRoot)
	if err != nil {
		return nil, err
	}
	var statuses []agentStatus
	err = filepath.WalkDir(sessionsDir, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if entry.IsDir() || !codex.IsSessionLog(entry.Name()) {
			return nil
		}
		status, ok := readAgentStatus(path, wantRoot)
		if ok {
			statuses = append(statuses, status)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sortAgentStatuses(statuses)
	return statuses, nil
}

func sortAgentStatuses(statuses []agentStatus) {
	sort.Slice(statuses, func(i, j int) bool {
		if (statuses[i].State == stateRunning) != (statuses[j].State == stateRunning) {
			return statuses[i].State == stateRunning
		}
		return statuses[i].UpdatedAt.After(statuses[j].UpdatedAt)
	})
}

func readAgentStatus(path, sessionRoot string) (agentStatus, bool) {
	f, err := os.Open(path)
	if err != nil {
		return agentStatus{}, false
	}
	defer f.Close()

	status := agentStatus{State: stateDone}
	metaSeen := false
	scanner := bufio.NewScanner(f)
	// Rollout records can contain large instruction payloads.
	scanner.Buffer(make([]byte, rolloutBufferInitial), rolloutBufferMax)
	for scanner.Scan() {
		var record rolloutRecord
		if json.Unmarshal(scanner.Bytes(), &record) != nil {
			continue
		}
		if record.Type == rolloutSessionMeta {
			cwd, _ := filepath.Abs(record.Payload.CWD)
			if record.Payload.ParentThreadID == "" || cwd != sessionRoot {
				return agentStatus{}, false
			}
			status.Name = record.Payload.AgentNickname
			status.Path = record.Payload.AgentPath
			if status.Name == "" {
				status.Name = strings.TrimPrefix(filepath.Base(status.Path), "/")
			}
			metaSeen = true
			continue
		}
		if !metaSeen || record.Type != rolloutEventMsg {
			continue
		}
		switch record.Payload.Type {
		case rolloutTaskStarted:
			status.State = stateRunning
			status.UpdatedAt = record.Timestamp
		case rolloutTaskComplete:
			status.State = stateDone
			status.UpdatedAt = record.Timestamp
		case rolloutTurnAborted:
			status.State = stateFailed
			status.UpdatedAt = record.Timestamp
		}
	}
	return status, metaSeen && scanner.Err() == nil
}
