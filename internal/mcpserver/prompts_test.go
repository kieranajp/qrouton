package mcpserver

import (
	"context"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/kieranajp/qrouton/internal/session"
	"github.com/kieranajp/qrouton/internal/sessionpaths"
	"github.com/kieranajp/qrouton/prompts"
)

var (
	backtickedTool = regexp.MustCompile("`([a-z]+(?:_[a-z]+)+)(?:\\([^`]*\\))?`")
	backtickedRole = regexp.MustCompile("((?:`[a-z]+`(?:/| and ))*`[a-z]+`) (?:repo|repos|repository|repositories)\\b")
	roleName       = regexp.MustCompile("`([a-z]+)`")
)

func promptCorpus(t *testing.T) map[string]string {
	t.Helper()
	loaded, err := prompts.NewEmbeddedLoader().List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	corpus := map[string]string{}
	for _, prompt := range loaded {
		corpus[string(prompt.ID)] = string(prompt.Content)
		for _, file := range prompt.Files {
			corpus[string(prompt.ID)+"/"+file.Path] = string(file.Content)
		}
	}
	return corpus
}

// The prompts are markdown in a leaf that cannot import this package, so the
// tool names they teach are checked against what the server advertises.
func TestPromptsNameExactlyTheAdvertisedTools(t *testing.T) {
	advertised := advertisedTools(t, newMCPServer(t.TempDir(), testEditor, &vaultHost{}, session.ModeAssistant, true))
	corpus := promptCorpus(t)

	taught := map[string]bool{}
	for id, content := range corpus {
		for _, match := range backtickedTool.FindAllStringSubmatch(content, -1) {
			if !advertised[match[1]] {
				t.Errorf("prompt %q names tool %q, which the server does not advertise", id, match[1])
			}
		}
		for name := range advertised {
			if strings.Contains(content, "`"+name+"`") || strings.Contains(content, "`"+name+"(") {
				taught[name] = true
			}
		}
	}
	for name := range advertised {
		if !taught[name] {
			t.Errorf("tool %q is advertised but no prompt names it", name)
		}
	}
}

func TestPromptsNameOnlyManifestRoles(t *testing.T) {
	roles := map[string]bool{string(session.RepoRoleEditing): false, string(session.RepoRoleReference): false}
	for id, content := range promptCorpus(t) {
		for _, phrase := range backtickedRole.FindAllStringSubmatch(content, -1) {
			for _, role := range roleName.FindAllStringSubmatch(phrase[1], -1) {
				if _, ok := roles[role[1]]; !ok {
					t.Errorf("prompt %q calls %q a repository role, which the manifest does not", id, role[1])
					continue
				}
				roles[role[1]] = true
			}
		}
	}
	for role, named := range roles {
		if !named {
			t.Errorf("no prompt names the %q repository role", role)
		}
	}
}

func TestTheHandoffPathIsTheOneTheStamperReads(t *testing.T) {
	root := t.TempDir()
	rel, err := filepath.Rel(root, sessionpaths.Handoff(root))
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.ToSlash(rel)
	if !strings.Contains(descEscalate, want) {
		t.Errorf("escalate description does not name %s", want)
	}
	if !strings.Contains(promptCorpus(t)[string(prompts.Assistant)], "`"+want+"`") {
		t.Errorf("assistant prompt does not name %s", want)
	}
}

func TestSchemaDefaultsNameTheDefaultWindows(t *testing.T) {
	for _, tc := range []struct {
		input any
		want  string
	}{
		{openImagesInput{}, "Defaults to " + defaultImagesWindowName},
		{runCommandInput{}, "Defaults to \"" + defaultCommandWindowName + "\""},
	} {
		field, _ := reflect.TypeOf(tc.input).FieldByName("Name")
		if got := field.Tag.Get("jsonschema"); !strings.Contains(got, tc.want) {
			t.Errorf("%T name schema = %q, want it to say %q", tc.input, got, tc.want)
		}
	}
}
