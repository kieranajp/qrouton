package mcpserver

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kieranajp/qrouton/internal/config"
	"github.com/kieranajp/qrouton/internal/session"
	"github.com/kieranajp/qrouton/internal/sessionpaths"
	"github.com/kieranajp/qrouton/internal/workbench"
)

func reviewServerFixture(t *testing.T) (*windowManager, workbench.Handle, startReviewInput, *fakeHost) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	root := t.TempDir()
	provider := filepath.Join(t.TempDir(), "fake-codex")
	if err := os.WriteFile(provider, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{Reviewer: &config.Reviewer{Harness: "codex", Model: "chosen", Reasoning: "medium"}, Launch: map[string][]string{"codex": {provider}}}
	if err := config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	repo := filepath.Join(root, "src", "example")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"init", "-b", "main"}, {"-c", "user.name=Test", "-c", "user.email=test@example.org", "commit", "--allow-empty", "-m", "base"}, {"checkout", "-b", "implementation"}, {"-c", "user.name=Test", "-c", "user.email=test@example.org", "commit", "--allow-empty", "-m", "change"}} {
		if out, err := exec.Command("git", append([]string{"-C", repo}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git: %s %v", out, err)
		}
	}
	manifest := session.Manifest{Repos: []session.ManifestRepo{{Name: "example", Role: session.RepoRoleEditing, DefaultBranch: "main", WorktreePath: "src/example"}}}
	raw, _ := json.Marshal(manifest)
	if err := os.WriteFile(sessionpaths.Manifest(root), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	plan := filepath.Join(root, "thoughts", "shared", "plans", "P1.md")
	if err := os.MkdirAll(filepath.Dir(plan), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(plan, []byte("# Plan\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	host := &fakeHost{}
	return newWindowManager(root, testEditor, host), workbench.Handle{Socket: "/tmp/fake.sock", SessionRoot: root}, startReviewInput{Plan: "thoughts/shared/plans/P1.md", Brief: "Review completed scope", Verification: "Recorded checks passed"}, host
}

func TestDisabledReviewToolsCreateNothing(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	root := t.TempDir()
	host := &fakeHost{}
	m := newWindowManager(root, testEditor, host)
	out, err := m.startReview(context.Background(), workbench.Handle{}, startReviewInput{})
	if err != nil || out.Requested || out.Settings != nil || !strings.Contains(out.Message, "Settings") {
		t.Fatalf("None = %+v %v", out, err)
	}
	entries, _ := os.ReadDir(root)
	if len(entries) != 0 || len(host.opens) != 0 {
		t.Fatal("disabled request allocated artifacts or terminal")
	}
	cs := connectVault(t, host)
	_, state := callText(t, cs, toolReviewerSettings, struct{}{})
	if state["enabled"] != false || state["available"] != false {
		t.Fatalf("settings = %+v", state)
	}
}

func TestStartReviewReturnsPinnedReportAndRetainedBackgroundTab(t *testing.T) {
	m, handle, input, host := reviewServerFixture(t)
	out, err := m.startReview(context.Background(), handle, input)
	if err != nil || !out.Requested || out.RunID == "" || out.Settings.Model != "chosen" {
		t.Fatalf("request = %+v %v", out, err)
	}
	if len(host.opens) != 1 {
		t.Fatalf("opens = %+v", host.opens)
	}
	opts := host.opens[0]
	if opts.Kind != workbench.KindTerminal || opts.Select || opts.CloseOnExit || len(opts.Command) != 4 || opts.Command[1] != "reviewer" {
		t.Fatalf("window = %+v", opts)
	}
	data, err := os.ReadFile(out.ReportPath)
	if err != nil || !strings.Contains(string(data), "kind: review") || !strings.Contains(string(data), "No completed review") || !strings.Contains(string(data), "../plans/P1.md") {
		t.Fatalf("report = %s %v", data, err)
	}
}

func TestReviewOpenFailurePreservesStableInspectionPaths(t *testing.T) {
	m, handle, input, host := reviewServerFixture(t)
	host.openErr = errors.New("uncertain open")
	out, err := m.startReview(context.Background(), handle, input)
	if err != nil || out.Requested || out.Error == "" || out.RunID == "" || !strings.Contains(out.Message, "no automatic retry") {
		t.Fatalf("failure = %+v %v", out, err)
	}
	if _, err := os.Stat(out.ReportPath); err != nil {
		t.Fatal("failed launch lost report", err)
	}
	if _, err := os.Stat(sessionpaths.ReviewRequest(m.root, out.RunID)); err != nil {
		t.Fatal("failed launch lost request", err)
	}
}

func TestCanceledReviewClosesOnlyItsDelayedTab(t *testing.T) {
	m, handle, input, host := reviewServerFixture(t)
	conversation, err := m.open(context.Background(), "conversation", workbench.WindowOptions{Kind: workbench.KindTerminal, Command: []string{"conversation"}})
	if err != nil {
		t.Fatal(err)
	}
	entered, release := host.blockOpen()
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan startReviewOutput, 1)
	go func() { out, _ := m.startReview(ctx, handle, input); result <- out }()
	<-entered
	cancel()
	release()
	out := <-result
	if out.Requested || out.Error == "" || out.ReportPath == "" {
		t.Fatalf("cancel = %+v", out)
	}
	if len(host.closes) != 1 || host.closes[0] == conversation || !host.live[conversation] {
		t.Fatalf("closed = %+v", host.closes)
	}
	if _, err := os.Stat(out.ReportPath); err != nil {
		t.Fatal(err)
	}
}

func TestReviewDisabledAfterSettingsInspectionPreventsLaunch(t *testing.T) {
	m, handle, input, host := reviewServerFixture(t)
	cs := connectVault(t, host)
	_, state := callText(t, cs, toolReviewerSettings, struct{}{})
	if state["enabled"] != true || state["available"] != true {
		t.Fatalf("offer settings = %+v", state)
	}
	if err := config.Save(&config.Config{}); err != nil {
		t.Fatal(err)
	}
	out, err := m.startReview(context.Background(), handle, input)
	if err != nil || out.Requested || len(host.opens) != 0 {
		t.Fatalf("disabled = %+v %v", out, err)
	}
	if _, err := os.Stat(sessionpaths.ReviewRuns(m.root)); !os.IsNotExist(err) {
		t.Fatal("disabled allocated run")
	}
}

func TestUnavailableReviewerKeepsSavedSelection(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if err := config.Save(&config.Config{Reviewer: &config.Reviewer{Harness: "codex", Model: "chosen"}, Launch: map[string][]string{"codex": {"/missing/codex"}}}); err != nil {
		t.Fatal(err)
	}
	host := &fakeHost{}
	cs := connectVault(t, host)
	_, state := callText(t, cs, toolReviewerSettings, struct{}{})
	if state["enabled"] != true || state["available"] != false || state["settings"] == nil || state["error"] == "" {
		t.Fatalf("unavailable = %+v", state)
	}
	m := newWindowManager(t.TempDir(), testEditor, host)
	out, err := m.startReview(context.Background(), workbench.Handle{}, startReviewInput{})
	if err != nil || out.Requested || out.Settings.Harness != "codex" || out.Error == "" || len(host.opens) != 0 {
		t.Fatalf("unavailable start = %+v %v", out, err)
	}
}

func TestReviewToolSchemaHasNoCallerLaunchOptions(t *testing.T) {
	tools := listedTools(t, newMCPServer(t.TempDir(), testEditor, &fakeHost{}, session.ModeRPI))
	raw, _ := json.Marshal(tools[toolStartReview].InputSchema)
	text := string(raw)
	for _, key := range []string{"plan", "spec", "bases", "brief", "verification"} {
		if !strings.Contains(text, key) {
			t.Errorf("schema lacks %s", key)
		}
	}
	for _, key := range []string{"command", "model", "reasoning", "foreground"} {
		if strings.Contains(text, `"`+key+`"`) {
			t.Errorf("schema grants %s", key)
		}
	}
}
