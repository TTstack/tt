# Authorized native acceptance harnesses

These are operator-run test fixtures, not ordinary Go tests. **Do not run them on
the development workstation or against an existing deployment.** They assume an
explicitly authorized disposable test scope. No host addresses or credentials are
stored here. Each host uses the private task directory `/tmp/tt-go-port-6426`;
use a new scope or inspect retained state before repeating a failed run.

The scripts preserve the tested phase-7 scenarios. They expect Go executables in
`bin/` and the pinned Rust executables in `rust-target/debug/` below that directory.
Build those from the revisions recorded in [the validation reports](../../docs/validation/).
For current optional-protocol binaries, set `TT_NATIVE_GO_ONLY=1` in the lab
environment; service restarts then use the Go binaries throughout. Rust takeover
requires the pre-extension baseline and its compatible v5/v6 databases. It must
not be attempted against a migrated v9/v10 database.
The reference is 0.6.0 at `1ad83a9feed5fac820d74b5e7ef752e50a552740`.

## Linux layout and bounds

Prepare the binaries, external tools and image downloads before starting the
network-isolated lab. Build tools need their own bounded unit (the recorded run
used four CPUs and 6 GiB). `prepare-images.sh` must run in a private mount namespace,
under a separate memory/CPU bound. It uses the real image CLI, adds OpenSSH/sudo
to a private Firecracker root, and calls `ssh-kernel.py` to select the upstream
5.10.233 kernel with virtio entropy. It does not start a VM or modify a firewall.

Copy `lab-{setup.sh,server.py,call.py}` into the task directory. Launch as root on
an authorized Linux host:

```sh
systemd-run --unit=tt-go-6426-lab \
  --property=CPUQuota=400% --property=MemoryMax=8G \
  --property=MemorySwapMax=0 --property=TasksMax=2048 --property=Delegate=yes \
  unshare --mount --net --cgroup /bin/bash /tmp/tt-go-port-6426/lab-setup.sh
```

This is below the half-capacity budget of the tested hosts; recompute the budget
including existing work on any different host. The lab has separate mount, network,
PID and cgroup namespaces. `/home/ttstack`, `/run` and the writable cgroup view are
private. The root-only Unix socket runs child commands within the same resource
bound. Namespace PID 1 reaps adopted engine children while a separate RPC worker
waits for its own commands. Mutating Python harnesses verify PID 1's exact
lab-server marker.

Before work, capture `nft -j list ruleset` to `host-nft-before.json` and `ip -j addr`
to `host-addresses-before.json` in the task directory, from the real host. After
cleanup, `host-check.py` compares rules and interfaces, ignoring only counter
values and IPv6 address-lifetime countdowns.

All commands below go through the private lab, for example:

```sh
python3 /tmp/tt-go-port-6426/lab-call.py \
  /bin/bash /tmp/tt-go-port-6426/prepare-file.sh
python3 /tmp/tt-go-port-6426/lab-call.py \
  python3 /tmp/tt-go-port-6426/vm-file.py
```

| Harness | Preparation and observations |
|---|---|
| `prepare-file.sh`, `vm-file.py` | Private 12 GiB file-backed Btrfs mount; QEMU/Firecracker SSH, cold boot, resize, live Go/Rust takeover, stale PID and strict reflink backup/replay |
| `prepare-zvol.sh`, `vm-zvol.py` | Separate host/scope; unique `ttgo6426` pool backed only by the task's 16 GiB regular file, with no cachefile; QEMU/Firecracker zvol lifecycle and backup/replay |
| `backup-fault.py` | Zvol scope after its VM matrix; disconnect the submitting HTTP client, retry through Rust, hold a retired snapshot across refresh/restore, then release and clean it |
| `mixed.py` | File scope; two agents with separate network/run paths, one environment spanning both, outer SSH ingress, live swaps, blocked host access and denied egress |
| `docker.py` | File scope; separate Docker daemon/socket/storage with cgroupfs and vfs, published HTTP, runtime binding refusal, live swap and cold lifecycle |
| `systemd-driver.py` | File scope; a further isolated PID 1, root, network, UTS and IPC namespace, readonly `/usr` and global sysctls, private unit catalog, 1 CPU/2 GiB sub-budget; executes `systemd-workflow.py` |

`TT_NATIVE_ENGINES=qemu` or `firecracker` selects one VM family. The default tests
both. The extension harnesses also import `native_request.py`; copy it beside them.
A transient unreadable process identity is recorded and the *same stop*
is retried after inspection; backup mutations always reuse their explicit UUID
and revision when replayed. A failed run retains the service state/disks needed
for diagnosis instead of silently discarding the evidence.

The Docker fixture is the checksum-verified Alpine minirootfs used by the image
recipe, plus the static `native-http.go` binary at `usr/local/bin/native-http`.
Build that named Go file with CGo disabled and repack the archive as
`downloads/docker-fixture.tar.gz`. The harness imports it with a long-running
HTTP default command. It never uses the host's Docker socket or store.

`control.py --lab-root /tmp/tt-go-port-6426 --attempt fresh1` exercises the modern
external-authority profile using real Go controller/agent services and one small
QEMU guest. Use a fresh lab and attempt name, current `bin/tt-{agent,ctl}` and a
verified file-backed `images/alpine-cloud`. If using another lab root, replace
the default path in the three lab helpers before launching them. This harness
does not require Rust binaries or `native_request.py`.

The checks cover frozen inventory paging, changed-inventory refusal, durable
quarantine capacity, explicit enrollment without workload adoption, epoch refusal,
the distinction between controller quiescence and host physical fencing, and
both receivers' GC gates. A new empty reviewed epoch must release the retained
capacity floor. Its JSON evidence and ownership manifest remain in the private
attempt directory; API keys never enter the report. On failure, services stop
but owned guest state is retained for inspection. Fence and explicitly delete
those exact resources before another attempt; do not infer absence from a failed
request or reuse the state as a fresh scope.

`systemd-driver.py` requires `systemd-root.sh`, `systemd-cgroup.sh` and
`systemd-workflow.py` beside it. It uses a private root login to supply standard
login environment variables, and checks HTTP readiness after service activation.
It does not start the host's systemd units or expose the host's mutable filesystem
through its root. The prepared image exercises the CLI's existing-image check;
the actual download/build is covered by the image preparation stage.

For cleanup, first delete the owned guests through the API and stop their services.
`cleanup-zvol.py` requires empty VM inventories, the exact owned dataset set and
its file vdev before destroying the test pool. In the file scope, `cleanup-file.py` checks empty inventories, unmounts
`/home/ttstack/store` inside the lab and verifies the associated loop device is gone.
Only then terminate the owned lab unit, remove its stale socket/ready marker and
temporary SSH key, and run `host-check.py` from the host. Keep unrelated namespaces,
units, containers, pools and host firewall rules untouched.

## Dedicated FreeBSD

The FreeBSD helpers require the dedicated authorized machine. `freebsd-isolation.sh`
creates only `tt-port-6426`, a separate VNET with PF anchors and child-jail/bhyve
permissions; host PF stays disabled. It refuses an existing jail or run directory.
CPU use is unrestricted under the operator's authorization, while guest memory
stays conservative. Existing gateway jails are never modified.

Prepare `native/images/freebsd-base` with the real image CLI. For future archive
preparation, the operator prefers a Chinese mirror outside the international
network jail, with the official release manifest used for verification. The
production recipe itself retains the pinned reference's upstream URL.

Run `freebsd-jail.py` via `jexec tt-port-6426` first; it generates a temporary test
key and checks real root-key SSH and live implementation swaps. Then prepare the
1536 MiB UFS image using `freebsd-bhyve-image.sh` on the host, and run
`freebsd-bhyve.py` in the test VNET. `freebsd-zvol-setup.sh` creates a unique,
12 GiB-quota dataset tree and delegates it to that VNET; `freebsd-zvol.py` tests
real restore/replay through both implementations.

After empty inventory and no live VMMs are confirmed, detach and destroy only
`zroot/tt-go-port-6426-native`, remove only the test jail, and remove the empty
owned `/home/ttstack/run` parent directories. Verify host PF remains disabled,
the original gateway jail remains active and management SSH still works.
Native FreeBSD validation is FreeBSD 15.x only.
