#!/bin/sh
set -eu
[ "$(uname -s)" = FreeBSD ]
base=/tmp/tt-go-port-6426/native
source=$base/bhyve-root
image=$base/images/bhyve-freebsd
[ ! -e "$source" ]
[ ! -e "$image" ]
mkdir "$source"
cp -R "$base/images/freebsd-base/." "$source"
cp -R /boot/kernel/. "$source/boot/kernel"
# cp expands /rescue hard links; rescue tools are unnecessary in this fixture.
rm -rf "$source/rescue"
mkdir "$source/rescue"
cat > "$source/boot/loader.conf" <<'CONF'
console="comconsole"
boot_serial="YES"
autoboot_delay="0"
vfs.root.mountfrom="ufs:/dev/vtbd0"
CONF
cat > "$source/etc/rc.conf" <<'CONF'
hostname="tt-bhyve"
ifconfig_vtnet0="inet 10.10.0.2/16"
defaultrouter="10.10.0.1"
sshd_enable="YES"
sendmail_enable="NONE"
syslogd_flags="-ss"
background_fsck="NO"
CONF
printf '/dev/vtbd0 / ufs rw 1 1\n' > "$source/etc/fstab"
printf 'PermitRootLogin prohibit-password\nPasswordAuthentication no\nKbdInteractiveAuthentication no\n' > "$source/etc/ssh/sshd_config"
mkdir -p "$source/root/.ssh"
cp "$base/guest-key.pub" "$source/root/.ssh/authorized_keys"
chmod 700 "$source/root/.ssh"
chmod 600 "$source/root/.ssh/authorized_keys"
makefs -t ffs -B little -s 1536m -o version=2 "$image" "$source"
ls -lh "$image"
