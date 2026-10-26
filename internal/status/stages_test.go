package status

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
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
		{"a late summary is a question", "# R\n\n## How?\n\n## Summary\n\nFindings.\n", true},
		{"a tilde-fenced heading is not a question", "# R\n\n## How?\n\nLike this.\n\n~~~md\n## Example\n\n> framing\n~~~\n", true},
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
		{[]string{prompts.AuditLead}, Stages{Research: true}},
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

const (
	emptyAudit = "# Tells\n\n## Summary\n\n> what was audited\n\n## Duplicated behaviour or facts\n\n> findings\n\n" +
		"## Dead or needlessly defensive code\n\n> findings\n\n## Homegrown library or platform behaviour\n\n> findings\n\n## Tools\n\n> tools\n"
	completeAudit = "# Tells\n\n## Summary\n\nTwo findings.\n\n## Duplicated behaviour or facts\n\n### F1 — Two clocks\n\nTell: duplicated behaviour or facts\n\nTriage:\n\n" +
		"## Dead or needlessly defensive code\n\nNone.\n\n## Homegrown library or platform behaviour\n\n### F2 — A URL parser\n\nTriage:\n\n## Tools\n\n> tools\n"
	subsetAudit = "---\nkind: audit\ntells: [dead, homegrown]\n---\n\n# Tells\n\n## Summary\n\nOne finding.\n\n" +
		"## Dead or needlessly defensive code\n\nNone.\n\n## Homegrown library or platform behaviour\n\n### F2 — A URL parser\n\nTriage:\n\n## Tools\n\n> tools\n"
)

func TestACompleteAuditFillsResearch(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		want       bool
	}{
		{"skeleton only", emptyAudit, false},
		{"one tell left framed", strings.Replace(completeAudit, "None.", "> findings", 1), false},
		{"complete and untriaged", completeAudit, true},
		{"triaged", strings.ReplaceAll(completeAudit, "Triage:\n", "Triage: reject\nkept on purpose\n"), true},
		{"a subset without its unchosen sections", subsetAudit, true},
		{"a subset with a chosen section framed", strings.Replace(subsetAudit, "None.", "> findings", 1), false},
		{"every tell with tell 4 framed", "---\ntells: [duplicated, dead, homegrown, volkswagen]\n---\n" + completeAudit + "\n## Tests that pass or fail for the wrong reason\n\n> findings\n", false},
		{"every tell with tell 4 filled", "---\ntells: [duplicated, dead, homegrown, volkswagen]\n---\n" + completeAudit + "\n## Tests that pass or fail for the wrong reason\n\nNone.\n", true},
		{"an old tell-4 heading framed", "---\ntells: [duplicated, dead, homegrown, volkswagen]\n---\n" + completeAudit + "\n## Tests that cannot fail or pass for the wrong reason\n\n> findings\n", false},
		{"an old tell-4 heading filled", "---\ntells: [duplicated, dead, homegrown, volkswagen]\n---\n" + completeAudit + "\n## Tests that cannot fail or pass for the wrong reason\n\nNone.\n", true},
		{"an unknown tell asks for every tell", strings.Replace(subsetAudit, "tells: [dead, homegrown]", "tells: [nonsense]", 1), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := thoughtsDir(t.TempDir())
			dir.write(t, auditsDir, "A1-tells.md", tc.body)
			if got := stagesIn(string(dir)).Research; got != tc.want {
				t.Fatalf("Research = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestEitherAnsweredResearchOrACompleteAuditFillsResearch(t *testing.T) {
	dir := thoughtsDir(t.TempDir())
	dir.write(t, researchDir, "R1-retry.md", framedResearch)
	dir.write(t, auditsDir, "A1-tells.md", completeAudit)
	if !stagesIn(string(dir)).Research {
		t.Fatal("a complete audit left R empty beside framed research")
	}
	dir.write(t, auditsDir, "A2-again.md", emptyAudit)
	if stagesIn(string(dir)).Research {
		t.Fatal("an older complete audit filled R while the latest is a skeleton")
	}
}

func TestAuditTellsReadsTheTellsKey(t *testing.T) {
	every := []string{AuditHeadingDuplicated, AuditHeadingDead, AuditHeadingHomegrown, AuditHeadingVolkswagen}
	legacy := []string{AuditHeadingDuplicated, AuditHeadingDead, AuditHeadingHomegrown}
	front := func(lines string) string { return "---\nkind: audit\n" + lines + "---\n\n# Tells\n" }
	for _, tc := range []struct {
		name, text string
		want       []string
	}{
		{"no frontmatter", "# Tells\n\ntells: [dead]\n", legacy},
		{"no key", front("target: repo\n"), legacy},
		{"empty list", front("tells: []\n"), every},
		{"empty value", front("tells:\n"), every},
		{"a subset out of order", front("tells: [volkswagen, homegrown, dead]\n"), []string{AuditHeadingDead, AuditHeadingHomegrown, AuditHeadingVolkswagen}},
		{"a block list", front("tells:\n  - dead\n"), []string{AuditHeadingDead}},
		{"a duplicate", front("tells: [dead, dead]\n"), []string{AuditHeadingDead}},
		{"an unknown slug", front("tells: [dead, nonsense]\n"), every},
		{"a scalar", front("tells: dead\n"), every},
		{"malformed frontmatter", front("tells: [dead\n"), every},
		{"an old tell-4 heading", front("tells: [volkswagen]\n") + "\n## Tests that cannot fail or pass for the wrong reason\n\nNone.\n", []string{legacyVolkswagenHeading}},
		{"both tell-4 headings", front("tells: [volkswagen]\n") + "\n## Tests that cannot fail or pass for the wrong reason\n\nNone.\n\n## Tests that pass or fail for the wrong reason\n\nNone.\n", []string{AuditHeadingVolkswagen}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := AuditTells(tc.text); !slices.Equal(got, tc.want) {
				t.Fatalf("AuditTells = %q, want %q", got, tc.want)
			}
		})
	}
}
