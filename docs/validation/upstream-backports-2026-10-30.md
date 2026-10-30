# Upstream baseline and feature backports — 2026-10-30

Baseline: upstream `34c4666343541de110f672a67f6e1dbb445d733e`.
The candidate ports generic recovery protocols from source `d4357a3`, configurable
Linux stop grace, and short Unix-socket test paths. The upstream module path,
internal API/model packages, six-hour default lifetime, implicit SSH forwarding,
dependency versions and Dependabot configuration are retained. New commits use
`Your Name <you@example.com>` for both author and committer.

The current controller/agent schema markers remain 9/10. Before v1.0, maintain
only the latest development layout without incrementing these markers or
requiring compatibility with earlier development databases. Version checks and
migration infrastructure remain for future stable releases; see the
[DB policy](../development.md#db-schema-policy-before-v10). Current state is
outside pinned Rust state compatibility, as described by the
[upgrade boundary](../compatibility.md#10-durable-command-envelope-optional-extension).
No product version bump or new dependency is included.

## Isolation and provenance

The operator authorized one Linux/KVM host and one dedicated FreeBSD host.
Fresh task-owned service state and small sequential guests were used. Linux
agents/guests ran in private mount, network, PID and cgroup namespaces, capped at
4 CPU and 8 GiB on a 24-thread, 30-GiB host. Private nested fixtures covered a
second agent, containers and systemd; the latter had a 1-CPU/2-GiB sub-budget.
Build/race work used `GOMAXPROCS=2` and Go `-p 1`. FreeBSD used one small guest at
a time; its existing gateway jail was untouched. Native FreeBSD coverage for this run is 15.1 only.

The tested final Go-source digest, over sorted repository-relative Go filenames,
`go.mod` and `go.sum`, separated from their contents by NUL bytes, is
`188624b292604ffdbaba5f3580043f292ebdaf31c1ae93ff63d594559b1f409e`.
Later edits only complete documentation. Baseline and candidate binaries and
SQLite directories were separate; baseline results below do not refer to the
initial source-comparison runs.

Scoped copies of `tools/native/` used distinct root/binary/data paths and Go-only
service restarts. No cached Rust executable was treated as an oracle. The pinned
Rust checkout and provenanced native binaries were unavailable, so fresh Rust
source comparisons, backward state interchange and cross-language takeover were
not rerun. Committed Rust HTTP/JSON/command/CLI fixtures ran in the Go suites.
Historical feature-source reports linked here are not current acceptance evidence.

## Native matrix

| Surface | Upstream baseline | Backport candidate |
|---|---|---|
| FreeBSD Jail: real SSH, live service restart, retained marker after cold stop/start, delete | PASS | PASS |
| FreeBSD bhyve/file: same lifecycle and retained-disk checks | PASS | PASS |
| FreeBSD bhyve/zvol: backup refresh, restore, exact replay retaining later writes, delete | PASS | PASS |
| Linux QEMU/file and zvol: SSH account/published port, egress, stale PID, service restart, CPU/RAM/disk growth, backup refresh/restore/replay, delete | PASS | PASS |
| Linux Firecracker/file and zvol: same matrix plus jailer CPU/memory limits, sudo, opaque read-only config disk | PASS | PASS |
| Docker: published HTTP, retained writable layer, runtime-switch refusal, restart and cold lifecycle | Not separately rerun | PASS |
| Podman: published HTTP, observed pause/resume, retained writes, agent restart, runtime binding refusal, cold lifecycle and delete | Not separately rerun | PASS |
| Two isolated Linux agents: one environment on both, outer SSH ingress, blocked host access, allowed/denied egress and cleanup | Not separately rerun | PASS |
| Two physical hosts: one QEMU/Jail environment, real SSH markers, API auth/dashboard, catalogs, controller/Linux-agent restart, cold lifecycle, host removal | Not separately rerun | PASS |
| Private systemd: `deploy all`, key reuse, login config, image validation, README lifecycle and live deployment restart | Not separately rerun | PASS |
| Six-hour API lifetime default and real two-second expiry reclaimed by the 60-second sweeper | Unit/fixture | PASS |

The native Docker/Podman engine code is unchanged from upstream. Individual
Debian/Ubuntu/container recipe variants were covered by their existing verified
recipe tests, rather than booting every distribution. Actual native preparation
used `alpine-cloud`, `fc-alpine`, a prepared SSH-capable Firecracker fixture, a
FreeBSD base recipe and a manual bhyve UFS image. Firecracker/jailer 1.12.0 and
image/kernel checksums were verified. ZFS 2.4.1 userspace tools and Podman 5.7.0
were extracted into the private test scope; no host Docker store was used.

| Optional protocol | Candidate observation |
|---|---|
| Stop policy | 1-second graceful-only refusal retains the guest; force and bounded escalation stop it; later explicit start works |
| Persistent volumes, direct agent and controller, file/zvol | Stopped provision, root/data binding, boot, VM-delete retention, rebind, growth, immutable checkpoint clone, ancestry/retention refusal and cleanup |
| Complete workspace recovery, file/zvol | Complete root/data manifest, writer-fence release, new stopped VM/new disk identities, content preservation, boot and dependency refusal |
| Portable archives/image cache | File Firecracker/QEMU and zvol Firecracker root/data round trips, digest/size envelope, refreshed-grant replay, restored boot, verified image pull/boot, corruption retention, explicit abort and cache cleanup |
| Backup fault/recovery | Disconnect after durable admission, same-ID/revision replay, held retired snapshot across refresh/restore, explicit release and cleanup |
| Physical recovery, QEMU and Firecracker | Passive surprise-exit observation preserves durable claims; actual predecessor survives receiver stop; SQLite rollback loses its row; new epoch refuses old commands; pidfd fence stops the scoped untracked process; receipt replay, complete orphan inventory and data preservation |
| Controller/receiver activation and enrollment | Real services verify reads/mutations/GC chains, stable replay, wrong plan/scope/host refusal, legacy-registration refusal and explicit enrollment without adopting an existing running agent VM |
| Quarantine and physical paging | A 32-MiB orphan reserves capacity across disappearance and receiver restart; over-capacity allocation is refused; one-item pages retain snapshot/digest identity, bad cursors are refused, epoch changes expire pages and changed evidence prevents activation |
| Recovery boundaries and cleanup gates | Old workload success cannot replay in a new epoch; controller barrier leaves QEMU running, physical receiver fence stops it; forwarded deletion requires both GC gates; a new reviewed empty epoch releases the retained physical-capacity floor |
| ZFS native unit gates | Real backup/restore/ownership and logical-capacity inventory with missing-root detection both PASS under dedicated quota-limited parents |

The supplementary `tools/native/control.py` run used a separate private Linux
lab capped at 4 CPU/4 GiB, with a single 1-CPU/256-MiB QEMU guest. Its three
reviewed epochs used separate receiver authority files and one shared deployment
identity. The retained QEMU capacity envelope was 80 MiB after the orphan was
resolved; VM deletion and restart retained that floor until the next reviewed
empty epoch explicitly replaced it with zero. These checks ran through actual
HTTP services, not a fleet orchestration product. The managed cgroup fence does
not prove whole-host power fencing or containment of unrelated legacy workloads.

## Corrections made during validation

- Firecracker file archive publication used `rename` from a state-directory stage
  into runtime storage. Different filesystems produced `EXDEV` and retained an
  unpublished import. Publication now exclusively copies verified members and
  syncs the complete target before readiness. A regression verifies source
  preservation and existing-target refusal; native cross-filesystem import,
  image boot and cleanup pass.
- Shared archive/volume code referenced a directory-sync helper compiled only on
  Linux/FreeBSD, breaking macOS builds. The helper now lives in shared storage.
- Linux physical-inventory tests ran on unsupported platforms. Their successful
  physical-fencing cases are Linux-scoped; a non-Linux regression verifies that
  incomplete process evidence cannot become a complete recovery inventory.
- Stop grace now rejects values outside -1..300 before duration conversion.
  Signed-option checking accepts `--stop-grace -1` without treating another
  option as its value. Existing string/unsigned flag rules remain intact.
- Native stop fixtures replay the same command identity only for the documented
  transient unreadable-process stop result. Unknown outcomes are still retained;
  identity checks were not relaxed. Disconnect injection now establishes durable
  admission before disconnecting. The private systemd fixture supplies the
  `which` alternatives link needed for honest capability probing.
- The private lab PID 1 now reaps adopted engine children independently of the
  RPC worker. Without a namespace reaper, an exited daemonized QEMU remained a
  zombie and the receiver correctly refused to confirm its physical fence. No
  product process-identity guard was weakened. Failed fixture attempts retained
  their metadata; their exact owned VMs were physically fenced and explicitly
  deleted before rerunning the corrected fixture.
- Schema documentation and optional-section references now agree with v9/v10.
  Native oracle tests describe forward migration and old-reader refusal, while
  preserving offline v5-to-v6 converter checks. Those oracle-dependent cases
  remain unverified without the pinned executables.

Local formatting, vet, full tests/build and Linux/FreeBSD cross-builds passed.
Final native Linux (Go 1.26.8) and FreeBSD (Go 1.26.10) full uncached race suites
with tool/disk integration passed; Linux also required strict reflink support.
The socket regressions passed with a deliberately long `TMPDIR`. The shutdown
commit independently passed the full local gates in its staged tree. Remote CI
is not claimed by this report.

## Cleanup

Successful fixtures ended with empty inventories. Failed attempts retained their
private diagnostic metadata while their owned payloads were inspected and stopped;
no unknown resource was silently adopted or treated as absent. A device-link
fixture initially reopened ZFS devices while cleanup ran; change-triggered lookup
removed that interference. Re-entered private mount namespaces explicitly mounted
their own ZFS runtime/cache datasets before retrying.

The Linux teardown checked 25 successful service databases and absence of live
payloads, removed the exact owned ZFS dataset set/file-vdev pool and Btrfs loop
filesystem, and stopped the lab unit. All Linux `memory.events` counters, including OOM,
were zero. Host firewall/interface comparison passed and the existing Docker
process remained PID 1786. Only the original network namespaces remained.
The ZFS kernel module was left loaded because its initial load ownership was not
captured; the host's container-configuration directory was likewise not removed
without a prior ownership record. Neither has a running test service or pool.

FreeBSD teardown verified empty VM databases/run paths/VMMs, removed only the
12-GiB task parent dataset and test VNET, unloaded the task's PF/VMM modules and
confirmed the original gateway jail (JID 4) and `re0`/`lo0` interfaces unchanged.
Management SSH remained reachable on both hosts. Temporary test credentials and
SSH forwards were removed; private logs and diagnostic metadata remain outside git.

The supplementary controller lab likewise ended with empty VM/environment and
physical runtime inventories, zero active volume claims and no test credentials.
Its bounded unit was stopped after checking its memory counters. Host firewall,
interfaces and the original Docker process were compared with the captured
pre-run state. Its diagnostic metadata remains separate from the original runs.
