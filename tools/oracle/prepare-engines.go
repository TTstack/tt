//go:build ignore

// Extract pure construction fragments from the pinned archived Rust tree.
// Host operations are excluded. No Go engine output supplies expected values.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	if len(os.Args) != 2 {
		panic("usage: prepare-engines.go ARCHIVED_RUST_TREE")
	}
	root := os.Args[1]
	read := func(name string) string {
		data, err := os.ReadFile(filepath.Join(root, "crates/core/src/engine", name))
		if err != nil {
			panic(err)
		}
		return string(data)
	}
	qemu, docker, bhyve, jail, fc, sandbox := read("qemu.rs"), read("docker.rs"), read("bhyve.rs"), read("jail.rs"), read("firecracker.rs"), read("firecracker/sandbox.rs")
	var out strings.Builder
	out.WriteString(`use ruc::*;
use std::path::Path;
use std::process::Command;
use ttcore::model::{Vm, RUN_DIR};
mod net {
 pub use ttcore::net::tap_name;
 pub fn bhyve_tap_device(_: &str) -> ruc::Result<String> { Ok("tap42".into()) }
}
struct Qemu;
impl Qemu {
`)
	for _, name := range []string{"build_cmd", "mac_address", "network_config", "pid_path", "monitor_path", "seed_path"} {
		out.WriteString(method(qemu, name))
	}
	out.WriteString("}\nfn configuration(" + between(fc, "fn configuration(", "\n}\n") + "\n}\n")
	out.WriteString("struct Docker;\nimpl Docker {\n" + method(docker, "container_name"))
	out.WriteString("fn fixture(vm: &Vm, rt: &str) -> Command {\nlet name = Self::container_name(vm);\n")
	fragment := between(docker, "        let name = Self::container_name(vm);", "        let output = cmd.bounded_output()")
	out.WriteString(strings.ReplaceAll(fragment, "        let rt = self.runtime();", "") + "\ncmd\n}\n}\n")
	out.WriteString("fn bhyve_command(vm: &Vm, disk: &Path) -> Result<Command> {\nlet mut cmd = Command::new(\"bhyve\");\ncmd")
	out.WriteString(between(bhyve, "let mut child = Command::new(\"bhyve\")", "            .stdin(Stdio::null())") + ";\nOk(cmd)\n}\n")
	out.WriteString("struct Jail;\nimpl Jail {\n" + method(jail, "jail_name"))
	out.WriteString("fn fixture(vm: &Vm, root_str: &str) -> Result<String> {\nlet config = format!(")
	out.WriteString(between(jail, "        let config = format!(", "        std::fs::write(Self::config_path(vm), config)") + "\nOk(config)\n}\n}\n")
	out.WriteString(`struct Sandbox { marker: String }
fn jailer_command(vm: &Vm) -> Command {
let jailer = "/tools/jailer";
let binary = "/tools/firecracker";
let id = net::tap_name(&vm.id);
let octets = vm.ip.parse::<std::net::Ipv4Addr>().unwrap().octets();
let uid = 100_000 + u32::from(octets[2])*256 + u32::from(octets[3]);
let base = "/images/.jailer";
let sandbox = Sandbox { marker: format!("/fc-{id}.sock") };
let mut command = Command::new(jailer);
`)
	out.WriteString(between(sandbox, "        let mut command = Command::new(jailer);", "        Ok((sandbox, command))") + "\ncommand\n}\n")
	if err := os.WriteFile(filepath.Join(root, "crates/core/examples/engine-fragments.rs"), []byte(out.String()), 0600); err != nil {
		panic(err)
	}
}
func between(text, start, end string) string {
	_, tail, ok := strings.Cut(text, start)
	if !ok {
		panic(fmt.Sprintf("missing start: %q", start))
	}
	middle, _, ok := strings.Cut(tail, end)
	if !ok {
		panic(fmt.Sprintf("missing end: %q", end))
	}
	return middle
}
func method(text, name string) string {
	start := "    fn " + name + "("
	return start + between(text, start, "\n    }") + "\n    }\n"
}
