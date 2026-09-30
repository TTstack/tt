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

## Dependencies

The direct dependency set is intentionally closed. Adding a direct module requires
a note in this document and a reviewer-visible reason. Required indirect modules
from these dependencies are allowed and pinned with checksums; "four modules" does
not mean SQLite has no transitive dependencies. Use versions compatible with Go
1.26 and test with that minimum toolchain, not just the developer's installed Go.

| Module | Use |
|---|---|
| `modernc.org/sqlite` | SQLite driver (pure Go, no cgo) |
| `github.com/google/uuid` | UUID v4 identifiers |
| `github.com/BurntSushi/toml` | `deploy.toml` parsing |
| `golang.org/x/sys/unix` | `flock`, TUN ioctls, `FICLONE`, `statfs` |

Everything else comes from the standard library. In particular: no CLI framework
(standard `flag`), no HTTP router (Go 1.22 method patterns), no logging framework
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

`../TTstack` is the reference. When implementing a package:

1. Read the matching Rust file (see [mapping.md](mapping.md)).
2. Port the unit tests that encode contracts, keeping their names recognizable.
3. Record every deliberate deviation in [compatibility.md](compatibility.md) §7
   before writing it.

Golden fixtures are generated from the pinned Rust source with a documented
command; a small harness is added under `tools/` in phase 1, without modifying the
reference checkout. It records the source revision and lockfile hash, generates
synthetic examples, and tests both accepted and rejected input. Normal Go tests
consume committed JSON/SQL fixtures without invoking Rust. Compare JSON with
integer precision and missing/null distinctions; only contractual text and
guest-config hash inputs are byte-exact. Generate actual Rust databases in
temporary paths for bidirectional differential tests; do not commit `.db` files.
Before running a cached Rust executable as an oracle, rebuild it at the pinned
revision or verify its provenance; its path alone does not prove which code it runs.

## Git conventions

- One commit per roadmap phase; the message states the phase and its scope.
- A prerequisite review/fix of the plan is its own documentation commit, before
  implementation phases. Unfinished acceptance work is recorded as unfinished.
- Commit messages: imperative subject, body explaining behavior and verification,
  ending with the attribution line required by the tooling in use.
- Never commit generated binaries, databases, keys or host inventories; see
  `.gitignore`.
- Documentation-only changes still run `gofmt`/`go vet` when Go files are touched
  and always check that links in the touched documents resolve.

## Documentation conventions

- English, in the repository.
- One maintained place per behavior: this `docs/` tree describes the Go
  implementation; do not copy Rust documentation that would drift. Where the Rust
  document is still authoritative (for example `rest-api.md` during phase 5), link
  to it explicitly.
- Any change to a wire field, path, default or printed output updates
  [compatibility.md](compatibility.md) in the same commit.
