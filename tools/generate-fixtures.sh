#!/bin/sh
# Run on an authorized build host: heavy Rust compilation is not a local gate.
set -eu
reference=${1:?usage: generate-fixtures.sh RUST_CHECKOUT OUTPUT_JSON}
output=${2:?usage: generate-fixtures.sh RUST_CHECKOUT OUTPUT_JSON [wire|network]}
kind=${3:-wire}
revision=1ad83a9feed5fac820d74b5e7ef752e50a552740
script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
reference=$(CDPATH= cd -- "$reference" && pwd)
output_dir=$(CDPATH= cd -- "$(dirname -- "$output")" && pwd)
output="$output_dir/$(basename -- "$output")"
scratch=$(mktemp -d)
trap 'rm -rf "$scratch"' EXIT HUP INT TERM
git -C "$reference" archive "$revision" | tar -xf - -C "$scratch"
mkdir -p "$scratch/crates/core/examples"
case "$kind" in
    wire) cp "$script_dir/oracle/fixtures.rs" "$scratch/crates/core/examples/go-fixtures.rs" ;;
    network)
        cp "$script_dir/oracle/network.rs" "$scratch/crates/core/examples/go-fixtures.rs"
        cat >> "$scratch/crates/core/src/net/isolation.rs" <<'RUST'

// Exported only in the archived oracle build, never in the reference checkout.
pub fn fixture_rules(vm_id: &str, address: &str) -> ruc::Result<String> {
    rules(vm_id, address)
}
RUST
        ;;
    *) echo 'fixture kind must be wire or network' >&2; exit 2 ;;
esac
(cd "$scratch" && CARGO_BUILD_JOBS=2 cargo run --locked -p ttcore --example go-fixtures) > "$scratch/fixtures.json"
mv "$scratch/fixtures.json" "$output"
