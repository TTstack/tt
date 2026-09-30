// Package cli implements the TTstack command-line client.
package cli

import (
	_ "embed"
	"flag"
	"io"
	"os"

	"github.com/TTstack/tt/internal/commandline"
)

//go:embed help.txt
var help string

func Run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("tt", flag.ContinueOnError)
	var server, apiKey string
	fs.StringVar(&server, "server", "", "Controller address")
	fs.StringVar(&server, "s", "", "Controller address")
	fs.StringVar(&apiKey, "api-key", os.Getenv("TT_API_KEY"), "API key for controller authentication")
	fs.StringVar(&apiKey, "k", os.Getenv("TT_API_KEY"), "API key for controller authentication")
	return commandline.Stub(fs, args, help, stdout, stderr)
}
