package core

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// TestMain points the XDG config and cache at a scratch directory, so a test or a
// goroutine that outlives it cannot write the user's own config or pull request cache.
func TestMain(m *testing.M) {
	os.Exit(func() int {
		scratch, err := os.MkdirTemp("", "qrouton-core-test")
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		defer os.RemoveAll(scratch)
		for _, name := range []string{"XDG_CONFIG_HOME", "XDG_CACHE_HOME"} {
			dir := filepath.Join(scratch, name)
			if err := os.Mkdir(dir, 0o700); err != nil {
				fmt.Fprintln(os.Stderr, err)
				return 1
			}
			os.Setenv(name, dir)
		}
		return m.Run()
	}())
}
