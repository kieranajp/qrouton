package agentevent

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func identityFile(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "identity.jsonl")
	if err := os.WriteFile(path, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func identityRecord(id, name string) string {
	body, _ := json.Marshal(map[string]any{"type": codexIdentityKind, "payload": map[string]string{"id": id, "agent_path": name}})
	return string(body)
}

func TestCodexIdentityReadsOnlyTheFirstRecord(t *testing.T) {
	valid := identityRecord("child", "/root/lead/review")
	for _, tc := range []struct{ name, body, want string }{
		{"EOF terminated", valid, "/root/lead/review"},
		{"newline terminated", valid + "\n", "/root/lead/review"},
		{"later malformed record", valid + "\nnot JSON", "/root/lead/review"},
		{"copied history", identityRecord("parent", "/root/parent") + "\n" + valid, ""},
		{"wrong first type", `{"type":"message"}` + "\n" + valid, ""},
		{"empty first line", "\n" + valid, ""},
		{"wrong UUID", identityRecord("other", "/root/review"), ""},
		{"missing name", `{"type":"session_meta","payload":{"id":"child"}}`, ""},
		{"unsupported payload", `{"type":"session_meta","payload":[]}`, ""},
		{"invalid name", identityRecord("child", "/root/../review"), ""},
		{"malformed", "not JSON", ""},
		{"empty file", "", ""},
		{"oversized", strings.Repeat(" ", codexIdentityLimit) + valid, ""},
		{"extra JSON", valid + valid, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := codexTaskName(identityFile(t, tc.body), "child"); got != tc.want {
				t.Fatalf("name = %q, want %q", got, tc.want)
			}
		})
	}
	for _, terminator := range []string{"", "\n"} {
		body := strings.Repeat(" ", codexIdentityLimit-len(valid)-len(terminator)) + valid + terminator
		if got := codexTaskName(identityFile(t, body), "child"); got != "/root/lead/review" {
			t.Fatalf("record at byte limit = %q", got)
		}
		if got := codexTaskName(identityFile(t, " "+body), "child"); got != "" {
			t.Fatalf("record beyond byte limit = %q", got)
		}
	}
}

func TestCodexIdentityRejectsUnavailableAndSpecialFiles(t *testing.T) {
	dir := t.TempDir()
	fifo := filepath.Join(dir, "pipe")
	if err := syscall.Mkfifo(fifo, 0600); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"", "relative.jsonl", filepath.Join(dir, "missing"), dir, fifo, "/dev/null"} {
		if got := codexTaskName(path, "child"); got != "" {
			t.Fatalf("%s name = %q", path, got)
		}
	}
	if got := codexTaskName(identityFile(t, identityRecord("", "/root/review")), ""); got != "" {
		t.Fatalf("empty UUID name = %q", got)
	}
}

func TestCanonicalCodexTaskName(t *testing.T) {
	for _, valid := range []string{"/root/review", "/root/lead/review_2", "/root/é", "/root/" + strings.Repeat("x", codexTaskNameLimit-6)} {
		if !canonicalCodexTaskName(valid) {
			t.Errorf("rejected %q", valid)
		}
	}
	for _, invalid := range []string{"", "root/review", "/root", "/root/", "/rooted/review", "/root//review", "/root/review/", "/root/.", "/root/../review", "/root/review/..", "/root/re view", "/root/re\tview", "/root/re\nview", "/root/re\x00view", "/root/re\u0085view", "/root/re\u00a0view", "/root/" + strings.Repeat("x", codexTaskNameLimit-5)} {
		if canonicalCodexTaskName(invalid) {
			t.Errorf("accepted %q", invalid)
		}
	}
}
