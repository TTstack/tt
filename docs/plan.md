# Implementation plan — TTstack in Go

Status: approved; implementation has not started. This document is the contract for
what will be built, how it maps onto the Rust release, and how each step is verified.

## 1. Purpose

TTstack is a lightweight private cloud for developers and small teams: a CLI
(`tt`), a controller (`tt-ctl`) and a per-host agent (`tt-agent`) that together
create, access, stop, restart and delete VMs and containers on a small fleet.
The Rust workspace at `../TTstack` is the reference implementation.

This repository re-implements the same product in Go, with the same observable
behavior, the same API, and the same persistent state format, so that the two
implementations can be mixed in one fleet and so that an existing deployment can be
taken over by the Go binaries without data conversion.

## 2. Goals and non-goals

Goals:

1. Feature parity with the Rust release: all engines, storage backends, networking
   modes, SSH bootstrap, offline resource updates, disk backup, image recipes,
   dashboard, local and distributed deployment.
2. Wire compatibility: identical HTTP routes, JSON documents, status codes, CLI
   output and exit codes.
3. State compatibility: identical SQLite files (schema versions and table layouts),
   identical on-disk runtime paths and identifiers, identical service files.
4. Idiomatic Go: `cmd/` binaries, `internal/` domain packages, `context`, `net/http`,
   `database/sql`, table-driven tests. Structure follows Go practice, not the Rust
   module tree.
5. Minimal dependencies: the standard library plus four modules (see §5).

Non-goals (matching the Rust product boundaries):

- No new features, no "improvements" that change behavior. Anything not in the Rust
  release does not belong here.
- No guest migration, high availability, distributed storage, image distribution,
  automatic guest restart after host reboot, or per-owner authorization.
- No TLS or identity system for the service APIs; authentication stays a single
  shared bearer key.

## 3. Confirmed decisions

| Decision | Choice |
|---|---|
| Compatibility target | Full wire compatibility, including SQLite schema versions v5 (controller) and v6 (agent) |
| FreeBSD engines | Ported (`bhyve`, `jail`, PF paths), runtime-gated to FreeBSD hosts exactly as in Rust |
| Dependencies | Standard library + `modernc.org/sqlite`, `github.com/google/uuid`, `github.com/BurntSushi/toml`, `golang.org/x/sys` |
| Delivery | Phased (0–7), each phase builds, vets, tests and is committed separately |
| Documentation language | English (this repository), same convention as TTstack |
| Author metadata | `fh <hui.fan@mail.ru>`, matching TTstack |
| Module path | `github.com/TTstack/tt` |

## 4. What "compatible" means precisely

Three levels, all in scope:

1. **API level.** Same routes, methods, request/response JSON field names and
   omission rules, error strings, and HTTP status mapping (including the
   error-string-prefix mapping used by the backup protocol). An unmodified Rust
   `tt` CLI must work against a Go `tt-ctl`, and a Rust `tt-ctl` must be able to
   drive a Go `tt-agent`.
2. **State level.** Same SQLite schema version numbers, same tables, indices and
   pragmas, same JSON blob layout inside rows, same `_meta` keys, same
   `service.lock` behavior. Same runtime paths (`/home/ttstack/run`, clone
   directories, image directories, backup directories, `@ttsnap` / `@ttbackup-*`
   snapshot names and ZFS user properties).
3. **Host-visible identifiers.** Names that outlive a process or are shared with
   the host kernel must be produced bit-exactly: TAP names (`tt-` + 12 hex digits
   from a frozen SipHash-1-3 over the VM id plus a `0xff` terminator), guest MAC
   addresses (`02:54:a.b.c.d` from the guest IPv4), bridge `tt0`/`10.10.0.1/16`,
   nftables tables/chains, cgroup names and jailer UIDs, PID/log/socket file
   names, and systemd unit names.

Details, field by field, are in [compatibility.md](compatibility.md).

## 5. Technology choices

| Concern | Rust release | Go implementation |
|---|---|---|
| HTTP server | axum | `net/http` with Go 1.22 method patterns (`mux.HandleFunc("POST /api/vms/{id}/stop", …)`) |
| HTTP client | reqwest (rustls) | `net/http` with redirects disabled and per-request timeouts via `context` |
| Async runtime | Tokio | goroutines, `context.Context`, channels; a small counting semaphore type |
| Database | rusqlite (bundled SQLite) | `database/sql` + `modernc.org/sqlite` (pure Go, no cgo) |
| CLI parsing | clap | standard `flag` with an explicit subcommand tree |
| JSON | serde | `encoding/json` with custom marshal/unmarshal for enums and omission rules |
| TOML | toml crate | `github.com/BurntSushi/toml` |
| UUID | uuid crate | `github.com/google/uuid` |
| SHA-256 | sha2 crate | `crypto/sha256` |
| File locking, ioctls, statfs | nix | `golang.org/x/sys/unix` |
| Dashboard assets | embedded HTML string | `//go:embed` of `internal/ctl/web/index.html` |
| Errors | `ruc::Result` with context macros | `error` values wrapped with `fmt.Errorf("…: %w", err)`; HTTP layer flattens to strings |

Deliberate behavioral equivalence, exercised by tests: bounded command execution
with a 300 s default timeout and 4 MiB per-stream capture; 15 s background loops;
8-way concurrent host refresh; 5 s/360 s/600 s client timeouts; WAL with
`synchronous=FULL`.

### Deliberate deviations (documented, not behavioral regressions)

- Downloads in the image builder and release/deploy helpers use Go's HTTP client
  instead of shelling out to `curl`. Same URLs, same SHA-256/SHA-512 verification,
  same atomic publication, same error text where observable.
- Firecracker control calls go through a `net/http` client bound to the VM's Unix
  socket instead of `curl --unix-socket`. Same endpoints, same bodies, same 5 s
  timeout, same failure text contract at the API boundary.
- Guest log capture and process supervision use Go primitives (goroutine reaper,
  `os/exec`) rather than detached threads, with identical file names and contents.

## 6. Repository layout

```text
tt/
├── cmd/
│   ├── tt/            # CLI binary
│   ├── tt-ctl/        # controller binary
│   ├── tt-agent/      # host agent binary
│   └── upgrade-agent-v5/  # offline v5→v6 database converter
├── internal/
│   ├── model/         # entities, enums, constants, validation
│   ├── api/           # wire DTOs and the ApiResp envelope
│   ├── capability/    # design matrix, scoped host report, legacy tag gates
│   ├── backup/        # backup protocol types and replay/precondition logic
│   ├── guestconfig/   # opaque guest files, digest, ext4 config disk
│   ├── sshboot/       # SSH options/info, seed script, readiness probes
│   ├── auth/          # API-key parsing, constant-time compare, middleware
│   ├── runcmd/        # bounded command execution and small process helpers
│   ├── engine/        # Engine interface, factory, PID recovery; per-engine files
│   ├── netcfg/        # bridge, TAPs, nftables NAT/DNAT/isolation, FreeBSD PF
│   ├── storage/       # ImageStore: file, zvol; backup artifacts and reflink
│   ├── agent/         # agent runtime, HTTP handlers, backup worker, SSH ingress
│   ├── ctl/           # controller DB, scheduler, handlers, dashboard, forwarding
│   └── cli/           # client, command tree, deploy, image builder, ttconfig
├── docs/              # this documentation
├── tools/             # deploy.toml.example and helper scripts
└── .github/workflows/ # CI
```

Package-by-package responsibilities and the Rust mapping are in
[architecture.md](architecture.md) and [mapping.md](mapping.md).

## 7. Delivery phases

Each phase ends with `gofmt -l`, `go vet ./...`, `go test ./...`, `go build ./...`
all clean, and its own commit. Details and acceptance criteria per phase are in
[roadmap.md](roadmap.md).

| Phase | Content | Key verification |
|---|---|---|
| 0 | Repository skeleton: `go.mod`, `Makefile`, CI, `cmd/` stubs | builds, vet, empty test run |
| 1 | `model`, `api`, `capability`, `backup`, `guestconfig`, `sshboot`, `auth`, `runcmd` | unit tests plus JSON golden fixtures produced from the Rust build |
| 2 | `storage` (file, zvol, backup artifacts), `netcfg` (Linux + FreeBSD) | unit tests; TAP-name and IP/MAC vectors; `mkfs.ext4`/`debugfs` tests |
| 3 | `engine` (qemu, firecracker + jailer sandbox, docker/podman, bhyve, jail) | unit tests with fake tools; command-line golden tests |
| 4 | `agent`: runtime, handlers, backup worker, SSH ingress, agent DB v6 | `httptest` API tests; schema rejection tests; reconciliation tests |
| 5 | `ctl`: DB v5, scheduler, handlers, dashboard, backup forwarding | mock-agent HTTP workflow tests (interrupted creation, partial deletion, expiry retry, duplicate names, resize conflicts) |
| 6 | `cli`: client, commands, `~/.ttconfig`, local + distributed deploy, image recipes | CLI golden output tests; deploy script tests; recipe dry runs |
| 7 | Compatibility acceptance: cross-implementation tests, converter tool, docs | Go controller ↔ Rust agent and Rust controller ↔ Go agent smoke runs on an authorized host |

## 8. Verification strategy

1. **Golden JSON fixtures.** For every wire type, the Rust build emits canonical
   JSON (including omission/null edge cases); Go tests decode and re-encode it and
   compare byte-for-byte. Fixtures live in `internal/<pkg>/testdata` and are
   regenerated only by a documented command.
2. **Golden SQLite fixtures.** A Rust-created `ctl.db` (v5) and `agent.db` (v6)
   are opened by the Go code; rows must round-trip unchanged.
3. **Behavioral unit tests.** Port the Rust unit tests that encode contracts
   (scheduler placement, capability admission, backup replay, validation,
   SipHash vectors, ext4 growth).
4. **Mock-agent workflow tests.** `httptest` servers stand in for agents to
   exercise creation timeouts, partial deletion, expiry, detach and resize
   recovery end to end, mirroring the Rust controller's test coverage.
5. **Live checks (authorized hosts only).** Phase 7 smoke tests boot one QEMU and
   one Docker workload per implementation pairing. Bounded, cleaned up, no stress
   testing, never against production guests.

## 9. Risks and mitigations

| Risk | Mitigation |
|---|---|
| Silent JSON drift (omission/null rules differ from serde) | Golden fixtures per type; a test fails on any byte difference |
| SipHash-1-3 tap names differ | Port the algorithm with the Rust test vectors as unit tests |
| SQLite driver differences (`modernc.org/sqlite`) | Use identical pragmas; verify with Rust-created fixture databases; single-writer discipline |
| Rust `error`-string → HTTP status coupling | Centralize mapping in one table mirrored from `core/src/backup.rs`; tests enumerate every prefix |
| Scope creep into "improvements" | Non-goals in §2; compatibility document is the review checklist |
| FreeBSD paths untestable on Linux hosts | Same runtime gating as Rust; code review against source; document as experimental |

## 10. Definition of done

The port is complete when:

1. `make build`, `make test`, `make lint` are clean and CI is green.
2. Golden fixture tests cover every wire type and both database schemas.
3. The CLI reference commands in `README.md` work end to end on a Linux systemd host.
4. A Rust controller can drive a Go agent and a Go CLI can drive a Rust controller
   on the same schema versions (phase 7 evidence recorded in `docs/validation/`).
5. Every feature listed in the Rust `README.md` "Core behavior and boundaries"
   section has a corresponding implementation and a test.
