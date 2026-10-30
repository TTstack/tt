# Development

## Prerequisites

- Go 1.26 or newer (`go version`).
- For tests that build configuration disks: `e2fsprogs` (`mkfs.ext4`, `debugfs`,
  `e2fsck`, `resize2fs`, `dumpe2fs`). Tests skip themselves when the tools are
  missing.
- For host-side manual testing: QEMU/KVM, Firecracker + jailer, Docker or Podman,
  `nftables`/`iproute2`, ZFS — only the ones matching the engines you exercise.

## Commands

```bash
make build     # debug binaries into ./bin
make release   # optimized binaries
make test      # go test ./...
make lint      # gofmt check + go vet
make fmt       # gofmt -w
make clean
make install PREFIX=/opt/ttstack   # copy release binaries
make help
```

Direct equivalents:

```bash
go build ./...
go vet ./...
go test ./...
gofmt -l .
go test ./internal/ctl/ -run TestScheduler -v
```

On the shared development workstation, run lightweight checks sequentially with
`GOMAXPROCS=2` and `-p 1` (`go vet -p 1 ./...`, `go test -p 1 ./...`,
`go build -p 1 ./...`). Do not create local VM/container/network infrastructure.
Real tool/disk tests are opt-in, and heavy Rust reference builds, integration and
race suites run on authorized remote test machines or CI. Live testing starts on
the dedicated FreeBSD host; Linux testing follows stable basic flows, within the
resource and namespace limits in [roadmap.md](roadmap.md#live-validation-order-and-isolation).
The [AI workflow validation matrix](ai-workflow.md#go-validation) distinguishes
documentation-only work, ordinary Go gates, cross-builds and native acceptance,
and lists the opt-in test environment variables.

## DB schema policy before v1.0

The project has no formal deployment yet. Before v1.0, database changes maintain
only the latest development schema; compatibility with earlier development
databases is not required.

Keep the existing schema-version fields, checks and migration infrastructure for
future stable releases. Do not increment `schemaVersion` or `agentSchemaVersion`,
or add historical migration steps, for pre-v1.0 feature iterations. Update the
current schema definitions and their tests/fixtures directly. The existing
controller/agent markers remain 9/10; equal markers across development commits
do not guarantee identical layouts or compatibility. Retained migrations cover
only their implemented cases and do not create a compatibility obligation.

When a layout change makes a development database obsolete, recreate the
explicitly owned test state as needed. Do not silently delete or repair existing
databases, guest disks, ownership records or reservations. Resource ownership and
unknown-outcome safeguards still apply.

Revisit schema version increments and supported upgrade paths for the v1.0
release. Until then, schema compatibility does not gate development changes.

## Dependencies

The direct dependency set is intentionally closed. Adding a direct module requires
a note in this document and a reviewer-visible reason. Required indirect modules
from these dependencies are allowed and pinned with checksums; "four modules" does
not mean SQLite has no transitive dependencies. Use versions compatible with Go
1.26 and test with that minimum toolchain, not just the developer's installed Go.

| Module | Use |
|---|---|
| `modernc.org/sqlite` | SQLite driver (pure Go, no cgo); selected to avoid a C toolchain |
| `github.com/google/uuid` | UUID v4 identifiers |
| `github.com/BurntSushi/toml` | `deploy.toml` parsing |
| `golang.org/x/sys/unix` | `flock`, TUN ioctls, `FICLONE`, `statfs` |

The SQLite driver is now imported by the agent. Its required pure-Go indirect
modules are recorded in `go.mod`; the `modernc.org/libc` module does not introduce
CGo into production builds. No additional direct dependency was added.

Everything else comes from the standard library. In particular: no CLI framework
(explicit subcommand tree), no external HTTP router (explicit raw-path routing), no logging framework
(`log` to stderr), no assertion library (plain `testing`).

## Repository conventions

- `cmd/<binary>` contains only startup wiring; behavior lives in `internal/`.
- `internal/` packages are dependency-ordered: `model`/`api` at the bottom,
  `agent`/`ctl`/`cli` at the top. No import cycles; `internal/agent` and
  `internal/ctl` never import each other.
- Embedded wire types live in the leaf `model` package; protocol policy stays in
  its behavior package. Follow the concrete graph in [architecture.md](architecture.md#2-package-layout),
  not the Rust module imports, which contain cycles Go cannot express.
- Platform-specific code uses file suffixes (`_linux.go`, `_freebsd.go`,
  `_other.go`); behavior selection is by host OS, never a build tag that changes
  the API surface.
- `_other.go` is not an OS selector; explicit negative build constraints select
  fallback files. Cross-build every binary with `CGO_ENABLED=0` for Linux/amd64
  and FreeBSD/amd64, and run native checks separately.
- Compatible JSON is produced from struct tags plus small `MarshalJSON`/
  `UnmarshalJSON` methods for enums and omission edge cases. Never "fix" a wire
  field name or omission rule without updating
  [compatibility.md](compatibility.md) and its golden fixture in the same change.
- Errors carry context with `%w` chains. Only the HTTP layer converts them to
  strings, and only using the mappings in `internal/backup` and the controller
  handler.
- Reservations use saturating arithmetic, matching the Rust `saturating_*`
  helpers; never let a subtraction underflow a resource total.
- Background work takes a `context.Context`, is cancellable on shutdown, and
  logs to stderr with a `[ctl]`/`[agent]` prefix, matching the Rust messages.
- Tests are table-driven and live next to the code. Golden fixtures live in
  `testdata/`; tests compare parsed values for maps and byte-exact output for
  documents and command lines.

## Working with the Rust reference

The Rust revision pinned in [plan.md](plan.md) is the reference; `../TTstack` is
its usual checkout location. When maintaining a package:

1. Read the matching Rust file (see [mapping.md](mapping.md)).
2. Port the unit tests that encode contracts, keeping their names recognizable.
3. Record every deliberate deviation in [compatibility.md](compatibility.md) §7
   before writing it.

Golden fixtures are generated from the pinned Rust source using the commands in
[the oracle guide](../tools/oracle/README.md), without modifying the reference
checkout. The harness records the source revision and lockfile hash, generates
synthetic examples, and tests both accepted and rejected input. Normal Go tests
consume committed JSON/SQL fixtures without invoking Rust. Compare JSON with
integer precision and missing/null distinctions; only contractual text and
guest-config hash inputs are byte-exact. Generate actual Rust databases in
temporary paths for bidirectional differential tests; do not commit `.db` files.
Before running a cached Rust executable as an oracle, rebuild it at the pinned
revision or verify its provenance; its path alone does not prove which code it runs.

## Git conventions

- For roadmap delivery, one commit per phase; the message states the phase and
  its scope. Phases 0–7 are complete. Routine maintenance uses one independent
  issue/root cause per commit, with its tests and contract docs.
- A prerequisite review/fix of the plan is its own documentation commit, before
  implementation phases. Unfinished acceptance work is recorded as unfinished.
- Commit messages: imperative subject, body explaining behavior and verification,
  ending with the attribution line required by the tooling in use.
- Never commit generated binaries, databases, keys or host inventories; see
  `.gitignore`.
- Documentation-only changes check `git diff --check` and local links/anchors;
  skill changes also check metadata and references. Go changes run formatting,
  vet, tests and builds as described in the workflow validation matrix.
- [AI maintenance workflows](ai-workflow.md) define optional explicit review,
  fix and commit skills, worktree ownership and local commit validation. They
  do not automatically bump versions, tag, push or authorize live systems.

## Documentation conventions

- English, in the repository.
- One maintained place per behavior: this `docs/` tree describes the Go
  implementation; do not copy Rust documentation that would drift. Link to the
  pinned Rust source when it defines a contract rather than duplicating it.
  [AGENTS.md](../AGENTS.md) is the agent entrypoint, `.agents/skills/` contains
  task procedures, and the shared AI guides live here rather than in a parallel
  tool-specific documentation tree.
- Any change to a wire field, path, default or printed output updates
  [compatibility.md](compatibility.md) in the same commit.
