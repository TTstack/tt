---
name: x-review
description: Review tt changes or the full repository for Go correctness, Rust parity, and lifecycle recovery defects. Invoke explicitly as $x-review; accepts a scope and optional --fix.
---

# Review tt

Read [the shared workflow](../../../docs/ai-workflow.md) and
[the review guide](../../../docs/ai-review.md). Resolve repository paths from the
Git root; the links above are relative to this file.

Accept one scope (`N`, `all`, `staged`, `worktree`, commit, or `A..B`) and optional
`--fix`. Empty scope means the latest commit. Apply the shared scope-resolution
rules; reject unknown options.

1. Record the worktree baseline and freeze the review scope. Inspect the complete
   diff, surrounding functions, relevant callers and tests. For `all`, keep a
   tracked-file coverage ledger, including docs, tools, CI and `.agents/`.
2. Apply the subsystem checks and try to refute each candidate. Historical
   scopes report only defects still present in the current implementation.
3. Deduplicate confirmed findings and reconcile in-scope entries in
   `docs/audit.md`. Code stays read-only without `--fix`; the audit registry is
   the only allowed write unless the user requested a completely read-only run.
4. With `--fix`, apply confirmed in-scope fixes sequentially, add meaningful
   regression coverage, run the applicable validation, and re-review. Preserve
   inseparable baseline edits and leave blocked findings Open.
5. Report scope, coverage, findings, audit changes and validation limits. A clean
   review means no confirmed findings within the stated coverage, not full parity.

Never stage, commit, bump versions, tag or push in this workflow.
