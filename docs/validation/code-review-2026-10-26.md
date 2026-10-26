# Code review — 2026-10-26

Reviewer-wide pass over the whole Go implementation (156 `.go` files at the
time) hunting for defects, contract deviations and robustness gaps. Every
finding below was re-verified against the source it cites before recording;
reported-but-unverifiable or rejected items are listed at the end.

Constraints of this review: the Rust oracle tree (`../TTstack`) is no longer
present on this machine, so parity judgements rely solely on
[compatibility.md](../compatibility.md), the golden fixtures and the live-test
records. Items that need a Rust source comparison are marked
**(parity unverified)**. Nothing here was fixed or changed; this document is a
record only.

Severity scale: **critical** (data loss/security/malfunction on supported
platforms), **high** (breaks stated project guarantees or the documented
development workflow), **medium** (real defect with bounded impact or narrow
trigger), **low** (robustness/determinism/observability improvement).

## Findings

### R1 — `go build ./...` fails on darwin (high)

`internal/agent/backup.go:21,108` and `internal/agent/backup_context.go:18-63`
use `storage.ProbeReflink`/`storage.Witness`/`storage.BackupContext`, which
live behind `//go:build linux || freebsd` in `internal/storage/backup.go:1`.
Agent backup files carry no build tag and have no non-linux/freebsd stub, so
the full-tree build and `go vet ./internal/agent` fail on macOS (reproduced
locally, 7 undefined symbols). The package otherwise deliberately supports
other platforms (`cpu_other.go`, `memory_other.go`, `factory_other.go`),
[roadmap.md](../roadmap.md) phases end with `go build ./...` /
`go test ./...`, and CI only runs Ubuntu — so the gate is silently broken for
development hosts. **Direction:** give `storage.ProbeReflink`/`Witness`/
`BackupContext` an `!linux && !freebsd` stub that reports backup unsupported,
mirroring the existing stubs.

### R2 — Corrupt-inventory admission pin diverges between Runtime and `/api/info` (medium)

`internal/agent/runtime.go:165-169` (`recount`, the internal admission view)
pins CPU/Mem/Disk **and `VmCount = model.MaxVMs`** when VM rows are
unreadable. `internal/agent/server.go:85-89` (`/api/info`) pins only the three
Used fields. Two copies of one policy with different scope: the controller
learns a truthful `VmCount` while the agent itself refuses admission. 
**Direction:** single shared pinning function; add a corrupt-DB `/api/info`
fixture. Parity: the single-struct Rust implementation likely cannot diverge —
verify and reconcile.

### R3 — Agent busy-retry keys off error message text (low)

`internal/agent/backup_worker.go:55` retries while
`strings.Contains(err.Error(), "backup worker is running")`. Control flow
coupled to a prose string: any message edit silently turns "wait up to 300 s"
into "fail immediately" with no test catching it (the string is built
elsewhere: `... conflict: backup worker is running ...`). Same pattern would
be avoided with a typed sentinel returned by `BackupClaim` (wire prefix
unchanged). **Direction:** sentinel/`errors.As` internally; keep the wire
message.

### R4 — Resize retry re-runs disk growth when the disk is already at target (low)

`internal/agent/lifecycle.go:199-213`: `grow := uint64(target.Disk) > current
|| vm.PendingResources != nil`. After a power-fail where the disk reached the
target but the final `saveVM` did not, the retry forces another
`ResizeDisk`/`ResizeFirecracker` at an equal size; recovery then silently
depends on the storage layer tolerating a same-size resize. **Direction:**
drive re-entry from the persisted discrepancy instead (drop the
`|| PendingResources != nil` shortcut) — but this touches the
crash-consistency protocol, so it needs a Rust protocol comparison first
**(parity unverified)**.

### R5 — Controller reservation release trails deletion by one refresh tick (low)

`internal/ctl/environment.go:288-299`: after the agent confirms deletion, the
VM row is removed but the host's Used counters are not released; the plan-side
heal (`environment.go:137-139`) only takes `max(stored, tracked)` and the full
rebuild happens in the 15 s refresh (`refresh.go:96` re-accounts from
scratch). Consequences: within one tick a same-size recreate can be refused by
`CanFit`. Self-healing, conservative direction; still an observable CLI wrinkle
(delete → immediate create on a full host → 422, works 15 s later).
**(parity unverified)** — if Rust releases synchronously on delete success,
this is a behavioral deviation worth aligning.

### R6 — Corrupt host rows vanish from the refresh loop permanently (medium)

`internal/ctl/refresh.go:18` uses `db.RecoveryHosts`, whose reader
(`internal/ctl/db.go:239-258`) drops undecodable rows after one stderr line.
The refresh loop only visits returned hosts, so a corrupt host row is never
snapshotted again: it stays `online` with stale capacity in `FleetStatus`,
keeps feeding the scheduler (whose create requests then fail at the real
agent), and never gets marked offline. Pollution is continuous, not one-shot.
**Direction:** have recovery readers also return the corrupt ids and persist
`State=offline` + an error string for them once.

### R7 — Env/VM insert and host accounting are not one transaction; mid-way failure parks envs in `Creating` (low-medium)

`internal/ctl/environment.go:172-184`: `PutEnvironment` commits env+VMs, then
host decrements run as independent statements. A `PutHost` failure in between
returns 500 with the env row committed, creation never started, state stuck
`EnvCreating` until the refresh marks it Failed, and the name stays reserved
(409 on retry). **Direction:** fold the host updates into `PutEnvironment`'s
transaction.

### R8 — Agent address validation accepts a trailing slash; requests then 502 (low)

`internal/ctl/hosts.go:20` explicitly allows `Path == "/"`, after which
`client.go:44` builds `http://host:9000//api/info` → route miss → misleading
"agent returned no data". Rust's URL join normalizes this
**(parity unverified)**. **Direction:** reject any non-empty path and/or
normalize before persisting.

### R9 — Unordered SQL makes placement tie-breaks and `vm_ids` order non-deterministic (low, determinism)

`internal/ctl/db.go:267` (`ListHosts`) and `db.go:305-309` (`VMsByEnv`) have no
`ORDER BY`, and `INSERT OR REPLACE` rewrites move rows to the table end.
`scheduler.go:123-129`'s `sort.SliceStable` best-fit therefore breaks
equal-MemFree ties by physical row order — the same fleet state can pick
different hosts across plans — and `env.VmIds` order jitters on the wire.
**Direction:** `ORDER BY id` on both queries.

### R10 — Silent error swallow in the env refresh path (low)

`internal/ctl/refresh.go:65-68` returns silently on `RecoveryEnvs` failure
(the host listing a few lines above logs). Env convergence stops with no trace.
**Direction:** one `s.log` line.

### R11 — `changeEnvironment` can persist an empty failure reason (low)

`internal/ctl/environment.go:414-421`: when `failures` is empty but the state
mismatches, the stored/returned error becomes `"stop incomplete: "`. Reachable
only via unusual intermediate states; **(parity unverified)** on the message
shape. **Direction:** distinct reason string for the empty-failures case.

### R12 — Docker `Create` cannot recover from a lost success (medium)

`internal/engine/docker_linux.go:57-64`: if `docker run` actually succeeded
but the client side timed out, the container `tt-<id>` exists; every retry of
`Create` fails with a name conflict, and nothing adopts the existing container
(`Create` only inspects state after a successful run). Violates "a timeout
does not prove that creation failed" — recovery requires manual
`docker rm`. **Direction:** on run failure/timeout, `inspect` the container;
adopt it if it matches this VM's spec (and is running), else error.
**(parity unverified)**

### R13 — ZFS clone leaves non-retryable partial state (medium)

`internal/storage/zvol.go:141-152,155-160`: after the first `zfs clone`
succeeds, any later step failing (second clone, device wait, `rootfs.ext4`
symlink) returns without rolling back; retries hit "dataset exists" forever
until an operator runs `zfs destroy`. The error gives no recovery hint.
**Direction:** identify the clone as ours (creation/txg or name ownership) and
roll it back conservatively, or emit the manual recovery command in the error.

### R14 — `FileStore.CloneImage` preclaims the target then fails non-retryably (low)

`internal/storage/file.go:41-50`: `Mkdir` / `O_EXCL` placeholder first, `cp`
second; a mid-copy failure (disk full, signal) leaves the placeholder and every
retry gets EEXIST. Destroy paths clean it up, so the blast radius is one
stuck VM id. **Direction:** stage into a temp name and rename to publish, or
remove the freshly created placeholder on copy failure.

### R15 — bhyve `Create` leaks the VM process if pid-artifact write fails (medium)

`internal/engine/bhyve_freebsd.go:167-173`: `startChild` succeeded → pid write
failed → plain `return` with an unsupervised bhyve running; the retry is
refused as "bhyve is still running". The firecracker sibling kills the child
in the same situation (`firecracker_linux.go:193-196`); qemu self-writes its
pidfile and has no such window. Clear asymmetry. **Direction:** mirror
firecracker: `child.kill()` (+ tear down the created vmm/tap) on that error.

### R16 — Cannot-parse NAT handle is skipped silently, rule stays behind (low)

`internal/netcfg/net_linux.go:212-218`: a matched prerouting rule whose handle
fails `ParseUint` is skipped; the removal reports success while the DNAT rule
remains live — a later guest can inherit a forward to an old IP. Cleanup paths
should fail closed with "resources retained". **Direction:** return the parse
error.

### R17 — Network-config input validation gaps vs. the engine siblings (low)

- `internal/netcfg/net_freebsd.go:17-33`: existing `tt0` is never verified to
  be a bridge before `inet 10.10.0.1/16 … up` (Linux checks
  `info_kind == bridge`).
- `internal/engine/jail_freebsd.go:110`: `vm.IP` goes into `jail.conf` without
  `netip.ParseAddr`/`Is4`/10.10-subnet checks that qemu/firecracker perform.
- `internal/engine/sandbox_linux.go:139-144`: jailer uid derives from IP
  octets for **any** IPv4 (up to uid ≈16.9 M), not just the 10.10/16 space
  `IsolationRules` enforces.

None are reachable with normally-allocated fleet IPs; they are defense-in-depth
divergences between engines sharing one input type. **Direction:** one shared
"fleet IPv4" validator used by all three.

### R18 — Ownership guard asymmetric in `DestroyTap` (low)

`internal/netcfg/net_freebsd.go:145-148` vs `:154-161`: destroying by the
`tt-<hash>` name skips the `description: ttstack:<id>` check that the raw
`tapN` fallback enforces. **Direction:** verify the description before
destroying in both paths.

### R19 — Linux env file (with `TT_API_KEY`) written without fsync; `Close` error dropped (medium)

`internal/cli/deploy.go:213-221`: `WriteString`, unchecked `Close()`, no
`Sync`, while the unit file and the API-key file right around it do sync. A
crash in this window leaves a truncated env file and the service starts
keyless. **Direction:** check `Close` error, `Sync` before close.

### R20 — Remote key import silently prefers `tt-ctl.env` while the local path errors on the same divergence (low)

`internal/cli/deploy_remote.go:130` (`if ctl … elif agent`) vs.
`internal/cli/deploy.go:113-115` (fails with "existing agent/controller keys
differ"). Divergent keys are a misconfiguration the local flow refuses to
continue under; the remote flow proceeds and then writes that same key into
the new agent, cementing the split. **Direction:** same divergence check in
the remote script.

### R21 — A cleanup failure downgrades a successful deploy to exit 1 (low)

`internal/cli/deploy_remote.go:167-174,218-225`: service installed and
restarted successfully, then a hiccup in `rm -rf` of the staging dir makes the
whole command fail — opposite of the conservative "unknown outcome" posture
elsewhere. **(parity unverified)** **Direction:** degrade the staging cleanup
to a warning after the install half succeeded (or say so in the message).

### R22 — Deploy binary checks and error text are not demand-driven (low)

`internal/cli/deploy_remote.go:95-100` requires local `tt`/`tt-ctl`/`tt-agent`
binaries even for an agents-only config; missing-binary text says
"run 'cargo build --release'" in a Go tree. **Direction:** check per-role
inside the deploy branch; align the message with `make release`.

### R23 — Unknown-field skip is capped at 10 000 nesting levels; serde_json is not (low)

`internal/model/json.go:202-211`: non-strict decoding reads unknown values via
`json.RawMessage`, which is subject to `encoding/json`'s global 10 000-level
depth cap, while serde_json's `ignore_value` is iterative and only bounded by
memory. A legal body with a >10 000-deep unknown field (≈20-40 KB, well under
the 2 MiB cap) is accepted by the Rust stack and rejected by this port.
Mechanism confirmed locally. **(parity unverified on the pinned axum stack)**
**Direction:** iterative unknown-value skipper matching `ignore_value`, or
accept and document the deviation.

## Deferred for verification (recorded, not concluded)

1. **nft `add table` idempotency boundary.** Reported as *high* ("agent
   restart breaks network recovery: `nat.nft:1` uses `add table`"); empirically
   **refuted on the supported platform**: re-applying the full ruleset via
   `nft -f` succeeds on nftables v1.1.6 (tested on the Ubuntu agent; the
   2026-10-26 live test also restarted `tt-agent` with a running QEMU guest
   without any recovery failure). `nat.nft`/`isolation.nft` are byte-locked to
   the Rust golden fixtures, so the template cannot diverge anyway. Open
   question: on older nftables (≈1.0.x era, e.g. Debian stable servers) the
   same replay may fail with EEXIST. No minimum version is documented; if old
   servers are in scope, either probe-then-create in `SetupNAT` (requiring a
   deliberate deviation entry) or document the version floor.
2. **`tt deploy all` schedules with full detected capacity** (README quick
   path). Deployment docs explicitly tell operators to budget on shared hosts;
   usage oversight, not a code bug. Optionally improve the README.
3. **DELETE on absent entities returns 200** (agent `lifecycle.go:237-239`,
   ctl `hosts.go:removeHost` has no existence check). Whether Rust 404s here is
   undecidable without the oracle; the compatibility table's 404 row is
   ctl-worded. Keep with `(parity unverified)`; consider one fixture once the
   oracle revision is available again. Note the Go ctl↔agent pair self-heals:
   `docker State()` maps "no such container" to deletion semantics, so the
   externally-removed-VM scenario does not wedge the controller.
4. **Agent VM-row deletion after engine destroy is best-effort** (`lifecycle.go`
   around `DestroyVM`): `restoreEngine` runs before the row is removed, so a
   removal-time DB fault (single writer connection, cannot wrap engine ops in a
   tx) leaves a dirty row that is never re-cleaned; the next recount then pins
   capacity **forever** with only a warning (mechanic verified in
   `runtime.go:165-169`; `Account` also pins `VmCount`). Bootstrap is atomic by
   enqueue, so only a DB/writer fault reaches this. Conservative by contract,
   but permanence may exceed the intent — `(parity unverified)`; worth a
   best-effort cleanup retry or a documented note.
5. **Wire messages without golden cover.** Unexplained-outcome hint text,
   `"read request failed"`, settled headers on absent backups (`sequence 0`),
   empty-failure trailing text (R11). Fixture-only after oracle access.
6. **Producer-side guards absent at `internal/cli/parse.go:36`**: `tt image
   create <name> --engine docker` parses but silently drops `--engine` (only
   `create all` consumes it, `internal/cli/root.go:81-92`). Rust clap might
   accept-and-ignore identically — `(parity unverified)`; otherwise reject the
   flag in the non-`all` branch.
7. **Guest-config admission vs. runtime double validation may diverge on the
   SSH seed**: ctl admission (`internal/ctl/environment.go:45`) validates only
   the caller's files; the agent merges `ttstack-ssh.sh` into the same map
   before `WriteDisk`'s `Validate` (`internal/agent/runtime.go:489-515`). With
   exactly 32 caller files plus SSH enabled the env passes admission and fails
   at runtime — yet the documented limit text itself says "32 files / 64 KiB,
   **including the managed SSH seed**" (`capability.go:92`), which implies the
   seed-inclusive count belongs at admission. If Rust admits upfront, the Go
   port accepts one env shape the reference rejects and surfaces the failure
   later with a different status/message. `(parity unverified)` — descriptive
   note, low likelihood but visible in behavior.
8. Minor client ergonomics and further unverifiable parity details, recorded
   compactly: `--server httpfoo` treated as schemed (`client.go:31`);
   host-group response bodies read without a byte cap (`client.go:79`); GET
   timeout floor 60 s unrecorded in compatibility.md; probe timeouts in
   sshboot (200 ms/2 s) absent from the §6 timing table; 405 `Allow` header
   separator `","` vs axum's formatting; guest-config files written mode 0400
   inside ext4 images vs unknown Rust mode; `backup.Check`'s
   result→pending→revision ordering vs the unseen Rust order; sshboot
   error-priority order (user validation before key/engine checks). All low
   confidence; fixture-candidates once the oracle tree is restored.

## Areas with no findings (coverage statement)

PID/marker identity chains on all engines (proc/ps scanning, zombie exclusion,
multi-match rejection, disk-artifact ownership on FreeBSD), stop/delete
conservative semantics and SIGTERM→SIGKILL escalation, backup witness/manifest
identity protocol (owner files, inode identity, strict reflink, anonymous fds),
the ctl lock order (dbMu↔operationsMu, no network IO under lock, operation
serialization + revision fencing), receipt semantics end to end,
SQLite hygiene both sides (bindings, MaxOpenConns, WAL/busy-timeout values,
schema gates, recovery reads), JSON/presence/number-format wire conformance
(including fresh scrutiny of float formatting, timestamp shapes and
`null`-vs-missing handling), runcmd capture/invariants, guestconfig/sshboot
injections (cloud-init, kld config, param escaping), deployment TOML
whitelisting + sha256 staging + redacted logs, dashboard byte-equality and
escaping, CLI output/exit-code golden coverage. `gofmt`/`go vet` clean under
the linux build; linux and freebsd cross-builds pass.

## Disposition (same day, follow-up round)

Under the project owner's "no legacy burden; best design first" mandate, every
finding judged valuable was fixed and tested; the rest are documented. Mapping:

**Fixed with tests** — R1 (`storage/backup_other.go` stub; `runcmd.LockState`
also gained a darwin build so the full test gate passes on development
hosts), R2 (shared `pinToCapacity`, `/api/info` pins `VmCount`; fixture test),
R3 (typed `backupBusyError` sentinel), R4 (resize grow predicate; retry test),
R5 (`RemoveVMRelease` removes the row and returns reservations atomically,
plus `model.Resource.Release`; rollback test), R6 (corrupt ctl rows become
visible offline/failed markers; handler-level test), R7 (`PutEnvironment`
commits env+VMs+host reservations atomically; injection test), R8 (trailing
slash trimmed at registration), R9 (`ORDER BY id` on all fleet listings;
rewrite-order test), R10 (log line), R11 (distinct no-failures message), R12
(`inspect` adoption for lost `docker run` responses; unpause before stop;
tests + §7 row), R13 (zfs clone rollback of provably-owned datasets; unit
tests), R14 (remove the created placeholder on copy failure; retry test), R15
(bhyve kills its just-spawned child), R16 (fail-closed instead of a silent
rule leak; test), R17 (shared `netcfg.FleetIPv4` for jail/firecracker-uid
paths and the FreeBSD bridge-group check; platform table tests), R18
(ownership proof required for the named TAP destroy path too), R19 (env file
sync + close checking), R20 (remote key probe refuses divergent role keys),
R21 (staging cleanup failure degrades to a warning after a successful
install), R22 (per-role binary checks, Go build instructions). Companion
fixes from the same review: scheduler shadow accounting saturates (`SatAdd`),
guest-config admission for Firecracker + SSH is seed-inclusive (§7 row), SSH
ingress and PF anchors are applied from unlinked scratch files over stdin
exactly as §9 states, corrupt-inventory notices now reach the configured log
writer from the first boot line, and a malformed cgroup v2 `cpu.max` no
longer hides tighter ancestor limits.

**Documented, no code change** — R23 (unknown-member JSON depth cap became a
§7 deviation row; an iterative skipper was priced out), the older-nftables
replay caveat (deployment.md floor note), `deploy all` full-capacity startup
(already warned), idempotent DELETE for absent entities (kept), the
best-effort VM-row cleanup after engine destroy (conservative by contract),
and the wire-message trivia slated for fixtures once the oracle tree is
restored.

The subsequent 24-hour commit review found regressions in the resize retry,
host-marker recovery and Jail SSH changes. Their corrections and authorized-host
verification are recorded in [review-fixes-2026-10-26.md](review-fixes-2026-10-26.md).
