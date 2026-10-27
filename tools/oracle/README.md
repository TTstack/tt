# Rust compatibility oracle

`../generate-fixtures.sh RUST_CHECKOUT OUTPUT_JSON` archives the pinned reference
revision into private staging, adds the example harness, and builds it with the
reference Cargo.lock. It never edits the reference checkout. Run it on an
authorized remote build host, not the constrained development workstation.
Use the official latest stable Rust: update with `rustup update stable` and
select it with `RUSTUP_TOOLCHAIN=stable` or `cargo +stable`. This repository
defines no compatible Rust version range or minimum compiler. The pinned source
revision and Cargo.lock identify the behavioral oracle, not a compiler pin.

For phase 1 the output is `internal/api/testdata/wire.json`. The result records the
source revision and lockfile hash. It covers each wire structure and enum with
accepted/rejected presence, null, spelling, duplication and extension cases. The
Go tests compare semantic JSON with exact integers. Random default backup revisions
are checked as UUIDs then normalized in both languages; other values stay exact.
Guest-config digest inputs are compared byte-for-byte, including HTML characters,
Unicode line separators and control escapes. No database or private inventory is
included. Normal Go tests consume this file without requiring Rust.

Pass `network` as the third argument to generate
`internal/netcfg/testdata/network.json`. This compiles the reference isolation
renderer from the archived source with a fixture-only export. A firewall mutation
shim panics if invoked; the harness calls only the pure renderer and identifier
functions, including UTF-8 TAP-name input. The reference checkout stays unchanged.

Pass `engines` as the third argument to generate
`internal/engine/testdata/engines.json`. The small Go extraction helper copies
pure Rust command/configuration fragments from the archived source; the compiled
Rust example emits QEMU arguments/network YAML, Docker arguments, bhyve arguments,
jail configuration, jailer arguments and Firecracker JSON (semantic and exact
bytes). It substitutes only fixture inputs: image/runtime paths, runtime choice,
and bhyve's TAP lookup. It never launches an engine or changes networking.

All modes clean the `ttcore` Cargo artifacts before compiling the oracle. Cargo
can otherwise reuse an example from a different archived tree when source mtimes
are older than its cache. Dependency artifacts stay cached. The engine mode needs
Go as well as Rust; it introduces no Python dependency. Generation is sequential
when sharing one Cargo target directory. After the cache fix, all three modes were
rebuilt on FreeBSD; existing wire/network fixtures were byte-identical.

## Agent state and HTTP

`../build-agent-oracle.sh RUST_CHECKOUT NEW_BUILD_DIRECTORY` builds `agent-db`,
`agent-http` and the original `upgrade-agent-v5` examples at the pinned revision.
The destination must be new. Run on an authorized native build host; retain a
Cargo dependency cache and use two compilation jobs. `agent-db create|v5|check
PATH` creates or validates synthetic SQLite state using the actual Rust runtime
DB functions. Set `TT_TEST_AGENT_ORACLE` to this freshly rebuilt `agent-db` binary
to enable Go's native bidirectional interchange test. Its sibling Rust converter
must also be present.

`python3 capture-agent-http.py PATH_TO_AGENT_HTTP > OUTPUT_JSON` captures the HTTP
matrix. On the FreeBSD test host the interpreter command is `python3.11`. The
oracle starts on a temporary loopback port against private empty SQLite state,
with no available engines, background reconciliation or host setup. It cannot
admit guest creation. The capture tool terminates each owned child and removes
temporary databases. Only synthetic keys/inputs are emitted. Rust's volatile
`ruc` business-error decoration is explicitly normalized; other body/status/header
observations remain exact. Normal Go tests require neither Python nor Rust.

The HTTP matrix prompted additional core enum fixtures: serde accepts externally
tagged unit objects such as `{"qemu":null}`. The current wire fixture contains
1,677 cases, and the HTTP fixture contains 174 cases.

## Controller state and HTTP

`../build-ctl-oracle.sh RUST_CHECKOUT NEW_BUILD_DIRECTORY` builds the original
controller binary and the `ctl-db` example. `ctl-db create|v4|check PATH` creates
synthetic state or opens it using the actual Rust controller DB methods. Set
`TT_TEST_CTL_ORACLE` to the rebuilt example to enable native Go/Rust database and
migration interchange tests.

`python3 capture-ctl-http.py PATH_TO_TT_CTL > OUTPUT_JSON` captures 52 cases against
private empty state. It starts only a loopback service, registers no hosts and
cannot schedule a guest. The dashboard body is represented by a marker in this
fixture and separately pinned by its original source digest. Volatile Rust error
decoration is normalized using the same rule as the agent oracle.

## CLI output and deployment templates

`../build-cli-oracle.sh RUST_CHECKOUT NEW_BUILD_DIRECTORY` builds the pinned `tt`
and `cli-templates` example on the authorized FreeBSD host. The example only
renders systemd/OpenRC/SSH script text; it executes no deployment commands. Its
JSON output is `internal/cli/testdata/templates-rust.json`.

`python3 capture-cli-help.py PINNED_TT_BINARY OUTPUT_DIRECTORY` captures the help
tree. `python3 capture-cli-output.py PINNED_TT_BINARY WIRE_FIXTURE` emits the 24
synthetic loopback output/request cases in `internal/cli/testdata/output-rust.json`.
Both use the freshly built binary at the pinned revision. The output capture
starts no guests and performs no deployment, image download or host mutation.

## Independent audit probes

`controller-lifecycle.py PATH_TO_TT_CTL` uses a synthetic loopback agent to observe
SIGTERM during detached creation and a client disconnect after stop has reached
the agent. It never launches an agent/guest or mutates networking. Run it against
both freshly built binaries. The pinned Rust keeps the unfinished creation plan
on shutdown; both implementations finish the already-forwarded HTTP/1 stop after
disconnection.

`cli-input.py PATH_TO_TT` checks option/subcommand boundaries against a recording
loopback server. `capture-cli-help.py` now also captures six help-for-help screens,
in addition to the original 32 command help screens.

The `config-primitives` example, included by `build-agent-oracle.sh`, imports the
pinned argument parser without starting an agent. For example:

```sh
config-primitives --cpu-total 010 --mem-total +12 --disk-total 008 --port-start 02000
```

It emits decimal values 10, 12, 8 and 2000, rejects hexadecimal/binary numeric
inputs, reports rusqlite's default 5000 ms busy timeout, and shows that the prepared
root marker does not trim vertical tab. None of those checks needs a real engine.
