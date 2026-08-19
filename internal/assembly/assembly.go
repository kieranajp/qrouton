// Package assembly holds the rules a session is assembled by, with no display
// of its own: what a draft must satisfy, the branch its repositories are cut
// on, and the manifest write that adds repositories to a live session.
package assembly

import (
	"fmt"

	"github.com/kieranajp/qrouton/internal/config"
	"github.com/kieranajp/qrouton/internal/session"
)

// Draft is the work as it has been described, before any of it is on disk.
// Repos is ordered: chosen first means worked in most, which is the ranking the
// manifest keeps and the rail truncates against.
type Draft struct {
	Name        string
	Description string
	Ticket      string
	Prefix      string
	Mode        session.SessionMode
	Repos       []session.RepoSelection
	// Upgrades names repositories the session already holds and reads, to be
	// checked out for editing instead. Only the picker fills it: a session that
	// does not exist yet holds nothing.
	Upgrades []session.RepoRef
}

// Assembler carries what the rules cannot reach for themselves. Signal is
// launch.SignalSupervisor: this package must not import launch, because
// everything it imports is linked into the workbench with it.
type Assembler struct {
	Cfg    *config.Config
	Signal func(root string)
}

// Prefixes is the branch-prefix vocabulary, and the only copy of it. The MCP
// escalate tool describes the same list in a struct tag, which a test holds to
// this one.
func Prefixes() []string {
	return []string{"feat", "fix", "chore", "refactor", "docs", "test"}
}

// Preview is the branch a first assembly cuts, which is also the hint under the
// name field. session.Slugify stays the one implementation of the transform.
func Preview(d Draft) string {
	return fmt.Sprintf(branchFormat, d.Prefix, session.Slugify(d.Name))
}

// branchFor is the branch repositories being added land on. A session that
// already has one keeps it: work that started small acquires more of the
// codebase, so the checkout it started in is the last thing that should move.
func branchFor(m session.Manifest, d Draft) string {
	if b := m.Branch(); b != "" {
		return b
	}
	return Preview(d)
}
