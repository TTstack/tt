# Workspace recovery validation — 2026-10-27

Historical feature-source evidence. Current upstream acceptance and limits are
recorded in [the backport report](upstream-backports-2026-10-30.md).

Scope: complete root/data manifests, persistent writer freeze and clone-only
restoration through the controller. Source is the reconstruction worktree based
on `10b35e8`; its final combined commit contains this implementation and record.
This record does not qualify off-host sealing, application backup authorization
or the guest research runtime.

`tools/native/recovery.py` passed on the explicitly authorized Linux/KVM host in
separate task-owned controller/agent state, mount and network namespaces. The
file backend used the prepared `fc-alpine` image and one small guest at a time.
Existing services/guests were not replaced or stopped. Access credentials and
private deployment details remain outside this repository.

Successful run: `1916a7a0-8a4f-420a-83a2-d039a0eddfaf`.

| Acceptance | Result |
|---|---|
| Complete root+data manifest reaches ready only after freeze/snapshot/thaw | PASS |
| Source starts after checkpoint completion, proving writer fence release | PASS |
| Restore creates new disk identities and a new stopped VM | PASS |
| Root filesystem and raw-data markers restored from checkpoint | PASS |
| Source data remains readable after clone-only restoration | PASS |
| Restored VM boots and stops | PASS |
| Deletion refuses checkpoint ancestry still used by restored disks | PASS |
| Run-owned guests/disks/checkpoints cleaned and absence confirmed | PASS |
| Freeze blocks start, growth, delete and independent snapshots | unit PASS |
| An unrelated checkpoint cannot thaw the owner | unit PASS |

The initial native attempt exposed a real step-identity defect: freeze and thaw
reused one command key, and receiver-side fingerprint checking correctly rejected
the different path. Each substep now has a distinct deterministic key; the native
regression runs through both and confirms fence release. The failed run's own
partial point was removed through the newly implemented fenced abort path, then
its guest and disks were destroyed explicitly. No source data was inferred from
artifact existence or silently recreated (identity and recovery contract §13).

Zvol complete-workspace recovery also passed native run
`226c3c32-0882-4f74-a3e3-a2162e4a4b5a` with the same content, stopped-target, boot
and dependency checks, and cleanup. Both backends'
individual volume snapshot/clone/growth acceptance is recorded separately in
`persistent-volumes-2026-10-27.md`. Default local tests and vet pass; the full remote
race suite is rerun at each integrated checkpoint and before final release.
