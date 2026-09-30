package agent

import (
	"flag"
	"io"

	"github.com/TTstack/tt/internal/commandline"
)

func RunUpgrade(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("upgrade-agent-v5", flag.ContinueOnError)
	fs.String("source", "", "Source database")
	fs.String("output", "", "New destination database")
	return commandline.Stub(fs, args, "Copy stopped-agent v5 metadata to a new v6 database without replacing the source\n\nUsage: upgrade-agent-v5 --source <SOURCE> --output <OUTPUT>\n\nOptions:\n      --source <SOURCE>\n      --output <OUTPUT>\n  -h, --help             Print help\n", stdout, stderr)
}
