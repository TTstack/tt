# Rust wire oracle

`../generate-fixtures.sh RUST_CHECKOUT OUTPUT_JSON` archives the pinned reference
revision into private staging, adds the example harness, and builds it with the
reference Cargo.lock. It never edits the reference checkout. Run it on an
authorized remote build host, not the constrained development workstation.

For phase 1 the output is `internal/api/testdata/wire.json`. The result records the
source revision and lockfile hash. It covers each wire structure and enum with
accepted/rejected presence, null, spelling, duplication and extension cases. The
Go tests compare semantic JSON with exact integers. Random default backup revisions
are checked as UUIDs then normalized in both languages; other values stay exact.
Guest-config digest inputs are compared byte-for-byte, including HTML characters,
Unicode line separators and control escapes. No database or private inventory is
included. Normal Go tests consume this file without requiring Rust.

Pass `network` as the third argument to generate
`internal/netcfg/testdata/network.json`. This compiles the reference isolation
renderer from the archived source with a fixture-only export. A firewall mutation
shim panics if invoked; the harness calls only the pure renderer and identifier
functions, including UTF-8 TAP-name input. The reference checkout stays unchanged.
