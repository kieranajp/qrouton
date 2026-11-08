package versionformat

import (
	_ "embed"
	"regexp"
	"strings"
)

//go:embed grammar.txt
var grammar string

var numeric = regexp.MustCompile(strings.TrimSpace(grammar))

func Valid(value string) bool { return numeric.MatchString(value) }
