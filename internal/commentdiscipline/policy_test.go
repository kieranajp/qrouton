package commentdiscipline

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadPolicy(t *testing.T) {
	path := writePolicy(t, `{
  "maxCommentRun": 4,
  "narrationPhrases": ["turns out"],
  "pathExtensions": ["go", "js"]
}`)
	policy, err := LoadPolicy(path)
	if err != nil {
		t.Fatal(err)
	}
	if policy.MaxCommentRun != 4 || len(policy.NarrationPhrases) != 1 || len(policy.PathExtensions) != 2 {
		t.Fatalf("LoadPolicy() = %+v", policy)
	}
}

func TestLoadPolicyRejectsInvalidInput(t *testing.T) {
	cases := map[string]string{
		"malformed JSON":      `{`,
		"multiple values":     validPolicyJSON + `{}`,
		"unknown field":       `{"maxCommentRun":4,"narrationPhrases":["x"],"pathExtensions":["go"],"extra":true}`,
		"zero cap":            `{"maxCommentRun":0,"narrationPhrases":["x"],"pathExtensions":["go"]}`,
		"empty phrases":       `{"maxCommentRun":4,"narrationPhrases":[],"pathExtensions":["go"]}`,
		"uppercase phrase":    `{"maxCommentRun":4,"narrationPhrases":["Turns out"],"pathExtensions":["go"]}`,
		"invalid extension":   `{"maxCommentRun":4,"narrationPhrases":["x"],"pathExtensions":[".go"]}`,
		"duplicate extension": `{"maxCommentRun":4,"narrationPhrases":["x"],"pathExtensions":["go","go"]}`,
	}
	for name, input := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := LoadPolicy(writePolicy(t, input)); err == nil {
				t.Fatal("LoadPolicy() succeeded")
			}
		})
	}
}

func TestLoadPolicyRequiresFile(t *testing.T) {
	_, err := LoadPolicy(filepath.Join(t.TempDir(), "missing.json"))
	if err == nil || !strings.Contains(err.Error(), "read policy") {
		t.Fatalf("LoadPolicy() error = %v", err)
	}
}

const validPolicyJSON = `{"maxCommentRun":4,"narrationPhrases":["turns out"],"pathExtensions":["go"]}`

func writePolicy(t *testing.T, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "policy.json")
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}
