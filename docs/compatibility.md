# Compatibility contract with the Rust implementation

This document is normative for the Go port. Where the Go code and this document
disagree, one of them is a bug. Reference paths point into `../TTstack`.
The oracle is revision `1ad83a9feed5fac820d74b5e7ef752e50a552740` (0.6.0),
as pinned in [plan.md](plan.md). Rust source wins over an incorrect plan statement.

## 1. Scope

Three surfaces must match exactly:

1. **HTTP/JSON** — routes, methods, request and response documents, status codes.
2. **Persistent state** — SQLite files, schema versions, `_meta` keys, on-disk
   runtime artifacts.
3. **Host-visible identifiers** — names shared with the kernel, the init system or
   the filesystem that must survive a binary swap.

The CLI's printed output and exit codes are part of the contract as well, because
operators and scripts depend on them.

## 2. JSON rules (serde → Go)

serde has behaviors that Go's `encoding/json` does not reproduce by default.
Every wire struct must follow them:

| serde attribute | Meaning | Go implementation |
|---|---|---|
| `#[serde(default)]` on a field | Missing input uses its default; explicit null is not a missing non-optional field | Presence-aware decoding; use the Rust default, not automatically Go's zero value |
| `skip_serializing_if = "Option::is_none"` | Omitted from output when absent | `json:"name,omitempty"` on a pointer field |
| `skip_serializing_if = "Vec::is_empty"` / `"BTreeMap::is_empty"` | Omitted from output when empty | `json:"name,omitempty"` on a slice/map field |
| none of the above on `Option<T>` | Field is **always present**, `null` when absent | pointer field **without** `omitempty` |
| none of the above on `Vec`/`BTreeMap` | Always present, `[]`/`{}` when empty; null is invalid input | Non-nil empty slice/map plus explicit null checks, **without** `omitempty` |
| non-optional field without a default | Required on input, even when its zero value is meaningful | Track presence; ordinary Go decoding silently accepts missing fields |
| `deny_unknown_fields` | Unknown input keys are an error | Enforce in that type's codec, including nested objects and database decoding |
| `rename_all = "lowercase"` / `"snake_case"` | Enum/label spelling | custom `MarshalJSON`/`UnmarshalJSON` per enum |

Fields that are **always present and nullable** today (they must not gain
`omitempty`): `Host.error`, `Env.error`, `Vm.error`, `Pending.error`,
`Receipt.error`, `Status.reason`, `View.unsupported_reason`, `Request.generation`,
the optional fields of `backup.State`, and `VmSpec.cpu/mem/disk`.
`Generation` has no optional fields: all its scalars are required and non-null.
Fields that are **omitted when absent**: `Host.capability_report`, `Vm.ssh`,
`Vm.pending_resources`, `VmOptions.ssh`, `VmOptions.guest_config_digest`,
`SshInfo.observation_error`, `AgentInfo.capability_report`, `AgentInfo.vms`,
`ApiResp.data`, `ApiResp.error`, and every `Vec`/map that has an emptiness rule
(`warnings`, request `ssh_keys`, `guest_config`, `evidence`). In contrast,
`VmOptions.ssh_keys`, `VmOptions.ports` and `State.retired` are always present.
When in doubt, read the field's attributes in `crates/core/src/model.rs`,
`api.rs`, `capability.rs`, `backup.rs` and `ssh.rs`; those files are the source of
truth, and each wire type gets a golden fixture in this repository.

Codecs must also preserve exact field-name matching and duplicate-known-field
rejection. Go's default case-insensitive field matching, last-key-wins decoding,
null-to-zero handling and invalid-UTF-8 replacement for decoded strings are not
serde parity. Unknown
fields remain allowed on types without `deny_unknown_fields`; do not globally
enable strict rejection. HTTP bodies must contain exactly one JSON value.
Rust skips the values of unknown fields rather than decoding them as typed
strings; differential fixtures include ignored unpaired-surrogate escapes.

serde also accepts positional arrays for these derived structs. Their positions
follow Rust declaration order, including fields omitted by serialization. Missing
trailing fields use only explicit defaults; an `Option` field without a default
must occupy its position, even though it may be absent in object input. Output
remains an object. Positive and negative sequence fixtures fix this behavior.

Examples of non-zero defaults: absent `VmSpec.engine` means `qemu`; absent
`Vm.backup` creates a complete backup state with a fresh UUID revision. A present
`backup` must itself decode correctly and is not repaired by defaults. A wholly
absent `Vm.options` defaults as a unit, while a present options object still has
required fields. Apply these rules in persistence as well as HTTP codecs.

### Enums

JSON uses **canonical lowercase variant names only**; aliases exist only in CLI
parsing. Serde unit enums also accept an externally tagged object with exactly one
variant key and a null value, for example `{"qemu":null}`. Go accepts the same
input and always serializes it as the canonical string. Non-null values, extra
keys and duplicate variant keys remain invalid.
CLI case folding is ASCII-only, matching Rust `to_ascii_lowercase`; Unicode lookalikes
are not aliases.

| Type | JSON values | CLI aliases (not JSON) |
|---|---|---|
| `Engine` | `qemu`, `firecracker`, `bhyve`, `docker`, `jail` | `kvm`→qemu, `fc`→firecracker, `podman`→docker, case-insensitive |
| `Storage` | `file`, `zvol` | case-insensitive |
| `VmState` | `running`, `stopped`, `paused`, `creating`, `failed`, `deleting` | — |
| `EnvState` | `active`, `stopped`, `creating`, `deleting`, `failed` | — |
| `HostState` | `online`, `offline` | — |
| `Backend` | `zvol`, `reflink` | — |
| `backup.Action` | `create`, `restore`, `delete` | — |

An unknown value is a decode error — never a fallback. This matters for persisted
records: a row containing an unknown engine is *unreadable*, not upgradable, and
the strict reads (controller `get`/`list`) must fail while recovery reads skip the
row with a log line.

### Numbers and maps

- Timestamps are Unix seconds as unsigned 64-bit integers; `expires_at: 0` means
  "never".
- MiB units for memory and disk; ports are unsigned 16-bit.
- `port_map` is a JSON object (`{"22":20000}`) mapping guest port → host port.
  Use `map[uint16]uint16` and a compatible key decoder: Rust parses keys as u16,
  so arbitrary strings, leading zeroes, escaped digits and out-of-range values
  are invalid. Ordering is irrelevant
  to JSON semantics; CLI/rule construction must explicitly sort numeric ports.
- Saturating arithmetic is preserved everywhere reservations are computed.

Compare JSON fixtures structurally with integer-preserving decoding (`UseNumber`
or typed values), never through float64. Object order and equivalent JSON escapes
are not wire differences; missing, null, empty array and empty object are distinct.
Guest-config SHA-256 is an exception: its input is exactly serde_json's compact
UTF-8 serialization of the sorted string map. Go's default HTML escaping, even
`SetEscapeHTML(false)` for U+2028/U+2029, and Encoder's final newline need explicit
handling. Rust-generated digest vectors must include `<`, `>`, `&`, Unicode line
separators, control escapes and multibyte text.

## 3. HTTP surface

Both services answer with the envelope `{"ok":true,"data":…}` or
`{"ok":false,"error":"…"}`; `data`/`error` are omitted when absent; an empty
success through `ApiRespEmpty::ok()` is exactly `{"ok":true}`. Controller
environment delete/start/stop use `ApiResp::success(())` and include `"data":null`. Malformed JSON may produce a non-envelope body
(same as the framework behavior in Rust); clients check both the status and `ok`.

Controller routes (`crates/ctl/src/main.rs`):

| Method(s) | Path | Success |
|---|---|---|
| GET | `/` | dashboard HTML (public) |
| GET | `/api/capabilities` | capability `Matrix` |
| GET, POST | `/api/hosts` | `Host[]`; POST → 201 `Host` (200 on repeat) |
| GET, DELETE | `/api/hosts/{id}` | `Host`; DELETE → empty |
| POST | `/api/hosts/{id}/detach` | orphan `Vm[]` |
| GET, POST | `/api/envs` | `Env[]`; POST → 202 `EnvDetail` |
| GET, DELETE | `/api/envs/{id}` | `EnvDetail`; DELETE → empty |
| POST | `/api/envs/{id}/stop`, `/start` | empty |
| GET | `/api/vms/{id}` | `Vm` |
| POST | `/api/vms/{id}/resources` | updated stopped `Vm` |
| GET, PUT, DELETE | `/api/vms/{id}/backup` | `View` (+ `ETag: "<revision>"`) |
| POST | `/api/vms/{id}/backup/restore` | `View` |
| GET | `/api/images` | `ImageInfo[]` (online hosts) |
| GET | `/api/status` | `FleetStatus` |

Agent routes (`crates/agent/src/main.rs`): `GET /api/info`, `GET /api/images`,
`GET|POST /api/vms`, `GET|DELETE /api/vms/{id}`, `POST /api/vms/{id}/stop|start`,
`POST /api/vms/{id}/resources`, and the four backup routes.

Status overview (endpoint-specific, not one global error classifier):

| Condition | Status |
|---|---|
| Validation, design/capability rejection, invalid resize values, unsupported backup | 400 |
| Missing entity (`not found:`) | 404 |
| Busy environment, duplicate name, limits, conflicts, insufficient capacity on resize | 409 |
| Stale backup revision (`precondition:`) | 412 |
| Unschedulable placement | 422 |
| Missing backup precondition headers (`missing precondition:`) | 428 |
| Agent failure, unknown outcome, incomplete cleanup | 502 |
| Offline host during a mutation that needs it | 503 |
| Creation accepted | 202 |
| Agent creation capability rejection | 400 (agent) |
| Other agent create/start/stop/delete errors | 500 (agent) |

Backup UUIDs accept the Rust parser's hyphenated, simple-hex, brace-wrapped and
lowercase `urn:uuid:` forms. Hex digits may use either case; arbitrary wrappers
and uppercase URN prefixes are invalid. Validate this explicitly: Go's UUID parser
is more permissive for wrappers and URN prefix casing.

Backup mutations additionally carry `Idempotency-Key` (UUID) and `If-Match`
(quoted revision UUID), and the agent answers with `X-TT-Backup-Settled` /
`X-TT-Backup-Settled-Sequence` headers on failure when the writer has settled.
The message-prefix → status table lives in `crates/core/src/backup.rs`
(`error_status`) and is mirrored in `internal/backup`; its fallback is **500**.
Controller forwarding separately maps upstream 5xx, transport failure and invalid
responses to **502**, preserving upstream client errors. Other lifecycle endpoints
use their own status rules; do not apply the backup prefixes globally.

Authentication: `Authorization: Bearer <key>`, split at the first literal space,
scheme compared case-insensitively,
token compared in constant time (length is not hidden). Missing/invalid → 401 with
the standard envelope and `invalid or missing API key`. Do not trim the token or
normalize extra whitespace. Configured keys must be nonempty ASCII 0x21–0x7e.
No key configured → no authentication at all. There is no
per-owner or per-environment authorization anywhere.

Router/extractor behavior needs differential HTTP tests: HEAD, unsupported methods,
unknown routes, slash/escaped-path handling, JSON content type, size limits,
malformed/trailing JSON, and middleware order. `net/http.ServeMux` alone is not an
Axum compatibility layer; use an exact dashboard route (`GET /{$}`) instead of
letting `/` catch every unknown path. Capture Rust rejection status, headers and
body before choosing Go validation or HTTP server limits.

## 4. Persistent state

### Controller database (`ctl.db`)

```sql
CREATE TABLE _meta (key TEXT PRIMARY KEY, value TEXT NOT NULL);
CREATE TABLE hosts (id TEXT PRIMARY KEY, data TEXT NOT NULL);
CREATE TABLE envs  (id TEXT PRIMARY KEY, data TEXT NOT NULL);
CREATE TABLE vms   (id TEXT PRIMARY KEY, env_id TEXT NOT NULL, host_id TEXT NOT NULL, data TEXT NOT NULL);
CREATE INDEX idx_vms_env  ON vms(env_id);
CREATE INDEX idx_vms_host ON vms(host_id);
```

- `_meta.schema_version` = `5`; a database with a **higher** version refuses to
  open with the "upgrade TTstack first" error; `current < 5` runs the backup
  migration (materialize a default `backup` object with a fresh UUID `revision`
  only where the key is absent, preserving existing values and unknown fields).
  Parse each row as a JSON object, not a fully validated VM. An unparsable or
  non-object row aborts the transaction and leaves the version untouched.
- Pragmas: `journal_mode=WAL`, `synchronous=FULL`, `foreign_keys=ON`,
  `busy_timeout=5000`.
- Pending resource updates reserve only growth beyond the previous disk
  reservation. The VM intent and host disk counters commit atomically before
  forwarding; exact retries reuse that charge, and a definitive fresh rejection
  releases it with the intent.
- Row payloads are the JSON documents described in §2 — field names and omission
  rules included. Byte-identical serialization is not required (JSON object order
  is not meaningful), but decoding a Rust-written row and re-encoding it must
  produce the same document as Rust's decode-and-re-encode, including materialized
  defaults. Typed persistence does not promise to retain unknown fields; the raw
  JSON migration and offline converter do. Golden tests assert each contract.

### Agent database (`agent.db`)

```sql
CREATE TABLE _meta (key TEXT PRIMARY KEY, value TEXT NOT NULL);
CREATE TABLE vms   (id TEXT PRIMARY KEY, data TEXT NOT NULL);
```

- `_meta.schema_version` = `6`; a populated database with any other version is
  rejected (no automatic migration from v5 — the offline converter exists for
  that; see [the upgrade procedure](deployment.md#upgrade-and-offline-v5-conversion)). `_meta` also holds `host_id`, `network_config` and
  `container_runtime`.
- Unreadable VM rows are retained, mark the inventory as corrupt, pin resource
  reservations to capacity and disable admission.
- Changing the persisted network configuration or container-runtime binding while
  VM rows exist is refused.

"Populated" means any non-internal schema object exists, not that a VM row exists.
An empty v5 inventory is still rejected. Existing v6 state with missing tables is
not silently repaired. The writer uses a 5000 ms busy timeout and configures
WAL/FULL after the schema gate; read-only snapshots use 2000 ms. The agent does
not enable the controller's `foreign_keys=ON` pragma. Connection-local settings
must be applied to each actual Go connection, not only once to a pool. The offline
converter and host-identity connection retain rusqlite 0.40.2’s default 5000 ms
busy timeout as well.

### Host paths and artifacts

| Path | Content |
|---|---|
| `/home/ttstack/run` (`RUN_DIR`, fixed) | QEMU PID/monitor/seed files, Firecracker PID/socket/config/log/sandbox files, SSH seed scripts, bhyve/jail artifacts |
| `<runtime-dir>/clone-<vm-id>` | per-VM clone (directory or dataset) |
| `<runtime-dir>/.tt-backups/<vm>/<generation>/` | reflink backup: `owner`, `disk`, `ready.json` |
| `<data-dir>/service.lock` | exclusive `flock`, mode 0600 |
| `<data-dir>/backup-<vm-id>.lock` | backup writer lock |
| `/opt/ttstack/etc/api-key` | shared deployment key, mode 0600 |

ZFS names: base snapshot `@ttsnap`; backup snapshots `<disk>@ttbackup-<uuid>` with
user property `ttstack:backup=<vm-id>:<uuid>`; Firecracker zvol layout keeps
`vmlinux` in the parent dataset and the root filesystem in the `rootfs` child
volume, symlinked as `rootfs.ext4` inside the clone.

Backup dependency witnesses hash the exact compact serde JSON of `VmOptions`
with `requested_disk` normalized to zero, followed by each dependency's basename
and file bytes in the recorded order. CPU/RAM changes do not change this input.
As with guest-config digests, Go HTML/Unicode escaping must not change these
bytes. The storage layer takes this serialized options string unchanged; the
agent builds it when preparing a backup context.

## 5. Identifiers that must match bit-exactly

| Identifier | Rule |
|---|---|
| TAP name | `tt-` + zero-padded 12 hex digits of the **low 48 bits** of SipHash-1-3(zeros, `UTF8(vm_id) ‖ 0xff`); ≤15 chars |
| Guest MAC | `02:54:aa:bb:cc:dd`, two lowercase hex digits per IPv4 octet; `10.10.0.2` → `02:54:0a:0a:00:02` |
| Bridge | `tt0`, address `10.10.0.1/16`; with zero-based index `i`, set `n=i+1`, guest IP `10.10.<(n/254)&255>.<n%254+1>`; index 0 → `10.10.0.2` |
| nftables | table `tt-nat` (chains `prerouting`, `postrouting`, `forward`, set `denylist`); isolation tables named from the TAP with `-`→`_`; SSH ingress table `ip tt-ssh` |
| Firecracker jail | UID/GID `100000 + c*256 + d`; chroot `<image-parent>/.jailer/<binary>/<tap>/root`; cgroup `/sys/fs/cgroup/ttstack/<tap>`; socket `/fc-<tap>.sock` (<108 bytes) |
| Docker container | `tt-<vm-id>` |
| systemd units | `tt-ctl.service`, `tt-agent.service`, drop-in `90-ttstack-mounts.conf` |
| CLI config | `~/.ttconfig`, two lines (`addr`, optional key), mode 0600 |

Test vectors for the TAP name come from the Rust tests
(`crates/core/src/net.rs`): `""→tt-6ea523c53def`, `"vm1"→tt-e9e453b4d5a5`,
`"vm2"→tt-fc7c4d3fbec3`, `"83b7e95-test-vm"→tt-ab2cb9caf2f1`,
`"long-id-abcdefghijklmnopqrstuvwxyz"→tt-75e5d7552606`.

## 6. External commands and timeouts

Command lines, file names and timeouts are part of the contract where they affect
other software or the operator (for example the `nft` rule text, `zfs` snapshot
names, `jailer` arguments, `qemu-system-x86_64` arguments and the Firecracker boot
arguments). Defaults to preserve:

| Item | Value |
|---|---|
| Bounded command | 300 s default; stdout/stderr captured up to 4 MiB each |
| Agent/controller background loops | 15 s (reconcile, host refresh, image catalog), 60 s (expiry) |
| Client timeouts | 5 s health, 30 s registration/backup inspection, 360 s mutations, 600 s CLI mutations |
| Creation wait in the CLI | poll every 2 s, up to 600 s |
| QEMU shutdown grace | 10 s (`system_powerdown`), then SIGTERM/SIGKILL |
| Firecracker shutdown grace | 30 s after `SendCtrlAltDel`, then SIGTERM/SIGKILL |
| Backup storage budget | 240 s per attempt, 60 s per command, 16 MiB headroom probe |
| Port pool | 20000–65535 inclusive, at most 256 entries per VM, port 22 auto-added where the Rust version adds it |

QEMU's monitor is HMP text over a Unix socket, not QMP JSON. `cont` resumes a
paused process; agent start of a stopped VM boots the retained disk through the
engine's create path. PID recovery returns absence on a complete zero-match scan,
but errors on unreadable or ambiguous identity. Capture limits apply to returned
command output, not the size of the temporary files Rust uses during execution.

## 7. Deliberate deviations

These are the only intentional departures. Each must preserve observable behavior
at the API and filesystem boundary.

| Area | Rust | Go | Why |
|---|---|---|---|
| Downloads (image recipes, deployment helpers) | `curl` | Go HTTP client | fewer host prerequisites; same URLs, digests, atomically published files |
| Firecracker API calls | `curl --unix-socket` | HTTP client over the Unix socket | same requests, same 5 s timeout |
| CLI argument parsing | clap | explicit subcommand tree | dependency policy; pinned help and output fixtures, equivalent validation and exit codes |
| Error diagnostics | `ruc` may decorate business errors with ANSI escapes, timestamps, PID/namespace and Rust source locations | Go contextual error text without that process-specific decoration | preserve stable business messages and status/prefix mapping; avoid fabricated Rust stack locations |
| Async runtime | Tokio | goroutines/context | idiomatic Go |
| SQLite driver | rusqlite (cgo, bundled) | modernc.org/sqlite (pure Go) | no cgo; behavior fixed by pragmas and fixture tests |
| HTTP services | axum/hyper reads request headers without a timeout | 10 s `ReadHeaderTimeout` on both servers | bound drip-fed connections (Slowloris); complete requests at any speed are unaffected, no timeout is added for bodies or responses |
| Unknown JSON member depth | serde_json skips ignored members iteratively, at any nesting depth | `encoding/json`'s 10 000-level cap rejects bodies whose *unknown* members nest deeper | a faithful iterative skipper was judged disproportionate; only pathological bodies carrying >10 000-deep unknown members differ, and declared fields are unaffected |
| Docker creation after a lost response | a retry conflicts forever on the leftover container | a follow-up `inspect` adopts the container iff its image matches the immutable spec | a timeout must not prove creation failed; retries heal instead of wedging |
| Host reservation release | reservation drift converges on the 15 s refresh | a confirmed deletion returns the VM's reservations in the removal transaction | immediate availability; the accounting ceiling and the conservative floor are unchanged |
| Undecodable controller rows | every recovery read drops them (phantom rows keep their last state) | replaced with offline/failed markers carrying an operator-facing error; an offline host marker with no address can be re-registered by its verified host ID, retaining tracked VMs and reservation floors | keep damaged records visible and allow recovery without detaching their VMs |
| Guest-config admission | 32-file/64 KiB limit judged without the managed SSH seed at admission and with it at runtime | the seed-inclusive payload is judged at admission, matching the documented limit text | one Firecracker + SSH shape now fails earlier with a clear 400 instead of a runtime VM failure |
| Deploy diagnostics for missing binaries | `run 'cargo build --release'` | `run 'make release'` in local and distributed deployment, checked per deployed role | Go build instructions; agents-only deployments need no controller binaries |

Native phase-4 HTTP observation established that `ruc::Error::to_string()` can
include a timestamp, process ID, ANSI styling and source location inside the
JSON `error` string. These bytes vary across executions and build layouts. The
Go port retains the stable business message and contextual causes, but omits
that Rust diagnostic envelope. Differential tests explicitly normalize only this
envelope when comparing business failures; they do not normalize protocol
prefixes, status codes, JSON extractor errors or routing responses. This corrects
the earlier plan's overly broad promise of identical error strings.

CLI debug and diagnostic formatting may differ from Rust, as authorized for this
port; operation behavior, exit status, stable business prefixes and protocol
fields remain contractual. Deployment diagnostics redact `TT_API_KEY=` lines.
Go-native transport/storage/TOML diagnostic causes may also differ, including
human-readable `host.error`, `env.error` and `vm.error` text. Stable business
prefixes and status mapping remain exact. Header names are case-insensitive;
canonical spellings in this document do not prescribe wire capitalization.
CLI JSON maps with unsigned port keys retain numeric BTreeMap order. API clients
disable redirects and automatic retries; Go uses a fresh HTTP/1 connection for
each request to prevent transport replay of backup requests carrying idempotency
headers. Image downloads use a 15 s connect and 900 s overall timeout, an 8 GiB
limit, the curl limit of 50 redirects and the same pinned or manifest digests.

Anything else that changes a JSON field, a status code, a printed line, a path or
a host-visible name is a compatibility bug, not a design choice.

## 8. How compatibility is verified

1. **Golden JSON fixtures** per wire type, generated from the pinned Rust build
   with positive and negative decode cases; semantic round trips preserve number
   precision, presence and null rules. Byte fixtures cover digest input and text.
2. **Database fixtures**: checked-in synthetic SQL/JSON exports reconstruct
   temporary databases in Go tests. Differential tests create actual databases
   through Rust, mutate them through Go and reopen them through Rust. Record the
   oracle revision; never commit database files or private inventory.
3. **Identifier vectors**: TAP names, MAC addresses, IP allocation, digest and
   command-line golden tests.
4. **Cross-implementation runs** (phase 7, authorized hosts): Rust `tt-ctl` with a
   Go `tt-agent`, and Go `tt-ctl` with a Rust `tt-agent`, plus a Go CLI against
   both, on the same schema versions.

## 9. Reviewed input, process and shutdown boundaries

- `--` ends option/subcommand recognition. It can precede a leaf command's
  positional value but cannot introduce another command. A lone `-` is a string
  option value. Service resource/port flags use decimal unsigned integers,
  including a leading `+`; Go's octal/hexadecimal flag syntax is not accepted.
- Missing requirements are reported together in deterministic order. Parent help
  and help-subcommand help are available; error wording/order need not reproduce
  clap's cosmetic suggestions (see §7).
- Non-UTF-8 PID/process metadata is an unknown identity, not evidence of exit.
  Valid-text missing/stale PID metadata may still recover the exact process by
  marker. The prepared Firecracker version marker uses Rust's ASCII whitespace
  set, which excludes vertical tab.
- Controller shutdown first drains foreground HTTP handlers, then cancels detached
  creation/expiry waits while preserving durable plans and reservations. It waits
  for its own workers to exit before releasing the database/state lock. The agent's
  accepted-operation draining policy is unchanged. Client disconnection is distinct
  from service shutdown; accepted work may continue after disconnection.
- CLI runtime detection/pull, image inspection, `freebsd-version`, local deployment
  helpers and `scp` retain the Rust call sites' absence of an elapsed-time limit.
  SSH scripts remain bounded at 900 s; recipe conversion/population commands and
  core host operations keep their existing explicit bounds. Uncapped CLI commands
  still honor cancellation and the documented capture limit.
- Firewall stdin and reflink qualification use open, unlinked scratch files.
  Database compatibility means schema, pragmas, typed JSON values and recovery
  semantics; it does not promise identical SQLite page files, DDL whitespace or
  generic JSON key order/number spelling. Explicit digest inputs remain byte-exact.

The FreeBSD memory-discovery command remains bounded at 300 s and uses the
existing 8192 MiB fallback on failure; no guarantee of an indefinitely hung
startup probe is made.

The independent review and evidence are recorded in
[audit-resolution-2026-09-30.md](validation/audit-resolution-2026-09-30.md).
