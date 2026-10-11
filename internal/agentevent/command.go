package agentevent

import (
	"regexp"
	"strings"
)

var leadingCd = regexp.MustCompile(`^cd\s+(?:"([^"]*)"|'([^']*)'|(\S+))\s*(?:&&|;)\s*`)

// TrimCommand shortens a shell command for display: a leading cd into the
// working directory or the session drops, and absolute paths under the
// working directory or the session root are written relative to it.
func TrimCommand(command, cwd, root string) string {
	cwd, root = strings.TrimRight(cwd, "/"), strings.TrimRight(root, "/")
	base := cwd
	if m := leadingCd.FindStringSubmatch(command); m != nil {
		dir := strings.TrimRight(m[1]+m[2]+m[3], "/")
		if dir != "" && (dir == cwd || within(dir, root)) {
			if rest := command[len(m[0]):]; rest != "" {
				command, base = rest, dir
			}
		}
	}
	if base != "" {
		command = underDir(base, "").ReplaceAllString(command, "$1")
	}
	if root != "" {
		command = underDir(root, "(?:src/)?").ReplaceAllString(command, "$1")
	}
	return command
}

func within(dir, root string) bool {
	return root != "" && (dir == root || strings.HasPrefix(dir, root+"/"))
}

func underDir(dir, skip string) *regexp.Regexp {
	return regexp.MustCompile(`(^|[\s='"(:])` + regexp.QuoteMeta(dir) + "/" + skip)
}
