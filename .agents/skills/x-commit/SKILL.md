---
name: x-commit
description: Review intended tt worktree changes, fix confirmed regressions, validate, and create atomic local commits. Invoke explicitly as $x-commit, optionally with Git pathspecs.
---

# Review and commit tt changes

Read [the shared workflow](../../../docs/ai-workflow.md) and
[the review guide](../../../docs/ai-review.md). Resolve repository paths from the
Git root; the links above are relative to this file.

This explicit invocation authorizes local commits of intended changes and their
required fixes. Accept optional Git pathspecs, separated from Git options with
`--` when executing commands. Reject option-like input rather than executing it.

1. Record staged, unstaged and untracked changes. With pathspecs, only matching
   changes are candidates; without them, inspect all intended changes. Ask only
   if ownership or an intended untracked file cannot be established. Nothing
   intended means nothing to commit.
2. Freeze owned paths/hunks and split independent root causes into units. Preserve
   pre-staged boundaries and unrelated baseline work. Never absorb new concurrent
   changes merely because their paths match.
3. Review each unit with the mapped subsystem checks. Fix confirmed regressions
   in that unit with appropriate regression coverage and contract docs. Record
   unfixed confirmed findings in the audit; do not commit a known defective unit.
4. Run the shared per-unit checks, inspect exactly what will be committed, and
   commit each validated unit. If units depend on one another, use an order in
   which every commit builds and tests; combine inseparable changes to one cause.
5. Re-review the aggregate change and run the final applicable gates once. Report
   hashes/subjects, validation results, outstanding findings and untouched baseline.

Do not automatically bump a version, tag, push, amend or rewrite history.
