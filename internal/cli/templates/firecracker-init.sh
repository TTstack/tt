#!/bin/sh
mount -t proc proc /proc
mount -t sysfs sysfs /sys
mount -t devtmpfs devtmpfs /dev 2>/dev/null
mkdir -p /dev/pts
mount -t devpts devpts /dev/pts
hostname ttstack
echo "TTstack Firecracker guest [{name}] booted OK"

# Set up networking if virtio-net is available
ip link set eth0 up 2>/dev/null
# The agent supplies the allocated address through the kernel command line.
for arg in $(cat /proc/cmdline); do
    case "$arg" in
        ip=*) addr=${arg#ip=}; addr=${addr%%:*}; ip addr add "$addr/16" dev eth0 2>/dev/null ;;
    esac
done
ip route add default via 10.10.0.1 2>/dev/null

# Configuration is data, never executed by this generic image.
if [ -b /dev/vdb ]; then
    mkdir -p /run/ttstack-config
    chmod 700 /run/ttstack-config
    mount -t ext4 -o ro,nosuid,nodev,noexec /dev/vdb /run/ttstack-config || exit 1
fi
# BusyBox init stays alive and handles Ctrl-Alt-Del and shutdown.
