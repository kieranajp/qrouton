package mux

// The shell stack: pane enumeration and the numbered-shell bookkeeping behind
// Alt-g, driven from inside a Zellij pane rather than through a marshalled
// Handle.

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// CurrentShellStack returns pane control for the shell command running inside a
// Zellij pane. Unlike agent pane tools, this command does not need a marshalled
// handle: Zellij itself supplies the current session and pane IDs.
func CurrentShellStack() (ShellStack, error) {
	session := os.Getenv(zellijSessionEnvVar)
	paneID := terminalPaneID(os.Getenv(zellijPaneIDEnvVar))
	if session == "" || paneID == "" {
		return nil, ErrShellContext
	}
	bin, err := exec.LookPath(zellijBin)
	if err != nil {
		return nil, fmt.Errorf("zellij is unavailable")
	}
	return &zellijShellStack{
		zellijHost: zellijHost{bin: bin, session: session},
		currentID:  paneID,
	}, nil
}

type zellijShellStack struct {
	zellijHost
	currentID string
}

// JoinCurrent names this shell and stacks it with every existing shell pane.
// Existing IDs stay in Zellij's layout order so the original shell remains the
// stable anchor for the right-hand region when a shell was initially spawned
// beside the agent.
func (z *zellijShellStack) JoinCurrent(ctx context.Context, titlePrefix, titleSuffix string) (int, error) {
	panes, err := z.panes(ctx)
	if err != nil {
		return 0, err
	}
	next := 1
	var shellIDs []string
	for _, pane := range panes {
		if pane.IsPlugin || pane.IsFloating || pane.Exited {
			continue
		}
		id := pane.paneID()
		if id == z.currentID {
			continue
		}
		number, ok := shellNumber(pane.Title, titlePrefix)
		if !ok {
			continue
		}
		if number >= next {
			next = number + 1
		}
		shellIDs = append(shellIDs, id)
	}
	title := fmt.Sprintf("%s %d%s", titlePrefix, next, titleSuffix)
	if len(shellIDs) > 0 {
		shellIDs = append(shellIDs, z.currentID)
		if _, err := z.action(ctx, append([]string{stackPanesAction, endOfFlags}, shellIDs...)...); err != nil {
			return 0, err
		}
	}
	// Stack first: Zellij can redraw a newly-stacked pane with the name from its
	// Run action. Renaming last makes the numbered title and its controls the
	// final state the user sees.
	if _, err := z.action(ctx, renamePaneAction, paneIDFlag, z.currentID, title); err != nil {
		return 0, err
	}
	return next, nil
}

func (z *zellijShellStack) Count(ctx context.Context, titlePrefix string) (int, error) {
	panes, err := z.panes(ctx)
	if err != nil {
		return 0, err
	}
	count := 0
	for _, pane := range panes {
		if pane.IsPlugin || pane.IsFloating || pane.Exited {
			continue
		}
		if _, ok := shellNumber(pane.Title, titlePrefix); ok {
			count++
		}
	}
	return count, nil
}

func shellNumber(title, prefix string) (int, bool) {
	// Attached sessions can outlive the binary and staged layout that created
	// them. Before the shell-stack refactor the permanent pane was named
	// "shell · Alt-g"; the first stack-aware launch begins as plain "shell"
	// until this method renames it. Treat either as shell 1 so a freshly-staged
	// Alt-g can migrate into the old right-hand region instead of stacking with
	// whichever pane currently has focus.
	if title == prefix || strings.HasPrefix(title, prefix+" ·") {
		return 1, true
	}
	rest, ok := strings.CutPrefix(title, prefix+" ")
	if !ok {
		return 0, false
	}
	field := strings.Fields(rest)
	if len(field) == 0 {
		return 0, false
	}
	number, err := strconv.Atoi(field[0])
	return number, err == nil && number > 0
}
