package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/kieranajp/qrouton/internal/commentdiscipline"
)

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "commentdiscipline:", err)
		os.Exit(1)
	}
}

func run(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("commentdiscipline", flag.ContinueOnError)
	flags.SetOutput(stderr)
	root := flags.String("root", "", "repository root")
	policyPath := flags.String("policy", "", "comment discipline policy")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if *root == "" || *policyPath == "" {
		return fmt.Errorf("both -root and -policy are required")
	}
	policy, err := commentdiscipline.LoadPolicy(*policyPath)
	if err != nil {
		return err
	}
	diagnostics, err := commentdiscipline.CheckGoTree(*root, policy)
	if err != nil {
		return err
	}
	for _, diagnostic := range diagnostics {
		fmt.Fprintln(stdout, diagnostic.String())
	}
	if len(diagnostics) > 0 {
		return fmt.Errorf("found %d comment discipline violation(s)", len(diagnostics))
	}
	return nil
}
