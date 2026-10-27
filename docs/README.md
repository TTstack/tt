# Documentation index

This repository is the Go re-implementation of TTstack. These documents are the
maintained description of what is being built and why.

| Document | Read it when you want to… |
|---|---|
| [Plan](plan.md) | understand goals, scope, confirmed decisions and the delivery plan |
| [Architecture](architecture.md) | understand components, Go package layout, concurrency and data flow |
| [Compatibility](compatibility.md) | check the exact wire/SQLite/host contract with the Rust implementation |
| [Roadmap](roadmap.md) | see the phased task list, acceptance criteria and commit boundaries |
| [Rust → Go mapping](mapping.md) | find where a Rust module or type lands in Go |
| [Development](development.md) | build, test, lint and follow repository conventions |
| [AI workflow](ai-workflow.md) | invoke review/fix/commit skills and choose scope, ownership and validation |
| [Review guide](ai-review.md) | check Go/Rust parity, lifecycle invariants and finding evidence |
| [Current findings](audit.md) | track confirmed outstanding maintenance defects |

Operator guides:

| Document | Scope |
|---|---|
| [Deployment](deployment.md) | Prerequisites, service installation and offline upgrades |
| [Guest images](guest-images.md) | Recipes, config drives, storage, recovery and networking |
| [HTTP API](rest-api.md) | Routes, request defaults/limits and lifecycle |
| [Initial SSH](ssh.md) | Accounts, image contract, ingress and readiness |
| [Disk backup](disk-backup.md) | Qualification, stopped recovery points and exact replay |
| [Capabilities](capabilities.md) | Design matrix, scoped host reports and legacy gates |

## Reading paths

- **New to the project**: [plan.md](plan.md) → [architecture.md](architecture.md) → [roadmap.md](roadmap.md).
- **Implementing a phase**: [roadmap.md](roadmap.md) → the matching section of
  [architecture.md](architecture.md) → [compatibility.md](compatibility.md) →
  the corresponding file in `../TTstack/crates`.
- **Reviewing a change**: [compatibility.md](compatibility.md) first; anything that
  changes a wire field, schema version, path or printed output must be recorded there.
- **AI-assisted maintenance**: [AGENTS.md](../AGENTS.md) →
  [ai-workflow.md](ai-workflow.md) → [ai-review.md](ai-review.md), then only the
  subsystem guides needed for the selected task. Use the four `.agents/skills/`
  entrypoints explicitly when their review/fix/commit behavior is intended.

## Source of truth

The Rust revision pinned in [plan.md](plan.md) defines behavior and wire format.
When Go code or these documents disagree with that oracle, fix the incorrect code
or document; implemented Go behavior does not automatically override the contract.
Approved implementation substitutions are listed in compatibility §7 and must
preserve the observable contract.

## Validation evidence

- [Repository overhaul, 2026-10-27](validation/overhaul-2026-10-27.md): full-scope inventory, resize reservation/deployment diagnostic fixes, local gates and verification limits.
- [Shared-core parity, 2026-09-30](validation/shared-core-2026-09-30.md): Rust wire fixtures, bounded commands, native FreeBSD race/integration checks and limits.
- [Storage/network parity, 2026-09-30](validation/storage-network-2026-09-30.md): Rust network fixtures, native FreeBSD file/ext4/ZFS tests, ownership and crash recovery.
- [Engine parity](validation/engines-2026-09-30.md): command/configuration fixtures and ownership gates.
- [Agent parity](validation/agent-2026-09-30.md): schema, HTTP, lifecycle and backup failure contracts.
- [Controller parity](validation/controller-2026-09-30.md): scheduler, state interchange and service combinations.
- [CLI parity](validation/cli-2026-09-30.md): output/request goldens, deployment and recipe tests.
- [Feature coverage](validation/feature-coverage.md): every pinned README core behavior mapped to implementation/tests.
- [Native FreeBSD acceptance](validation/freebsd-native-2026-09-30.md): real Jail/bhyve SSH, live swaps and zvol restore/replay.
- [Native Linux acceptance](validation/linux-native-2026-09-30.md): QEMU/Firecracker file/zvol, mixed fleet/networking, Docker and isolated systemd deployment.

- [Independent audit resolution](validation/audit-resolution-2026-09-30.md): reproduced defects, rejected claims and regression scope.
- [Review regression fixes, 2026-10-26](validation/review-fixes-2026-10-26.md): resize recovery, controller host-marker recovery and native Jail SSH verification on the authorized hosts.
