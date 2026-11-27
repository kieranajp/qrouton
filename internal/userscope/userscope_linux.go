package userscope

import (
	"context"
	"os/exec"
	"strings"
	"time"

	"github.com/kieranajp/qrouton/internal/randomid"
)

const (
	Supported = true

	probeTimeout = 5 * time.Second
	suffixBytes  = 4
	maxSlugBytes = 64
)

// run is the probe's command seam.
var run = func(ctx context.Context, argv []string) error {
	return exec.CommandContext(ctx, argv[0], argv[1:]...).Run()
}

// Available reports whether a user manager will start a scope with every flag
// Wrap passes, so an older systemd falls back rather than failing each child.
func Available() bool {
	ctx, cancel := context.WithTimeout(context.Background(), probeTimeout)
	defer cancel()
	return run(ctx, append(prefix(), endOfFlags, probeCommand)) == nil
}

// Wrap prefixes argv so systemd-run creates the scope around itself and then
// execs argv in place, keeping its pid. --collect unloads a failed scope too.
func Wrap(argv []string, unit string, properties ...string) []string {
	out := append(prefix(), unitFlagPrefix+unit)
	for _, p := range properties {
		out = append(out, propertyFlag, p)
	}
	out = append(out, endOfFlags)
	return append(out, argv...)
}

func prefix() []string {
	return []string{systemdRun, userFlag, scopeFlag, quietFlag, noExpandFlag, collectFlag}
}

func SessionUnit(slug string) string { return unit(sessionUnitPrefix + unitSafe(slug) + "-") }

func TabUnit() string { return unit(tabUnitPrefix) }

func WorkbenchUnit() string { return unit(workbenchUnitPrefix) }

func unit(prefix string) string { return prefix + randomid.Hex(suffixBytes) + unitSuffix }

func unitSafe(slug string) string {
	if len(slug) > maxSlugBytes {
		slug = slug[:maxSlugBytes]
	}
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_', r == '.':
			return r
		}
		return '_'
	}, slug)
}
