# HTTP API

The agent API is implemented in phase 4. The controller API remains planned;
its routes are listed in [compatibility.md](compatibility.md#3-http-surface).
Services use a shared bearer key when configured. There is no per-owner ACL.

Successful responses use `{"ok":true,"data":...}`; actions without a result use
`{"ok":true}`. Business failures use `{"ok":false,"error":"..."}`. JSON
extractor failures are plain text, as in Axum. See the
[compatibility contract](compatibility.md) for wire defaults and error diagnostics.

## Agent routes

| Method | Path | Result |
|---|---|---|
| GET | `/api/info` | Host identity, capacities, scoped capabilities, cached images, warnings and recoverable VM inventory |
| GET | `/api/images` | Cached image names |
| GET / POST | `/api/vms` | Strict VM inventory / create with `CreateVmReq` (201 on success) |
| GET / DELETE | `/api/vms/{id}` | One VM / destructive cleanup after confirmed process stop |
| POST | `/api/vms/{id}/stop` | Release CPU/memory while retaining disk and identity |
| POST | `/api/vms/{id}/start` | Cold boot a stopped VM, or resume an existing paused process |
| POST | `/api/vms/{id}/resources` | Update stopped VM CPU, memory and root disk; interrupted growth retains its recorded target |
| GET / PUT / DELETE | `/api/vms/{id}/backup` | Inspect / create or refresh / delete disk backup |
| POST | `/api/vms/{id}/backup/restore` | Restore the requested generation |

Request/response structures live in [internal/api](../internal/api) and
[internal/model](../internal/model). Unknown routes return 404. Unsupported methods
on known routes return 405 with `Allow`; GET routes also support HEAD. Paths are
not redirected or normalized for extra slashes and dot segments. Captured path
parameters are percent-decoded and must be UTF-8.

JSON requests accept `application/json` and `application/*+json`, with a 2 MiB
body limit. Unsupported content type is 415, excess size is 413, malformed JSON
is 400 and model decoding failures are normally 422; enum parsing follows the
reference's syntax/data distinction. Unknown fields are allowed only where the
corresponding Rust type permits them.

## Lifecycle and durability

Creation persists identity, ports and reservations before external changes. An
identical retry returns the existing record, including after capabilities have
been withdrawn. Different creation parameters conflict with that identity. A
failed/uncertain launch retains its record and resources for inspection/deletion.

Mutations serialize on the agent's runtime lock and survive client disconnects.
Read APIs use separate SQLite connections. Image scans run separately and publish
cached results; a slow scan warns after five seconds and completes before another
scan starts. Reconciliation and catalog refresh run at 15-second intervals.

The agent accepts native schema v6 only. It does not repair unversioned or damaged
state. JSON-corrupt VM records remain on disk, disable new admission and appear
as a warning in `/api/info`; strict inventory queries still fail. Healthy records
remain available for cleanup. Container runtime and network configuration are
persistently bound to retained inventory.

## Backup protocol

Mutations require `Idempotency-Key` and a quoted `If-Match` backup revision.
Restore also requires `{"generation":"<uuid>"}`. GET and successful mutations
return the current revision as `ETag`. Error prefixes map to 400/404/409/412/428;
other failures are 500, as specified in the compatibility contract.

A settled failed operation includes `x-tt-backup-settled` and
`x-tt-backup-settled-sequence` only when the runtime confirms that exact operation
is no longer pending or running. Qualification/storage work holds an inherited
per-VM lock outside the runtime mutex. At most two backup workers are claimed at
once. Failed refresh preserves the published generation; failed restore/delete
keeps its durable intent. Cleanup retains reservations and uses bounded backoff.

Disk backup admission defaults to disabled and needs backend qualification.
FreeBSD engine support remains experimental; successful API/state checks are not
proof of guest boot or application readiness.
