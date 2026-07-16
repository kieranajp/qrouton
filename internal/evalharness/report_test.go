package evalharness

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestWriteReportAndCompare(t *testing.T) {
	leftDir := filepath.Join(t.TempDir(), "left")
	rightDir := filepath.Join(t.TempDir(), "right")
	if err := os.MkdirAll(leftDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(rightDir, 0o755); err != nil {
		t.Fatal(err)
	}

	left := testReport("asset-one", "old response", false)
	right := testReport("asset-two", "new response", true)
	if err := WriteReport(leftDir, left); err != nil {
		t.Fatal(err)
	}
	if err := WriteReport(rightDir, right); err != nil {
		t.Fatal(err)
	}

	markdown, err := Compare(leftDir, rightDir, "")
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"prompt asset hashes differ",
		"0/1 → 1/1",
		"Transcript",
		"changed",
	} {
		if !strings.Contains(markdown, expected) {
			t.Fatalf("comparison missing %q:\n%s", expected, markdown)
		}
	}
}

func testReport(assetHash, response string, passed bool) Report {
	return Report{
		Metadata: Metadata{
			CreatedAt:       time.Unix(0, 0).UTC(),
			AssetHash:       assetHash,
			ScenarioVersion: 1,
			CLIVersions:     map[string]string{"claude": "1"},
			Models:          map[string]string{"claude": "model"},
		},
		Cases: []CaseResult{{
			ID:            "case-one",
			FinalResponse: response,
			Assertions:    []Assertion{{Name: "check", Passed: passed}},
		}},
	}
}
