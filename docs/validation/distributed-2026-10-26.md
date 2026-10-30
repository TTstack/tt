# Distributed Go-only live test — 2026-10-26

Go implementation: `4b13e67` (post phase 7). No Rust binaries were involved;
the pinned reference remains 0.6.0 `1ad83a9feed5fac820d74b5e7ef752e50a552740`.
The day ran the same matrix in two topologies over the same three
operator-authorized disposable test machines (no pre-existing TTstack state or
guests; addresses and credentials omitted):

1. macOS CLI, a Gentoo/OpenRC controller, an Ubuntu 26.04 (systemd)
   QEMU/Docker agent, and a FreeBSD 15.1-RELEASE bhyve/Jail agent, all via
   `tt deploy dist` (Linux agent budget 8/24 CPU, 8 GiB/30 GiB RAM, 100 GiB).
2. After the operator dropped the Gentoo controller from the design, the
   controller moved onto the Ubuntu host beside its agent via the documented
   single-host path `tt deploy all`, which is also what the README quick start
   exercises. Guests stayed small (≤640 MiB, two at a time) and usage stayed
   far below half of the host. The FreeBSD agent advertised 2/4 CPU,
   2 GiB/12 GiB RAM and 16 GiB disk, one 512 MiB guest at a time.

## Setup path

Run 1 used `tt deploy dist`: controller on Gentoo via its OpenRC path, agent
on Ubuntu via systemd. Run 2 used `tt deploy all` on Ubuntu: controller and
agent co-located as systemd services. In both runs the FreeBSD agent was
cross-built (`CGO_ENABLED=0 GOOS=freebsd`), installed manually and run inside
a dedicated VNET jail (`tools/native/freebsd-isolation.sh` plus an epair
uplink) so that PF lived only inside that VNET. The host PF stayed disabled
throughout, the existing gateway jail was untouched, and the controller
reached the agent through a plaintext test-only L4 relay on the host.
Controller/agent/CLI registration, the shared bearer key, dashboard (200 with
key, 401 without), `capabilities`, host reports and the 15 s fleet refresh all
behaved as expected; host counters returned to zero after deletions once a
refresh tick elapsed. Run 2 additionally verified that restarting `tt-ctl`
left running guests and state intact (reconciled inventory), and that
restarting `tt-agent` under its `KillMode=process` unit kept the QEMU guest at
the same PID with uninterrupted SSH after reconciliation.

## Passed matrix

- **QEMU/Ubuntu agent, file storage on Btrfs** — `alpine-cloud` recipe with
  SHA-512 verification; cloud-init account/key provisioning; published-port SSH
  from a foreign client; guest marker; cold stop/start with retained disk;
  offline resize 1→2 CPU, 512→640 MiB RAM, 2048→4096 MiB disk verified inside
  the guest; `--deny-outgoing` allowed inbound SSH and bridge reachability
  while blocking public egress; a `--lifetime 240` environment and its host
  artifacts were reclaimed by the expiry sweeper; deletion removed clone, TAP,
  NAT rules and process.
- **Disk backup (file + strict reflink)** — create, refresh to a new
  generation, asynchronous retirement cleanup, restore rollback (files written
  after the generation disappeared, and files deleted after it reappeared),
  retained-receipt replay with the same operation UUID/revision returning
  without re-applying, and conflict rejection when that UUID was reused with
  different parameters. Backup admission is off by default: the `deploy all`
  run enabled it on the idle agent through a documented systemd drop-in with
  `--enable-disk-backup`, matching `enable_disk_backup` in distributed deploy.
- **Docker/Ubuntu agent** — published nginx HTTP, stop/start retaining the
  writable layer, capability rejection of offline resize, clean deletion.
- **FreeBSD Jail** — `freebsd-base` recipe with native fetch; root public-key
  injection; SSH into the guest; stop/start retention; deletion removed the
  child jail, PF anchors and clone.
- **FreeBSD bhyve** — manually prepared 1536 MiB UFS image (host kernel, base
  userland, static `10.10.0.x` config, key-only root SSH); bhyveload boot with
  serial-log capture and per-clone host keys; SSH; stop destroyed the VMM and
  device while start cold-created from the retained disk (marker preserved);
  deletion removed device, TAP and anchors. bhyve ran inside the VNET jail
  under `allow.vmm`.

## Findings and their handling

1. **OpenRC deployment script fails live on Gentoo** (reproduces identically
   with the reference; the Go script text is byte-equal and intentionally
   unchanged). `command_user="user:group"` is split into an argument list this
   supervise-daemon rejects, and `/var/log` output paths cannot be created
   after the privilege drop; `rc-service` reports started with nothing
   listening. Workarounds applied on the host and now documented in
   [deployment.md](../deployment.md#openrc-hosts-gentoo-and-others): user-only
   `command_user`, pre-created service-owned log files, and socket-level
   readiness checks.
2. **Docker's default `FORWARD` policy drop blocks published VM ports** on a
   shared host. The managed nftables DNAT was correct; the packets died in
   Docker's `ip filter` chain. Operator opt-in rules in `DOCKER-USER` for `tt0`
   restored access without touching product tables; documented in
   [deployment.md](../deployment.md#docker-co-hosted-with-an-agent).
3. **International downloads from these hosts were throttled or blocked** at
   the environment level (slow direct TCP, fast via the operator's gateway
   namespace/jail; Docker Hub unreachable directly). Image recipes themselves
   verified and published correctly; transfers were run through the gateway
   and the pinned nginx container tag was imported locally with an equivalent
   alias instead of the daemon-side pull. No product URL, checksum or recipe
   step was altered.
4. **Alpine NoCloud ignores seeded `dns_nameservers`** — guest egress works at
   IP level but `/etc/resolv.conf` is absent. The seed is byte-identical to the
   reference; noted in [guest-images.md](../guest-images.md). Alpine's missing
   sudo matched the previously recorded account-test limitation.
5. **FreeBSD guests answer SSH ~30 s late without operator outbound NAT** —
   resolver retry delay from a base image carrying a public resolver, exactly
   the documented operator-separate NAT precondition; noted in
   [guest-images.md](../guest-images.md).

## Cleanup evidence

After each run all test environments, backups, images, generated services
(OpenRC registration, systemd units and drop-ins) and their users were
removed. The Ubuntu host's `DOCKER-USER` additions, `tt0` bridge and managed
nftables table were deleted; Docker itself was never restarted and ran empty
before and after. The FreeBSD VNET jail, epair plumbing, relay, agent files
and test-loaded kernel modules were removed, host PF remained disabled (its
module unloaded again) and the pre-existing gateway jail kept working;
`if_tap` retained one module reference and was left loaded (inert, no
devices). Credentials and keys were not committed anywhere. The repository
working tree was left clean.

Limits: functional tests, not benchmarks. Native FreeBSD coverage in this run is 15.1 only.
The bhyve image, mirror downloads and L4 relay were operator plumbing for this
lab, not product behavior. Podman, Firecracker and zvol paths were not in
scope; their live evidence remains with the 2026-09-30 native records.
