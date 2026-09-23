package vault

import (
	"strings"
	"testing"
)

func TestChunksFollowHeadingsOutsideFences(t *testing.T) {
	body := "# Guide\n\nIntro.\n\n## Setup\n\n```sh\n# not a heading\n```\n\n### Linux\n\nApt.\n\n## Use\n\nRun it.\n"
	chunks := chunkDocument("Guide", body, 5)
	want := []struct {
		crumb      string
		start, end int
	}{
		{"Guide", 5, 7},
		{"Guide > Setup", 9, 13},
		{"Guide > Setup > Linux", 15, 17},
		{"Guide > Use", 19, 21},
	}
	if len(chunks) != len(want) {
		t.Fatalf("got %d chunks: %+v", len(chunks), chunks)
	}
	for i, w := range want {
		c := chunks[i]
		if c.breadcrumb != w.crumb || c.start != w.start || c.end != w.end {
			t.Errorf("chunk %d = %q lines %d-%d, want %q lines %d-%d", i, c.breadcrumb, c.start, c.end, w.crumb, w.start, w.end)
		}
	}
	if !strings.Contains(chunks[1].text, "# not a heading") {
		t.Error("a heading inside a fence split the chunk")
	}
}

func TestHeadingOnlySectionsAreNotChunks(t *testing.T) {
	chunks := chunkDocument("T", "# T\n\n## Empty\n\n## Full\n\nText.\n", 1)
	if len(chunks) != 1 || chunks[0].breadcrumb != "T > Full" {
		t.Fatalf("chunks = %+v", chunks)
	}
}

func TestWindowsStayWithinBoundsAndKeepLines(t *testing.T) {
	var body strings.Builder
	body.WriteString("## Long\n\n")
	for range 12 {
		body.WriteString(strings.Repeat("alpha beta gamma ", 12) + "\n\n")
	}
	body.WriteString(strings.Repeat("x", 2500) + "\n")
	chunks := chunkDocument("Doc", body.String(), 1)
	if len(chunks) != 1 {
		t.Fatalf("chunks = %d", len(chunks))
	}
	windows := chunks[0].windows
	if len(windows) < 5 {
		t.Fatalf("windows = %d", len(windows))
	}
	last := 0
	for i, w := range windows {
		if len(w.text) > windowChars {
			t.Errorf("window %d holds %d bytes", i, len(w.text))
		}
		if !strings.HasPrefix(w.input, "Doc > Long\n\n") {
			t.Errorf("window %d input lacks its breadcrumb: %q", i, w.input[:20])
		}
		if w.start < last || w.end < w.start {
			t.Errorf("window %d lines %d-%d follow %d", i, w.start, w.end, last)
		}
		last = w.start
	}
	tail := windows[len(windows)-1]
	if tail.start != 27 || tail.end != 27 {
		t.Errorf("hard-split line kept lines %d-%d, want 27", tail.start, tail.end)
	}
}

func TestHalvesPreferALineBoundary(t *testing.T) {
	if a, b := halves("one\ntwo\nthree\nfour"); a != "one\ntwo\n" || b != "three\nfour" {
		t.Fatalf("halves = %q %q", a, b)
	}
	if a, b := halves("ééé"); a+b != "ééé" || a == "" || b == "" {
		t.Fatalf("rune halves = %q %q", a, b)
	}
}
