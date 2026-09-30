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

Planned documents that land together with their features:

| Document | Lands with |
|---|---|
| `deployment.md` | Phase 6 (CLI deploy) |
| `guest-images.md` | Phase 6 (image recipes) |
| `rest-api.md` | Phase 5 (controller API) — interim: use the [Rust reference](../../TTstack/docs/rest-api.md) |
| `ssh.md` | Phase 4 (agent SSH bootstrap) |
| `disk-backup.md` | Phase 4/5 (backup protocol) |

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
