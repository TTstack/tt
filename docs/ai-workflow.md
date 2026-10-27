# AI maintenance workflow

Use [AGENTS.md](../AGENTS.md) for repository constraints and
[Development](development.md) for build conventions. These workflows adapt the
review/fix/commit separation used in `vsdb` to this Go port. They do not introduce
product features, a release process, or new host permissions.

## Entry points

Codex skills live in [`.agents/skills/`](../.agents/skills/). Invoke them by name
in a prompt; arguments are ordinary prompt text. Each skill's
`agents/openai.yaml` sets `allow_implicit_invocation: false`, preserving explicit
selection for these workflows, following the
[official Codex skills documentation](https://learn.chatgpt.com/docs/build-skills).

| Invocation | Default scope | Writes | Local commits |
|---|---|---|---|
| [`$x-review`](../.agents/skills/x-review/SKILL.md) | Latest commit | Audit registry only | No |
| `$x-review <scope> --fix` | Selected scope | Confirmed fixes, tests, docs and audit | No |
| [`$x-fix`](../.agents/skills/x-fix/SKILL.md) | Current audit Open entries | Verified fixes and dispositions | One per independent issue |
| [`$x-commit [pathspecs]`](../.agents/skills/x-commit/SKILL.md) | Intended worktree changes | Required fixes and audit | Intended units |
| [`$x-overhaul [scope]`](../.agents/skills/x-overhaul/SKILL.md) | Latest commit | In-scope fixes and audit | One per independent issue |

For example, `$x-review worktree` reviews pending changes; `$x-review 3 --fix`
fixes still-present defects introduced in the last three commits without
committing; `$x-commit internal/ctl docs/compatibility.md` reviews and commits
intended changes matching those paths. `$x-overhaul all` is a whole-repository
review and repair, not permission for unrelated refactoring or new features.

Ordinary requests such as "fix this test" use repository conventions without
automatically invoking a commit workflow. Explicit instructions in the current
task take precedence: a completely read-only review also leaves the audit alone.

## Scope resolution

Resolve and freeze revisions at invocation start. Verify each as a commit using
`git rev-parse --verify --quiet '<revision>^{commit}'`. Treat user arguments as
data: quote them, reject options where revisions are expected, and put pathspecs
after `--`. Do not splice unchecked arguments into shell commands.

| Input | Reviewed content |
|---|---|
| Empty | `HEAD` versus its first parent; a root commit versus the empty tree |
| Positive integer `N` | Last N commits and net diff from `HEAD~N` to `HEAD` |
| `staged` | Index diff, preserving existing staged/unstaged boundaries |
| `worktree` | `git diff HEAD` plus intended untracked files listed by `git ls-files --others --exclude-standard` |
| `all` | Tracked-file inventory from `git ls-files`; report any relevant untracked work separately |
| Commit | That commit versus its first parent; a root commit versus the empty tree |
| `A..B` | Commits in `git log A..B` and endpoint diff `git diff A B` |

`A..B` uses those endpoints consistently; do not silently substitute the
merge-base diff `A...B`. A merge commit is reviewed against its first parent,
with other parents inspected when needed to explain resolution behavior. If a
count exceeds available parents or a revision is ambiguous, explain the invalid
scope and request a valid one. An all-digit token is a count unless it also
resolves to a commit, in which case clarify. Reject multiple scopes and unknown
flags; do not silently broaden scope. Only `x-review` accepts `--fix`.

For historical scopes, verify findings against the current code before recording
or fixing them. Do not check out an old revision over a dirty worktree. Related
callers/tests are evidence scope, not permission to fix unrelated defects. An
`all` review records coverage by subsystem, including tools, CI, docs and agent
instructions; passing tests or searches alone do not establish a full review.

## Worktree ownership

Before edits or commits, record the branch, starting HEAD, `git status --short`,
staged and unstaged diffs, and untracked paths. A dirty tree is allowed. Separate
intended work from baseline belonging to other work; freeze owned paths/hunks
and planned commit units before changing them. Register additional fix paths
only after inspecting their current diffs. Recheck for concurrent changes before
staging, even inside owned files.

Do not stash, clean, restore, reset or overwrite baseline to obtain a clean tree.
Do not stage unrelated changes or use `git add -A`. If an intended fix overlaps
unowned edits inseparably, leave that unit alone and report the conflict;
continue independent work. Merge audit edits rather than replacing the file.
Edits and commits in one worktree are sequential.

Read-only reviews can inspect detached HEAD or an in-progress merge, stating the
scope limits. Before mutating/committing in a merge, rebase or cherry-pick, or
committing on detached HEAD, resolve the intended branch/operation with the user.
Do not change branches or rewrite history to bypass it.

## Go validation

Run the smallest meaningful regression first, then the gates appropriate to the
changed behavior. Reuse passing results while the relevant code/configuration
state is unchanged; do not repeat a full suite merely because workflows compose.

| Change | Required validation |
|---|---|
| Docs or instruction-only skill | `git diff --check`, local links/anchors and skill metadata/reference checks; no Go suite unless commands/build behavior changed |
| Go implementation or tests | Format owned Go files; clean `gofmt -l .`, vet, targeted regression, then full local test/build gates |
| Build, modules, platform files or command wiring | Go gates plus `make cross`; verify the Go 1.26 minimum when toolchain/dependency compatibility changes |
| Lifecycle, concurrency, storage, networking or platform behavior | Local Go gates, relevant cross-builds, and suitable race/integration/native checks on authorized hosts or CI; report unavailable checks explicitly |
| Oracle or committed fixtures | Check provenance and focused fixture consumers; regenerate only from the pinned reference on an authorized build host |

On this shared workstation, run checks **sequentially**, with `GOMAXPROCS=2`
and `-p 1`. The normal local code gates are:

```sh
gofmt -l .
GOMAXPROCS=2 go vet -p 1 ./...
GOMAXPROCS=2 go test -p 1 ./...
GOMAXPROCS=2 go build -p 1 ./...
```

`make lint`, `make test`, `make build` and `make cross` already default to these
resource bounds. `make build` writes binaries into ignored `bin/`; `go build
./...` only verifies compilation. Format owned files explicitly rather than
running `make fmt` across unrelated changes.

Inspect opt-in test settings before local test runs. Normal local runs leave
`TT_TEST_INTEGRATION`, `TT_TEST_AGENT_ORACLE`, `TT_TEST_CTL_ORACLE`,
`TT_TEST_ZVOL_PARENT` and `TT_TEST_REQUIRE_REFLINK` unset. Their uses are distinct:

- `TT_TEST_INTEGRATION=1` enables real tool/disk/service tests; CI installs their
  prerequisites. Missing tools fail an opted-in suite.
- `TT_TEST_AGENT_ORACLE` / `TT_TEST_CTL_ORACLE` select verified pinned Rust
  executables for database interchange on an authorized test host.
- `TT_TEST_ZVOL_PARENT` selects a dedicated, quota-limited ZFS parent; it enables
  destructive writes to task-created child volumes and is never a local default.
- `TT_TEST_REQUIRE_REFLINK=1` requires strict reflink qualification on a suitable
  filesystem instead of treating an unsupported backend as coverage.

Race suites, real integration and heavy Rust builds run on authorized hosts or
CI, not this workstation. Native checks follow
[the roadmap's order and isolation rules](roadmap.md#live-validation-order-and-isolation):
FreeBSD first, then Linux. Historical host access or a harness directory does
not establish current authorization. Read [the native harness guide](../tools/native/README.md)
before selecting a harness; its recorded paths need explicit inspection before
reuse. A timeout is not permission to rerun destructive cleanup.

## Atomic local commits

This section applies only when committing is authorized by the user or by an
explicit `x-fix`, `x-commit` or `x-overhaul` invocation.

1. One independent issue/root cause per commit, including its regression, contract
   docs and audit update. Bundle multiple symptoms only when they share that
   cause. The original roadmap's one-commit-per-phase rule applies to phase
   delivery; routine maintenance uses these smaller units.
2. Run per-unit format/diff checks and relevant vet/tests before committing.
   Before the final code unit, complete the applicable full local gates. A
   required failed check prevents claiming that unit is validated. Report
   independently demonstrated baseline failures and unavailable remote gates.
3. Combined dirty-tree tests do not prove that each intermediate commit works.
   If another unit or baseline could mask a failure, validate the candidate in
   an isolated temporary worktree, copying only its intended diff and untracked
   files. Do not run multiple heavy suites or discard the original baseline.
4. Stage exact owned paths/hunks and inspect `git diff --cached` before committing.
   If unrelated staged changes exist, preserve the index; use an isolated index
   or carefully scoped `git commit --only` only when the entire selected paths
   belong to the unit. Otherwise separate ownership before proceeding.
5. Use an English imperative subject and describe the behavior and validation.
   Follow actual tooling attribution requirements. Verify the new commit and
   compare status with the baseline.

After the last change, review the aggregate invocation diff and complete any
remaining applicable gates. A new failure needs a focused correction and affected
checks, not repeated unproductive retries. Keep validated local commits when a
later unit is blocked and state what remains. Findings awaiting a required check
stay Open; if a final gate exposes a regression after an entry was removed,
restore that unresolved finding. Do not make empty commits.

There is no automatic patch bump, changelog generation, tag or push. Change a
product version or the pinned oracle revision only within an explicit release or
compatibility-review task. Never amend, rebase or force-push as part of these
skills. Release, deployment, fixture rebaselining and host mutation require their
own task scope.

## Evidence and handoff

Use [the review guide](ai-review.md) for evidence and
[the audit registry](audit.md) for confirmed outstanding work. Summarize scope,
coverage gaps, fixes/dispositions, commands and results, compatibility impact,
local commits when any, and remaining verification. Distinguish current tests
from [historical validation](README.md#validation-evidence). Cross-compilation is
not native execution; a skipped case is not a pass; an absent Rust checkout
prevents fresh source comparisons, not all useful Go maintenance.
