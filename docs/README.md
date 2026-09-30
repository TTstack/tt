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

Feature guides and remaining documentation:

| Document | Lands with |
|---|---|
| [Deployment](deployment.md) | Implemented in phase 6 |
| [Guest images](guest-images.md) | Implemented in phase 6 |
| [HTTP API](rest-api.md) | Implemented in phase 5 |
| [Initial SSH](ssh.md) | Implemented; native acceptance tracked in phase 7 |
| [Disk backup](disk-backup.md) | Implemented; native acceptance tracked in phase 7 |

## Reading paths

- **New to the project**: [plan.md](plan.md) → [architecture.md](architecture.md) → [roadmap.md](roadmap.md).
- **Implementing a phase**: [roadmap.md](roadmap.md) → the matching section of
  [architecture.md](architecture.md) → [compatibility.md](compatibility.md) →
  the corresponding file in `../TTstack/crates`.
- **Reviewing a change**: [compatibility.md](compatibility.md) first; anything that
  changes a wire field, schema version, path or printed output must be recorded there.

## Source of truth

The Rust revision pinned in [plan.md](plan.md) defines behavior and wire format.
When Go code or these documents disagree with that oracle, fix the incorrect code
or document; implemented Go behavior does not automatically override the contract.
Approved implementation substitutions are listed in compatibility §7 and must
preserve the observable contract.

## Validation evidence

- [Shared-core parity, 2026-09-30](validation/shared-core-2026-09-30.md): Rust wire fixtures, bounded commands, native FreeBSD race/integration checks and limits.
- [Storage/network parity, 2026-09-30](validation/storage-network-2026-09-30.md): Rust network fixtures, native FreeBSD file/ext4/ZFS tests, ownership and crash recovery.

- [Feature coverage](validation/feature-coverage.md): every pinned README core behavior mapped to implementation/tests.
- [Native FreeBSD acceptance](validation/freebsd-native-2026-09-30.md): real Jail/bhyve SSH, live swaps and zvol restore/replay.
- [Native Linux acceptance](validation/linux-native-2026-09-30.md): QEMU/Firecracker file/zvol, mixed fleet/networking, Docker and isolated systemd deployment.
