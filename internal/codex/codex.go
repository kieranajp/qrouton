// Package codex owns what qrouton knows about the Codex CLI's own files: its
// home, its configuration, and the one setting qrouton reasons about — subagent
// nesting depth, which the launcher raises to what a lead needs before it
// starts a Codex runner.
package codex

import (
	"bufio"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	homeEnvVar  = "CODEX_HOME"
	homeDirName = ".codex"
	configFile  = "config.toml"

	// Binary is the Codex CLI's command name, as it appears in argv[0] of a
	// launched runner and in qrouton's runner identifiers.
	Binary = "codex"

	// agentsSection and maxDepthKey address agents.max_depth, which Codex
	// accepts either under an [agents] table or as a dotted key at top level.
	agentsSection  = "agents"
	maxDepthKey    = "max_depth"
	dottedMaxDepth = agentsSection + "." + maxDepthKey

	// DefaultMaxDepth is Codex's own default when the setting is absent: one
	// level of nesting, which is too shallow for qrouton's lead-and-worker
	// topology.
	DefaultMaxDepth = 1

	// RequiredMaxDepth is the nesting a lead needs to spawn its own workers.
	RequiredMaxDepth = 2

	configComment  = "#"
	tableOpen      = "["
	tableClose     = "]"
	keyValueSep    = "="
	configFlag     = "-c"
	configFlagLong = "--config"
)

// Home is the directory Codex keeps its state in: $CODEX_HOME when set,
// otherwise ~/.codex. It is empty only if neither is discoverable.
func Home() string {
	if home := os.Getenv(homeEnvVar); home != "" {
		return home
	}
	userHome, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(userHome, homeDirName)
}

// MaxDepth returns the subagent nesting depth a Codex invocation will run with:
// the value from its configuration file, overridden by any -c/--config argument
// in argv, matching Codex's own precedence. Absent both, Codex's default.
func MaxDepth(argv []string) int {
	depth := configuredMaxDepth()
	for index := 1; index < len(argv); index++ {
		var override string
		switch {
		case argv[index] == configFlag || argv[index] == configFlagLong:
			if index+1 < len(argv) {
				index++
				override = argv[index]
			}
		case strings.HasPrefix(argv[index], configFlagLong+keyValueSep):
			override = strings.TrimPrefix(argv[index], configFlagLong+keyValueSep)
		}
		if value, ok := strings.CutPrefix(override, dottedMaxDepth+keyValueSep); ok {
			if parsed, err := strconv.Atoi(strings.TrimSpace(value)); err == nil {
				depth = parsed
			}
		}
	}
	return depth
}

// configuredMaxDepth scans config.toml for agents.max_depth. It reads the file
// line by line rather than parsing TOML: this is the only key qrouton cares
// about, and an unparseable config should leave the default standing rather than
// fail a launch.
func configuredMaxDepth() int {
	file, err := os.Open(filepath.Join(Home(), configFile))
	if err != nil {
		return DefaultMaxDepth
	}
	defer file.Close()

	depth := DefaultMaxDepth
	section := ""
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(strings.SplitN(scanner.Text(), configComment, 2)[0])
		if strings.HasPrefix(line, tableOpen) && strings.HasSuffix(line, tableClose) {
			section = strings.TrimSpace(line[len(tableOpen) : len(line)-len(tableClose)])
			continue
		}
		key, value, ok := strings.Cut(line, keyValueSep)
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		if (section == agentsSection && key == maxDepthKey) || (section == "" && key == dottedMaxDepth) {
			if parsed, err := strconv.Atoi(strings.TrimSpace(value)); err == nil {
				depth = parsed
			}
		}
	}
	return depth
}
