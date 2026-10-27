---
name: x-fix
description: Recheck and fix confirmed tt defects in docs/audit.md, with focused regressions and atomic local commits. Invoke explicitly as $x-fix; takes no arguments.
---

# Fix recorded tt defects

Read [the shared workflow](../../../docs/ai-workflow.md) and
[the review guide](../../../docs/ai-review.md). Resolve repository paths from the
Git root; the links above are relative to this file.

This explicit invocation authorizes local fix commits. Accept no arguments.

1. Record the baseline and starting HEAD. Read `docs/audit.md`; if Open is empty,
   report nothing to fix. Do not infer new tasks from historical reports.
2. Recheck each Open entry against current code, callers, tests and the pinned
   contract, in severity order. Source unavailable is a verification limit, not
   evidence of parity. Remove disproven or already-fixed entries with evidence;
   keep blocked or insufficiently verified entries Open.
3. For each confirmed actionable root cause, define owned paths, implement the
   smallest parity-preserving fix and a meaningful regression, and update the
   relevant docs in that unit. Re-review failure/retry paths; remove the Open
   entry only after the required fix checks pass, keeping pending checks explicit.
4. Validate and make one local commit per independent issue using the shared
   commit protocol. Registry-only corrections are separate documentation units.
   Continue independent safe units if another entry is blocked.
5. Review the invocation's aggregate diff for interactions and run the final
   applicable gates once on the stable code state. Report remaining Open,
   dispositions, validation limits, and commit hashes/subjects.

Do not silently move blocked work to Won't Fix. Preserve validated commits on an
incomplete run; report the remaining work. No automatic version bump, tag, push
or history rewrite.
