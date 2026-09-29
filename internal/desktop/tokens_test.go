package desktop

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/kieranajp/qrouton/internal/theme"
)

var (
	tokenDeclared = []*regexp.Regexp{
		regexp.MustCompile(`(--[a-z][a-z0-9-]*)\s*:`),
		regexp.MustCompile(`style:(--[a-z][a-z0-9-]*)`),
		regexp.MustCompile(`setProperty\(\s*["'](--[a-z][a-z0-9-]*)["']`),
	}
	tokenUsed = []*regexp.Regexp{
		regexp.MustCompile(`var\(\s*(--[a-z][a-z0-9-]*)`),
		regexp.MustCompile(`["'](--[a-z][a-z0-9-]*)["']`),
	}
)

// CSS cannot import the palette, and a property nothing declares draws as
// though the rule were absent. Every token the page reads must be one the
// palette serves or one the page itself sets.
func TestEveryTokenThePageReadsIsDeclared(t *testing.T) {
	declared := map[string]bool{}
	for _, pattern := range tokenDeclared {
		for _, match := range pattern.FindAllStringSubmatch(theme.CSS(), -1) {
			declared[match[1]] = true
		}
	}
	used := map[string][]string{}
	err := filepath.WalkDir(frontendSource, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		switch filepath.Ext(path) {
		case ".css", ".svelte", ".js":
		default:
			return nil
		}
		if strings.HasSuffix(path, ".test.js") {
			return nil
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, pattern := range tokenDeclared {
			for _, match := range pattern.FindAllStringSubmatch(string(body), -1) {
				declared[match[1]] = true
			}
		}
		for _, pattern := range tokenUsed {
			for _, match := range pattern.FindAllStringSubmatch(string(body), -1) {
				used[match[1]] = append(used[match[1]], path)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	var missing []string
	for name, paths := range used {
		if !declared[name] {
			missing = append(missing, name+" in "+strings.Join(paths, ", "))
		}
	}
	sort.Strings(missing)
	for _, name := range missing {
		t.Errorf("the page reads %s, which nothing declares", name)
	}
}
