// Command thoughtsmove moves each live session made before thoughts roots into
// the extra root its repos route to. Quit qrouton before applying.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"slices"

	"github.com/kieranajp/qrouton/internal/config"
)

func main() {
	dryRun := flag.Bool("dry-run", false, "report what would change and change nothing")
	flag.Parse()
	cfg, err := config.Load()
	var results []result
	if err == nil {
		results, err = mover{cfg: cfg, dryRun: *dryRun, device: device}.run()
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if *dryRun {
		fmt.Println("dry run: nothing was changed")
	}
	report(os.Stdout, results)
	if slices.ContainsFunc(results, func(r result) bool { return r.Outcome == failed }) {
		os.Exit(1)
	}
}

func report(w io.Writer, results []result) {
	for _, o := range outcomes {
		first := true
		for _, r := range results {
			if r.Outcome != o {
				continue
			}
			if first {
				fmt.Fprintf(w, "\n%s\n", o)
				first = false
			}
			fmt.Fprintf(w, "  %-40s %s\n", r.Slug, r.Detail)
		}
	}
}
