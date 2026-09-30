package cli

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

//go:embed templates/firecracker-init.sh
var firecrackerInit string

func (b *imageBuilder) createFirecracker(ctx context.Context, name, directory string) error {
	target := filepath.Join(directory, name)
	if err := os.MkdirAll(target, 0777); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}
	kernel, destination := filepath.Join(target, "vmlinux"), filepath.Join(target, "rootfs.ext4")
	if !fileExists(kernel) {
		b.app.out("[image] downloading Firecracker kernel...")
		pin := fcKernelPin
		if err := b.verified(ctx, fcKernelURL, kernel, &pin); err != nil {
			return err
		}
		b.app.out("[image] kernel: %s", humanSize(kernel))
	} else {
		b.app.out("[image] kernel already exists")
	}
	if fileExists(destination) {
		b.app.out("[image] rootfs already exists")
		return nil
	}
	staging, err := os.CreateTemp(target, ".tt-rootfs-*")
	if err != nil {
		return fmt.Errorf("rootfs staging file: %w", err)
	}
	defer staging.Close()
	retained := false
	defer func() {
		if !retained {
			os.Remove(staging.Name())
		}
	}()
	rootfs := staging.Name()
	b.app.out("[image] creating 128MB rootfs with Alpine userspace...")
	if err := b.run(ctx, "dd", "if=/dev/zero", "of="+rootfs, "bs=1M", "count=128"); err != nil {
		return err
	}
	if err := b.run(ctx, "mkfs.ext4", "-q", rootfs); err != nil {
		return err
	}
	mount, err := os.MkdirTemp("", "tt-image-*")
	if err != nil {
		return fmt.Errorf("private mount directory: %w", err)
	}
	defer func() {
		if !retained {
			os.RemoveAll(mount)
		}
	}()
	if err := b.run(ctx, "mount", "-o", "loop", rootfs, mount); err != nil {
		return err
	}
	populateErr := b.populateFirecracker(ctx, name, mount)
	if err := b.run(ctx, "umount", mount); err != nil {
		retained = true
		return fmt.Errorf("%w; staging rootfs retained at %s; unmount %s before removing it", err, rootfs, mount)
	}
	_ = os.Remove(mount)
	if populateErr != nil {
		return populateErr
	}
	if err := os.Rename(rootfs, destination); err != nil {
		return err
	}
	b.app.out("[image] %s ready: kernel=%s, rootfs=%s", name, humanSize(kernel), humanSize(destination))
	return nil
}
func (b *imageBuilder) populateFirecracker(ctx context.Context, name, mount string) error {
	tarball := filepath.Join(mount, "alpine.tar.gz")
	pin := alpineRootPin
	if err := b.verified(ctx, alpineRootURL, tarball, &pin); err != nil {
		return err
	}
	if err := b.run(ctx, "tar", "xzf", tarball, "-C", mount); err != nil {
		return err
	}
	_ = os.Remove(tarball)
	init := filepath.Join(mount, "etc/ttstack-boot")
	script := strings.ReplaceAll(firecrackerInit, "{name}", name)
	if err := os.WriteFile(init, []byte(script), 0666); err != nil {
		return fmt.Errorf("write init: %w", err)
	}
	if err := b.run(ctx, "chmod", "755", init); err != nil {
		return err
	}
	sbin := filepath.Join(mount, "sbin")
	_ = os.MkdirAll(sbin, 0777)
	path := filepath.Join(sbin, "init")
	_ = os.Remove(path)
	if err := os.Symlink("/bin/busybox", path); err != nil {
		return fmt.Errorf("install guest init: %w", err)
	}
	if err := os.WriteFile(filepath.Join(mount, "etc/inittab"), []byte("::sysinit:/etc/ttstack-boot\n::ctrlaltdel:/sbin/reboot\n::shutdown:/bin/sync\n::shutdown:/bin/umount -a -r\n"), 0666); err != nil {
		return fmt.Errorf("write guest shutdown configuration: %w", err)
	}
	etc := filepath.Join(mount, "etc")
	_ = os.MkdirAll(etc, 0777)
	_ = os.WriteFile(filepath.Join(etc, "resolv.conf"), []byte("nameserver 8.8.8.8\n"), 0666)
	return nil
}
func freeBSDBaseURL(version string) (string, error) {
	release := strings.SplitN(strings.TrimSpace(version), "-p", 2)[0]
	numbers, ok := strings.CutSuffix(release, "-RELEASE")
	if !ok {
		return "", fmt.Errorf("freebsd-base requires a RELEASE host; prepare other roots manually")
	}
	parts := strings.Split(numbers, ".")
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid FreeBSD release")
	}
	for _, part := range parts {
		if part == "" {
			return "", fmt.Errorf("invalid FreeBSD release")
		}
		for _, c := range []byte(part) {
			if c < '0' || c > '9' {
				return "", fmt.Errorf("invalid FreeBSD release")
			}
		}
	}
	return "https://download.freebsd.org/releases/amd64/amd64/" + release + "/base.txz", nil
}
func jailImageComplete(target string) bool {
	for _, name := range []string{"bin/sh", "etc/rc", "etc/master.passwd", ".ttstack-image-complete"} {
		info, err := os.Stat(filepath.Join(target, name))
		if err != nil || !info.Mode().IsRegular() {
			return false
		}
	}
	return true
}
func (b *imageBuilder) installJailArchive(ctx context.Context, archive, target string) error {
	staging, err := os.MkdirTemp(filepath.Dir(target), ".freebsd-base-*")
	if err != nil {
		return fmt.Errorf("create jail image staging directory: %w", err)
	}
	defer os.RemoveAll(staging)
	args := []string{"-xpf", archive, "-C", staging}
	if b.platform == "freebsd" {
		args = append(args, "--no-fflags")
	}
	if err := b.run(ctx, "tar", args...); err != nil {
		return err
	}
	for _, name := range []string{"bin/sh", "etc/rc", "etc/master.passwd"} {
		info, err := os.Stat(filepath.Join(staging, name))
		if err != nil || !info.Mode().IsRegular() {
			return fmt.Errorf("incomplete jail archive: missing %s", name)
		}
	}
	for _, file := range []struct{ name, content, label string }{{"etc/resolv.conf", "nameserver 8.8.8.8\n", "write jail resolver"}, {"etc/rc.conf", "sendmail_enable=\"NONE\"\nsyslogd_flags=\"-ss\"\nsshd_enable=\"YES\"\n", "write jail rc.conf"}, {".ttstack-image-complete", "freebsd-base\n", "mark completed jail image"}} {
		if err := os.WriteFile(filepath.Join(staging, file.name), []byte(file.content), 0666); err != nil {
			return fmt.Errorf("%s: %w", file.label, err)
		}
	}
	if err := os.Rename(staging, target); err != nil {
		return fmt.Errorf("publish jail image: %w", err)
	}
	return nil
}
func (b *imageBuilder) createJail(ctx context.Context, name, directory string) error {
	if b.platform != "freebsd" {
		return fmt.Errorf("freebsd-base must be built on FreeBSD")
	}
	target := filepath.Join(directory, name)
	if fileExists(target) {
		if !jailImageComplete(target) {
			return fmt.Errorf("existing jail image is incomplete or unmanaged; inspect and move it before retrying")
		}
		b.app.out("[image] %s already exists", name)
		return nil
	}
	out, err := b.output(ctx, "freebsd-version", 0, "-u")
	if err != nil {
		return fmt.Errorf("read FreeBSD release: %w", err)
	}
	if !out.Success() {
		return fmt.Errorf("freebsd-version failed")
	}
	url, err := freeBSDBaseURL(strings.ToValidUTF8(string(out.Stdout), "\ufffd"))
	if err != nil {
		return err
	}
	if err := os.MkdirAll(directory, 0777); err != nil {
		return fmt.Errorf("create image directory: %w", err)
	}
	download, err := os.MkdirTemp(directory, ".freebsd-download-*")
	if err != nil {
		return fmt.Errorf("create download directory: %w", err)
	}
	defer os.RemoveAll(download)
	archive := filepath.Join(download, "base.txz")
	b.app.out("[image] fetching %s...", url)
	if err := b.run(ctx, "fetch", "-q", "-T", "120", "-o", archive, url); err != nil {
		return err
	}
	if err := b.installJailArchive(ctx, archive, target); err != nil {
		return err
	}
	b.app.out("[image] %s ready: FreeBSD jail base", name)
	return nil
}
