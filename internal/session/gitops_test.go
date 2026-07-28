package session

import (
	"strings"
	"testing"
)

// git separates progress updates with carriage returns, so one read of the pipe
// carries several and only the newest should be drawn. The phase matters as much
// as the number: a clone runs Counting → Compressing → Receiving → Resolving,
// each 0–100%, and without the phase the bar looks like it keeps restarting.
func TestScanProgressReportsLatestPhaseAndPercent(t *testing.T) {
	stderr := "Cloning into bare repository 'svc.git'...\n" +
		"remote: Counting objects:  50% (3/6)\r" +
		"remote: Counting objects: 100% (6/6), done.\r\n" +
		"Receiving objects:  47% (1234/2626)\r" +
		"Receiving objects: 100% (2626/2626), 1.10 MiB | 5.00 MiB/s, done.\r\n" +
		"Resolving deltas:  80% (400/500)\r"

	var got []string
	tail := scanProgress(strings.NewReader(stderr), func(phase string, percent int) {
		got = append(got, phase)
		if percent < 0 || percent > 100 {
			t.Fatalf("percent out of range: %d", percent)
		}
	})

	if len(got) == 0 {
		t.Fatal("no progress parsed from git's stderr")
	}
	// Phases arrive in git's order, and the last thing seen is the last reported.
	if got[len(got)-1] != "Resolving deltas" {
		t.Fatalf("final phase = %q, want %q (all: %v)", got[len(got)-1], "Resolving deltas", got)
	}
	for _, phase := range got {
		if strings.Contains(phase, "remote") || strings.Contains(phase, ":") {
			t.Fatalf("phase %q kept git's prefix punctuation (all: %v)", phase, got)
		}
	}
	// The tail is what explains a failure, so it must survive the parse.
	if !strings.Contains(tail, "Cloning into bare repository") {
		t.Fatalf("tail lost git's own output: %q", tail)
	}
}

// A nil callback is the silent path (resume, tests); it must not panic and must
// still capture the tail for error reporting.
func TestScanProgressToleratesNoConsumer(t *testing.T) {
	tail := scanProgress(strings.NewReader("fatal: Could not read from remote repository.\n"), nil)
	if !strings.Contains(tail, "Could not read from remote") {
		t.Fatalf("tail = %q", tail)
	}
}

func TestVerbosityFlagAsksForProgressOnlyWhenDrawn(t *testing.T) {
	// Without --progress git stays silent on a pipe, so a drawn bar would never move.
	if got := verbosityFlag(func(string, int) {}); got != progressFlag {
		t.Fatalf("verbosityFlag with a consumer = %q, want %q", got, progressFlag)
	}
	if got := verbosityFlag(nil); got != quietLongFlag {
		t.Fatalf("verbosityFlag without a consumer = %q, want %q", got, quietLongFlag)
	}
}
