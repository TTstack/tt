#!/bin/sh
set -eu
[ "$(uname -s)" = FreeBSD ]
base=/tmp/tt-go-port-6426/native
pool=zroot/tt-go-port-6426-native
if zfs list "$pool" >/dev/null 2>&1; then
    echo 'dataset already exists; inspect before reuse' >&2
    exit 1
fi
zfs create -o quota=12G -o mountpoint="$base/zfs" "$pool"
zfs create "$pool/images"
zfs create "$pool/runtime"
zfs create -V 1536M -o volmode=dev -o sync=standard "$pool/images/bhyve-freebsd"
dd if="$base/images/bhyve-freebsd" of="/dev/zvol/$pool/images/bhyve-freebsd" bs=1m conv=fsync
jail -m name=tt-port-6426 allow.mount.zfs
zfs set jailed=on "$pool"
zfs jail tt-port-6426 "$pool"
jexec tt-port-6426 zfs list -r "$pool"
