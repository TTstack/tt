# Engine and host capabilities

`tt capabilities` (or `--json`) and `GET /api/capabilities` describe the design
matrix. `tt host show HOST_ID` and agent `/api/info` describe what a particular
host advertises. Both levels must permit a fresh request; an advertised feature
still requires suitable guest images, privileges and current runtime state.

| Engine | Storage | Managed SSH / root keys | Guest config | Egress denial / isolation | Explicit disk size / stopped resize | Disk backup |
|---|---|---|---|---|---|---|
| QEMU | file, zvol | Managed SSH | No | Both | Both; guest filesystem growth is separate | Qualified reflink or zvol |
| Firecracker | file, zvol | Managed SSH on a prepared image | Readonly drive | Both | Both; unpartitioned ext4 growth | Qualified reflink or zvol |
| Docker/Podman | Runtime's own store | No | No | Neither | Neither | No |
| bhyve (experimental FreeBSD) | file, zvol | No | No | Egress denial only | Image-sized disk; no resource-update API | Zvol only |
| Jail (experimental FreeBSD) | file only | Root keys only | No | Neither | Neither | No |

All eligible engine/storage combinations support workload lifecycle and TCP
publication. Docker's own image store is independent of the agent's VM storage
setting. Jail root-key injection does not provide custom users, sudo or managed
SSH metadata. Process state does not establish application readiness.

The canonical tags are `vm.run`, `net.tcp.publish`, `guest.ssh.bootstrap`,
`guest.ssh.root_keys`, `guest.config.drive`, `net.egress.deny`, `net.isolation`,
`disk.size.create`, `vm.resources.offline` and `disk.backup.point`. The matrix
includes definitions, explicit limits and `create_fields` rendering hints; the
latter do not replace validation of the whole request.

A version-1 `capability_report` is scoped to an engine and the configured storage.
Each tag has `enabled`, optional denial `reason` and `evidence`; `backup_admission`
is separate from mechanism qualification. For example, disabling new backup
creation does not disable recovery or deletion of an existing recovery point.

Reports are based on startup probes and filesystem qualification, not continuous
proof of tool health, daemon availability, PF/nft setup or image compatibility.
Restart an agent after changing prerequisites. Offline hosts retain their last
report for inspection and cannot receive new placement.

A present report is authoritative: an unknown version, mismatched storage,
missing/duplicate required entry or explicit denial cannot borrow permission from
another engine or from legacy tags. Unknown tags survive serialization but grant
nothing. Hosts without a report use the explicit legacy mappings implemented in
[the registry](../internal/capability/capability.go); canonical tags placed in the
old flat `capabilities` list do not replace legacy protocol identifiers.

Recovery remains separate from fresh admission. Exact creation retries use the
saved request, and a recorded resize can retry its recorded target after a
capability disappears. Stop, delete and retained-disk start use their lifecycle
checks. Backup inspection/replay retains the `disk_backup_v1` protocol gate.
See [SSH](ssh.md), [images/networking](guest-images.md) and
[disk backup](disk-backup.md) for instance prerequisites and boundaries.
