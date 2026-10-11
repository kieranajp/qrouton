package lineartools

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func testTools(t *testing.T) Tools {
	t.Helper()
	return Tools{
		File: filepath.Join(t.TempDir(), "linear", "coding-tools.json"),
		Command: []string{
			"/Applications/qrouton.app/Contents/MacOS/qrouton",
			"--linear-issue",
		},
		Env: []string{"LINEAR_PROMPT"},
	}
}

func readDoc(t *testing.T, file string) map[string]any {
	t.Helper()
	b, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatalf("%s is not JSON: %v", file, err)
	}
	return doc
}

func TestNewReadsLinearsOwnPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	if got, want := New(nil, nil).File, filepath.Join(home, ".linear", "coding-tools.json"); got != want {
		t.Fatalf("File = %q, want %q", got, want)
	}
}

func TestSnippetIsTheOpenIssueEntryWithLinearsPlaceholderAndEnv(t *testing.T) {
	tools := testTools(t)
	raw, err := tools.Snippet()
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		OpenIssue command `json:"openIssue"`
	}
	if err := json.Unmarshal([]byte(raw), &doc); err != nil {
		t.Fatalf("snippet is not JSON: %v (%q)", err, raw)
	}
	want := command{Path: tools.Command[0], Args: []string{"--linear-issue", issueTemplate}, Env: tools.Env}
	if !reflect.DeepEqual(doc.OpenIssue, want) {
		t.Fatalf("openIssue = %#v, want %#v", doc.OpenIssue, want)
	}
	if _, err := os.Stat(tools.File); !os.IsNotExist(err) {
		t.Fatal("building the snippet wrote the file")
	}
}

func TestInstallCreatesTheFileAndItsDirectory(t *testing.T) {
	tools := testTools(t)
	if err := tools.Install(); err != nil {
		t.Fatal(err)
	}
	snippet, _ := tools.Snippet()
	var want map[string]any
	_ = json.Unmarshal([]byte(snippet), &want)
	if got := readDoc(t, tools.File); !reflect.DeepEqual(got, want) {
		t.Fatalf("installed %v, want %v", got, want)
	}
	if replaces, err := tools.Replaces(); err != nil || replaces {
		t.Fatalf("Replaces after install = %v, %v", replaces, err)
	}
}

func TestInstallKeepsEveryOtherKeyAndReportsAReplacement(t *testing.T) {
	tools := testTools(t)
	if err := os.MkdirAll(filepath.Dir(tools.File), 0o755); err != nil {
		t.Fatal(err)
	}
	existing := `{"openIssue": {"path": "/usr/local/bin/other", "args": []}, "openPullRequest": {"path": "/bin/gh"}}`
	if err := os.WriteFile(tools.File, []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}
	if replaces, err := tools.Replaces(); err != nil || !replaces {
		t.Fatalf("Replaces before install = %v, %v", replaces, err)
	}
	if err := tools.Install(); err != nil {
		t.Fatal(err)
	}
	doc := readDoc(t, tools.File)
	if !reflect.DeepEqual(doc["openPullRequest"], map[string]any{"path": "/bin/gh"}) {
		t.Fatalf("another key changed: %v", doc)
	}
	if issue := doc["openIssue"].(map[string]any); issue["path"] != tools.Command[0] {
		t.Fatalf("openIssue = %v", issue)
	}
}

func TestReplacesIsFalseWithoutAnOpenIssueOrAFile(t *testing.T) {
	tools := testTools(t)
	if replaces, err := tools.Replaces(); err != nil || replaces {
		t.Fatalf("no file = %v, %v", replaces, err)
	}
	_ = os.MkdirAll(filepath.Dir(tools.File), 0o755)
	_ = os.WriteFile(tools.File, []byte(`{"other": 1}`), 0o644)
	if replaces, err := tools.Replaces(); err != nil || replaces {
		t.Fatalf("no openIssue = %v, %v", replaces, err)
	}
}

func TestInstallRefusesAFileThatIsNotAJSONObject(t *testing.T) {
	for _, body := range []string{"null", "[1]", "{not json"} {
		tools := testTools(t)
		_ = os.MkdirAll(filepath.Dir(tools.File), 0o755)
		_ = os.WriteFile(tools.File, []byte(body), 0o644)
		if err := tools.Install(); err == nil {
			t.Fatalf("Install over %q succeeded", body)
		}
		if b, _ := os.ReadFile(tools.File); string(b) != body {
			t.Fatalf("Install over %q rewrote it to %q", body, b)
		}
	}
	tools := testTools(t)
	_ = os.MkdirAll(filepath.Dir(tools.File), 0o755)
	_ = os.WriteFile(tools.File, []byte("null"), 0o644)
	if err := tools.Install(); !errors.Is(err, ErrNotAnObject) {
		t.Fatalf("null = %v, want ErrNotAnObject", err)
	}
}

func TestInstallNamesAMissingCommand(t *testing.T) {
	tools := Tools{File: filepath.Join(t.TempDir(), "coding-tools.json")}
	if err := tools.Install(); !errors.Is(err, ErrNoCommand) {
		t.Fatalf("Install = %v, want ErrNoCommand", err)
	}
	if _, err := os.Stat(tools.File); !os.IsNotExist(err) {
		t.Fatal("a failed install wrote the file")
	}
}
