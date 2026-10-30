# Command policy validation — 2026-10-27

Historical feature-source evidence. Current upstream acceptance and limits are
recorded in [the backport report](upstream-backports-2026-10-30.md).

Source: uncommitted controller/agent command-contract reconstruction based on
`10b35e8`. The final reconstruction commit contains this record. This is new
native evidence, not a parity claim against the pinned Rust release.

Authorized Linux/KVM host. Separate task-owned network and mount
namespaces, run/state/clone directories and controller/agent services were used.
Existing services and guests were not stopped or replaced. The drill used one
1-CPU, 256-MiB Firecracker guest from the boot-only `fc-alpine` image, which lacks
a Ctrl-Alt-Del shutdown handler. Administrative credentials remained in private
files and did not appear in test output.

`tools/native/stop-policy.py` against the task-owned controller passed:

| Mode | Measured call latency | Observed result |
|---|---|---|
| 1-second grace, `graceful_only=true` | 1.010 s | typed `state/graceful_timeout`; guest remains running |
| `force=true` | 0.057 s | guest stopped, disk retained |
| 1-second grace, normal escalation | 1.059 s | guest stopped after bounded grace |
| Start after force stop | successful | same guest/disk identity running |
| Cleanup | successful | environment absence confirmed |

The environment was
`stop-policy-0ba64b95-3a98-47bb-b0da-93994e72b74d-g1`, confirmed deleted by the
harness. Unit regressions additionally verify isolated per-call engine settings,
invalid policy refusal, graceful-only retention and receiver-side replay/fencing.
Controller and agent unit suites and vet passed locally. The full fork race suite
passed on this host before this per-call policy extension; its final full rerun
remains a release gate. No claims are made for non-Linux engines' optional policy
support; they refuse unsupported overrides explicitly.
