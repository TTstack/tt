#!/bin/bash
set -euo pipefail
tr '\0' '\n' < /proc/1/cmdline | grep -Fx /tmp/tt-go-port-6426/lab-server.py >/dev/null
base=/tmp/tt-go-port-6426
# Every command here runs in the lab's private namespaces, never the host netns.
[ "$(cat /proc/1/comm)" = python3 ]
[ "$(cat /sys/fs/cgroup/memory.max)" = 8589934592 ]
[ ! -e "$base/reflink.img" ]
truncate -s 12G "$base/reflink.img"
mkfs.btrfs -q "$base/reflink.img"
mkdir -p /home/ttstack/store
mount -o loop "$base/reflink.img" /home/ttstack/store
mkdir -p /home/ttstack/store/run1 "$base/native-agent" "$base/native-ctl" "$base/http-fixture"
printf 'tt-native-egress\n' > "$base/http-fixture/index.html"
ip netns add tt-peer
ip link add tt-peer-link type veth peer name tt-peer-nic
ip link set tt-peer-nic netns tt-peer
ip addr add 198.18.0.1/24 dev tt-peer-link
ip link set tt-peer-link up
ip netns exec tt-peer ip link set lo up
ip netns exec tt-peer ip addr add 198.18.0.2/24 dev tt-peer-nic
ip netns exec tt-peer ip link set tt-peer-nic up
ssh-keygen -q -t ed25519 -N '' -f "$base/guest-key"
printf '{"opaque.txt":"opaque fixture data\\n"}\n' > "$base/guest-config.json"
printf 'Private Btrfs storage and peer network prepared\n'
