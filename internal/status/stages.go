package status

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/kieranajp/qrouton/internal/markdown"
	"github.com/kieranajp/qrouton/internal/session"
	"github.com/kieranajp/qrouton/internal/sessionpaths"
	"github.com/kieranajp/qrouton/prompts"
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
	plan := latest(artifacts(filepath.Join(thoughts, plansDir), ""))
	return Stages{
		Research:  research != "" && researched(research),
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
	sections := markdown.Sections(string(body))
	if len(sections) == 0 {
		return strings.TrimSpace(markdown.Body(string(body))) != ""
	}
	answered := false
	for _, section := range sections {
		switch {
		case section.State == markdown.SectionFramed:
			return false
		case section.State == markdown.SectionAnswered && !strings.EqualFold(section.Name, summaryHeading):
			answered = true
		}
	}
	return answered
}

// implemented means the plan has phases and every Verify item in each is ticked.
func implemented(path string) bool {
	body, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	phases := markdown.PlanPhases(string(body))
	for _, phase := range phases {
		if phase.Met != phase.Total {
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
		if entry.IsDir() || !strings.HasSuffix(lower, markdownSuffix) ||
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

// stageLeads names the lead whose run a stage mark pulses for.
var stageLeads = map[string]func(*Stages){
	prompts.ResearchLead:       func(s *Stages) { s.Research = true },
	prompts.PlanningLead:       func(s *Stages) { s.Plan = true },
	prompts.ImplementationLead: func(s *Stages) { s.Implement = true },
}

// StagesFor is which stage marks have their lead running, given the types of
// the agents live in the current run.
func StagesFor(types []string) Stages {
	var live Stages
	for _, typ := range types {
		if mark, ok := stageLeads[typ]; ok {
			mark(&live)
		}
	}
	return live
}
