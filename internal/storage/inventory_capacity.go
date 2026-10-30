package storage

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"

	"golang.org/x/sys/unix"
)

// CapacityEnvelope reads only bounded size evidence, never image validity or
// ownership. A qcow2 header supplies virtual capacity without following backing
// names or external data paths. Unknown layouts remain explicitly unbounded.
// Header offsets follow https://www.qemu.org/docs/master/interop/qcow2.html.
func CapacityEnvelope(ctx context.Context, path string) (uint64, bool, error) {
	file, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		return 0, false, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return 0, false, err
	}
	if info.Mode().IsRegular() {
		return fileCapacity(file, true)
	}
	if !info.IsDir() {
		return 0, false, nil
	}
	entries, err := file.ReadDir(17)
	if err != nil && err != io.EOF {
		return 0, false, err
	}
	if len(entries) > 16 {
		return 0, false, nil
	}
	var total uint64
	root := false
	for _, entry := range entries {
		if ctx.Err() != nil {
			return 0, false, ctx.Err()
		}
		switch entry.Name() {
		case "rootfs.ext4":
			root = true
		case "vmlinux", "guest-config.ext4":
		default:
			return 0, false, nil
		}
		fd, err := unix.Openat(int(file.Fd()), entry.Name(), unix.O_RDONLY|unix.O_CLOEXEC|unix.O_NONBLOCK|unix.O_NOFOLLOW, 0)
		if err != nil {
			return 0, false, err
		}
		member := os.NewFile(uintptr(fd), filepath.Join(path, entry.Name()))
		size, known, e := fileCapacity(member, false)
		closeErr := member.Close()
		if e != nil {
			return 0, false, e
		}
		if closeErr != nil {
			return 0, false, closeErr
		}
		if !known {
			return 0, false, nil
		}
		if total > ^uint64(0)-size {
			return 0, false, fmt.Errorf("capacity inventory overflow")
		}
		total += size
	}
	after, err := file.Stat()
	if err != nil {
		return 0, false, err
	}
	if !info.ModTime().Equal(after.ModTime()) || info.Size() != after.Size() {
		return 0, false, fmt.Errorf("capacity inventory changed")
	}
	return total, root, nil
}
func fileCapacity(file *os.File, allowQcow bool) (uint64, bool, error) {
	info, err := file.Stat()
	if err != nil {
		return 0, false, err
	}
	if !info.Mode().IsRegular() || info.Size() < 0 {
		return 0, false, nil
	}
	if !allowQcow {
		return uint64(info.Size()), true, nil
	}
	var header [104]byte
	n, err := file.ReadAt(header[:], 0)
	if err != nil && err != io.EOF {
		return 0, false, err
	}
	size := uint64(info.Size())
	if n < 4 || !bytes.Equal(header[:4], []byte{'Q', 'F', 'I', 0xfb}) {
		return size, true, nil
	}
	if n < 72 {
		return 0, false, nil
	}
	version := binary.BigEndian.Uint32(header[4:8])
	clusterBits := binary.BigEndian.Uint32(header[20:24])
	virtual := binary.BigEndian.Uint64(header[24:32])
	if (version != 2 && version != 3) || clusterBits < 9 || clusterBits > 21 || virtual == 0 || virtual > 1<<52 || binary.BigEndian.Uint32(header[32:36]) != 0 {
		return 0, false, nil
	}
	if version == 3 {
		if n < 104 {
			return 0, false, nil
		}
		features := binary.BigEndian.Uint64(header[72:80])
		length := binary.BigEndian.Uint32(header[100:104])
		if features&^uint64(0x18) != 0 || length < 104 || length%8 != 0 || uint64(length) > 1<<clusterBits {
			return 0, false, nil
		}
	}
	// The ordinary root reservation includes 72 MiB of bounded boot metadata
	// plus virtual-size/128 qcow2 metadata; a larger container is charged in full.
	mib := (virtual + (1 << 20) - 1) >> 20
	allowance := (uint64(72) + mib/128) << 20
	virtual = mib << 20
	if virtual > ^uint64(0)-allowance {
		return 0, false, nil
	}
	return max(size, virtual+allowance), true, nil
}

// StagingFootprint bounds resident unpublished bytes. These files are not live
// disks and cannot expand through an automatic replay in the modern profile.
func StagingFootprint(ctx context.Context, path string) (uint64, bool, error) {
	file, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		return 0, false, err
	}
	defer file.Close()
	count := 0
	var walk func(*os.File, int) (uint64, bool, error)
	walk = func(file *os.File, depth int) (uint64, bool, error) {
		info, err := file.Stat()
		if err != nil {
			return 0, false, err
		}
		if info.Mode().IsRegular() {
			return uint64(max(info.Size(), 0)), true, nil
		}
		if !info.IsDir() {
			return 0, false, nil
		}
		var total uint64
		for {
			entries, readErr := file.ReadDir(128)
			if readErr != nil && readErr != io.EOF {
				return 0, false, readErr
			}
			for _, entry := range entries {
				if ctx.Err() != nil {
					return 0, false, ctx.Err()
				}
				count++
				if count > MaxNamespaceEntries {
					return 0, false, fmt.Errorf("stage inventory exceeds its entry budget")
				}
				var stat unix.Stat_t
				if err = unix.Fstatat(int(file.Fd()), entry.Name(), &stat, unix.AT_SYMLINK_NOFOLLOW); err != nil {
					return 0, false, err
				}
				mode := stat.Mode & unix.S_IFMT
				if mode != unix.S_IFREG && mode != unix.S_IFDIR {
					return 0, false, nil
				}
				if mode == unix.S_IFDIR && depth >= 2 {
					return 0, false, nil
				}
				fd, err := unix.Openat(int(file.Fd()), entry.Name(), unix.O_RDONLY|unix.O_CLOEXEC|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0)
				if err != nil {
					return 0, false, err
				}
				child := os.NewFile(uintptr(fd), entry.Name())
				size, known, e := walk(child, depth+1)
				closeErr := child.Close()
				if e != nil {
					return 0, false, e
				}
				if closeErr != nil {
					return 0, false, closeErr
				}
				if !known {
					return 0, false, nil
				}
				if total > ^uint64(0)-size {
					return 0, false, fmt.Errorf("stage inventory overflow")
				}
				total += size
			}
			if readErr == io.EOF {
				break
			}
		}
		after, err := file.Stat()
		if err != nil {
			return 0, false, err
		}
		if !info.ModTime().Equal(after.ModTime()) || info.Size() != after.Size() {
			return 0, false, fmt.Errorf("stage inventory changed")
		}
		return total, true, nil
	}
	return walk(file, 0)
}
