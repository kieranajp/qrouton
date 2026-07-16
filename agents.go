package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
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

func runAgentStatus(args []string) error {
	root, runner := "", ""
	for i := 0; i < len(args); i++ {
		if args[i] == "--session-root" && i+1 < len(args) {
			i++
			root = args[i]
		} else if args[i] == "--runner" && i+1 < len(args) {
			i++
			runner = args[i]
		}
	}
	if root == "" {
		return fmt.Errorf("agents requires --session-root")
	}
	for {
		fmt.Print("\033[H\033[2J")
		fmt.Println("\033[1magents\033[0m")
		var statuses []agentStatus
		var err error
		if runner == "claude" {
			statuses, err = scanClaudeAgentStatuses(root)
		} else {
			statuses, err = scanAgentStatuses(codexSessionsDir(), root)
		}
		if err != nil {
			fmt.Println("\033[2mCodex status unavailable\033[0m")
		} else if len(statuses) == 0 {
			fmt.Println("\033[2mNo subagents yet\033[0m")
		} else {
			for i, status := range statuses {
				if i == 4 {
					fmt.Printf("\033[2m+%d more\033[0m\n", len(statuses)-i)
					break
				}
				mark, color := "✓", "32"
				if status.State == "running" {
					mark, color = "●", "36"
				} else if status.State == "failed" {
					mark, color = "!", "31"
				}
				fmt.Printf("\033[%sm%s\033[0m %s  \033[2m%s\033[0m\n", color, mark, status.Name, status.State)
			}
		}
		time.Sleep(2 * time.Second)
	}
}

type claudeAgentEvent struct {
	HookEventName string `json:"hook_event_name"`
	AgentID       string `json:"agent_id"`
	AgentType     string `json:"agent_type"`
	Timestamp     string `json:"timestamp,omitempty"`
}

func recordClaudeAgentEvent(args []string, input io.Reader) error {
	root := ""
	for i := 0; i < len(args); i++ {
		if args[i] == "--session-root" && i+1 < len(args) {
			i++
			root = args[i]
		}
	}
	if root == "" {
		return fmt.Errorf("agent-event requires --session-root")
	}
	var event claudeAgentEvent
	if err := json.NewDecoder(input).Decode(&event); err != nil {
		return err
	}
	if event.AgentID == "" || (event.HookEventName != "SubagentStart" && event.HookEventName != "SubagentStop") {
		return nil
	}
	event.Timestamp = time.Now().UTC().Format(time.RFC3339Nano)
	b, err := json.Marshal(event)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(filepath.Join(root, ".qrouton", "claude-agents.jsonl"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(append(b, '\n'))
	return err
}

func scanClaudeAgentStatuses(root string) ([]agentStatus, error) {
	byID := make(map[string]agentStatus)
	f, err := os.Open(filepath.Join(root, ".qrouton", "claude-agents.jsonl"))
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
			state := "running"
			if event.HookEventName == "SubagentStop" {
				state = "done"
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
		state := strings.ToLower(firstString(agent, "status", "state"))
		if state == "" || state == "active" || state == "working" {
			state = "running"
		} else if state == "completed" || state == "stopped" {
			state = "done"
		}
		byID[id] = agentStatus{Name: name, Path: id, State: state, UpdatedAt: time.Now()}
	}
}

func firstString(values map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := values[key].(string); ok && value != "" {
			return value
		}
	}
	return ""
}

func codexSessionsDir() string {
	home := os.Getenv("CODEX_HOME")
	if home == "" {
		if userHome, err := os.UserHomeDir(); err == nil {
			home = filepath.Join(userHome, ".codex")
		}
	}
	return filepath.Join(home, "sessions")
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
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".jsonl") {
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
		if (statuses[i].State == "running") != (statuses[j].State == "running") {
			return statuses[i].State == "running"
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

	status := agentStatus{State: "done"}
	metaSeen := false
	scanner := bufio.NewScanner(f)
	// Rollout records can contain large instruction payloads.
	scanner.Buffer(make([]byte, 64*1024), 8*1024*1024)
	for scanner.Scan() {
		var record rolloutRecord
		if json.Unmarshal(scanner.Bytes(), &record) != nil {
			continue
		}
		if record.Type == "session_meta" {
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
		if !metaSeen || record.Type != "event_msg" {
			continue
		}
		switch record.Payload.Type {
		case "task_started":
			status.State = "running"
			status.UpdatedAt = record.Timestamp
		case "task_complete":
			status.State = "done"
			status.UpdatedAt = record.Timestamp
		case "turn_aborted":
			status.State = "failed"
			status.UpdatedAt = record.Timestamp
		}
	}
	return status, metaSeen && scanner.Err() == nil
}
