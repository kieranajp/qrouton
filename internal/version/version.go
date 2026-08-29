// Package version is the build stamp and the ordering the update floor is
// decided by. Releases are tagged `vN.N.N` and the macOS bundle carries the
// same number, so one comparison serves both the tag a release advertises and
// the string an installed app reports.
package version

import (
	"strconv"
	"strings"
)

// Current is the tag this binary was cut from, stamped by the release build
// through -ldflags. An unstamped build reports Development, which is below
// every release and above every floor: a working tree is not a version anyone
// can be held to.
var Current = Development

// Trim drops the tag's leading v, so a release tag and a bundle's
// CFBundleShortVersionString compare as the same string.
func Trim(v string) string {
	return strings.TrimPrefix(strings.TrimSpace(v), tagPrefix)
}

// Released reports whether v is a stamped release rather than a working tree.
func Released(v string) bool {
	return Trim(v) != Development && Trim(v) != ""
}

// Before reports whether a orders before b. Both are trimmed first. Segments
// are compared numerically and a missing one reads as zero, so 0.4 is neither
// before nor after 0.4.0. Anything unparseable orders first, which keeps a
// malformed version from being mistaken for a new one.
func Before(a, b string) bool {
	return compare(segments(Trim(a)), segments(Trim(b))) < 0
}

func compare(a, b []int) int {
	for i := 0; i < len(a) || i < len(b); i++ {
		x, y := at(a, i), at(b, i)
		if x != y {
			if x < y {
				return -1
			}
			return 1
		}
	}
	return 0
}

func at(s []int, i int) int {
	if i < len(s) {
		return s[i]
	}
	return 0
}

// segments splits a dotted numeric version. A segment that is not a number
// truncates the parse, so "0.4.0-rc1" reads as 0.4 rather than as a number the
// comparison would have to invent an ordering for.
func segments(v string) []int {
	if v == "" || v == Development {
		return nil
	}
	var out []int
	for _, part := range strings.Split(v, ".") {
		n, err := strconv.Atoi(part)
		if err != nil {
			break
		}
		out = append(out, n)
	}
	return out
}
