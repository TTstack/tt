#!/bin/sh
set -eu
reference=${1:?usage: build-ctl-oracle.sh RUST_CHECKOUT NEW_BUILD_DIRECTORY}
destination=${2:?usage: build-ctl-oracle.sh RUST_CHECKOUT NEW_BUILD_DIRECTORY}
script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
if [ -e "$destination" ]; then
    echo 'oracle build directory must not already exist' >&2
    exit 1
fi
mkdir -p "$destination"
git -C "$reference" archive 1ad83a9feed5fac820d74b5e7ef752e50a552740 | tar -xf - -C "$destination"
mkdir -p "$destination/crates/ctl/examples"
cp "$script_dir/oracle/ctl-db.rs" "$destination/crates/ctl/examples/ctl-db.rs"
cat "$script_dir/oracle/ctl-db-export.rs" >> "$destination/crates/ctl/src/db.rs"
(cd "$destination" && cargo clean -p tt-ctl && CARGO_BUILD_JOBS=2 cargo build --locked -p tt-ctl --example ctl-db --bin tt-ctl)
