# Review evidence and tt invariants

Use this guide for the [AI maintenance workflows](ai-workflow.md), together with
the task's relevant existing documentation. Review Go behavior against the Rust
revision pinned in [the plan](plan.md), using the Go-specific checks below.

## Evidence standard

For each candidate, establish the required contract, reachable input or operation
sequence, observable wrong result, and why existing guards/recovery do not
prevent it. Read complete functions, callers, error paths and relevant tests.
Separate a code trace from an actually reproduced failure. Prefer a focused Go
regression using `testing`, `httptest`, temporary SQLite state or fake commands;
use real disks/engines only in the authorized integration/native environment.

Use [mapping.md](mapping.md) to locate the corresponding Rust implementation.
If `../TTstack` is absent, do not invent Rust behavior or silently fetch another
revision. Committed oracle fixtures and normative docs can establish their
covered contract; mark a source-dependent comparison **parity unverified** when
they do not. Read [the oracle guide](../tools/oracle/README.md) before regenerating
fixtures or running an oracle. A cached executable's filename is not provenance.

Severity follows demonstrated consequences:

| Severity | Consequence |
|---|---|
| CRITICAL | Data loss/corruption, destruction or adoption of another workload's resources, credential disclosure or authentication bypass on a reachable path |
| HIGH | Supported lifecycle failure, persistent reservation/state inconsistency, deadlock, or material API/state incompatibility |
| MEDIUM | Bounded incorrect behavior, recovery failure or leak under a concrete narrower trigger |
| LOW | Incorrect docs, verification or diagnostics with a demonstrated operational cost |

Assign severity from the actual trigger and impact, not the subsystem name or
review effort. Performance findings need evidence of material cost; this project
does not authorize stress tests to manufacture that evidence.

## Subsystem checks

Map each changed file to its behavior. For `all`, inventory every tracked source,
test, fixture, build/tooling and documentation group; mark coverage or a reason
for exclusion. Load only the guides relevant to that group.

| Scope | Contracts and failure paths | Read when relevant |
|---|---|---|
| `cmd/`, build, CI | Wiring only; Go 1.26 minimum; closed dependencies; Linux/FreeBSD `CGO_ENABLED=0` builds; unsupported-OS fallbacks; `_other.go` needs explicit negative constraints | [Development](development.md), [Architecture](architecture.md#2-package-layout) |
| `model`, `api`, HTTP codecs | Integer bounds, enum spellings, presence/null/default/empty distinctions, exact omission rules, status/error mapping; decode as well as encode | [Compatibility](compatibility.md), Rust-generated `testdata/` |
| `auth`, `runcmd` | Same bearer-key parsing and comparison; redirect/timeout rules; cancellation, 300 s default command bound and 4 MiB per-stream capture; no secret output | [Compatibility](compatibility.md), [Architecture](architecture.md) |
| `agent`, `ctl`, SQLite | Schema gates v6/v5, WAL/pragmas, `_meta`, lock/host identities, atomic state transitions, reservations and reconciliation after interrupted or unknown outcomes | [Compatibility](compatibility.md), [Architecture](architecture.md) |
| `engine` | Validate PID marker and ownership before action; stale metadata, unknown create outcome, restart recovery; cold stop/start retains disks but loses memory; runtime selection does not silently change stores | [Architecture](architecture.md), [Guest images](guest-images.md), [Initial SSH](ssh.md) |
| `storage`, `backup` | Qualify backend; retain previous generation on failure; exact replay and preconditions; pending intent survives partial disk/filesystem growth; no cleanup of unowned/unknown resources | [Disk backup](disk-backup.md), [Guest images](guest-images.md) |
| `netcfg`, ingress | Bit-exact TAP/MAC names and fixed paths; bridge/firewall/PF ownership; isolated cleanup; no accidental management-network changes | [Compatibility](compatibility.md), [Initial SSH](ssh.md) |
| `guestconfig`, `sshboot`, capabilities | Opaque file bytes/digests, ext4 growth, SSH option/script quoting, process versus application readiness, host-OS gates and admission matrix | [Capabilities](capabilities.md), [Guest images](guest-images.md), [Initial SSH](ssh.md) |
| `cli`, dashboard, deploy | Byte-exact contractual text and scripts, exit codes, request construction, config defaults, downloads/checksums and atomic publication, service restart/rollback | [HTTP API](rest-api.md), [Deployment](deployment.md), CLI goldens |
| Docs, tools, `.agents/` | Commands and references match current Go behavior; fresh evidence is distinguished from historical reports; no invented validation, automatic releases or implicit host authorization | [Workflow](ai-workflow.md), [Development](development.md) |

Cross-cutting Go checks apply when relevant:

- Derive lock ordering and mutation serialization from actual callers. Check
  goroutine ownership, context cancellation, shutdown, channel/timer lifetimes
  and retry boundaries. A race-detector pass does not establish deadlock freedom.
- Trace reservations, pending intent and external side effects across every
  database/HTTP/process failure. Lost responses do not prove failure; retries
  must not duplicate destructive work or release resources prematurely.
- Check HTTP request contexts, body closure, redirects, raw paths and timeouts
  against the existing contract. Error wrapping may be internal, while strings
  exposed on the wire remain contractual.
- Keep platform behavior selected by the native host OS. Linux and FreeBSD
  cross-builds verify selection/compilation; only native evidence verifies
  kernel, filesystem, firewall, service and engine behavior.

## Refute before recording

- A Go/Rust difference is actionable only against the pinned contract and its
  documented deviations. Go's API shape need not mirror Rust's module structure.
- Conservative resource retention after unknown outcomes can be required. Trace
  eventual reconciliation before alleging a leak; never fix it by early release.
- Missing/null/empty JSON, route spelling, status codes, defaults and error strings
  may look unusual because parity requires them. Do not normalize them for style.
- A `panic`, `TODO`, empty result or ignored error needs a reachable supported
  trigger and consequence. Test-helper panics and explicit unsupported-platform
  gates are not automatically defects.
- Check Go build constraints and normal callers before claiming dead code or a
  missing implementation. Do not flag FreeBSD 15.x's documented limits
  as newly missing features; do report regressions in its supported paths.
  Native FreeBSD validation is FreeBSD 15.x only.
- Tests using mocks/fakes establish the contract they exercise. They do not
  establish native engine, strict reflink, ZFS or guest SSH acceptance. Historical
  reports prove their recorded revision/matrix, not current HEAD.
- Suggestions for generic hardening, observability or refactoring need a project
  requirement and concrete failure. The Rust release remains the product boundary.
  Static tools handle formatting/lint; review findings need additional substance.

## Audit registry

[audit.md](audit.md) contains current actionable findings, not a copy of every
historical review. An empty registry means no recorded Open entries; it does not
certify a complete audit. Existing dated reports under `docs/validation/` remain
historical evidence; recheck their claims before promoting one into the registry.

Use two sections:

- **Open**: confirmed actionable defect, including one awaiting verification of
  a proposed fix, ownership resolution or required validation.
- **Won't Fix**: real defect with a concrete reason a safe fix is currently
  disproportionate. A failed check or missing access alone is not this reason.

Do not record speculative parity claims as confirmed findings. Report unresolved
candidates and coverage/access limits in the review output. A confirmed defect
can remain Open with the proposed fix's parity verification explicitly pending.

Use a stable identifier that is never reused (check Git history when needed),
deduplicate by root cause and order by severity. Each entry contains:

```markdown
### TT-<id> [SEVERITY] subsystem: summary

- Where: `path` (`function`; line if needed).
- Contract and evidence: required behavior, source/fixture/test, reproduction or trace.
- Trigger and result: reachable sequence and observable wrong outcome; why guards fail.
- Fix and validation: minimal direction, regression, compatibility impact and blockers.
```

Won't Fix entries also include **Reason** and a condition for reconsideration.
Recheck relevant retained entries when their code or assumptions change. Remove
fixed, obsolete or disproven entries with evidence in the fix, report or Git
history. Keep unrelated entries during narrow reviews. Do not add a Resolved
section or timestamps that imply current validation. In `x-review`, registry
edits are the only default write; an explicit no-writes instruction overrides it.
