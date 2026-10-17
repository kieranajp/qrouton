package theme

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

const fixturePages = "../desktop/frontend/tests/*.html"

var (
	rootBlock   = regexp.MustCompile(`(?s):root\s*\{(.*?)\}`)
	declaration = regexp.MustCompile(`--([\w-]+)\s*:\s*([^;]+);`)
	hexColour   = regexp.MustCompile(`^#[0-9a-fA-F]{3,8}$`)
	reference   = regexp.MustCompile(`^var\(--([\w-]+)\)$`)
)

func resolvedPalette(t *testing.T) map[string]string {
	t.Helper()
	raw := map[string]string{}
	for _, d := range declaration.FindAllStringSubmatch(CSS(), -1) {
		raw[d[1]] = strings.TrimSpace(d[2])
	}
	resolved := map[string]string{}
	for name := range raw {
		value := raw[name]
		for range len(raw) {
			m := reference.FindStringSubmatch(value)
			if m == nil {
				break
			}
			value = raw[m[1]]
		}
		if !hexColour.MatchString(value) {
			t.Fatalf("--%s resolves to %q, not a colour", name, value)
		}
		resolved[name] = strings.ToLower(value)
	}
	return resolved
}

// Fixture pages carry their own palettes because the workbench that serves the
// real one does not run under Playwright.
func TestFixturePagesUseThePalette(t *testing.T) {
	palette := resolvedPalette(t)
	pages, err := filepath.Glob(fixturePages)
	if err != nil {
		t.Fatal(err)
	}
	if len(pages) == 0 {
		t.Fatalf("no fixture pages match %s", fixturePages)
	}
	for _, page := range pages {
		body, err := os.ReadFile(page)
		if err != nil {
			t.Fatal(err)
		}
		for _, block := range rootBlock.FindAllStringSubmatch(string(body), -1) {
			for _, d := range declaration.FindAllStringSubmatch(block[1], -1) {
				name, value := d[1], strings.ToLower(strings.TrimSpace(d[2]))
				want, known := palette[name]
				if !known || !hexColour.MatchString(value) {
					continue
				}
				if value != want {
					t.Errorf("%s: --%s is %s, the theme says %s", filepath.Base(page), name, value, want)
				}
			}
		}
	}
}
