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

## Dependencies

The dependency set is intentionally closed. Adding a module requires a note in
this document and a reviewer-visible reason.

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
- Platform-specific code uses file suffixes (`_linux.go`, `_freebsd.go`,
  `_other.go`); behavior selection is by host OS, never a build tag that changes
  the API surface.
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

Golden fixtures are generated from the Rust build with a documented command; the
generator is added in phase 1 (`internal/testdata/gen` or an `xtask`-style
command) so fixtures can be regenerated deliberately rather than by hand.

## Git conventions

- One commit per roadmap phase; the message states the phase and its scope.
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
