// Package ctl implements the TTstack controller.
package ctl

import (
	_ "embed"
	"flag"
	"io"
	"os"

	"github.com/TTstack/tt/internal/commandline"
)

//go:embed help.txt
var help string

type Config struct {
	Listen  string
	DataDir string
	APIKey  string
}

func Run(args []string, stdout, stderr io.Writer) int {
	var cfg Config
	fs := flag.NewFlagSet("tt-ctl", flag.ContinueOnError)
	fs.StringVar(&cfg.Listen, "listen", "0.0.0.0:9200", "Listen address for the HTTP API")
	fs.StringVar(&cfg.DataDir, "data-dir", "/home/ttstack/ctl", "Directory for persistent state")
	fs.StringVar(&cfg.APIKey, "api-key", os.Getenv("TT_API_KEY"), "API key for authentication")
	return commandline.Stub(fs, args, help, stdout, stderr)
}
