#!/bin/bash
set -euo pipefail
tr '\0' '\n' < /proc/1/cmdline | grep -Fx /tmp/tt-go-port-6426/lab-server.py >/dev/null
base=/tmp/tt-go-port-6426
root=$base/systemd-root
[ "$(cat /proc/1/comm)" = python3 ]
[ ! -e "$root" ]
mkdir -p "$root"/{usr,etc/systemd/system,root,home,run,var/lib,var/log,var/tmp,tmp,opt,proc,sys,dev,build,empty}
mount --bind "$root" "$root"
chmod 1777 "$root/tmp" "$root/var/tmp"
for item in bin sbin lib lib64; do ln -s "usr/$item" "$root/$item"; done
mount --bind /usr "$root/usr"
mount -o remount,bind,ro "$root/usr"
# Hide the host unit catalog and generators, so no host sysctl/device/service
# setup can be pulled into the minimal validation manager.
mount -t tmpfs -o mode=755 tmpfs "$root/usr/lib/systemd/system"
for file in systemd-journald.service systemd-journald.socket systemd-journald-dev-log.socket; do
    cp "/usr/lib/systemd/system/$file" "$root/usr/lib/systemd/system/$file"
done
for name in system-generators system-environment-generators; do
    test ! -d "$root/usr/lib/systemd/$name" || mount --bind "$root/empty" "$root/usr/lib/systemd/$name"
done
for name in tt-validation basic sysinit sockets multi-user shutdown network; do
    printf '[Unit]\nDescription=Private validation %s\nDefaultDependencies=no\n' "$name" > "$root/etc/systemd/system/$name.target"
done
printf 'root:x:0:0:root:/root:/bin/bash\nnobody:x:65534:65534:nobody:/nonexistent:/usr/sbin/nologin\n' > "$root/etc/passwd"
printf 'root:*:20000:0:99999:7:::\n' > "$root/etc/shadow"
chmod 600 "$root/etc/shadow"
printf 'root:x:0:\nadm:x:4:\nusers:x:100:\nsystemd-journal:x:190:\nnogroup:x:65534:\n' > "$root/etc/group"
printf 'root:*::\n' > "$root/etc/gshadow"
chmod 600 "$root/etc/gshadow"
printf 'passwd: files\ngroup: files\nshadow: files\nhosts: files dns\n' > "$root/etc/nsswitch.conf"
cp /etc/login.defs "$root/etc/login.defs"
printf 'nameserver 1.1.1.1\n' > "$root/etc/resolv.conf"
cp /etc/ld.so.cache "$root/etc/ld.so.cache"
if [ -L /usr/bin/which ]; then
    mkdir -p "$root/etc/alternatives"
    ln -s "$(readlink -f /usr/bin/which)" "$root/etc/alternatives/which"
fi
ln -s /usr/lib/os-release "$root/etc/os-release"
: > "$root/etc/fstab"
cat /proc/sys/kernel/random/uuid | tr -d '-' > "$root/etc/machine-id"
ln -s /run "$root/var/run"
mount -t tmpfs -o mode=755 tmpfs "$root/dev"
for spec in 'null 1 3' 'zero 1 5' 'random 1 8' 'urandom 1 9' 'tty 5 0' 'kvm 10 232'; do
    read -r name major minor <<< "$spec"
    mknod -m 666 "$root/dev/$name" c "$major" "$minor"
done
touch "$root/dev/console"
mount --bind "$TT_TEST_CONSOLE" "$root/dev/console"
mkdir -p "$root/dev/net" "$root/dev/pts" "$root/dev/shm"
mknod -m 666 "$root/dev/net/tun" c 10 200
ln -s /proc/self/fd "$root/dev/fd"
mount -t sysfs -o ro sysfs "$root/sys"
mkdir -p "$root/build/bin"
mount --bind "$base/bin" "$root/build/bin"
mount -o remount,bind,ro "$root/build/bin"
cp "$base/guest-key.pub" "$root/fixture.pub"
# The manager has a smaller cgroup below the lab's existing limits.
mkdir /sys/fs/cgroup/systemd-test
echo 2147483648 > /sys/fs/cgroup/systemd-test/memory.max
echo '100000 100000' > /sys/fs/cgroup/systemd-test/cpu.max
echo $$ > /sys/fs/cgroup/systemd-test/cgroup.procs
exec unshare --cgroup /bin/bash "$base/systemd-cgroup.sh"
