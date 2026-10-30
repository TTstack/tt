# Storage and network parity validation — 2026-09-30

Scope: roadmap phase 2, following shared-core commit `9fa21ec`. The tested Go
source is the phase-2 commit containing this report. No VM, container, bridge or
firewall was started or configured by this phase's tests.

## Sources and gates

The Rust oracle remains `1ad83a9feed5fac820d74b5e7ef752e50a552740` (0.6.0).
[Network fixture regeneration](../../tools/oracle/README.md) compiles its original
pure isolation renderer in an archived checkout and records source/lock provenance.
Go's isolation-rule bytes, UTF-8 TAP names and allocation vectors match that output.

The final source archive sent to FreeBSD (go.mod, go.sum, Makefile, cmd, internal,
tools) has SHA-256
`d2113ca7b255f48797812808e09dc11f6470f90850cdde1d788abe29c820130b`.
Local Go 1.26.0 formatting, vet, build, tests and CGo-disabled Linux/FreeBSD
cross-builds pass with GOMAXPROCS=2 and package parallelism 1. Native FreeBSD
race/integration checks pass for storage, networking and bounded commands; the
updated native storage test binary also passes the isolated ZFS smoke test.
Remote CI has not run.

## Observations

| Surface | Evidence |
|---|---|
| File catalog and resolution | Hidden/clone entries are filtered without deleting contents; ambiguous and linked disks are rejected; one qcow2 candidate wins as in Rust |
| File clones | Real FreeBSD cp retains private clone permissions while leaving base permissions unchanged |
| QEMU resizing | Fake qemu-img verifies command arguments, shrink rejection and format checks; no local QEMU infrastructure |
| ext4 growth | Real e2fsprogs expands a clone 64→96 MiB, preserves its marker, rejects shrink, then completes a simulated interrupted growth to 128 MiB; filesystem block count × block size confirms completion |
| ZFS command plans | Fake zfs captures clone/snapshot/resize arguments and refuses incomplete Firecracker base snapshots before cloning |
| Backup witnesses and cleanup | Linked dependencies, changed disk capacity and foreign directory contents are rejected; empty owned tombstones recover idempotently |
| Backup worker crash | A real child process retains the writer lock after its parent is killed; command capture files are unlinked before execution and do not remain after that crash |
| Native ZFS backup | A dedicated 16 MiB zvol snapshots, restores its marker, rejects a newer foreign snapshot, refuses removal after ownership changes, then removes the point and grows to 32 MiB without changing content |
| Linux networking | Fake execution/writes check bridge setup, NAT retention and exact-guest rule removal; compiled Rust isolation text matches byte-for-byte |
| FreeBSD networking | Fake ifconfig/PF paths check raw TAP-device records, ownership rejection, PF wildcard preflight and qualified child-anchor matching |
| Reflink | FreeBSD's explicit unsupported path leaves files unchanged and never falls back to copying; successful Linux FICLONE/restore remains unverified until Linux validation |

## Bounds, cleanup and remaining scope

The same dedicated FreeBSD 15.1-RELEASE-p3/amd64 host was used. The native ZFS
parent was newly created, quota-limited to 256 MiB and tagged as test-owned. The Go
test created only a UUID-named child volume. Its foreign snapshot and changed-owner
cases involved that test volume alone. The runner checked ownership before removing
the parent. Both dataset and mount were removed; a fresh SSH command confirmed
cleanup and reachability. Test files and keys are temporary; build prerequisites
and caches remain for later phases.

The shared workstation ran only lightweight code checks and fake host commands.
The Linux test hosts were not contacted. No root firewall, route, gateway jail
configuration, existing service or guest was changed. Native Linux nftables/TUN,
strict reflink success, Firecracker zvol boot and all engine lifecycles still need
their later native acceptance tests. This record covers FreeBSD 15.1 only; these storage
results are not evidence of a working VM or application.
