# Native FreeBSD guests — 2026-09-30

Reference: TTstack `1ad83a9feed5fac820d74b5e7ef752e50a552740` (0.6.0).
Go: phase 6 (`cc6028b`); the final zvol run rebuilt the current Go source.
Host: authorized dedicated FreeBSD 15.1-RELEASE-p3/amd64, four logical CPUs,
approximately 12 GiB RAM. Under the operator's updated authorization, FreeBSD CPU
usage was not limited to half; VM memory stayed at 512 MiB, one guest at a time.
The agent advertised two CPUs, 2 GiB memory and 16 GiB logical disk capacity.
These are functional tests, not fleet sizing or performance measurements.

## Isolation and preparation

A separate task-owned VNET jail held the agent, controller, guest network and PF.
The existing gateway jail was preserved. Host PF stayed disabled before and after
testing; rules were loaded only into the new VNET. The test jail allowed child
jails, raw sockets, devfs mounts and bhyve; ZFS mounting was enabled for the zvol
case. Its devfs ruleset was set to 4 to match the child Jail engine's default.
A ruleset mismatch initially caused the kernel to reject a child devfs mount;
the failed VM record and clone were retained and subsequently deleted through the
API. No product behavior was changed to bypass this prerequisite.

This arrangement follows the FreeBSD handbook's
[VNET/Jail and delegated-ZFS guidance](https://docs.freebsd.org/en/books/handbook/jails/)
and [bhyve-in-jail guidance](https://docs.freebsd.org/en/books/handbook/virtualization/).
The matching-parent-ruleset requirement is enforced by the
[FreeBSD 15.1 devfs mount code](https://github.com/freebsd/freebsd-src/blob/releng/15.1/sys/fs/devfs/devfs_vfsops.c).

The actual Go `freebsd-base` image recipe downloaded and published a complete
15.1 RELEASE base. A first slow download hit the contractual 300-second command
limit without publishing an incomplete root; a later download directly from the
host completed. The operator subsequently authorized domestic mirrors outside
the international gateway for future prerequisite downloads. No production URL
or pinned digest was changed.

The bhyve image was a manually prepared 1536 MiB UFS disk containing that base,
the matching host kernel, static guest networking and a temporary test public
key. Guest SSH host keys were generated in each clone. Host administrative keys
were never copied into images. A preparation error initially placed the kernel
in a nested directory; the loader failure retained the VM and its disk, and the
test image was corrected before the successful matrix.

## Real guest results

For both Jail and bhyve file storage:

- Start with Go controller/Go agent, Rust controller/Go agent and Go
  controller/Rust agent, exercising both CLIs.
- Create a real guest, log in over SSH and write a marker to its disk/root.
- Replace both control-plane implementations while the guest is running;
  verify the retained guest and marker remain accessible.
- Stop, inspect retained disk/root, cold start and read the same marker.
- Delete through the controller and verify clone and engine metadata removal.

All six lifecycle runs passed. Application readiness was established by a real
SSH command, separately from process state. Jail root-key injection was exercised;
bhyve used its prepared guest key because the product does not inject bhyve keys.

For bhyve zvol backup, a task-owned dataset tree had a 12 GiB quota and was
delegated only to the test jail. The runtime root was a native zvol. The run:

1. Wrote `first`, stopped and created a recovery point.
2. Wrote `second`, stopped and refreshed, producing a new generation.
3. Wrote `third`, stopped and restored the second generation.
4. Replaced Go agent/Rust controller with Rust agent/Go controller and verified
   the restored `second` marker through real SSH.
5. Wrote `after-replay`, stopped and replayed the exact prior restore UUID and
   revision using the Rust CLI. A subsequent boot retained `after-replay`.
6. Deleted the backup and environment and confirmed the runtime dataset tree
   contained no clone or snapshot artifacts.

This passed end to end with both implementations writing the same persistent
state and operating on the same actual disk.

## Cleanup and limits

All test services, guests, TAPs, runtime metadata, the delegated ZFS dataset tree
and test VNET jail were removed. `/home/ttstack/run` and its task-created parent
were empty and removed. The original gateway jail remained active; host PF
remained disabled and SSH remained reachable. Build tools, image caches and
private task logs remain under the task directory for reproduction. No OOM or
host reboot occurred.

This record covers FreeBSD 15.1. The project is validated on FreeBSD 15.x only. This validates the tested Jail/bhyve lifecycle and
bhyve zvol recovery combinations; it does not imply Linux engine coverage,
FreeBSD file reflink support, Jail disk backup, bhyve SSH injection or guest
resource enforcement beyond the documented engine contract.
