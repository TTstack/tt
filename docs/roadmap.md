# Roadmap

Phased delivery of the Go port. Every phase ends with a green build, `go vet`,
`go test ./...`, and exactly one commit. Later phases must not start before the
previous phase's acceptance criteria are met — the point of the staging is that a
failure is always localized.

Reference sources for each phase are under `../TTstack/crates`.

## Phase 0 — Repository skeleton

Deliverables

- `go.mod` (module `github.com/TTstack/tt`, Go 1.26) with the four dependencies.
- `Makefile`: `build`, `release`, `test`, `lint`, `fmt`, `clean`, `install`,
  `deploy-agent`, `deploy-ctl`, `deploy`, `help`.
- `.github/workflows/ci.yml`: gofmt check, `go vet`, `go build`, `go test` with
  `e2fsprogs` installed (for config-disk and ext4 tests).
- `cmd/{tt,tt-ctl,tt-agent,upgrade-agent-v5}/main.go` stubs that parse flags and
  print usage.
- `AGENTS.md`, `CLAUDE.md`, `docs/` (this set).

Acceptance

- `go build ./...`, `go vet ./...`, `go test ./...` clean.
- `go run ./cmd/tt --help` and the two service binaries with `--help` print the
  documented flags.

## Phase 1 — Shared core

Deliverables (`internal/model`, `api`, `capability`, `backup`, `guestconfig`,
`sshboot`, `auth`, `runcmd`)

- Entities, enums (strict JSON), constants, resource accounting, validation.
- Wire DTOs and the `ApiResp` envelope with exact omission rules.
- Capability design matrix, scoped `Report` admission, legacy tag mapping.
- Backup protocol types with replay/precondition logic and the status table.
- Guest configuration validation, digest, ext4 config-disk builder.
- SSH options/info, seed-script generation, the embedded bootstrap script,
  readiness probes.
- API key parsing/comparison and bearer middleware.
- Bounded command execution.

Acceptance

- Golden JSON fixtures for every wire type, produced from the Rust build, pass.
- Ported unit tests for validation, capability admission, backup replay,
  resource accounting and the envelope pass.
- `mkfs.ext4`/`debugfs` based config-disk test passes (guarded by tool presence).
- `sh -n` check of the generated seed script with hostile key material.

## Phase 2 — Storage and networking

Deliverables (`internal/storage`, `internal/netcfg`)

- `ImageStore` with `file` and `zvol` backends: clone, remove, list, resolve,
  sizes, resize, Firecracker directory/layout handling, ext4 growth.
- Backup artifact storage: zvol snapshots with ownership properties, reflink
  artifacts with owner/manifest files, verification, restore staging, cleanup.
- Linux networking: bridge/TAP lifecycle, nftables NAT/DNAT/denylist, per-VM
  isolation, TAP owner changes, SSH ingress rules.
- FreeBSD networking: `ifconfig` bridge/taps with ownership descriptions, PF
  anchor preflight and rules.

Acceptance

- TAP-name, MAC and IP-allocation vector tests pass.
- Reflink probe tests (skip when the filesystem cannot reflink) and zvol command
  golden tests pass.
- Isolation rule text golden tests pass; FreeBSD code compiles behind build tags.

## Phase 3 — Engines

Deliverables (`internal/engine`)

- `Engine` interface, platform factory, PID recovery and termination helpers.
- QEMU: command construction, NoCloud seed ISO, QMP control, lifecycle.
- Firecracker: config JSON, jailer sandbox, cgroup v2, configuration drive,
  shutdown escalation, `Sandbox` metadata.
- Docker/Podman: run/inspect/start/stop/rm, persisted runtime binding.
- bhyve and jail: FreeBSD lifecycle with process/device ownership checks.

Acceptance

- Command-line golden tests with fake `PATH` executables for every engine.
- Sandbox tests: hard links preserved, block devices recreated, symlinks rejected.
- Boot-wait semantics tests (missing socket ≠ exited process, timeout retains
  resources) pass.

## Phase 4 — Agent

Deliverables (`internal/agent`, `cmd/tt-agent`)

- Runtime: create/start/stop/resize/destroy/reconcile with durable intent,
  idempotent creation, IP/port allocation, SSH bootstrap, network restore.
- HTTP handlers with the exact routes and status mapping.
- Backup worker: claim/admit/finish, cleanup with backoff, settle headers.
- SSH ingress application/removal.
- Agent database v6 with the corruption policy.
- `cmd/upgrade-agent-v5`: offline converter preserving unknown JSON fields.

Acceptance

- `httptest` tests for create idempotency, capability rejections, resize
  conflicts and backup header/status behavior.
- Schema gate tests: v5 and garbage databases are rejected, v6 opens.
- Converter tests: source unchanged, output mode 0600, unknown fields preserved.

## Phase 5 — Controller

Deliverables (`internal/ctl`, `cmd/tt-ctl`)

- Database v5 with migration and recovery read semantics.
- Scheduler with the exact eligibility and preference rules and error messages.
- Handlers: hosts, environments, VMs, resize, images, status, capabilities,
  detach, and the backup forwarding path.
- Lifecycle: plan-persist-then-execute creation, deletion retry, expiry loop,
  start/stop with reservation handling, snapshot merge rules.
- Dashboard (embedded HTML/JS) driven by the capability matrix.

Acceptance

- Mock-agent workflow tests pass: interrupted creation, partial deletion, expiry
  retry, duplicate names, unschedulable requests, resize unknown outcomes,
  absent-VM retention, untracked-VM logging.
- Golden fixture database from Rust opens, reads and round-trips.
- Dashboard serves at `/` and its API calls work against the test server.

## Phase 6 — CLI, deployment, images

Deliverables (`internal/cli`, `cmd/tt`)

- Client with redirects disabled, per-request timeouts and no implicit retries.
- Command tree: `config`, `capabilities`, `status`, `host`, `env` (including
  `resize` and `backup`), `image`, `deploy`.
- `~/.ttconfig` handling (atomic write, mode 0600, address-scoped credentials).
- Local deployment (systemd units, env files, key lifecycle, mount-guard
  drop-ins) and distributed deployment (`deploy.toml`, SSH stdin scripts, SHA-256
  verified uploads, systemd/OpenRC/manual fallback).
- Image recipes with pinned digests for `alpine-cloud` and `fc-alpine`, checksum
  manifest verification for Debian/Ubuntu, Docker pulls with local aliases, and
  the FreeBSD jail base recipe.

Acceptance

- CLI golden output tests for tables, expiry labels, access lines and errors.
- Deploy script tests with a fake root/SSH environment; unit files match the
  documented content.
- `tt image recipes`, dry-run recipe paths and checksum verification tests pass.

## Phase 7 — Compatibility acceptance

Deliverables

- Cross-implementation tests: Go controller ↔ Rust agent, Rust controller ↔ Go
  agent, Go CLI against both.
- Feature checklist walk-through of the Rust `README.md` "Core behavior and
  boundaries" section, each item mapped to a test.
- `docs/validation/` record of the tested revisions, hosts and limits.
- Final documentation pass: `deployment.md`, `rest-api.md`, `guest-images.md`,
  `ssh.md`, `disk-backup.md`.

Acceptance

- Mixed-fleet smoke run on an authorized host: one environment spanning a Go host
  and a Rust host, with create, access, stop/start, resize, backup/restore and
  delete.
- All documentation links resolve; every documented command has been executed at
  least once.
