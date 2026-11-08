package evalharness

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/kieranajp/qrouton/internal/session"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestEvalWorkflowToolsExecuteRequiredStampedCalls(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	root := t.TempDir()
	writeSessionManifest(t, root, map[string]string{})
	if err := os.MkdirAll(filepath.Join(root, "thoughts"), 0o755); err != nil {
		t.Fatal(err)
	}
	log := filepath.Join(t.TempDir(), "mcp.jsonl")
	ctx := context.Background()
	server := newMockMCP(root, log)
	st, ct := mcp.NewInMemoryTransports()
	ss, err := server.Connect(ctx, st, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ss.Close()
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, nil)
	cs, err := client.Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()
	tools, err := cs.ListTools(ctx, nil)
	if err != nil || len(tools.Tools) != 4 {
		t.Fatalf("workflow tools = %+v, %v", tools, err)
	}
	for _, call := range []mcp.CallToolParams{
		{Name: "create_artefact", Arguments: map[string]any{"kind": "research", "title": "Fixture work"}},
		{Name: "set_summary", Arguments: map[string]any{"stage": "research", "summary": "First summary"}},
		{Name: "set_summary", Arguments: map[string]any{"stage": "plan", "summary": "Replacement summary"}},
		{Name: "set_digest", Arguments: map[string]any{"digest": "First digest"}},
		{Name: "set_digest", Arguments: map[string]any{"digest": "Replacement digest"}},
	} {
		result, err := cs.CallTool(ctx, &call)
		if err != nil || result.IsError {
			t.Fatalf("%s = %+v, %v", call.Name, result, err)
		}
	}
	manifest, err := session.Load(root)
	if err != nil || manifest.Summary.Text != "Replacement summary" || manifest.Summary.Stage != "plan" || manifest.Digest.Text != "Replacement digest" {
		t.Fatalf("workflow manifest = %+v, %v", manifest, err)
	}
	paths, err := filepath.Glob(filepath.Join(root, "thoughts", "shared", "research", "*.md"))
	if err != nil || len(paths) != 1 {
		t.Fatalf("created artifacts = %v, %v", paths, err)
	}
	rel, err := filepath.Rel(root, paths[0])
	if err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "outside.md")
	writeTestFile(t, outside, "outside fixture")
	for _, path := range []string{rel, outside} {
		result, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "open_file", Arguments: map[string]any{"path": path}})
		if path == rel && (err != nil || result.IsError) {
			t.Fatalf("open artifact = %+v, %v", result, err)
		}
		if path != rel && err == nil && !result.IsError {
			t.Fatal("open_file accepted a path outside the fixture")
		}
	}
	events := ReadMCPEvents(log, 1)
	if len(events) != 1 || events[0].Path != filepath.ToSlash(rel) || events[0].Name != "open_file" {
		t.Fatalf("recorded open_file calls = %+v", events)
	}
}
