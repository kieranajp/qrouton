package api

import (
	"fmt"
	"os"
	"testing"
)

// TestMain points every XDG write the core makes at a scratch directory, so a
// goroutine that outlives its test cannot reach the user's own config or cache.
func TestMain(m *testing.M) {
	os.Exit(func() int {
		scratch, err := os.MkdirTemp("", "qrouton-api-test")
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		defer os.RemoveAll(scratch)
		for _, name := range []string{"HOME", "XDG_CONFIG_HOME", "XDG_CACHE_HOME"} {
			dir := scratch + "/" + name
			if err := os.Mkdir(dir, 0o700); err != nil {
				fmt.Fprintln(os.Stderr, err)
				return 1
			}
			os.Setenv(name, dir)
		}
		return m.Run()
	}())
}
