package launch

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

var (
	showShellTree = printShellTree
	execve        = syscall.Exec
)

// Shell prints the session's tree, then replaces this process with the user's
// login shell, so the tab's pid is the shell's pid. It returns only on failure.
func Shell(dir string) error {
	showShellTree(dir)
	shell := os.Getenv(shellEnvVar)
	if shell == "" {
		shell = defaultShell
	}
	path, err := exec.LookPath(shell)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrLoginShell, err)
	}
	if err := os.Chdir(dir); err != nil {
		return fmt.Errorf("%w: %w", ErrLoginShell, err)
	}
	if err := execve(path, []string{shell, loginShellFlag}, os.Environ()); err != nil {
		return fmt.Errorf("%w: %w", ErrLoginShell, err)
	}
	return nil
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
