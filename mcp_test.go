package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestEditorPaneOpensInSidebarAndReplacesPreviousPane(t *testing.T) {
	dir := t.TempDir()
	doc := filepath.Join(dir, "doc.md")
	os.WriteFile(doc, []byte("hello"), 0o644)
	log := filepath.Join(dir, "calls")
	helper := filepath.Join(dir, "zellij")
	os.WriteFile(helper, []byte("#!/bin/sh\nprintf '%s\\n' \"$*\" >> \"$CALL_LOG\"\ncase \"$*\" in *new-pane*) echo terminal_9;; esac\n"), 0o755)
	t.Setenv("CALL_LOG", log)
	t.Setenv("ZELLIJ", "1")
	p := &editorPane{root: dir, zellij: helper, editor: editorCommand{Argv: []string{"vi", "+{line}", "{path}"}, Template: true}}
	if _, err := p.open(context.Background(), openFileInput{Path: "doc.md", Line: 7}); err != nil {
		t.Fatal(err)
	}
	if _, err := p.open(context.Background(), openFileInput{Path: "doc.md"}); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(log)
	s := string(b)
	for _, want := range []string{"--x 65%", "--width 35%", "--height 100%", "Editor — exit editor or Alt-x to close", "close-pane --pane-id terminal_9"} {
		if !strings.Contains(s, want) {
			t.Fatalf("calls missing %q:\n%s", want, s)
		}
	}
}

func TestMCPServerAdvertisesOpenFile(t *testing.T) {
	ctx := context.Background()
	server := newMCPServer(t.TempDir(), editorCommand{Argv: []string{"vi"}}, "zellij")
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, nil)
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	ss, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ss.Close()
	cs, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()
	for tool, err := range cs.Tools(ctx, nil) {
		if err != nil {
			t.Fatal(err)
		}
		if tool.Name == "open_file" {
			return
		}
	}
	t.Fatal("open_file was not advertised")
}
