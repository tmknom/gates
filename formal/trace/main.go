// Command trace checks the links between the IDs defined in docs/ and the
// documents, models, and tests that refer to them. Run it from the repository
// root. It exits with 1 when it reports findings and 2 when it cannot read.
package main

import (
	"fmt"
	"os"
)

func main() {
	os.Exit(run())
}

func run() int {
	findings, err := check(os.DirFS("."))
	if err != nil {
		fmt.Fprintf(os.Stderr, "trace: %v\n", err)
		return 2
	}
	for _, f := range findings {
		if _, err := fmt.Fprintln(os.Stdout, f); err != nil {
			fmt.Fprintf(os.Stderr, "trace: write findings: %v\n", err)
			return 2
		}
	}
	if len(findings) > 0 {
		fmt.Fprintf(os.Stderr, "trace: %d findings\n", len(findings))
		return 1
	}
	return 0
}
