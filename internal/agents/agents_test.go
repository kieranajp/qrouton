package agents

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestScanAgentStatusesFindsRunningAndCompletedSubagents(t *testing.T) {
	dir := t.TempDir()
	root := t.TempDir()
	writeRollout := func(name, events string) {
		t.Helper()
		content := `{"timestamp":"2026-07-16T12:00:00Z","type":"session_meta","payload":{"cwd":` + quoteJSON(root) + `,"parent_thread_id":"parent","agent_nickname":` + quoteJSON(name) + `,"agent_path":"/root/task"}}` + "\n" + events
		if err := os.WriteFile(filepath.Join(dir, name+".jsonl"), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writeRollout("Ada", `{"timestamp":"2026-07-16T12:01:00Z","type":"event_msg","payload":{"type":"task_started"}}`+"\n")
	writeRollout("Grace", `{"timestamp":"2026-07-16T12:02:00Z","type":"event_msg","payload":{"type":"task_started"}}`+"\n"+`{"timestamp":"2026-07-16T12:03:00Z","type":"event_msg","payload":{"type":"task_complete"}}`+"\n")

	statuses, err := scanAgentStatuses(dir, root)
	if err != nil {
		t.Fatal(err)
	}
	if len(statuses) != 2 || statuses[0].Name != "Ada" || statuses[0].State != "running" || statuses[1].Name != "Grace" || statuses[1].State != "done" {
		t.Fatalf("unexpected statuses: %#v", statuses)
	}
}

func TestClaudeAgentHooksRecordLifecycle(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".qrouton"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, input := range []string{
		`{"hook_event_name":"SubagentStart","agent_id":"agent-1","agent_type":"Explore"}`,
		`{"hook_event_name":"SubagentStop","agent_id":"agent-1","agent_type":"Explore"}`,
	} {
		if _, err := RecordEvent(root, bytes.NewBufferString(input)); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", t.TempDir())
	statuses, err := scanClaudeAgentStatuses(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(statuses) != 1 || statuses[0].Name != "Explore" || statuses[0].State != "done" {
		t.Fatalf("unexpected Claude statuses: %#v", statuses)
	}
}

func quoteJSON(value string) string {
	b, _ := json.Marshal(value)
	return string(b)
}
