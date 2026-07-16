package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
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
	root := ""
	for i := 0; i < len(args); i++ {
		if args[i] == "--session-root" && i+1 < len(args) {
			i++
			root = args[i]
		}
	}
	if root == "" {
		return fmt.Errorf("agents requires --session-root")
	}
	for {
		fmt.Print("\033[H\033[2J")
		fmt.Println("\033[1magents\033[0m")
		statuses, err := scanAgentStatuses(codexSessionsDir(), root)
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
	sort.Slice(statuses, func(i, j int) bool {
		if (statuses[i].State == "running") != (statuses[j].State == "running") {
			return statuses[i].State == "running"
		}
		return statuses[i].UpdatedAt.After(statuses[j].UpdatedAt)
	})
	return statuses, nil
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
