# Deployment

Build the Go binaries with `make release`. The CLI retains the Rust default
release directory (`./target/release`) for compatibility; pass `--release-dir
./bin` for a Go build. Deployment installs binaries and services. It does not
install hypervisors, prepare images, provision ZFS or register agents.

## Prerequisites

Build with Go 1.26 or newer. `make release` produces CGo-disabled executables;
there is no C compiler or libc requirement for the Go control-plane binaries.
Native engine tools retain their own operating-system and library requirements.
Linux agents need root; the controller normally runs as `ttstack`.

| Workflow | Required host preparation |
|---|---|
| QEMU/KVM | Usable `/dev/kvm`, `qemu-system-x86_64`, `qemu-img`, `genisoimage` or `mkisofs` |
| Linux VM networking | Full `iproute2`, `nftables` and TUN/TAP support |
| Firecracker | Compatible `firecracker`/`jailer`, KVM, writable cgroup v2 CPU/memory/PID controllers, prepared kernel/rootfs |
| Config drives / ext4 growth | `mkfs.ext4`, `debugfs`, `e2fsck`, `resize2fs` and associated e2fsprogs tools |
| Managed SSH | Host `ssh-keygen`/`ssh-keyscan` and the [guest prerequisites](ssh.md) |
| Containers | Working Docker or Podman runtime and a long-running image command |
| Zvol / file backup | Prepared ZFS datasets, or a qualified Linux reflink filesystem; see [storage](guest-images.md#storage-and-recovery) |
| Distributed deploy | Local `ssh`/`scp`, target `sudo`, and prebuilt binaries for the target OS/architecture |

Go handles recipe HTTP downloads directly; `curl` is not a Go recipe or
Firecracker API prerequisite. Firecracker root preparation additionally needs
`dd`, `tar` and mount/unmount privileges. FreeBSD uses native `fetch`/`tar` and
manual bhyve/Jail/PF preparation.

Reserve jailer UID/GIDs 100000–165535 from unrelated host accounts. Keep runtime
storage private and paths short enough for Unix sockets. Each Firecracker VMM
has a vCPU-count CPU quota, guest RAM + 128 MiB memory ceiling, no swap and bounded
threads. The agent service's cgroup alone does not necessarily contain these
separate jailer cgroups; bound/delegate their parent as well on shared hosts.
Engine detection is not a complete health check, and a bound container runtime
never silently switches stores.

## Local Linux with systemd

```sh
sudo ./bin/tt deploy all --release-dir ./bin
export TT_API_KEY="$(sudo cat /opt/ttstack/etc/api-key)"
/opt/ttstack/bin/tt config 127.0.0.1:9200
unset TT_API_KEY
/opt/ttstack/bin/tt host add 127.0.0.1:9100
```

When automating, wait for `tt status` to succeed after deployment and for the
prepared image to appear in `tt image list` before creating an environment.
Service activation and the periodic image catalog are separate readiness checks.

Use `deploy agent` or `deploy ctl` to install one role. The agent runs as root;
the controller runs as `ttstack`. Binaries go to `/opt/ttstack/bin`, state and
images to `/home/ttstack`, units to `/etc/systemd/system`. Existing role keys are
reused and persisted in `/opt/ttstack/etc/api-key` (0600). Conflicting role keys
cause an error; they are never silently replaced. Service environment files are
0600. Keep administrative keys out of commands, logs and version control.

The agent unit uses `KillMode=process` so restarting the service does not kill
its guests, and `TimeoutStopSec=360`. Deployment preserves existing
`RequiresMountsFor` and mountpoint `ExecStartPre` guards in
`tt-agent.service.d/90-ttstack-mounts.conf`. Put operator overrides in drop-ins.
A restart reconciles retained inventory; it does not automatically boot stopped
guests. `stop` retains disks but loses guest memory, and `delete` destroys disks.

## Distributed Linux deployment

Copy [deploy.toml.example](../tools/deploy.toml.example) into an untracked
`deploy.toml`, set addresses and resource budgets, then run:

```sh
./bin/tt deploy dist deploy.toml
```

`general.release_dir` and each agent's optional `release_dir` point to local
binaries built for that target. SSH must work with noninteractive `sudo`, including
root logins. The CLI checks paths, users, storage and budgets before remote work,
copies each binary to a private staging directory, and verifies SHA-256 before
installation. Setup scripts go through SSH stdin. Systemd, OpenRC and the Rust
manual fallback are retained. A timeout leaves the outcome unknown: inspect the
services before retrying.

The shared key is reused from `deploy.toml.api-key`, imported from an existing
service, or generated. An explicit `general.api_key` replaces that sidecar; keep
it private. `tt config ADDRESS` saves address-scoped credentials in `~/.ttconfig`
using an atomic 0600 replacement. `--server` only reuses the saved key when the
address exactly matches. `--api-key` or `TT_API_KEY` supplies an explicit key.

The API is plaintext HTTP with one shared bearer key. Expose it only through the
intended trusted network or an SSH tunnel. Agent CPU/memory defaults expose all
host capacity to scheduling; set explicit budgets on shared hosts. Fixed engine
paths and networking require full namespace/root isolation for parallel native
test fleets, as described in [roadmap.md](roadmap.md).

## Experimental FreeBSD

The deployment commands intentionally reject FreeBSD. Cross-build with
`CGO_ENABLED=0 GOOS=freebsd GOARCH=amd64 go build -o tt-agent ./cmd/tt-agent`, copy
binaries and start services manually with private data directories and the same
shared `TT_API_KEY`. Prepare `/home/ttstack/run`, authorized PF/networking and
eligible bhyve/Jail prerequisites first. These engines remain experimental;
consult the [validation records](validation/) for actual native coverage.

## Upgrade and offline v5 conversion

Controller state is schema v5; older compatible controller schemas migrate on
open. Agent state must be native v6: even an empty v5 database is rejected, and
changing only `_meta.schema_version` is unsafe. The offline converter assigns
stable backup revisions while preserving old fields, host identity and bindings.

Stop the agent service before conversion and back up its metadata directory and
retained runtime/SSH metadata. Keep images, runtime paths, namespaces, host ID,
container-runtime choice and ingress/port settings unchanged. The generated
`KillMode=process` unit keeps existing guests alive when the agent stops; this
metadata conversion does not rewrite their disks or require stopping them.

For a native v5 database, run the following from the repository after building.
Use new backup/output names and keep the service stopped throughout replacement:

```sh
sudo sh <<'SH'
set -eu
tt_agent_data=/home/ttstack/data
tt_saved=/home/ttstack/data.pre-v6
test "$(systemctl show tt-agent --property=KillMode --value)" = process
systemctl stop tt-agent
test ! -e "$tt_saved"
test ! -e "$tt_agent_data/agent-v5.db"
cp -a "$tt_agent_data" "$tt_saved"
chmod 700 "$tt_saved"
./bin/upgrade-agent-v5 --source "$tt_agent_data/agent.db" \
  --output "$tt_agent_data/agent-v6.db"
mv "$tt_agent_data/agent.db" "$tt_agent_data/agent-v5.db"
for suffix in -wal -shm; do
  if test -e "$tt_agent_data/agent.db$suffix"; then
    mv "$tt_agent_data/agent.db$suffix" "$tt_agent_data/agent-v5.db$suffix"
  fi
done
mv "$tt_agent_data/agent-v6.db" "$tt_agent_data/agent.db"
SH
```

The converter holds the source service lock, reads a consistent snapshot and
publishes a new 0600 database without overwriting an existing output. Keep old WAL
and shared-memory sidecars with the old database; never leave them next to its
replacement. A failure leaves the source intact. Review the new database and
install the compatible controller/agent binaries before restarting the service;
use the same configuration and verify `tt status`, host identity and inventory.
Do not run the converter against v6, do not delete retained runtime files to
recover from a schema error, and do not downgrade an active v6 database to an
older agent. Preserve backups and inspect incomplete operations before rollback.
