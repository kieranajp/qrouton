package mcpserver

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/kieranajp/qrouton/internal/session"
	"github.com/kieranajp/qrouton/internal/sessionpaths"
)

type diffRequest struct {
	staged   bool
	unstaged bool
	base     string
}

// newDiffRequest refuses a base git would read as an option, such as
// --output=<path>, which would make the diff write a file.
func newDiffRequest(input showDiffInput) (diffRequest, error) {
	base := strings.TrimSpace(input.Base)
	if strings.HasPrefix(base, gitOptionPrefix) {
		return diffRequest{}, ErrDiffBaseOption
	}
	if input.Unstaged && (input.Staged || base != "") {
		return diffRequest{}, ErrDiffUnstagedMixed
	}
	return diffRequest{staged: input.Staged, unstaged: input.Unstaged, base: base}, nil
}

func (r diffRequest) fromMergeBase() bool {
	return !r.staged && !r.unstaged && r.base == ""
}

// comparison names what an explicit request compares, the same in every repo.
func (r diffRequest) comparison() string {
	switch {
	case r.unstaged:
		return unstagedComparison
	case r.staged && r.base != "":
		return fmt.Sprintf(stagedBaseFormat, r.base)
	case r.staged:
		return stagedComparison
	}
	return fmt.Sprintf(baseComparisonFormat, r.base)
}

type diffRepo struct {
	dir   string
	name  string
	entry *session.ManifestRepo
}

type diffResult struct {
	text  string
	notes []string
}

func (r diffRequest) run(ctx context.Context, repos []diffRepo, sectioned bool) diffResult {
	var text strings.Builder
	var notes []string
	for _, repo := range repos {
		args := []string{}
		if r.staged {
			args = append(args, gitStagedFlag)
		}
		rev := r.base
		if r.fromMergeBase() {
			var note string
			rev, note = mergeBase(ctx, repo)
			notes = append(notes, note)
		}
		if rev != "" {
			args = append(args, rev)
		}
		out := gitDiff(ctx, repo.dir, append(args, gitEndOfOptions)...)
		if !sectioned {
			text.WriteString(out)
			continue
		}
		if strings.TrimSpace(out) != "" {
			fmt.Fprintf(&text, diffRepoHeaderFormat, repo.name)
			text.WriteString(out)
		}
	}
	if !r.fromMergeBase() {
		notes = []string{r.comparison()}
	}
	return diffResult{text: text.String(), notes: notes}
}

// mergeBase picks the revision a repo's own work is measured from. Without one
// the working tree is compared with HEAD, so uncommitted work still shows.
func mergeBase(ctx context.Context, repo diffRepo) (string, string) {
	if repo.entry == nil {
		return gitHead, fmt.Sprintf(headFallbackFormat, repo.name, noManifestEntryReason)
	}
	ref := repo.entry.BaseRef()
	if ref == "" {
		return gitHead, fmt.Sprintf(headFallbackFormat, repo.name, noBaseBranchReason)
	}
	out, err := exec.CommandContext(ctx, gitBin, gitDirFlag, repo.dir, gitMergeBaseCmd, ref, gitHead).Output()
	rev := strings.TrimSpace(string(out))
	if err != nil || rev == "" {
		return gitHead, fmt.Sprintf(headFallbackFormat, repo.name, fmt.Sprintf(noMergeBaseFormat, ref))
	}
	return rev, fmt.Sprintf(mergeBaseNoteFormat, repo.name, ref, rev[:min(len(rev), shortRevision)])
}

// gitDiff returns everything git printed, stderr included: a failed diff is
// the window's content.
func gitDiff(ctx context.Context, dir string, args ...string) string {
	out, _ := exec.CommandContext(ctx, gitBin, append([]string{gitDirFlag, dir, gitNoPager, gitDiffCmd}, args...)...).CombinedOutput()
	return string(out)
}

// sessionRepos lists every git worktree under src, leaving out reference repos.
// An unreadable manifest leaves each repo unmatched rather than failing the call.
func sessionRepos(ctx context.Context, root string) []diffRepo {
	entries, _ := os.ReadDir(sessionpaths.Src(root))
	manifest := manifestByDir(root)
	repos := make([]diffRepo, 0, len(entries))
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), hiddenPrefix) {
			continue
		}
		dir := filepath.Join(sessionpaths.Src(root), entry.Name())
		if info, err := os.Stat(dir); err != nil || !info.IsDir() {
			continue
		}
		if exec.CommandContext(ctx, gitBin, gitDirFlag, dir, gitRevParseCmd, gitGitDirFlag).Run() != nil {
			continue
		}
		repo := diffRepo{dir: dir, name: filepath.Join(sessionpaths.SrcDirName, entry.Name()), entry: manifest[realPath(dir)]}
		if repo.entry != nil && repo.entry.Role.Effective() == session.RepoRoleReference {
			continue
		}
		repos = append(repos, repo)
	}
	return repos
}

func oneRepo(root, dir, name string) []diffRepo {
	return []diffRepo{{dir: dir, name: name, entry: manifestByDir(root)[realPath(dir)]}}
}

func manifestByDir(root string) map[string]*session.ManifestRepo {
	m, err := session.Load(root)
	if err != nil {
		return nil
	}
	byDir := make(map[string]*session.ManifestRepo, len(m.Repos))
	for i := range m.Repos {
		byDir[realPath(m.Repos[i].Dir(root))] = &m.Repos[i]
	}
	return byDir
}

func realPath(path string) string {
	if real, err := filepath.EvalSymlinks(path); err == nil {
		return real
	}
	return filepath.Clean(path)
}

func compared(notes []string) string {
	if len(notes) == 0 {
		return ""
	}
	return fmt.Sprintf(comparedFormat, strings.Join(notes, comparedJoiner))
}
