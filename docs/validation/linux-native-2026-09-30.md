# Native Linux acceptance — 2026-09-30

Reference: TTstack 0.6.0, `1ad83a9feed5fac820d74b5e7ef752e50a552740`.
Go implementation: `cc6028b`; this phase adds validation tools/documentation.
Both authorized hosts ran Ubuntu 24.04, Linux 6.8.0-138/amd64 with 32 logical CPUs
and approximately 123 GiB RAM. Host addresses and administrative credentials are
intentionally omitted. Tests were functional, not stress or capacity benchmarks.

## Isolation and gates

Each host received a private systemd task unit bounded to four CPUs and 8 GiB,
with swap disabled and separate network, mount, PID and cgroup namespaces. These
limits, plus the initial low host load, stayed below the authorized half-capacity
budget. Compilation used a separate four-CPU/6-GiB unit; image preparation used a
two-CPU/2-GiB unit and a private mount namespace. No existing service was restarted.
The first host needed build tools for the Rust oracle; installation added packages
without upgrading installed packages and deferred service restarts.

The native Go 1.26.0 race/integration suite passed. All four Go binaries were built
with CGo disabled. The Rust CLI, agent and controller were rebuilt from the pinned
reference using Rust 1.96.1 and Cargo.lock. Production SQLite stayed pure Go.

The lab hid the host's `/home/ttstack`, `/run` and writable cgroup tree. All bridges,
TAPs, nftables rules and outer SSH ingress lived inside task network namespaces.
A separate Docker socket, store and daemon were used. The ZFS pool used a unique
name and only a task-owned 16 GiB regular-file vdev, with `cachefile=none`; no host
block device or existing pool was used. The file backend used a private 12 GiB
Btrfs image for strict reflinks.

## Real VM and storage matrix

Both QEMU and Firecracker passed on both file and zvol storage:

- Native image preparation, real guest boot and SSH to a provisioned account.
- Published SSH access from a separate peer namespace, and routed HTTP egress to
  a controlled peer fixture.
- A stale PID file pointing at PID 1, followed by Go/Rust agent and controller
  replacement while the actual guest and its disk marker remained available.
- Cold stop/start with retained disk, followed by offline CPU, RAM and disk growth.
- Backup creation, refresh to a new generation, real disk restore and boot-time
  verification of guest data.
- Another implementation swap and an exact completed restore replay after new
  guest writes, proving that replay did not apply the disk a second time.
- Backup deletion, environment deletion and removal of the owned runtime clone.

QEMU used the pinned Alpine cloud image, growing 4096→5120 MiB; Firecracker used
a prepared Alpine ext4 root, growing 512→768 MiB. Resource updates changed one
CPU/512 MiB RAM to two CPUs/640 MiB. Firecracker additionally passed readonly opaque
configuration, actual sudo access, and inspection of its per-VM cgroup CPU/memory
limits. Firecracker/jailer was 1.12.0, verified against the release SHA-256 file.

The prepared SSH kernel was the upstream Firecracker CI 5.10.233 image with
`CONFIG_HW_RANDOM_VIRTIO=y`, SHA-256
`3a30d91f3bf475e3950b9825d047c6839bc64249f383f6173912fe8afe89b5c3`.
Its source and preparation follow the
[upstream getting-started instructions](https://github.com/firecracker-microvm/firecracker/blob/v1.12.0/docs/getting-started.md).
The old builtin boot-smoke kernel did not provide usable entropy for this SSH
fixture. Also, Alpine cloud lacks sudo: its account test omitted sudo, while the
prepared Firecracker image explicitly installed it. Neither prerequisite issue
was bypassed by changing product admission or claiming application readiness from
process state alone.

One Rust stop observed a process between identity reads while it exited. The
reference retained its record and reported uncertainty. The harness inspected
state and retried the identical stop, which completed. The conservative behavior
was preserved; no production retry or identity check was weakened.

## Mixed fleet, networking and failure recovery

A single environment placed two QEMU guests across distinct Go and Rust agents,
with separate network and run paths. Both agents and the controller were then
swapped between implementations while the guests kept running. External SSH,
retained markers, stop/start/delete and cleanup passed. Managed SSH from the
private agent used an outer ingress namespace that was itself task-owned.
Deleting its guests removed only their owned outer rules.

Network tests demonstrated allowed egress, blocked guest-to-host access under
isolation, and denied outgoing traffic while inbound SSH replies remained usable.
The peer's return route was configured explicitly; it never used the real host's
management network.

A native zvol backup request was submitted and the HTTP client disconnected before
reading the result. It settled durably. After replacing both services with Rust,
an exact retry preserved the already-created generation. Holding an old snapshot
then demonstrated that refresh could publish a new recovery point while retaining
the old generation/reservation, and that the new point could be restored despite
the older hold. Releasing the owned hold allowed complete cleanup.

A private Docker workload passed published HTTP, retained writable-layer data,
Go/Rust takeover, stop/start/delete, and refusal to switch its retained inventory
to Podman. Native Podman was not run; its command and binding contracts remain
covered by fake-tool/unit tests. No claim is made about every external runtime
version or filesystem variant.

## Deployment

A further private systemd PID 1 used an isolated root, network, UTS and IPC
namespace, readonly host binaries/global sysctls, a private unit catalog and a
one-CPU/2-GiB sub-budget. Real `tt deploy all` installed and started the services.
The test ran CLI configuration under a normal root login, image validation, host
registration and the README's guest creation/show/stop/start/delete workflow.
Redeploying while the guest ran retained its state and reused the private shared
key. The actual agent unit reported `KillMode=process`.

The harness waited for HTTP readiness after systemd activation, since active
process state does not establish API readiness. OpenRC/distributed script text
and failure paths were validated through the phase-6 Rust goldens and fake SSH
harness, rather than modifying another host init system.

## Reproduction and cleanup

[tools/native/README.md](../../tools/native/README.md) describes the retained
operator-run harnesses and their isolation requirements. They are excluded from
ordinary local Go test execution and must not be run on the development machine.

Owned guests, services, mounts, loop devices, the file-vdev ZFS pool and task
namespaces are removed after validation. Host nftables rules and interface
configuration are compared against pre-test snapshots, excluding only counters
and IPv6 lifetime countdowns. The first lab’s cgroup reported zero OOM and OOM-kill events. Original Docker services remain active and
management SSH remains reachable. Private build/image caches may remain in the
task directory; no keys, images, databases or host inventories are committed.
