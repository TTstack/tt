---
name: x-overhaul
description: Review a tt repository scope, fix confirmed parity and lifecycle defects, and create atomic local commits. Invoke explicitly as $x-overhaul; accepts the same scopes as x-review without --fix.
---

# Review, fix and commit tt

Read [the shared workflow](../../../docs/ai-workflow.md) and
[the review guide](../../../docs/ai-review.md). Resolve repository paths from the
Git root; the links above are relative to this file.

Accept one scope (`N`, `all`, `staged`, `worktree`, commit, or `A..B`); empty means
the latest commit. Reject `--fix` and unknown options. This explicit invocation
authorizes local fix commits. `staged`/`worktree` also authorizes committing the
intended scoped changes after reviewing them.

1. Record one baseline, starting HEAD, scope and ownership ledger for the whole
   invocation. Apply shared scope resolution. For `all`, inventory tracked files
   and record reviewed paths/invariants and coverage gaps.
2. Review with the evidence standard and subsystem checks. Reconcile confirmed
   findings with the audit. Historical scopes fix only still-present defects
   rooted in that diff; callers provide evidence, not unrelated fix scope.
3. For `staged`/`worktree`, first review, fix and commit the owned changes under
   the [x-commit protocol](../x-commit/SKILL.md), using the frozen outer scope
   rather than its standalone default. For other scopes, leave unrelated baseline
   untouched.
4. Recheck and resolve new findings and still-applicable in-scope Open entries
   under the [x-fix protocol](../x-fix/SKILL.md). Do not clear unrelated Open
   entries. Include each finding's audit removal with its fix; if a separate
   inventory commit is useful, keep it documentation-only.
5. Re-review the aggregate diff, including interactions and recovery paths. Run
   the final applicable gates once. Reuse unchanged-state results from the inner
   protocols; do not recursively launch standalone workflows or duplicate their
   finalization.
6. Report scope, coverage/gaps, dispositions, validation and local commits.
   Blocked findings stay Open; preserve validated commits and continue other
   independent safe work. No changes means no empty commit.

This composes the review/fix/commit protocols under this invocation's authority;
`x-review`'s no-commit rule applies only to standalone review. No automatic
version bump, tag, push or history rewrite.
