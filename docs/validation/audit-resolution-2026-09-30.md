# Independent parity-audit review — 2026-09-30

Reviewed [the supplied audit](https://github.com/TTstack/tt/blob/0d7a14102fa3a285bf86c96d3c23125537c29769/audit.md)
against Go `866a305` and the pinned Rust
`1ad83a9feed5fac820d74b5e7ef752e50a552740`. The audit is evidence to investigate,
not a specification. Functional parity and conservative recovery remain the goal;
the operator explicitly permits differences in debug/log/diagnostic formatting.

## Applied changes

| Finding | Independent conclusion and change |
|---|---|
| CLI-1, CLI-2 | Confirmed acceptance inversions. `--` cannot select a new command; lone `-` remains a string option value, including shared service parsers. Leaf positional values after `--` still work. |
| CLI-3, CLI-4, CLI-6, CLI-7 | Improve actual command discovery and correction: aggregate missing requirements deterministically, show parent help, accept help-for-help, and name the missing backup counterpart. Exact clap suggestion text/order is not emulated. |
| IMG-1, DEP-1 | Source-confirmed absent deadlines. Runtime detection/pull, image inspection, local deploy helpers and scp now use an explicit uncapped runner. Explicit 300 s/900 s command bounds remain where the reference has them; cancellation and capture limits remain active. |
| DEP-3 | Confirmed duplicated context. Malformed TOML and structural parsing errors receive one `parse deploy.toml:` prefix. |
| CTL-1 | Reproduced with both binaries. After HTTP draining, Go now cancels detached creation/expiry waits, preserves their durable plans and waits for its workers to exit before releasing the state lock. Foreground HTTP mutations still drain. |
| ENG-1 | Confirmed conservative-identity mismatch. Non-UTF-8 PID metadata fails closed; invalid UTF-8 process-status data cannot establish exit either. Valid-text stale/missing PID recovery remains supported. |
| CORE-1 | Confirmed by the actual Rust byte primitive. Prepared-root marker trimming now excludes vertical tab. |
| STO-1 | Scratch files that need only a descriptor are unlinked immediately, including nft stdin and reflink qualification. A shared tested helper also serves command capture. |
| DOC-1–DOC-6 | Complete capabilities, prerequisites, offline conversion/WAL handling, request defaults and limits, guest configuration, ZFS preparation/recovery and network reachability guidance; fix cross-references and index all validation reports. SSH/backup guides and real guest evidence already landed in phase 7. |

Independent review also found a budget-parsing issue not listed in the audit:
Go's standard unsigned flags accept octal/hexadecimal syntax, while Rust uses
decimal integers. For example `--cpu-total 010` meant 8 in Go and 10 in Rust.
Resource and port flags now use decimal parsing with their original u32/u16 widths,
accept a leading `+`, and reject nondecimal prefixes and overflow.

## Claims not used as implementation instructions

| Finding | Disposition |
|---|---|
| CONV-2 | Incorrect. The pinned rusqlite 0.40.2 connection constructor installs `sqlite3_busy_timeout(db, 5000)`. An executable in-memory probe reports 5000 too. The existing Go timeout is retained and a short externally-held-lock test protects that behavior. See the [versioned API documentation](https://docs.rs/rusqlite/0.40.2/rusqlite/struct.Connection.html#method.busy_timeout). |
| CTL-5 | Not reproduced after actual forwarding: both native controllers finish the stop and record `stopped` after the HTTP/1 client disconnects. Keep service-owned accepted-work contexts; do not equate disconnection with cancellation. |
| CTL-3, CONV-1 | DDL whitespace and semantically equivalent JSON spelling/order do not break schema/state interchange. SQLite page bytes are not a portable contract. Preserve unknown fields and explicit digest byte contracts rather than rewriting valid state for cosmetic equality. |
| AGT-1 | Header names are case-insensitive. The documented conventional spelling is valid; clarify this instead of bypassing Go's header handling. |
| DEP-5 | `./bin` is the correct Go build example. Clarify that the fields/types match Rust while this example selects Go artifacts; do not point users at a Cargo output directory. |
| ENG-4 | The platform-specific goldens run in native FreeBSD race/integration suites, and real bhyve/Jail runs now cover both implementations. A Linux runner cannot execute FreeBSD-only implementation files; cross-builds remain separate evidence. |
| ACC-1 | Superseded by phase 7. Phase 5 already had synthetic service interchange; phase 7 adds real guests, file/zvol restore, networking, containers and isolated systemd deployment. |
| TOOL-1 | `make help` already includes install. Rust-specific build/doc aliases are not product parity requirements; existing Go build/install/deploy workflows work. No destructive uninstall shortcut is added merely to copy another Makefile. |
| CLI-5, SVC-1, DEP-2/DEP-4, CTL-2/CTL-4, AGT-3, STO-2, ENG-2/ENG-3, CORE-2 | Diagnostic wording, library contexts, ordering and cosmetic formatting do not warrant replacing Go libraries or cloning clap/ruc internals. Preserve validation, exits, stable business prefixes and HTTP status mapping; document native diagnostic causes, including persisted human-readable error text. |
| AGT-2 | Keep the bounded FreeBSD startup probe and its existing fallback. Matching an indefinitely hung sysctl is not a useful operator guarantee; document the bound rather than weakening startup behavior. |

## Evidence and regression scope

The actual pinned configuration parser produced CPU 10, memory 12, disk 8 and port
2000 for `010`, `+12`, `008` and `02000`. Its in-memory rusqlite connection reported
5000 ms, and `b"1\x0b".trim_ascii()` did not equal `b"1"`.

The native lifecycle probe first demonstrated the mismatch: Rust exited before a
blocked agent create replied and retained `creating`; pre-fix Go waited and later
stored `active/running`. Both retained normal accepted stop work after the client
disconnected. The regression tests distinguish detached cancellation from
foreground draining, retain planned/deleting rows and reservations, and reconcile
a remote completion after reopening the controller.

Additional tests cover unsafe subcommand selection, dash values, help discovery,
complete missing-argument reports, decimal service budgets, invalid PID/status
encoding, exact prepared-marker whitespace, private anonymous descriptors,
uncapped command cancellation and the separate CLI timeout plans. No test weakens
PID checks, retries an ambiguous backup under a new UUID, or relies on the stale
Rust agent binary in a developer `target/` directory.

Native revalidation uses the authorized FreeBSD host first, then bounded Linux
checks. The small lifecycle/parser probes use only synthetic loopback state;
there is no need to recreate the already-passed guest fleet for parser/document
changes. The anonymous reflink path is checked separately on a private Btrfs image.

Final results: Go 1.26.0 formatting/vet/unit/build and CGo-disabled Linux/FreeBSD
cross-build gates passed. Full native race/integration suites passed on FreeBSD
and Linux. The rebuilt Go controller produced the same lifecycle-probe result as
Rust (`creating` retained at shutdown, `stopped` after the forwarded disconnected
stop), and the parser probe rejected command selection after `--` without issuing
HTTP requests. The strict reflink create/restore test also passed with anonymous
qualification on a private 256 MiB Btrfs image; the mount and loop device were
removed afterward. No new VM fleet, host firewall changes or local virtualization
infrastructure were required for this review.
