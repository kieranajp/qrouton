package launch

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"

	"github.com/kieranajp/qrouton/internal/mux"
)

// The seams below keep the interactive shell loop testable without starting a
// real login shell or printing a directory tree into the test process.
var (
	showShellTree = printShellTree
	runLoginShell = startLoginShell
)

// Shell runs one member of the permanent right-hand shell stack. The pane
// joins that stack before becoming interactive, which is what lets Alt-g work
// from the agent or any other tiled pane without changing the workspace
// geometry. The final shell restarts after exit so the region never disappears.
func Shell(ctx context.Context, dir string, stack mux.ShellStack) error {
	if _, err := JoinShellStack(ctx, stack, shellPaneTitleSuffix); err != nil {
		return fmt.Errorf("%s: %w", joinShellError, err)
	}
	showShellTree(dir)
	for {
		err := runLoginShell(ctx, dir)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		var exitErr *exec.ExitError
		if err != nil && !errors.As(err, &exitErr) {
			return err
		}
		count, err := stack.Count(ctx, shellPaneName)
		if err != nil {
			return fmt.Errorf("%s: %w", countShellError, err)
		}
		if count > 1 {
			return nil
		}
		fmt.Fprintln(os.Stdout, lastShellMessage)
	}
}

// JoinShellStack is shared with the short-lived de-escalation command: it also
// runs in the permanent shell region so quick reference and the escalation
// picker remain the only user-driven floating panes.
func JoinShellStack(ctx context.Context, stack mux.ShellStack, titleSuffix string) (int, error) {
	return stack.JoinCurrent(ctx, shellPaneName, titleSuffix)
}

func printShellTree(dir string) {
	var cmd *exec.Cmd
	if tree, err := exec.LookPath(treeCommand); err == nil {
		cmd = exec.Command(tree, treeDepthFlag, treeDepth, treeColourFlag)
	} else {
		cmd = exec.Command(findCommand, findRoot, findDepthFlag, findDepth, findPrintFlag)
	}
	cmd.Dir = dir
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	_ = cmd.Run()
}

func startLoginShell(ctx context.Context, dir string) error {
	shell := os.Getenv(shellEnvVar)
	if shell == "" {
		shell = defaultShell
	}
	cmd := exec.CommandContext(ctx, shell, loginShellFlag)
	cmd.Dir = dir
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	return cmd.Run()
}
