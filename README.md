# tt — TTstack in Go

`tt` is a Go re-implementation of [TTstack](../TTstack) (the Rust workspace at
`github.com/TTstack/TTstack`): a lightweight private cloud that manages VMs and
containers across a small fleet of hosts. Workloads are grouped into environments,
have a lifetime, and can be stopped, restarted, resized and deleted.

**Phases 0–7 are implemented and validated** against the pinned Rust 0.6.0
release. The CLI, deployment, image recipes, agent, controller and offline
converter are functional. Native testing covers mixed Go/Rust services, real
QEMU/Firecracker file and zvol workflows, Docker, isolated systemd deployment,
and experimental FreeBSD Jail/bhyve. See the [Linux](docs/validation/linux-native-2026-09-30.md)
and [FreeBSD](docs/validation/freebsd-native-2026-09-30.md) evidence for the tested
matrix and limits, and [feature coverage](docs/validation/feature-coverage.md).

Optional compute extensions add durable command receipts, persistent volumes,
workspace checkpoints, portable archives, verified image caches and epoch
recovery. They preserve legacy API defaults but upgrade databases forward to
controller v9/agent v10. These are retained development markers: before v1.0,
schema versions do not advance with feature iterations and older development
DB compatibility is not required. See the
[DB schema policy](docs/development.md#db-schema-policy-before-v10).
Current native evidence is in the
[upstream backport report](docs/validation/upstream-backports-2026-10-30.md).

## Why Go

TTstack is a control plane: it schedules workloads, keeps state, and drives
hypervisors and container runtimes through their CLIs and devices. The
CPU-intensive work happens inside the guests, so the language is chosen for
engineering properties rather than raw speed:

- fast builds and a closed, pure-Go dependency set make frequent
  rebuild-and-redeploy across a fleet cheap;
- cross-compilation from one machine covers every target (including the
  experimental FreeBSD hosts) without a cross toolchain;
- statically linked single binaries deploy by file copy, with no libc or
  shared-library coupling;
- the standard library already covers the HTTP server and client, JSON, SQL,
  process execution, hashing and testing.

The full rationale, including what is given up and how the port compensates, is
in [docs/plan.md §3](docs/plan.md#3-why-go).

## Compatibility

The Go implementation targets **full wire compatibility** with the Rust release:

- the same HTTP API and JSON envelopes (`{"ok":…,"data":…}` / `{"ok":…,"error":…}`);
- forward migration from the pinned controller v5/agent v6 SQLite layouts;
  optional recovery protocols use controller v9/agent v10 (see
  [the upgrade boundary](docs/compatibility.md#10-durable-command-envelope-optional-extension));
- the same CLI surface, output formatting and exit codes;
- the same host-local contracts that survive a binary swap (tap names, bridge
  addresses, runtime paths, systemd units, key files).

A Rust controller can talk to a Go agent, and a Go controller can manage a
compatible mixed fleet. Native service, state and guest interoperability passes for the recorded
validation matrix.
The exact contract is written down in [docs/compatibility.md](docs/compatibility.md).

## Architecture

```text
tt CLI / Browser → HTTP → tt-ctl → HTTP → tt-agent (one per host)
                          │                 │
                     SQLite state      SQLite state
                                       engines, disks, networking
```

| Component | Responsibility |
|---|---|
| `tt` | CLI client, local image recipes, deployment over SSH |
| `tt-ctl` | Scheduling, environment lifecycle, fleet state and web dashboard |
| `tt-agent` | Host-local VM/container lifecycle, storage and networking |

Linux x86_64 is the supported host platform with QEMU/KVM, Firecracker and
Docker/Podman. The experimental FreeBSD bhyve/Jail paths use the same native-host
gates as Rust, with manual setup and limited validation.

## Building

Requires Go 1.26 or newer. Builds use four direct Go modules, including the
pure-Go `modernc.org/sqlite` driver. Release and cross-builds disable CGo.

```bash
make build     # debug binaries in ./bin
make release   # optimized binaries
make test      # unit tests
make lint      # gofmt check, go vet
make cross     # compile Linux/amd64 and FreeBSD/amd64 without CGo
```

## Quick start (single Linux/systemd host)

This workflow is covered by native validation in an isolated Linux/systemd root.
Prepare the host prerequisites in [deployment.md](docs/deployment.md), and wait
for API/image-catalog readiness when automating the commands.
```bash
make release
sudo ./bin/tt deploy all --release-dir ./bin
export PATH="/opt/ttstack/bin:$PATH"

export TT_API_KEY="$(sudo cat /opt/ttstack/etc/api-key)"
tt config 127.0.0.1:9200
unset TT_API_KEY

sudo mkdir -p /home/ttstack/images
sudo /opt/ttstack/bin/tt image create alpine-cloud
tt host add 127.0.0.1:9100
tt image list  # wait until alpine-cloud appears in the periodic catalog
tt env create demo --image alpine-cloud --engine qemu \
  --cpu 1 --mem 256 --disk 2048 --ssh-key ~/.ssh/id_ed25519.pub
tt env show demo
tt env stop demo && tt env start demo && tt env delete demo
```

## Documentation

| Document | Scope |
|---|---|
| [Plan](docs/plan.md) | Goals, scope, decisions and the delivery plan |
| [Architecture](docs/architecture.md) | Components, packages, concurrency and data flow |
| [Compatibility](docs/compatibility.md) | Wire/SQLite/host contract with the Rust implementation |
| [Roadmap](docs/roadmap.md) | Phased task list and acceptance criteria |
| [Rust → Go mapping](docs/mapping.md) | Where each Rust module lands in Go |
| [Development](docs/development.md) | Build, test, lint, dependency policy |
| [AI workflow](docs/ai-workflow.md) | Explicit Codex skills, review evidence, worktree safety and scoped validation |

## License

MIT — same as TTstack. See [LICENSE](LICENSE).
