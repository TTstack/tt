# Rust → Go mapping

Where each part of the Rust implementation lands in this repository. The Go
structure is deliberately flatter: one package per domain instead of one crate
per binary plus a shared crate, because Go packages, not crates, are the unit of
visibility and reuse here.

## Crates

| Rust | Go |
|---|---|
| `crates/core` (`ttcore`) | `internal/{model,api,capability,backup,guestconfig,sshboot,auth,runcmd,engine,netcfg,storage}` |
| `crates/agent` (`tt-agent`) | `internal/agent` + `cmd/tt-agent` |
| `crates/ctl` (`tt-ctl`) | `internal/ctl` + `cmd/tt-ctl` |
| `crates/cli` (`tt`) | `internal/cli` + `cmd/tt` |
| `crates/agent/examples/upgrade-agent-v5.rs` | `cmd/upgrade-agent-v5` |
| `crates/core/src/ssh_bootstrap.sh` | `internal/sshboot/bootstrap.sh` (embedded with `//go:embed`) |
| `crates/ctl/src/web.rs` HTML constant | `internal/ctl/web/index.html` (embedded) |

## Core modules

| Rust file | Go package/files | Notes |
|---|---|---|
| `core/src/model.rs` | `internal/model` | entities, enums, constants, `Resource`, validation |
| `core/src/api.rs` | `internal/api` | DTOs, `ApiResp[T]` → generic `Resp[T]` |
| `core/src/capability.rs` | `internal/capability` | matrix, `Report`, legacy tags |
| `core/src/backup.rs` | `internal/backup` | protocol types, `Check`, `ErrorStatus` |
| `core/src/guest_config.rs` | `internal/guestconfig` | validation, digest, ext4 disk |
| `core/src/ssh.rs` | `internal/sshboot` | options/info, seed script, probes |
| `core/src/auth.rs` | `internal/auth` | key parsing, constant-time compare |
| `core/src/command.rs` | `internal/runcmd` | bounded execution |
| `core/src/lib.rs` `lock_state` | `internal/runcmd` (or `internal/fslock`) | `flock` on the state directory |
| `core/src/engine/mod.rs` | `internal/engine/engine.go` | interface, factory, PID recovery |
| `core/src/engine/qemu.rs` | `internal/engine/qemu.go` | |
| `core/src/engine/firecracker.rs` | `internal/engine/firecracker.go` | |
| `core/src/engine/firecracker/sandbox.rs` | `internal/engine/sandbox.go` | |
| `core/src/engine/docker.rs` | `internal/engine/docker.go` | |
| `core/src/engine/bhyve.rs` | `internal/engine/bhyve.go` | `_freebsd.go` |
| `core/src/engine/jail.rs` | `internal/engine/jail.go` | `_freebsd.go` |
| `core/src/net.rs` | `internal/netcfg/net.go` + `net_linux.go`/`net_freebsd.go` | TAP naming, bridge, NAT |
| `core/src/net/isolation.rs` | `internal/netcfg/isolation_linux.go` | |
| `core/src/net/freebsd.rs` | `internal/netcfg/pf_freebsd.go` | |
| `core/src/storage/mod.rs` | `internal/storage/storage.go` | `ImageStore` interface |
| `core/src/storage/file.rs` | `internal/storage/file.go` | |
| `core/src/storage/zvol.rs` | `internal/storage/zvol.go` | |
| `core/src/storage/backup.rs` | `internal/storage/backup.go` | artifacts, reflink, witnesses |

## Agent modules

| Rust file | Go files | Notes |
|---|---|---|
| `agent/src/main.rs` | `cmd/tt-agent/main.go` | startup order, loops, routing |
| `agent/src/config.rs` | `internal/agent/config.go` | flags, auto-detection |
| `agent/src/auth.rs` | `internal/auth/middleware.go` | shared with the controller |
| `agent/src/handler.rs` | `internal/agent/handler.go` + `catalog.go` | routes, image catalog |
| `agent/src/runtime.rs` | `internal/agent/runtime.go` (+ `runtime_db.go`) | lifecycle, reconcile, bindings |
| `agent/src/backup.rs` | `internal/agent/backup.go` | claim/admit/finish, cleanup |
| `agent/src/ssh_ingress.rs` | `internal/agent/sshingress.go` | outer-netns DNAT |

## Controller modules

| Rust file | Go files | Notes |
|---|---|---|
| `ctl/src/main.rs` | `cmd/tt-ctl/main.go` | startup, loops, routing |
| `ctl/src/config.rs` | `internal/ctl/config.go` | three flags |
| `ctl/src/auth.rs` | `internal/auth/middleware.go` | shared |
| `ctl/src/db.rs` | `internal/ctl/db.go` | schema v5, migration, detach |
| `ctl/src/scheduler.rs` | `internal/ctl/scheduler.go` | placement rules |
| `ctl/src/handler.rs` | `internal/ctl/handler.go` (+ `lifecycle.go`, `refresh.go`) | endpoints, state machine |
| `ctl/src/web.rs` | `internal/ctl/web.go` + `web/index.html` | dashboard |
| `ctl/src/backup.rs` | `internal/ctl/backup.go` | forwarding, reservations |

## CLI modules

| Rust file | Go files | Notes |
|---|---|---|
| `cli/src/main.rs` | `internal/cli/commands.go` + per-command files | command tree, output |
| `cli/src/client.rs` | `internal/cli/client.go` + `ttconfig.go` | HTTP client, `~/.ttconfig` |
| `cli/src/deploy.rs` | `internal/cli/deploy.go` (+ `deploy_remote.go`) | local and distributed |
| `cli/src/image_builder.rs` | `internal/cli/imagebuilder.go` (+ `recipes.go`) | recipes, checksums |

## Type and function naming

Go naming differs (exported identifiers, `Err`/`New` prefixes, receiver names),
but wire names, constants and messages match the Rust source (see
[compatibility.md](compatibility.md)). A few deliberate renames:

| Rust | Go |
|---|---|
| `ApiResp<T>` | `api.Resp[T]` |
| `VmEngine` trait | `engine.Engine` interface |
| `ImageStore` trait | `storage.ImageStore` interface |
| `ruc::Result<T>` | `(T, error)` / `error` |
| `CtlShared` / `AgentShared` | `ctl.Server` / `agent.Server` |
| `Placement` | `ctl.Placement` |
| `Witness`, `Setup`, `Context` (backup) | `storage.Witness`, `agent.BackupSetup`, `storage.BackupContext` |
