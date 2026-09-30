#!/bin/bash
set -euo pipefail
[ "$(readlink /proc/self/ns/mnt)" != "$(readlink /proc/1/ns/mnt)" ]
base=/tmp/tt-go-port-6426
mount --make-rprivate /
export PATH="$base/tools:$base/bin:$PATH"
mkdir -p "$base/images" "$base/tools" "$base/downloads"
cd "$base/downloads"
curl -fsSL --connect-timeout 15 --max-time 180 -o firecracker-v1.12.0-x86_64.tgz https://github.com/firecracker-microvm/firecracker/releases/download/v1.12.0/firecracker-v1.12.0-x86_64.tgz
curl -fsSL --connect-timeout 15 --max-time 60 -o firecracker.sha256 https://github.com/firecracker-microvm/firecracker/releases/download/v1.12.0/firecracker-v1.12.0-x86_64.tgz.sha256.txt
sha256sum -c firecracker.sha256
tar -xzf firecracker-v1.12.0-x86_64.tgz
install -m 755 release-v1.12.0-x86_64/firecracker-v1.12.0-x86_64 "$base/tools/firecracker"
install -m 755 release-v1.12.0-x86_64/jailer-v1.12.0-x86_64 "$base/tools/jailer"
tt image create alpine-cloud --image-dir "$base/images"
tt image create fc-alpine --image-dir "$base/images"
mkdir -p "$base/images/fc-ssh" "$base/fc-mount"
cp "$base/images/fc-alpine/vmlinux" "$base/images/fc-ssh/vmlinux"
cp --reflink=auto "$base/images/fc-alpine/rootfs.ext4" "$base/images/fc-ssh/rootfs.ext4"
truncate -s 512M "$base/images/fc-ssh/rootfs.ext4"
e2fsck -fy "$base/images/fc-ssh/rootfs.ext4" || [ "$?" -eq 1 ]
resize2fs "$base/images/fc-ssh/rootfs.ext4"
mount -o loop "$base/images/fc-ssh/rootfs.ext4" "$base/fc-mount"
trap 'umount "$base/fc-mount"' EXIT
cp /etc/resolv.conf "$base/fc-mount/etc/resolv.conf"
mkdir -p "$base/fc-mount/etc/ssl/certs"
cp /etc/ssl/certs/ca-certificates.crt "$base/fc-mount/etc/ssl/certs/ca-certificates.crt"
ln -sf certs/ca-certificates.crt "$base/fc-mount/etc/ssl/cert.pem"
for spec in 'null 1 3' 'zero 1 5' 'random 1 8' 'urandom 1 9'; do
    read -r name major minor <<< "$spec"
    test -e "$base/fc-mount/dev/$name" || mknod -m 666 "$base/fc-mount/dev/$name" c "$major" "$minor"
done
chroot "$base/fc-mount" /sbin/apk --no-cache add openssh-server sudo
mkdir -p "$base/fc-mount/etc/ttstack"
printf '1\n' > "$base/fc-mount/etc/ttstack/ssh-bootstrap-version"
cat >> "$base/fc-mount/etc/ttstack-boot" <<'BOOT'
if [ -f /run/ttstack-config/ttstack-ssh.sh ]; then
    /bin/sh /run/ttstack-config/ttstack-ssh.sh || exit 1
fi
/usr/sbin/sshd
BOOT
umount "$base/fc-mount"
trap - EXIT
firecracker --version
jailer --version
python3 "$base/ssh-kernel.py"
printf 'Native image prerequisites ready\n'
