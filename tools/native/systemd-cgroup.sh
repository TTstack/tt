#!/bin/bash
set -euo pipefail
base=/tmp/tt-go-port-6426
root=$base/systemd-root
mount -t cgroup2 -o nsdelegate,memory_recursiveprot none "$root/sys/fs/cgroup"
mkdir "$root/sys/fs/cgroup/launcher"
echo $$ > "$root/sys/fs/cgroup/launcher/cgroup.procs"
echo '+cpu +memory +pids' > "$root/sys/fs/cgroup/cgroup.subtree_control"
ip link set lo up
export container=systemd-nspawn
exec unshare --pid --fork --mount-proc="$root/proc" /bin/bash -c '
printf "inner PID=%s\n" "$$" > /tmp/tt-go-port-6426/systemd-startup.log
while read -r name rest; do
    if [ "$name" = NSpid: ]; then printf "NSpid=%s\n" "$rest" >> /tmp/tt-go-port-6426/systemd-startup.log; set -- $rest; printf "%s\n" "$1" > /tmp/tt-go-port-6426/systemd-lab.pid; break; fi
done < /proc/self/status
for name in fs kernel vm; do
    path=/tmp/tt-go-port-6426/systemd-root/proc/sys/$name
    mount --bind "$path" "$path"
    mount -o remount,bind,ro "$path"
done
exec chroot /tmp/tt-go-port-6426/systemd-root /usr/lib/systemd/systemd --system --unit=tt-validation.target --log-level=warning --log-target=console --show-status=false
'
