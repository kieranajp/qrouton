package assembly

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/kieranajp/qrouton/internal/session"
	"github.com/kieranajp/qrouton/internal/ticket"
)

// Field names the control a problem belongs under, so the footer status can be
// read against the thing that has to change.
type Field string

const (
	FieldName   Field = "name"
	FieldTicket Field = "ticket"
	FieldRepos  Field = "repos"
)

// Problem is one reason a draft cannot be assembled yet, carrying its own copy.
type Problem struct {
	Field   Field  `json:"field"`
	Message string `json:"message"`
}

// Check is everything a draft can be judged on without touching disk, so it can
// run on every keystroke.
func Check(d Draft) []Problem {
	var problems []Problem
	if url := strings.TrimSpace(d.Ticket); url != "" {
		if _, err := ticket.ParseURL(url); err != nil {
			problems = append(problems, Problem{Field: FieldTicket, Message: err.Error()})
		}
	}
	if session.Slugify(d.Name) == "" {
		problems = append(problems, Problem{Field: FieldName, Message: msgNameRequired})
	}
	return append(problems, checkRepos(d)...)
}

// CheckAdditions is the only one of these rules the picker shares. A session that
// exists has already been named and branched, so its name and ticket are not the
// picker's to judge again — and one already holding an editing repo cannot be
// made to lack one by adding to it, so only a session still without one has to be
// given one here.
func CheckAdditions(m session.Manifest, d Draft) []Problem {
	if holdsEditingRepo(m) || len(d.Upgrades) > 0 {
		return nil
	}
	return checkRepos(d)
}

func checkRepos(d Draft) []Problem {
	if hasEditingRepo(d) {
		return nil
	}
	return []Problem{{Field: FieldRepos, Message: msgNoEditingRepo}}
}

// CheckSlug is the half that stats the disk, so it runs on advance rather than
// on every keystroke. An abandoned half-assembly does not block the name;
// session.Create reclaims it.
func (a Assembler) CheckSlug(d Draft) []Problem {
	slug := d.Slug()
	if slug == "" {
		return []Problem{{Field: FieldName, Message: msgNameRequired}}
	}
	dir := filepath.Join(a.Cfg.Root, slug)
	if session.Abandoned(dir) {
		return nil
	}
	if _, err := os.Stat(dir); err == nil {
		return []Problem{{Field: FieldName, Message: msgSessionExists}}
	}
	return nil
}

func hasEditingRepo(d Draft) bool {
	return anyEditing(d.Repos, func(sel session.RepoSelection) session.RepoRole { return sel.Role })
}

func holdsEditingRepo(m session.Manifest) bool {
	return anyEditing(m.Repos, func(r session.ManifestRepo) session.RepoRole { return r.Role })
}

func anyEditing[T any](repos []T, role func(T) session.RepoRole) bool {
	for _, repo := range repos {
		if role(repo).Effective() == session.RepoRoleEditing {
			return true
		}
	}
	return false
}
