# HTTP API

Both agent and controller APIs are implemented through phase 5. Their complete
route contracts are listed in [compatibility.md](compatibility.md#3-http-surface).
Services use a shared bearer key when configured. There is no per-owner ACL.

Successful responses use `{"ok":true,"data":...}`. Agent empty actions and
controller host removal use `{"ok":true}`; controller environment actions use
`{"ok":true,"data":null}`. Business failures use `{"ok":false,"error":"..."}`. JSON
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

## Controller coordination

The controller stores native v5 state and migrates older versions by materializing
backup revisions without dropping unknown VM fields. It exposes fleet status,
host registration/removal/detach, environments, VM resource updates and backup
forwarding. The exact `/` dashboard route is public; API routes use configured
bearer authentication. Unknown URLs never fall through to the dashboard.

Environment creation returns 202 after persisting the complete plan. A background
worker contacts agents while holding only that environment's operation guard.
Placement prefers eligible ZFS hosts for VM engines, then the least free memory
that still fits; Docker placement is independent of VM storage. Pending plans and
restart intents remain reserved against concurrent placement.

Health refresh fetches at most eight hosts concurrently. A revision guard rejects
snapshots overtaken by newer mutations; per-environment guards protect active
operations. Missing VMs remain tracked and reserved. An unavailable agent retains
its last observed inventory. Deletion and expiry forget a VM only after confirmed
agent cleanup; failures leave an environment available for retry.

Backup forwarding persists operation identity and staging reservations before the
HTTP call. Lost responses retain intent; exact replay and sufficiently new settled
observations resolve it. Resource-update retries follow the recorded target even
after capability withdrawal. Network requests do not hold the fleet database mutex.
