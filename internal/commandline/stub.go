// Package commandline holds common command-line plumbing.
package commandline

import (
	"errors"
	"flag"
	"fmt"
	"io"
)

const Version = "0.6.0"

// Stub parses the phase-0 entry point. It deliberately refuses service startup
// until its implementation phase; help and version are available immediately.
func Stub(fs *flag.FlagSet, args []string, help string, stdout, stderr io.Writer) int {
	fs.SetOutput(stderr)
	var showHelp, version bool
	fs.BoolVar(&showHelp, "help", false, "Print help")
	fs.BoolVar(&showHelp, "h", false, "Print help")
	fs.BoolVar(&version, "version", false, "Print version")
	fs.BoolVar(&version, "V", false, "Print version")
	fs.Usage = func() { fmt.Fprint(stderr, help) }
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if showHelp {
		fmt.Fprint(stdout, help)
		return 0
	}
	if version {
		fmt.Fprintf(stdout, "%s %s\n", fs.Name(), Version)
		return 0
	}
	fmt.Fprintf(stderr, "%s: implementation pending\n", fs.Name())
	return 1
}
