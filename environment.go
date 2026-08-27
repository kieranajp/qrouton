package main

import (
	"os"
	"path/filepath"
	"strings"
)

func prepareEnvironment() {
	executable, err := os.Executable()
	if err != nil {
		return
	}
	home, _ := os.UserHomeDir()
	path, bundled := bundledPath(executable, home, os.Getenv(pathEnvVar))
	if bundled {
		_ = os.Setenv(pathEnvVar, path)
	}
}

func bundledPath(executable, home, current string) (string, bool) {
	macOSDir := filepath.Dir(filepath.Clean(executable))
	contentsDir := filepath.Dir(macOSDir)
	appDir := filepath.Dir(contentsDir)
	if filepath.Base(macOSDir) != macOSExecutablesDirName ||
		filepath.Base(contentsDir) != macOSContentsDirName ||
		filepath.Ext(appDir) != macOSAppExtension {
		return current, false
	}

	paths := filepath.SplitList(current)
	for _, parts := range bundledUserBinDirs {
		if home != "" {
			paths = append(paths, filepath.Join(append([]string{home}, parts...)...))
		}
	}
	paths = append(paths, homebrewBinDir, homebrewSbinDir, localBinDir, localSbinDir)

	seen := make(map[string]bool, len(paths))
	out := paths[:0]
	for _, path := range paths {
		if path != "" && !seen[path] {
			seen[path] = true
			out = append(out, path)
		}
	}
	return strings.Join(out, string(os.PathListSeparator)), true
}
