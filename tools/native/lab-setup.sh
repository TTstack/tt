#!/bin/bash
set -euo pipefail
for namespace in mnt net cgroup; do
    [ "$(readlink /proc/self/ns/$namespace)" != "$(readlink /proc/1/ns/$namespace)" ]
done
base=/tmp/tt-go-port-6426
mount --make-rprivate /
# A new cgroup mount exposes only this delegated systemd unit.
umount /sys/fs/cgroup
mount -t cgroup2 -o nsdelegate,memory_recursiveprot none /sys/fs/cgroup
mkdir /sys/fs/cgroup/supervisor
echo $$ > /sys/fs/cgroup/supervisor/cgroup.procs
echo '+cpu +memory +pids' > /sys/fs/cgroup/cgroup.subtree_control
mkdir -p "$base/lab-home/run"
mount --bind "$base/lab-home" /home/ttstack
mount -t tmpfs -o mode=755,nosuid,nodev tmpfs /run
ip link set lo up
exec unshare --pid --fork --mount-proc python3 "$base/lab-server.py"
