# Durable workspace compute checkpoint — 2026-10-28

Historical feature-source evidence. Current upstream acceptance and limits are
recorded in [the backport report](upstream-backports-2026-10-30.md).

This checkpoint publishes the optional, generic command-policy, persistent-volume,
archive, image-catalog and recovery extensions documented normatively in
[compatibility.md](../compatibility.md). Existing parity routes remain available;
managed command-policy clients use durable receipts, generation/fencing admission
and explicit external control epochs. Observations do not repair or start work.

The relevant native behavior and isolation boundaries are recorded in
[command policy](command-policy-2026-10-27.md),
[persistent volumes](persistent-volumes-2026-10-27.md) and
[workspace recovery](workspace-recovery-2026-10-27.md). Their limitations remain
binding: a managed cgroup fence does not prove independent whole-host power
fencing or containment of unrelated legacy workloads.

The checkpoint passed the full local test, vet and build gates, Linux/FreeBSD
amd64 cross-builds and the full uncached race suite on the authorized Linux host.
The final cross-build found and corrected a FreeBSD-only helper reference left
after Linux observation refactoring; the device inspection behavior is unchanged.
FreeBSD behavior is compile-checked here, not claimed as native acceptance.

Publishing this compute checkpoint does not complete the downstream platform or
guest-agent reconstruction. Later work is delivered as separate validated commits.
