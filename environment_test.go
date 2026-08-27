package main

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestBundledPathLeavesStandaloneBinaryEnvironmentAlone(t *testing.T) {
	current := "/custom/bin:/usr/bin"
	got, bundled := bundledPath("/usr/local/bin/qrouton", "/Users/kieran", current)
	if bundled || got != current {
		t.Fatalf("bundledPath = %q, %t", got, bundled)
	}
}

func TestBundledPathAddsCommonUserAndPackageManagerBins(t *testing.T) {
	got, bundled := bundledPath(
		"/Applications/qrouton.app/Contents/MacOS/qrouton",
		"/Users/kieran",
		"/usr/bin:/opt/homebrew/bin:/usr/bin",
	)
	if !bundled {
		t.Fatal("app-bundled executable was not recognised")
	}
	want := []string{
		"/usr/bin",
		"/opt/homebrew/bin",
		"/Users/kieran/.local/bin",
		"/Users/kieran/.opencode/bin",
		"/Users/kieran/.bun/bin",
		"/Users/kieran/.cargo/bin",
		"/Users/kieran/.volta/bin",
		"/Users/kieran/.asdf/shims",
		"/Users/kieran/.mise/shims",
		"/Users/kieran/Library/pnpm",
		"/opt/homebrew/sbin",
		"/usr/local/bin",
		"/usr/local/sbin",
	}
	if paths := filepath.SplitList(got); !reflect.DeepEqual(paths, want) {
		t.Fatalf("PATH = %#v, want %#v", paths, want)
	}
}
