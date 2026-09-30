#!/bin/sh
set -eu
[ "$(uname -s)" = FreeBSD ]
base=/tmp/tt-go-port-6426/native
name=tt-port-6426
if jls -j "$name" jid >/dev/null 2>&1; then
    echo 'test jail already exists; inspect before reuse' >&2
    exit 1
fi
[ ! -e /home/ttstack ]
mkdir -p "$base"
# Module loading does not enable PF on the host. All PF rules below are VNET-local.
kldload -n pf
kldload -n if_bridge
kldload -n if_tap
kldload -n vmm
jail -c name="$name" path=/ host.hostname=tt-port-6426 persist vnet children.max=8 allow.raw_sockets allow.mount allow.mount.devfs allow.vmm enforce_statfs=0 devfs_ruleset=4
cpuset -j "$name" -l 0-3
jexec "$name" ifconfig lo0 inet 127.0.0.1/8 up
cat > "$base/pf.conf" <<'PF'
rdr-anchor "ttstack/*"
anchor "ttstack/*"
pass all
PF
jexec "$name" pfctl -nf "$base/pf.conf"
jexec "$name" pfctl -f "$base/pf.conf"
jexec "$name" pfctl -e
mkdir -p /home/ttstack/run
jexec "$name" pfctl -s info
jexec "$name" sysctl security.jail.children.max
pfctl -s info
jls -n -j gw jid vnet
