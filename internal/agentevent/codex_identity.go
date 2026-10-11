package agentevent

import (
	"bufio"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

func codexTaskName(path, id string) string {
	if !filepath.IsAbs(path) || id == "" {
		return ""
	}
	file, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		return ""
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return ""
	}
	reader := bufio.NewReaderSize(io.LimitReader(file, codexIdentityLimit+1), codexIdentityLimit+1)
	line, err := reader.ReadSlice('\n')
	if (err != nil && err != io.EOF) || len(line) > codexIdentityLimit {
		return ""
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
	if len(record.Payload.AgentPath) > codexTaskNameLimit || !strings.HasPrefix(record.Payload.AgentPath, codexTaskPrefix) {
		return ""
	}
	return record.Payload.AgentPath
}
