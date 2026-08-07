package launch

import (
	"context"
	"errors"
	"os"
	"os/exec"
)

// The seams below keep the interactive shell loop testable without starting a
// real login shell or printing a directory tree into the test process.
var (
	showShellTree = printShellTree
	runLoginShell = startLoginShell
)

// Shell runs the session's user shell. It restarts after the shell exits,
// because the affordance that matters is having a shell at all.
func Shell(ctx context.Context, dir string) error {
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
	}
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
