#!/bin/sh
# Native Rust build on an authorized host. Destination is an owned build tree.
set -eu
reference=${1:?usage: build-agent-oracle.sh RUST_CHECKOUT NEW_BUILD_DIRECTORY}
destination=${2:?usage: build-agent-oracle.sh RUST_CHECKOUT NEW_BUILD_DIRECTORY}
script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
if [ -e "$destination" ]; then
    echo 'oracle build directory must not already exist' >&2
    exit 1
fi
mkdir -p "$destination"
git -C "$reference" archive 1ad83a9feed5fac820d74b5e7ef752e50a552740 | tar -xf - -C "$destination"
mkdir -p "$destination/crates/agent/examples"
cp "$script_dir/oracle/agent-db.rs" "$destination/crates/agent/examples/agent-db.rs"
cp "$script_dir/oracle/config-primitives.rs" "$destination/crates/agent/examples/config-primitives.rs"
cp "$script_dir/oracle/agent-http.rs" "$destination/crates/agent/examples/agent-http.rs"
cat "$script_dir/oracle/agent-db-export.rs" >> "$destination/crates/agent/src/runtime.rs"
(cd "$destination" && cargo clean -p tt-agent && CARGO_BUILD_JOBS=2 cargo build --locked -p tt-agent --example agent-db --example agent-http --example upgrade-agent-v5 --example config-primitives)
