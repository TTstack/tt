package cli

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type option struct {
	short            string
	kind             string
	repeat, required bool
	fallback         string
}
type commandSpec struct {
	position         string
	optionalPosition bool
	flags            map[string]option
}

var commandSpecs = map[string]commandSpec{
	"config": {position: "ADDR"}, "capabilities": {flags: map[string]option{"json": {kind: "bool"}}}, "status": {},
	"host add": {position: "ADDR"}, "host list": {}, "host show": {position: "ID"}, "host remove": {position: "ID"}, "host detach": {position: "ID"},
	"env create": {position: "NAME", flags: map[string]option{
		"image": {short: "i", kind: "string", repeat: true, required: true}, "engine": {kind: "string", fallback: "qemu"}, "cpu": {kind: "u32"}, "mem": {kind: "u32"}, "disk": {kind: "u32"}, "dup": {kind: "u32", fallback: "1"}, "port": {short: "p", kind: "u16", repeat: true}, "lifetime": {kind: "u64"}, "deny-outgoing": {kind: "bool"}, "isolated-network": {kind: "bool"}, "guest-config": {kind: "string"}, "owner": {kind: "string"}, "ssh-key": {kind: "string", repeat: true}, "ssh-user": {kind: "string", fallback: "root"}, "ssh-sudo": {kind: "bool"},
	}},
	"env list": {}, "env show": {position: "NAME"}, "env delete": {position: "NAME"}, "env stop": {position: "NAME"}, "env start": {position: "NAME"},
	"env resize":        {position: "VM_ID", flags: map[string]option{"cpu": {kind: "u32", required: true}, "mem": {kind: "u32", required: true}, "disk": {kind: "u32", required: true}}},
	"env backup show":   {position: "VM_ID"},
	"env backup create": {position: "VM_ID", flags: backupOptions(false)}, "env backup delete": {position: "VM_ID", flags: backupOptions(false)}, "env backup restore": {position: "VM_ID", flags: backupOptions(true)},
	"image list": {}, "image recipes": {}, "image create": {position: "NAME", flags: map[string]option{"image-dir": {kind: "string", fallback: "/home/ttstack/images"}, "engine": {kind: "string"}}},
	"deploy agent": {flags: map[string]option{"release-dir": {kind: "string", fallback: "./target/release"}}}, "deploy ctl": {flags: map[string]option{"release-dir": {kind: "string", fallback: "./target/release"}}}, "deploy all": {flags: map[string]option{"release-dir": {kind: "string", fallback: "./target/release"}}}, "deploy dist": {position: "CONFIG", optionalPosition: true},
}
var commandChildren = map[string][]string{"": {"config", "capabilities", "status", "host", "env", "image", "deploy"}, "host": {"add", "list", "show", "remove", "detach"}, "env": {"backup", "create", "list", "show", "delete", "stop", "start", "resize"}, "env backup": {"show", "create", "restore", "delete"}, "image": {"list", "recipes", "create"}, "deploy": {"agent", "ctl", "all", "dist"}}

func backupOptions(restore bool) map[string]option {
	flags := map[string]option{"operation-id": {kind: "string"}, "expected-revision": {kind: "string"}}
	if restore {
		flags["generation"] = option{kind: "string", required: true}
	}
	return flags
}

type parsed struct {
	path, argument string
	values         map[string][]string
	server, key    *string
	help, version  bool
}

func (p parsed) value(name string) string {
	values := p.values[name]
	if len(values) == 0 {
		return ""
	}
	return values[len(values)-1]
}
func (p parsed) has(name string) bool { return len(p.values[name]) > 0 }
func (p parsed) number(name string) uint64 {
	n, _ := strconv.ParseUint(strings.TrimPrefix(p.value(name), "+"), 10, 64)
	return n
}
func (p parsed) optional32(name string) *uint32 {
	if !p.has(name) {
		return nil
	}
	value := uint32(p.number(name))
	return &value
}
func parse(args []string) (parsed, error) {
	result := parsed{values: map[string][]string{}}
	if key, ok := os.LookupEnv("TT_API_KEY"); ok {
		result.key = &key
	}
	seen := map[string]bool{}
	positionals := []string{}
	literal := false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if !literal && arg == "--" {
			literal = true
			continue
		}
		if !literal && strings.HasPrefix(arg, "-") && arg != "-" {
			if arg == "--help" || arg == "-h" {
				result.help = true
				return result, nil
			}
			if (arg == "--version" || arg == "-V") && result.path == "" {
				result.version = true
				return result, nil
			}
			name, value, attached := strings.Cut(strings.TrimPrefix(arg, "--"), "=")
			short := !strings.HasPrefix(arg, "--")
			if short {
				name = arg[1:2]
				if len(arg) > 2 {
					value = arg[2:]
					value = strings.TrimPrefix(value, "=")
					attached = true
				}
			}
			var spec option
			known := false
			if name == "server" || (short && name == "s") {
				name = "server"
				spec = option{kind: "string"}
				known = true
			} else if name == "api-key" || (short && name == "k") {
				name = "api-key"
				spec = option{kind: "string"}
				known = true
			} else {
				flags := commandSpecs[result.path].flags
				if short {
					for key, candidate := range flags {
						if candidate.short == name {
							name = key
							spec = candidate
							known = true
							break
						}
					}
				} else {
					spec, known = flags[name]
				}
			}
			if !known {
				return result, fmt.Errorf("error: unexpected argument '%s' found", strings.SplitN(arg, "=", 2)[0])
			}
			if seen[name] && !spec.repeat {
				return result, fmt.Errorf("error: the argument '--%s' cannot be used multiple times", name)
			}
			seen[name] = true
			if spec.kind == "bool" {
				if attached {
					return result, fmt.Errorf("error: unexpected value for '--%s' found; no more were expected", name)
				}
				value = "true"
			} else if !attached {
				if i+1 >= len(args) || strings.HasPrefix(args[i+1], "-") {
					return result, fmt.Errorf("error: a value is required for '--%s' but none was supplied", name)
				}
				i++
				value = args[i]
			}
			if strings.HasPrefix(spec.kind, "u") {
				bits, _ := strconv.Atoi(spec.kind[1:])
				if _, err := strconv.ParseUint(strings.TrimPrefix(value, "+"), 10, bits); err != nil {
					return result, fmt.Errorf("error: invalid value for '--%s': %s", name, err)
				}
			}
			if name == "server" {
				copy := value
				result.server = &copy
			} else if name == "api-key" {
				copy := value
				result.key = &copy
			} else {
				result.values[name] = append(result.values[name], value)
			}
			continue
		}
		children, branch := commandChildren[result.path]
		if branch {
			if arg == "help" && !literal {
				for _, child := range args[i+1:] {
					next := strings.TrimSpace(result.path + " " + child)
					if _, ok := commandSpecs[next]; !ok {
						if _, ok := commandChildren[next]; !ok {
							return result, fmt.Errorf("error: unrecognized subcommand '%s'", child)
						}
					}
					result.path = next
				}
				result.help = true
				return result, nil
			}
			valid := false
			for _, child := range children {
				valid = valid || arg == child
			}
			if !valid {
				return result, fmt.Errorf("error: unrecognized subcommand '%s'", arg)
			}
			result.path = strings.TrimSpace(result.path + " " + arg)
			continue
		}
		positionals = append(positionals, arg)
	}
	if _, branch := commandChildren[result.path]; branch {
		return result, fmt.Errorf("error: a subcommand is required")
	}
	spec, ok := commandSpecs[result.path]
	if !ok {
		return result, fmt.Errorf("error: a subcommand is required")
	}
	if len(positionals) > 1 || (len(positionals) > 0 && spec.position == "") {
		return result, fmt.Errorf("error: unexpected argument '%s' found", positionals[len(positionals)-1])
	}
	if len(positionals) == 1 {
		result.argument = positionals[0]
	} else if spec.position != "" && !spec.optionalPosition {
		return result, fmt.Errorf("error: the following required arguments were not provided:\n  <%s>", spec.position)
	} else if result.path == "deploy dist" {
		result.argument = "deploy.toml"
	}
	for name, flag := range spec.flags {
		if !result.has(name) {
			if flag.required {
				return result, fmt.Errorf("error: the following required arguments were not provided:\n  --%s <%s>", name, strings.ToUpper(strings.ReplaceAll(name, "-", "_")))
			}
			if flag.fallback != "" {
				result.values[name] = []string{flag.fallback}
			}
		}
	}
	if strings.HasPrefix(result.path, "env backup ") && result.has("operation-id") != result.has("expected-revision") {
		return result, fmt.Errorf("error: --operation-id and --expected-revision must be supplied together")
	}
	return result, nil
}
