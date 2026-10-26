package commandline

import (
	"flag"
	"fmt"
	"strings"
)

// Version is the product version shared by every binary, matching the pinned
// Rust workspace version.
const Version = "0.6.0"

// CheckFlags enforces clap's non-repeating long options before flag.Parse.
// Go's flag package otherwise accepts duplicate options and values on switches.
func CheckFlags(fs *flag.FlagSet, args []string) error {
	seen := map[string]bool{}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" || arg == "-" || !strings.HasPrefix(arg, "-") {
			break
		}
		long := strings.HasPrefix(arg, "--")
		name := strings.TrimPrefix(arg, "-")
		if long {
			name = strings.TrimPrefix(name, "-")
		}
		name, _, hasValue := strings.Cut(name, "=")
		if !long && name != "h" && name != "V" {
			return fmt.Errorf("error: unexpected argument '%s' found", arg)
		}
		if long && (name == "h" || name == "V") {
			return fmt.Errorf("error: unexpected argument '%s' found", arg)
		}
		f := fs.Lookup(name)
		if f == nil {
			return fmt.Errorf("error: unexpected argument '%s' found", arg)
		}
		canonical := name
		if name == "h" {
			canonical = "help"
		}
		if name == "V" {
			canonical = "version"
		}
		if seen[canonical] {
			return fmt.Errorf("error: the argument '--%s' cannot be used multiple times", canonical)
		}
		seen[canonical] = true
		switchFlag, ok := f.Value.(interface{ IsBoolFlag() bool })
		isBool := ok && switchFlag.IsBoolFlag()
		if isBool && hasValue {
			return fmt.Errorf("error: unexpected value for '--%s' found; no more were expected", name)
		}
		if !isBool && !hasValue {
			if i+1 >= len(args) || (args[i+1] != "-" && strings.HasPrefix(args[i+1], "-")) {
				return fmt.Errorf("error: a value is required for '--%s' but none was supplied", name)
			}
			i++
		}
	}
	return nil
}
