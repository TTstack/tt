#!/bin/bash
set -euo pipefail
tr '\0' '\n' < /proc/1/cmdline | grep -Fx /tmp/tt-go-port-6426/lab-server.py >/dev/null
base=/tmp/tt-go-port-6426
pool=ttgo6426
[ "$(cat /proc/1/comm)" = python3 ]
if zpool list "$pool" >/dev/null 2>&1; then echo 'test pool exists; inspect before reuse' >&2; exit 1; fi
[ ! -e "$base/zpool.img" ]
truncate -s 16G "$base/zpool.img"
zpool create -o cachefile=none -O mountpoint="$base/zfs" -O compression=lz4 -O atime=off "$pool" "$base/zpool.img"
zfs create "$pool/images"
zfs create -o mountpoint=/home/ttstack/zrun "$pool/runtime"
size=$(qemu-img info --output=json "$base/images/alpine-cloud" | python3 -c 'import sys,json; print(json.load(sys.stdin)["virtual-size"])')
zfs create -s -V "$size" -o volmode=dev -o sync=standard "$pool/images/alpine-cloud"
for i in {1..100}; do [ ! -b "/dev/zvol/$pool/images/alpine-cloud" ] || break; sleep .1; done
qemu-img convert -n -f qcow2 -O raw "$base/images/alpine-cloud" "/dev/zvol/$pool/images/alpine-cloud"
zfs create "$pool/images/fc-ssh"
cp "$base/images/fc-ssh/vmlinux" "$base/zfs/images/fc-ssh/vmlinux"
zfs create -s -V 512M -o volmode=dev -o sync=standard "$pool/images/fc-ssh/rootfs"
for i in {1..100}; do [ ! -b "/dev/zvol/$pool/images/fc-ssh/rootfs" ] || break; sleep .1; done
dd if="$base/images/fc-ssh/rootfs.ext4" of="/dev/zvol/$pool/images/fc-ssh/rootfs" bs=4M conv=fsync status=none
# rootfs.ext4 must stay absent in the base dataset; the agent links each clone.
mkdir -p "$base/native-agent" "$base/native-ctl" "$base/http-fixture"
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
zfs list -r "$pool"
printf 'Private file-vdev ZFS pool and peer network prepared\n'
