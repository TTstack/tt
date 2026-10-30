# Persistent disk validation — 2026-10-27

Historical feature-source evidence. Current upstream acceptance and limits are
recorded in [the backport report](upstream-backports-2026-10-30.md).

Scope: independent disk primitives, host/controller journal and binding protocol,
file and zvol snapshots, and the optional consuming provider adapter. This does
not qualify application tenancy, sealed off-host backup or orphan sweeps.

Source: the reconstruction worktree based on `10b35e8`; its final combined commit
contains this implementation and record. The normative contract is
`docs/compatibility.md` §12. Service binaries were built with CGO disabled and
Go 1.27.3. Native validation used the explicitly authorized Linux/KVM host, one
small Firecracker guest at a time, and isolated network/mount namespaces with
private runtime, state, credentials, controller and agent services. Existing
services and guests were not stopped or replaced. Administrative credentials and
private access details remain outside this repository.

## Native acceptance

`tools/native/volumes.py` passed through both the agent and controller paths.
File-backed fixtures used the `fc-alpine` image. ZFS fixtures used the existing
`fc-zv` image, a distinct name in that catalog, and a new task-owned, quota-limited
parent dataset. The file agent was stopped while the ZFS agent used its isolated
network; the two agents were not allowed to create overlapping guest addresses.
The shared source images were not modified. All created guests, volumes and
checkpoints were removed through their own identities and absence was verified.

| Acceptance | File | Zvol |
|---|---|---|
| Provision managed root and data disk without boot | PASS | PASS |
| Explicit start/stop with both disks attached | PASS | PASS |
| Root filesystem marker survives VM deletion | PASS | PASS |
| Existing root/data disks bind to a new stopped VM on the same host | PASS | PASS |
| Data marker survives boot and rebinding | PASS | PASS |
| Immutable root/data snapshots clone into new independent disks | PASS | PASS |
| Root filesystem and raw-data contents match their checkpoints | PASS | PASS |
| Retained disks refuse destruction | PASS | PASS |
| Checkpoint deletion refuses live restored-disk dependencies | PASS | PASS |
| Cleanup removes only run-owned identities | PASS | PASS |
| Offline root/data growth and restoration of the earlier checkpoint | PASS | PASS |

Representative successful run IDs:

- Direct file agent: `4cea88b8-ad5d-404b-bef3-9a8b72d2d661`.
- File controller: `56b2c0c4-3d9c-4976-99b0-2c97e5da9ea9`.
- Zvol controller: `03eec999-0d6b-4033-84f9-e875d313c72c`.
- Zvol controller including growth: `009aa291-a70e-4510-8676-9b28b8332884`.
- File controller including growth: `dbf19ddd-d973-4a9a-b666-ec13dbdc17ee`.

## Regression evidence

- `TestManagedRootSurvivesVMDeletionAndOfflineRebinding`: independent root identity,
  no silent clone on rebinding, one disk reservation.
- `TestFailedManagedCreateResumesWithoutRecloningReadyRoot`: interrupted launch
  resumes the recorded sequence without replacing available user data.
- `TestUnpublishedVolumeRetryUsesJournalAndNeverAdoptsReadyBytes`: cleanup is
  confined to an owned unpublished stage; a missing available disk is not recreated.
- `TestVolumeGrowthIsDurableAndCannotShrink`: unknown growth keeps the larger claim.
- `TestUnattachedVolumeAdmissionAndMalformedCatalogFailClosed`: detached bytes are
  charged; malformed inventory cannot become apparent free space.
- `TestManagedEnvironmentPlanIsAtomicAndReplayUsesOriginalIdentity`: controller
  retry preserves VM/disk IDs, locality and reservation.
- `TestLegacyHostCannotReceiveManagedDiskOptions`: absent/newer protocol fails closed.
- `TestBatchPlacementReservesComposedRootFootprint`: root metadata/kernel overhead
  participates in batch admission; stopped provisioning does not consume seats.
- `TestDiskObservationCannotEraseUnsettledGrowthReservation` and
  `TestDelayedCheckpointReadCannotReviveDeletionTombstone`: stale reads preserve intent.
- `TestRestoredFirecrackerDatasetCannotAliasSourceDevice`: restored ZFS filesystem
  metadata points to the clone's own child device. The native restore-content check
  exercises that corrected layout with a real guest disk.

All local default tests and vet passed after these changes. The full remote race
suite passed for both sibling repositories at this checkpoint and remains a
final reconstruction gate after the remaining integration.
The optional adapter has unit coverage for identity forwarding, generation refusal,
bounded paging and persistent VM projection. Its application deployment gate stays
closed until team-owned disk claims and GC are implemented.
