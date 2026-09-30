# tt — TTstack in Go

`tt` is a Go re-implementation of [TTstack](../TTstack) (the Rust workspace at
`github.com/TTstack/TTstack`): a lightweight private cloud that manages VMs and
containers across a small fleet of hosts. Workloads are grouped into environments,
have a lifetime, and can be stopped, restarted, resized and deleted.

This repository is a work in progress. **Phases 0–1 provide build tooling and the shared core**. The binaries
currently expose help/version only and refuse service startup; implementation
continues in phases described in [docs/roadmap.md](docs/roadmap.md).

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
- the same SQLite schema versions (controller v5, agent v6) and table layouts;
- the same CLI surface, output formatting and exit codes;
- the same host-local contracts that survive a binary swap (tap names, bridge
  addresses, runtime paths, systemd units, key files).

The completed port is intended to let a Rust controller talk to a Go agent and a
Go controller manage a compatible fleet. This interoperability still requires the
service implementations and acceptance tests in the roadmap.
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
Docker/Podman. The planned experimental FreeBSD bhyve/Jail paths use the same native-host
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

The following is the target workflow; it becomes executable with phase 6.
It mirrors the Rust release. See [docs/plan.md](docs/plan.md) and the
phase-6 deployment documentation that lands with the CLI
([roadmap](docs/roadmap.md)).

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

## License

MIT — same as TTstack. See [LICENSE](LICENSE).
