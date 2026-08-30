package lineartools

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func testTools(t *testing.T) Tools {
	t.Helper()
	return Tools{
		File: filepath.Join(t.TempDir(), "coding-tools.json"),
		Command: []string{
			"/Applications/qrouton.app/Contents/MacOS/qrouton",
			"--linear-issue",
		},
		Env: []string{"LINEAR_PROMPT"},
	}
}

func TestNewReadsLinearsOwnPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	if got, want := New(nil, nil).File, filepath.Join(home, ".linear", "coding-tools.json"); got != want {
		t.Fatalf("File = %q, want %q", got, want)
	}
}

// The starter is what the user is offered before they have a file, so it has to
// name the running executable and carry Linear's own placeholder.
func TestLoadGeneratesAStarterPointingAtTheCommand(t *testing.T) {
	tools := testTools(t)

	raw, err := tools.Load()
	if err != nil {
		t.Fatal(err)
	}
	var starter document
	if err := json.Unmarshal([]byte(raw), &starter); err != nil {
		t.Fatalf("starter is not valid JSON: %v (%q)", err, raw)
	}
	if starter.OpenIssue.Path != tools.Command[0] {
		t.Fatalf("executable = %q, want %q", starter.OpenIssue.Path, tools.Command[0])
	}
	wantArgs := []string{tools.Command[1], issueTemplate}
	if !reflect.DeepEqual(starter.OpenIssue.Args, wantArgs) {
		t.Fatalf("args = %#v, want %#v", starter.OpenIssue.Args, wantArgs)
	}
	if !reflect.DeepEqual(starter.OpenIssue.Env, tools.Env) {
		t.Fatalf("env = %#v, want %#v", starter.OpenIssue.Env, tools.Env)
	}
	if _, err := os.Stat(tools.File); !os.IsNotExist(err) {
		t.Fatal("generating the starter wrote the file")
	}
}

// The file is the user's, so the panel is given the bytes it holds rather than a
// document reformatted through the starter's shape.
func TestLoadKeepsAnExistingFileVerbatim(t *testing.T) {
	tools := testTools(t)
	existing := "{\n  \"openIssue\": {\"path\": \"/other/tool\"}\n}\n\n"
	if err := os.WriteFile(tools.File, []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := tools.Load()
	if err != nil || got != existing {
		t.Fatalf("Load = %q, %v, want %q", got, err, existing)
	}
}

func TestLoadNamesAMissingCommandInsteadOfWritingABrokenStarter(t *testing.T) {
	tools := testTools(t)
	tools.Command = nil

	got, err := tools.Load()
	if got != "" || !errors.Is(err, ErrNoCommand) {
		t.Fatalf("Load = %q, %v, want ErrNoCommand", got, err)
	}
}

func TestValidateRefusesAnythingThatIsNotAJSONObject(t *testing.T) {
	for _, raw := range []string{"", "   ", "{not json", "null", "[]", `"text"`} {
		if body, err := Validate(raw); err == nil {
			t.Fatalf("Validate(%q) = %q, want a refusal", raw, body)
		}
	}
	if _, err := Validate("null"); !errors.Is(err, ErrNotAnObject) {
		t.Fatal("a JSON null was not refused as a non-object")
	}
}

// The body Validate answers is what lands on disk: trimmed, and newline
// terminated so the file is not one long line without an ending.
func TestValidateAnswersTheTrimmedDocumentWithOneTrailingNewline(t *testing.T) {
	body, err := Validate("  {\n  \"openIssue\": {}\n}\n\n")
	if err != nil {
		t.Fatal(err)
	}
	if want := "{\n  \"openIssue\": {}\n}\n"; string(body) != want {
		t.Fatalf("Validate = %q, want %q", body, want)
	}
}

func TestSaveCreatesTheFileAndItsDirectory(t *testing.T) {
	tools := testTools(t)
	tools.File = filepath.Join(t.TempDir(), ".linear", "coding-tools.json")

	body, err := Validate(`{"openIssue": {"path": "/bin/true"}}`)
	if err != nil {
		t.Fatal(err)
	}
	if err := tools.Save(body); err != nil {
		t.Fatal(err)
	}
	written, err := os.ReadFile(tools.File)
	if err != nil {
		t.Fatal(err)
	}
	if string(written) != string(body) {
		t.Fatalf("wrote %q, want %q", written, body)
	}
}

// A save that changed nothing must not reformat the file: the user's own
// spacing is part of what they wrote.
func TestSaveLeavesAFileWhoseContentIsUnchangedAlone(t *testing.T) {
	tools := testTools(t)
	existing := "{\n  \"custom\": true\n}\n\n"
	if err := os.WriteFile(tools.File, []byte(existing), 0o600); err != nil {
		t.Fatal(err)
	}

	body, err := Validate(existing)
	if err != nil {
		t.Fatal(err)
	}
	if err := tools.Save(body); err != nil {
		t.Fatal(err)
	}
	written, err := os.ReadFile(tools.File)
	if err != nil {
		t.Fatal(err)
	}
	if string(written) != existing {
		t.Fatalf("an unchanged document was rewritten as %q", written)
	}
}

func TestSaveReportsADirectoryItCannotCreate(t *testing.T) {
	blocked := filepath.Join(t.TempDir(), "notadir")
	if err := os.WriteFile(blocked, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	tools := testTools(t)
	tools.File = filepath.Join(blocked, ".linear", "coding-tools.json")

	err := tools.Save([]byte("{}\n"))
	if err == nil {
		t.Fatal("Save reported success for a directory it could not create")
	}
	if strings.TrimSpace(err.Error()) == "" {
		t.Fatal("Save refused without saying why")
	}
}
