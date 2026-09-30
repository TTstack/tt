# Working on tt

`tt` is the Go re-implementation of [TTstack](../TTstack) (Rust). Its purpose is
behavioral and wire parity with the Rust release, in idiomatic Go. Read
[docs/plan.md](docs/plan.md) first, then the document that matches your task.

## Scope and ownership

- `cmd/{tt,tt-ctl,tt-agent,upgrade-agent-v5}`: startup wiring only.
- `internal/model`, `api`, `capability`, `backup`, `guestconfig`, `sshboot`,
  `auth`, `runcmd`: shared, platform-independent behavior.
- `internal/engine`, `netcfg`, `storage`: host-side implementations, selected by
  host OS at runtime.
- `internal/agent`, `internal/ctl`, `internal/cli`: the three components.

The reference implementation lives at `../TTstack`; where the two disagree, the
Rust source defines the expected behavior and this repository has a bug.

## Ground rules

- No new features. Parity only: same API, same JSON, same paths, same printed
  output, same status codes. Anything not in the Rust release does not belong here.
- Compatibility is normative: [docs/compatibility.md](docs/compatibility.md)
  records every wire rule, identifier and deliberate deviation. Changing a field,
  path, default or message requires updating that document in the same commit.
- Dependencies are a closed set (see [docs/development.md](docs/development.md)).
  Adding one needs an explicit, written reason.
- Reservations, repository contents and state formats are never silently
  "improved"; conservative behavior (retaining resources on unknown outcomes) is
  part of the contract.

## Development and verification

- Phased delivery: follow [docs/roadmap.md](docs/roadmap.md); each phase ends with
  `gofmt -l .` clean, `go vet ./...`, `go test ./...`, `go build ./...` and one
  commit. Do not start the next phase on a red build.
- Port the Rust unit tests that encode contracts; keep their intent and names
  recognizable. Add golden fixtures for wire types instead of asserting by hand.
- Check lifecycle changes for partial failure and retries. A timeout does not
  prove that creation failed. Stop retains disks but not memory; delete is
  destructive. Process readiness is not application readiness.
- Never trust a stale PID file; always validate process identity by marker.
- Keep repository documentation, instructions and commit messages in English.
  Respond to the user in their language.

## Live systems

Functional VM testing requires an authorized host and is bounded: no stress
testing, keep well under half of host CPU/memory, never disturb existing guests,
and clean up only what the task created. Administrative keys stay out of logs and
reports. FreeBSD engines are experimental; state their limits when reporting
results.

## Repository layout notes

- `docs/` is the maintained description; keep it small and linked, not duplicated.
- `tools/` holds operator-facing templates such as `deploy.toml.example`.
- Do not commit keys, databases, images or generated binaries.
