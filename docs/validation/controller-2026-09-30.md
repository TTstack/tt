# Controller parity validation — 2026-09-30

Scope: phase 5, following agent commit `b28c64b` and PID-test fix `0ecd3b7`.
The [preceding CI run](https://github.com/TTstack/tt/actions/runs/36678246515)
passed Linux unit/integration/race checks and Linux/FreeBSD cross-builds.

The reference remains Rust `1ad83a9feed5fac820d74b5e7ef752e50a552740` (0.6.0),
with Cargo.lock SHA-256
`791f251f410e56db0ba247d5c0a8c82cdd1f97e26f8b5d74742042209192f62c`.

## State, HTTP and workflow evidence

- Actual Rust-created controller databases open in Go, Go mutations reopen in
  Rust, and Go-created databases open in Rust. Rust v4 state migrates in Go with
  stable backup revisions and then reopens in Rust. Synthetic tests verify that
  malformed migration rows roll back every backup change and the version marker.
- Controller v5 pragmas, tables and indices match; complete environment plans
  and backup observation/reservation changes commit atomically. Strict reads fail
  on corrupt records while recovery scans retain them and continue with healthy
  entries. Detach removes tracking and returns orphan metadata without calling
  an agent.
- Scheduler tests cover host/engine/capability/image eligibility, stable best-fit
  ordering, ZFS preference and file fallback, Docker independence, FreeBSD engine
  restrictions, base/config disk sizes, Firecracker memory overhead, and shadow
  reservations across a multi-VM plan.
- 52 native Rust HTTP cases match route/auth/method behavior, public dashboard
  headers, body extraction and controller-specific response shapes. Controller
  environment actions return `data:null`; host removal and agent empty actions
  omit `data`. Business diagnostic decoration is normalized as documented.
- The embedded dashboard matches the pinned Rust source exactly: SHA-256
  `d88d6227bb3cab8988843bc0da098f92303043b053295b4b1c9c5090b319fb96`.
- Mock-agent workflows exercise lost creation/resize/restore responses, exact
  replay, rejection versus unknown intent, retained absent VMs, partial deletion
  and expiry retry, stale expiry of reused names, capability withdrawal and
  reserved restart capacity. Older snapshots cannot erase pending resource or
  backup intent or overwrite a confirmed restore observation.
- Concurrency tests cover a stalled deletion alongside another environment and
  heartbeat, accepted creation after client disconnect, and a confirmed stop
  surviving a globally invalidated refresh. Shutdown drains accepted background
  creation and retains the state lock until that work has completed.

## Native FreeBSD service combinations

On the dedicated FreeBSD 15.1-RELEASE-p3/amd64 test host, all four pairings ran:
Go controller → Go agent, Rust controller → Go agent, Go controller → Rust agent,
and Rust controller → Rust agent. Each controller reused the same native v5 state;
agent implementations reused the same native v6 state.

Tests used loopback listeners, synthetic authentication, task-owned temporary
state and five synthetic stopped VM records. Agents advertised one CPU, 128 MiB
memory and a 1024 MiB disk budget; no real guest resources backed those rows.
Each pairing refreshed the host, retained VM/disk/backup identity, inspected
backup status, served the public dashboard, rejected unauthenticated API access,
and rejected an unavailable QEMU environment without persisting a creation plan.

Processes and private state were cleaned. The fixed runtime path was absent
before the test, created solely for it, and removed only while empty afterward.
No firewall, route, gateway jail, existing service or real guest was changed.

## Gates and remaining scope

Minimum Go 1.26.0 formatting, vet, tests, CGo-disabled build and Linux/FreeBSD
cross-builds pass locally with GOMAXPROCS=2 and package parallelism 1. The complete
native FreeBSD race/integration suite passes, including database interchange and
shutdown draining. The Rust controller/database/HTTP oracles were built on that
host, including its native TLS dependencies; none were built on the shared local
workstation.

The controller and agent are implemented. CLI/deployment/image workflows and the
bounded real-guest matrix remain phases 6–7. Neither authorized Linux test host
has been contacted. FreeBSD remains experimental; synthetic service interoperability
is not evidence of real bhyve/Jail boot, SSH readiness or Linux kernel behavior.
