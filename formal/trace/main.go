// Command trace checks the links between the IDs defined in docs/ and the
// documents, models, and tests that refer to them. Run it from the repository
// root. It exits with 1 when it reports findings and 2 when it cannot run.
// With -map, it writes which annotations formalize and check each clause and
// GOJ instead.
package main

import (
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
)

func main() {
	os.Exit(run(os.Args[1:], os.DirFS("."), os.Stdout, os.Stderr))
}

func run(args []string, fsys fs.FS, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("trace", flag.ContinueOnError)
	flags.SetOutput(stderr)
	printMap := flags.Bool("map", false, "write which annotations formalize and check each clause and GOJ")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *printMap {
		l, err := read(fsys)
		if err == nil {
			err = writeMap(stdout, l)
		}
		if err != nil {
			writeStderr(stderr, "trace: %v\n", err)
			return 2
		}
		return 0
	}
	findings, err := check(fsys)
	if err != nil {
		writeStderr(stderr, "trace: %v\n", err)
		return 2
	}
	for _, f := range findings {
		if _, err := fmt.Fprintln(stdout, f); err != nil {
			writeStderr(stderr, "trace: write findings: %v\n", err)
			return 2
		}
	}
	if len(findings) > 0 {
		writeStderr(stderr, "trace: %d findings\n", len(findings))
		return 1
	}
	return 0
}

// writeStderr writes a message for the person running the check. The exit code
// already tells the result, so a failure to write the message is ignored.
func writeStderr(stderr io.Writer, format string, args ...any) {
	_, _ = fmt.Fprintf(stderr, format, args...)
}
