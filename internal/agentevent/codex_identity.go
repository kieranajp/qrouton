package agentevent

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"unicode"
)

func codexTaskName(path, id string) string {
	if !filepath.IsAbs(path) || id == "" {
		return ""
	}
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		return ""
	}
	// Nonblocking open also rejects a FIFO swapped in after the path check.
	file, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		return ""
	}
	defer file.Close()
	info, err = file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return ""
	}
	line := make([]byte, 0, 1024)
	var next [1]byte
	for {
		n, err := file.Read(next[:])
		if n != 0 {
			if len(line) == codexIdentityLimit {
				return ""
			}
			line = append(line, next[0])
			if next[0] == '\n' {
				break
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return ""
		}
	}
	var record struct {
		Type    string `json:"type"`
		Payload struct {
			ID        string `json:"id"`
			AgentPath string `json:"agent_path"`
		} `json:"payload"`
	}
	if json.Unmarshal(line, &record) != nil || record.Type != codexIdentityKind || record.Payload.ID != id {
		return ""
	}
	if !canonicalCodexTaskName(record.Payload.AgentPath) {
		return ""
	}
	return record.Payload.AgentPath
}

func canonicalCodexTaskName(name string) bool {
	if len(name) > codexTaskNameLimit || !strings.HasPrefix(name, codexTaskPrefix) {
		return false
	}
	for _, component := range strings.Split(strings.TrimPrefix(name, codexTaskPrefix), "/") {
		if component == "" || component == "." || component == ".." {
			return false
		}
	}
	for _, r := range name {
		if unicode.IsSpace(r) || unicode.IsControl(r) {
			return false
		}
	}
	return true
}
