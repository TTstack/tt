# VM disk backup

Disk backup is disabled by default. Enable an eligible agent with
`--enable-disk-backup`, or set `enable_disk_backup = true` for distributed deploy.
It retains one published root-disk recovery point for an existing, stopped VM.
It excludes RAM, fleet databases and host boot/network metadata; it is not a
separate disaster-recovery copy.

| Engine/storage | Mechanism |
|---|---|
| QEMU or Firecracker, zvol | Owned ZFS snapshot of the runtime root volume |
| QEMU or Firecracker, file | Strict Linux filesystem reflink of an eligible standalone root disk |
| Experimental bhyve, zvol | Owned ZFS snapshot |
| bhyve file, Jail, Docker/Podman | Unsupported |

There is no full-copy fallback. Source and reflink backups must share an eligible
filesystem, and QEMU disks must be standalone qcow2 without unsupported external
or internal dependencies. Zvols must be writable and not use `sync=disabled`.
Storage headroom and configured logical disk reservations are both checked.

```sh
tt env stop demo
tt env show demo
tt env backup create VM_ID
tt env backup show VM_ID
tt env backup restore VM_ID --generation GENERATION
tt env start demo
```

Use the VM ID, not the environment name. Both recorded state and the actual
process must be stopped; paused guests are ineligible. Restore overwrites the
active disk, leaves the VM stopped and keeps the recovery point. Refresh verifies
a new candidate before replacing the published generation. Retired generations
are cleaned asynchronously; failed cleanup retains its disk reservation. Root
capacity changes require backup removal and completed cleanup.

The CLI prints an exact retry command before mutation, with an operation UUID and
expected revision. After an unknown result, inspect and reuse that command:

```sh
tt env backup restore VM_ID --generation GENERATION \
  --operation-id OPERATION_UUID --expected-revision REVISION
```

Do not substitute a new operation ID. Identical pending work is joined; a retained
completed receipt returns without applying the disk again. Changed parameters
conflict. Stale revisions outside the receipt window are rejected. Accepted work
survives client disconnection, and unknown writer outcomes retain exclusion and
reservations. Reads use durable database snapshots while storage work runs.

`tt env backup delete VM_ID` removes the recovery point and retired artifacts.
VM/environment deletion and expiry also remove backups. Turning admission off
prevents new create/refresh work but preserves inspection, restore and removal of
existing backups. Backup does not extend the environment lifetime.

Restore preserves VM identity, placement, resources, IP/ports and expiry, and
requires matching root capacity and retained boot dependencies. It clears SSH
readiness observations; the initial host identity must be observed again after
boot. Stop may force termination, so application quiescing remains the caller's
responsibility.

HTTP mutation headers, status mapping and lifecycle behavior are described in
[rest-api.md](rest-api.md) and [compatibility.md](compatibility.md). Native evidence
is recorded separately in [validation/](validation/); primitive/unit success does
not establish live recovery on every engine and filesystem.
