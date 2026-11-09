package status

import (
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/kieranajp/qrouton/internal/markdown"
	"github.com/kieranajp/qrouton/internal/session"
	"github.com/kieranajp/qrouton/internal/sessionpaths"
	"github.com/kieranajp/qrouton/prompts"
	"go.yaml.in/yaml/v3"
)

// stages reads the R, P and I marks from the session's own thoughts. Each mark
// says what the latest artifact of its kind says, read fresh on every call so
// it survives a restart.
func stages(root string, m session.Manifest) Stages {
	if len(m.Repos) == 0 {
		return Stages{}
	}
	return stagesIn(sessionpaths.Thoughts(root))
}

func stagesIn(thoughts string) Stages {
	research := latest(artifacts(filepath.Join(thoughts, researchDir), legacyQuestionsSuffix))
	audit := latest(artifacts(filepath.Join(thoughts, auditsDir), ""))
	plan := latest(artifacts(filepath.Join(thoughts, plansDir), ""))
	return Stages{
		Research:  (research != "" && researched(research)) || (audit != "" && audited(audit)),
		Plan:      plan != "",
		Implement: plan != "" && implemented(plan),
	}
}

// researched means at least one question is answered and none is left holding
// only its framing. A legacy document of prose with no sections counts once it
// has any.
func researched(path string) bool {
	body, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	text := string(body)
	if len(markdown.Sections(text)) == 0 {
		return strings.TrimSpace(markdown.Body(text)) != ""
	}
	return markdown.Researched(text)
}

// audited means the summary and every chosen tell section hold content. Triage
// changes nothing here.
func audited(path string) bool {
	body, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	state := map[string]markdown.SectionState{}
	for _, section := range markdown.Sections(string(body)) {
		state[strings.ToLower(section.Name)] = section.State
	}
	for _, heading := range append([]string{AuditHeadingSummary}, AuditTells(string(body))...) {
		if state[strings.ToLower(heading)] != markdown.SectionAnswered {
			return false
		}
	}
	return true
}

type auditTell struct{ slug, heading, legacy string }

// AuditTells is the headings of the tells an audit chose, in tell order, each as
// the document spells it: a renamed tell keeps its old heading in an audit that
// still uses it.
func AuditTells(text string) []string {
	present := map[string]bool{}
	for _, section := range markdown.Sections(text) {
		present[strings.ToLower(section.Name)] = true
	}
	var headings []string
	for _, tell := range chosenTells(text) {
		heading := tell.heading
		if tell.legacy != "" && present[strings.ToLower(tell.legacy)] && !present[strings.ToLower(tell.heading)] {
			heading = tell.legacy
		}
		headings = append(headings, heading)
	}
	return headings
}

// chosenTells is the tells an audit chose. An audit with no tells key ran the
// legacy set. A key that is empty, malformed or names an unknown tell counts as
// every tell, so nothing is judged complete early.
func chosenTells(text string) []auditTell {
	matter, _, _ := markdown.Split(text)
	var front map[string]yaml.Node
	if err := yaml.Unmarshal([]byte(matter), &front); err != nil {
		return tellsNamed(nil)
	}
	node, ok := front[tellsKey]
	if !ok {
		return tellsNamed(legacyTells)
	}
	var slugs []string
	if err := node.Decode(&slugs); err != nil || len(slugs) == 0 {
		return tellsNamed(nil)
	}
	for _, slug := range slugs {
		if !slices.ContainsFunc(auditTells, func(tell auditTell) bool { return tell.slug == slug }) {
			return tellsNamed(nil)
		}
	}
	return tellsNamed(slugs)
}

// tellsNamed is the named tells in tell order, and every tell when none is named.
func tellsNamed(slugs []string) []auditTell {
	var tells []auditTell
	for _, tell := range auditTells {
		if slugs == nil || slices.Contains(slugs, tell.slug) {
			tells = append(tells, tell)
		}
	}
	return tells
}

// implemented means the plan has phases, each has Verify items, and every one
// is ticked.
func implemented(path string) bool {
	body, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	phases := markdown.PlanPhases(string(body))
	for _, phase := range phases {
		if phase.Total == 0 || phase.Met != phase.Total {
			return false
		}
	}
	return len(phases) > 0
}

type artifact struct {
	path   string
	number int
	at     int64
}

func artifacts(dir, excludeSuffix string) []artifact {
	var out []artifact
	entries, _ := os.ReadDir(dir)
	for _, entry := range entries {
		name := entry.Name()
		lower := strings.ToLower(name)
		if entry.IsDir() || ignoredName(name) || !strings.HasSuffix(lower, markdownSuffix) ||
			(excludeSuffix != "" && strings.HasSuffix(lower, excludeSuffix)) {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		number := -1
		if id := ArtifactID(name); id != "" {
			number, _ = strconv.Atoi(id[1:])
		}
		out = append(out, artifact{path: filepath.Join(dir, name), number: number, at: info.ModTime().UnixNano()})
	}
	return out
}

// NextNumber is one more than the highest number any artifact in dir carries,
// and 1 for a missing or empty folder.
func NextNumber(dir string) int {
	highest := 0
	for _, a := range artifacts(dir, "") {
		highest = max(highest, a.number)
	}
	return highest + 1
}

// latest is the highest-numbered artifact, or the most recently written when
// none carries a number. A number is fixed when a workstream starts; a write
// time moves with every edit to an old document.
func latest(found []artifact) string {
	best := artifact{number: -1, at: -1}
	for _, a := range found {
		if a.number > best.number || (a.number == best.number && a.at > best.at) {
			best = a
		}
	}
	return best.path
}

// stageLeads names the stage each lead works on.
var stageLeads = map[string]string{
	prompts.ResearchLead:       StageResearch,
	prompts.AuditLead:          StageResearch,
	prompts.PlanningLead:       StagePlan,
	prompts.ImplementationLead: StageImplement,
}

// StageOf is the stage an agent type leads, and empty for any other agent.
func StageOf(agentType string) string { return stageLeads[agentType] }

// StagesFor is which stage marks have their lead running, given the types of
// the agents live in the current run.
func StagesFor(types []string) Stages {
	var live Stages
	for _, typ := range types {
		switch StageOf(typ) {
		case StageResearch:
			live.Research = true
		case StagePlan:
			live.Plan = true
		case StageImplement:
			live.Implement = true
		}
	}
	return live
}

// Latest is the furthest stage with a live lead, and empty when none is live.
func (s Stages) Latest() string {
	switch {
	case s.Implement:
		return StageImplement
	case s.Plan:
		return StagePlan
	case s.Research:
		return StageResearch
	}
	return ""
}
