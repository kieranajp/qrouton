package versionformat

import (
	"os/exec"
	"strings"
	"testing"
)

func TestNumericGrammarAcceptsOneToThreeComponentsInGoAndShell(t *testing.T) {
	for value, want := range map[string]bool{
		"0": true, "1.2": true, "01.002.0003": true, "123.456.789": true,
		"": false, "v1": false, "1.2.3.4": false, "1.": false, ".1": false,
		"-1": false, "1.2-beta": false, " 1": false, "1 ": false, "one": false,
	} {
		t.Run(value, func(t *testing.T) {
			if got := Valid(value); got != want {
				t.Errorf("Go validity = %v, want %v", got, want)
			}
			cmd := exec.Command("grep", "-Eqf", "grammar.txt")
			cmd.Stdin = strings.NewReader(value + "\n")
			err := cmd.Run()
			if code := cmd.ProcessState.ExitCode(); code != 0 && code != 1 {
				t.Fatalf("shell grammar did not run: %v", err)
			}
			if got := err == nil; got != want {
				t.Errorf("shell validity = %v, want %v", got, want)
			}
		})
	}
}
