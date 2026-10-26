# Review regression fixes — 2026-10-26

Validated the three regressions identified in the 24-hour commit review of
`a569b89` through `40be59b`. The tested changes are based on `40be59b`.
The pinned Rust checkout was unavailable; no new differential-parity claim is
made. Host addresses and temporary credentials are omitted.

## Changes and regression coverage

- Firecracker retries retain disk-resize intent until ext4 growth completes,
  including retries where the backing disk already has the target size. QEMU
  continues to skip completed disk growth. Covered by
  `TestFirecrackerResizeRetryFinishesFilesystemAtTargetDiskSize` and
  `TestResizeRetrySkipsDiskWorkAlreadyAtTarget`.
- A controller host marker with an empty address can be re-registered after
  verifying the agent's host ID. Tracked VMs, pending disk reservations and
  resource floors survive; registration also fences older refresh results.
  `TestHostMarkerReregistrationRetainsVMsAndReservations` uses SQLite and an
  HTTP mock agent. Existing peer-replacement rejection tests still pass.
- Jail key injection only skips configuration when the complete managed SSH
  stanza leads the file. Commented, partial and Match-scoped directives cannot
  suppress it. `TestJailSSHSettingsRequireCompleteLeadingStanza` also checks
  cold-recreation idempotence and retention of the original configuration.

## Verification

| Environment | Result |
|---|---|
| macOS/arm64 workstation | `gofmt -l .`, `git diff --check`, serial `go vet`, `go test` and `go build` passed |
| Linux/amd64, Go 1.26.8 | Full `TT_TEST_INTEGRATION=1 go test -p 1 ./...`, vet and build passed |
| FreeBSD 15.1/amd64, Go 1.26.10 | Full opt-in integration suite, vet and build passed before Linux validation |
| Both authorized hosts | Race tests for `internal/agent`, `internal/ctl` and `internal/engine` passed with integration enabled |
| Linux/amd64 and FreeBSD/amd64 | Local CGo-disabled cross-builds of all commands passed |

The seven changed Go files had identical content locally and on both hosts.
SHA-256 of their sorted relative paths, each followed by a NUL and file bytes:
`da76b5f96b7ee3fa1934e988c17a85e6c1f3a304091a61a4dbb8603ec4ac4b98`.

On Linux, `TestFirecrackerResizeRetryCompletesExt4Growth` used a real 16 MiB
ext4 image containing a marker file. A failing `resize2fs` wrapper let the
backing disk reach 32 MiB while ext4 remained at 16 MiB. The original request
and a repeated failure both retained intent. Removing the injected failure
allowed the same target to finish; `dumpe2fs` confirmed 32 MiB and `debugfs`
confirmed the marker survived.

On FreeBSD, a temporary native engine harness created a private VNET parent
and one child Jail at a time, using a minimal root with synthetic accounts
and fresh SSH keys. Four starting configurations were tested: a commented
root-login directive, a partial stanza with password authentication enabled,
a stanza inside `Match`, and an already-managed leading stanza. In all four,
real `sshd -T` confirmed key-only policy; root public-key SSH succeeded before
and after cold stop/start, the guest marker survived, and configuration bytes
did not change on recreation. The VNET was restricted to one CPU.

## Cleanup and limits

Both temporary host workspaces, keys and test roots were removed. No task Jail,
mount or service remained. The test-loaded bridge module was unloaded, the
original gateway Jail stayed active, PF remained unloaded, and management SSH
remained reachable. No existing service or guest was changed.

These were bounded functional checks. The resize regression was exercised with
real ext4 storage and a fake stopped engine; no Firecracker VM boot or live zvol
resize was performed. FreeBSD engines remain experimental. Existing broad
acceptance evidence remains in the native validation reports.
