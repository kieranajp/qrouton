// Package assembly holds the rules a session is assembled by, with no display
// of its own: what a draft must satisfy, the branch its repositories are cut
// on, and the manifest write that adds repositories to a live session.
package assembly

import (
	"fmt"
	"strings"

	"github.com/kieranajp/qrouton/internal/config"
	"github.com/kieranajp/qrouton/internal/session"
	"github.com/kieranajp/qrouton/internal/ticket"
)

// Draft is the work as it has been described, before any of it is on disk.
// Repos is ordered: chosen first means worked in most, which is the ranking the
// manifest keeps and the rail truncates against.
type Draft struct {
	Name              string
	BranchDescription string
	Entropy           string
	Description       string
	Ticket            string
	Prefix            string
	Mode              session.SessionMode
	Repos             []session.RepoSelection
	// Upgrades names repositories the session already holds and reads, to be
	// checked out for editing instead. Only the picker fills it: a session that
	// does not exist yet holds nothing.
	Upgrades []session.RepoRef
}

func (d Draft) Slug() string {
	description := session.Slugify(d.BranchDescription)
	if key := session.Slugify(ticket.Key(d.Ticket)); key != "" {
		if description == "" {
			return key
		}
		return session.SessionSlug(key, description)
	}
	if description != "" {
		return session.SessionSlug(description, d.Entropy)
	}
	return session.SessionSlug(d.Name, d.Entropy)
}

// SuggestBranchDescription keeps the specific clause, drops articles, and
// preserves word order in a compact, editable title fragment.
func SuggestBranchDescription(title string) string {
	clause := branchTitleClause(strings.TrimSpace(title))
	words := make([]string, 0, branchDescriptionMaxWords)
	length := 0
	for _, field := range strings.Fields(clause) {
		word := session.Slugify(field)
		if word == "" || branchArticles[word] {
			continue
		}
		added := len(word)
		if len(words) > 0 {
			added++
		}
		if len(words) == branchDescriptionMaxWords || length+added > branchDescriptionMaxLength {
			if len(words) == 0 {
				return word[:branchDescriptionMaxLength]
			}
			return session.Slugify(strings.Join(words, " "))
		}
		words = append(words, word)
		length += added
	}
	return session.Slugify(strings.Join(words, " "))
}

func branchTitleClause(title string) string {
	cut := len(title)
	separator := ""
	for _, candidate := range branchTitleSeparators {
		if index := strings.Index(title, candidate); index >= 0 && index < cut {
			suffix := strings.TrimSpace(title[index+len(candidate):])
			if len(strings.Fields(suffix)) >= branchDescriptionMinClauseWords {
				cut, separator = index, candidate
			}
		}
	}
	if separator == "" {
		return title
	}
	return strings.TrimSpace(title[cut+len(separator):])
}

// Assembler carries what the rules cannot reach for themselves. Signal asks the
// supervisor to relaunch after a mode or repository-context change; this package
// must not import launch, because everything it imports is linked into the
// workbench with it.
type Assembler struct {
	Cfg    *config.Config
	Signal func(root string)
}

// Prefixes is the branch-prefix vocabulary, and the only copy of it.
func Prefixes() []string {
	return []string{"feat", "fix", "chore", "refactor", "docs", "test"}
}

// Preview is the branch a first assembly cuts, which is also the hint under the
// name field.
func Preview(d Draft) string {
	return fmt.Sprintf(branchFormat, d.Prefix, d.Slug())
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
