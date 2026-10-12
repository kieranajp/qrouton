package status

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kieranajp/qrouton/prompts"
)

type thoughtsDir string

func (d thoughtsDir) write(t *testing.T, kind, name, body string) {
	t.Helper()
	dir := filepath.Join(string(d), kind)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

const (
	framedResearch   = "# Retry\n\n## Summary\n\nWhat is looked at.\n\n## How does it retry?\n\n> Start in retry.go.\n"
	answeredResearch = "# Retry\n\n## Summary\n\nIt backs off.\n\n## How does it retry?\n\nThree attempts.\n"
	partAnswered     = "# Retry\n\n## How does it retry?\n\nThree attempts.\n\n## Who calls it?\n\n> Look in api/.\n"
)

func TestResearchFillsOnlyOnceEveryQuestionIsAnswered(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		want       bool
	}{
		{"framing only", framedResearch, false},
		{"one question left framed", partAnswered, false},
		{"every question answered", answeredResearch, true},
		{"summary alone answered", "# R\n\n## Summary\n\nSo far.\n", false},
		{"legacy prose findings", "# Retry\n\nThe service bounds attempts.\n", true},
		{"empty", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := thoughtsDir(t.TempDir())
			dir.write(t, researchDir, "R1-retry.md", tc.body)
			if got := stagesIn(string(dir)).Research; got != tc.want {
				t.Fatalf("Research = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestResearchFollowsTheHigherNumberedDocument(t *testing.T) {
	dir := thoughtsDir(t.TempDir())
	dir.write(t, researchDir, "R2-next.md", framedResearch)
	dir.write(t, researchDir, "R1-retry.md", answeredResearch)
	if stagesIn(string(dir)).Research {
		t.Fatal("an answered R1 filled R while R2 is still framed")
	}
	dir.write(t, researchDir, "R2-next.md", answeredResearch)
	dir.write(t, researchDir, "R1-retry.md", framedResearch)
	if !stagesIn(string(dir)).Research {
		t.Fatal("an answered R2 left R hollow because R1 was edited later")
	}
}

func TestLegacyQuestionsFilesNeverCount(t *testing.T) {
	dir := thoughtsDir(t.TempDir())
	dir.write(t, researchDir, "R9-retry-questions.md", "# Questions\n\n- Where?\n")
	if stagesIn(string(dir)).Research {
		t.Fatal("a questions file read as research")
	}
}

func TestUnnumberedArtifactsFallBackToWriteTime(t *testing.T) {
	dir := thoughtsDir(t.TempDir())
	dir.write(t, researchDir, "old.md", answeredResearch)
	dir.write(t, researchDir, "new.md", framedResearch)
	past := time.Now().Add(-time.Hour)
	if err := os.Chtimes(filepath.Join(string(dir), researchDir, "old.md"), past, past); err != nil {
		t.Fatal(err)
	}
	if stagesIn(string(dir)).Research {
		t.Fatal("the older unnumbered document stood for the session")
	}
}

func TestImplementFillsWhenEveryVerifyItemIsTicked(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		plan, done bool
	}{
		{"every Verify ticked, a See open", "# Plan\n\n## Phase 1 — A\n\n### Verify\n- [x] one\n- [x] two\n\n### See\n- [ ] look\n", true, true},
		{"one Verify open", "# Plan\n\n## Phase 1 — A\n\n### Verify\n- [x] one\n- [ ] two\n", true, false},
		{"no phases", "# Plan\n\n- [x] all done\n", true, false},
		{"a phase without Verify items", "# Plan\n\n## Phase 1 — A\n\n### Verify\n- [x] one\n\n## Phase 2 — B\n\nProse only.\n", true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := thoughtsDir(t.TempDir())
			dir.write(t, plansDir, "P1-plan.md", tc.body)
			if got := stagesIn(string(dir)); got.Plan != tc.plan || got.Implement != tc.done {
				t.Fatalf("stages = %+v", got)
			}
		})
	}
}

func TestNoPlanLeavesPAndIHollow(t *testing.T) {
	if got := stagesIn(t.TempDir()); got != (Stages{}) {
		t.Fatalf("stages = %+v", got)
	}
}

func TestStagesForMapsEachLeadToItsMark(t *testing.T) {
	for _, tc := range []struct {
		types []string
		want  Stages
	}{
		{[]string{prompts.ResearchLead}, Stages{Research: true}},
		{[]string{prompts.PlanningLead}, Stages{Plan: true}},
		{[]string{prompts.ImplementationLead}, Stages{Implement: true}},
		{[]string{"code-reviewer", "default"}, Stages{}},
		{[]string{prompts.ResearchLead, "explorer", prompts.ImplementationLead}, Stages{Research: true, Implement: true}},
		{nil, Stages{}},
	} {
		if got := StagesFor(tc.types); got != tc.want {
			t.Errorf("StagesFor(%v) = %+v, want %+v", tc.types, got, tc.want)
		}
	}
}

func TestStagesLatestPicksTheFurthestLiveStage(t *testing.T) {
	for _, tc := range []struct {
		name string
		live Stages
		want string
	}{
		{"none", Stages{}, ""},
		{"research", Stages{Research: true}, StageResearch},
		{"plan", Stages{Plan: true}, StagePlan},
		{"implement", Stages{Implement: true}, StageImplement},
		{"research and plan", Stages{Research: true, Plan: true}, StagePlan},
		{"research and implement", Stages{Research: true, Implement: true}, StageImplement},
		{"plan and implement", Stages{Plan: true, Implement: true}, StageImplement},
		{"all three", Stages{Research: true, Plan: true, Implement: true}, StageImplement},
	} {
		if got := tc.live.Latest(); got != tc.want {
			t.Errorf("%s: Latest() = %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestSyncCopiesAndDotFilesNeverBecomeTheLatestResearch(t *testing.T) {
	dir := thoughtsDir(t.TempDir())
	dir.write(t, researchDir, "R1-retry.md", answeredResearch)
	dir.write(t, researchDir, "R9-retry.sync-conflict-20261012-120000-ABCDEFG.md", framedResearch)
	dir.write(t, researchDir, ".R10-hidden.md", framedResearch)
	if !stagesIn(string(dir)).Research {
		t.Fatal("a conflict copy numbered above R1 displaced it as the latest research")
	}
}
