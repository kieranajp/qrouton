package launch

import (
	"context"
	"os"
	"os/exec"
)

// The seams below keep the interactive shell testable without starting a real
// login shell or printing a directory tree into the test process.
var (
	showShellTree = printShellTree
	runLoginShell = startLoginShell
)

// Shell runs the session's user shell; the tab ends when it exits.
func Shell(ctx context.Context, dir string) error {
	showShellTree(dir)
	return runLoginShell(ctx, dir)
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
