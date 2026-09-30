# Shared-core parity validation — 2026-09-30

Scope: roadmap phase 1, after the plan review and phase-0 commit `3fa4f53`.
The Go implementation is the phase-1 commit containing this report. Service,
engine and CLI workflows remain later-phase work; no VM lifecycle claim is made.

## Revisions and reproducibility

- Rust oracle: `1ad83a9feed5fac820d74b5e7ef752e50a552740`, workspace 0.6.0.
- Reference Cargo.lock SHA-256:
  `791f251f410e56db0ba247d5c0a8c82cdd1f97e26f8b5d74742042209192f62c`.
- Final Go source archive tested on the dedicated FreeBSD host (go.mod, go.sum,
  Makefile, cmd, internal and tools) SHA-256:
  `38c24efce8da7defa91cd460d9b70ef9f8912a12f9334ed3de644ad5eb2261a9`.
- Fixture regeneration: [oracle instructions](../../tools/oracle/README.md).
  Compilation ran remotely using Rust 1.96.1, with two build jobs. The reference
  checkout was archived, not modified. Fixture output records its provenance.

## Observations

| Check | Result |
|---|---|
| 1,642 Rust-generated cases across 41 wire type/instantiation entries | Passed Go accepted/rejected decoding and semantic output comparison with exact integers |
| Capability matrix and scoped report | Go output equals Rust output, including reasons and evidence |
| Guest-config digest | Exact compact UTF-8/escape bytes and SHA-256 agree with Rust |
| Accounting and backup protocol | Saturation, stopped-disk retention, pending/forwarded reservation maximum, completed replay precedence and error statuses pass |
| Authentication | Invalid keys do not leak input; bearer scheme case and exact token whitespace behavior pass |
| SSH bootstrap | Hostile key material passes `sh -n`; completed marker prevents re-execution; real generated identities differ and private material stays out of metadata |
| Config disk | Real e2fsprogs build/debugfs inspection confirms exact contents, mode 0600 and removal; fake formatter failure preserves the previous disk and removes staging |
| Command execution and locks | Output truncation, direct timeout, inherited descriptors, exclusive service lock and surviving-child backup lock retention pass |
| Local minimum-toolchain gates | Go 1.26.0 gofmt, vet, tests, build and CGo-disabled Linux/FreeBSD cross-builds pass, with GOMAXPROCS=2 and package parallelism 1 |
| Native FreeBSD race/integration suite | `TT_TEST_INTEGRATION=1 go test -race -p 1 ./...` passed; final unit-envelope, CLI folding and UUID validation changes rechecked in affected packages with the race detector |

Differential tests found and fixed three additional decoding details: Rust skips
unpaired-surrogate escapes in ignored extension values; derived structs accept
positional arrays with sequence-specific trailing defaults; numeric port-map keys
reject leading zeroes and escaped digits. These are recorded in the compatibility
contract. Rust's unit value is represented as JSON null, not Go's empty object.
Source review also tightened ASCII-only CLI enum folding and UUID wrapper/URN
validation where standard Go helpers accept inputs Rust rejects.

## Environment and limits

The dedicated authorized host ran FreeBSD 15.1-RELEASE-p3/amd64, with four logical
CPUs and approximately 12 GiB physical memory. Go 1.26.0, Rust, Git and e2fsprogs
were prepared there. Build/dependency downloads used the existing gateway jail;
native Go tests ran outside it. No gateway configuration, firewall, routes, bridge,
service or guest inventory was changed. Fresh SSH commands remained functional.

The ext4 utility test calls private file/tool mechanics on FreeBSD. The public
config-disk entry point remains Linux-gated, and this result does not validate
Firecracker, Linux networking or a Linux agent. Real disk-image files and temporary
SSH keys were confined to test temporary directories and cleaned by the tests.
Build caches, source copies and installed test prerequisites remain available for
subsequent authorized phases. No persistent test guest, mount or namespace exists.

Local tests stayed lightweight and did not create virtualization infrastructure.
Neither Linux test host was contacted. Full native engine testing follows the
FreeBSD-first order in the roadmap. Remote CI has not run; database compatibility
is still phase-4/5 work.
