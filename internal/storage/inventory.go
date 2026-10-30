package storage

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const MaxNamespaceEntries = 65536

// NamespaceDirectory resolves only a configured namespace or immutable snapshot
// view. It does not require a Firecracker root-device child to exist.
func NamespaceDirectory(ctx context.Context, store ImageStore, path string) (string, error) {
	if store.Name() == "file" {
		return path, nil
	}
	if store.Name() != "zvol" {
		return "", fmt.Errorf("namespace directory unsupported")
	}
	base, snapshot, hasSnapshot := strings.Cut(path, "@")
	directory, err := zfsMountpoint(ctx, base)
	if err != nil {
		return "", err
	}
	if !hasSnapshot {
		return directory, nil
	}
	if snapshot == "" || snapshot == "." || snapshot == ".." || strings.ContainsAny(snapshot, "/@") {
		return "", fmt.Errorf("invalid snapshot selector")
	}
	return filepath.Join(directory, ".zfs", "snapshot", snapshot), nil
}

// NamespaceEntry exposes only a relative resource selector and bounded physical
// evidence. Symlinks are observed without traversal; failed reads are errors.
type NamespaceEntry struct {
	LogicalBytes  uint64
	CapacityKnown bool
	Name          string
	Type          string
	Bytes         uint64
	Evidence      string
}

func InventoryNamespace(ctx context.Context, store ImageStore, root string) ([]NamespaceEntry, error) {
	if store.Name() == "zvol" {
		raw, err := zfsCommand(ctx, "list", "-Hp", "-o", "name,type,referenced,logicalreferenced,volsize", "-r", "-t", "filesystem,volume,snapshot", root)
		if err != nil {
			return nil, err
		}
		result := []NamespaceEntry{}
		types := map[string]string{}
		for _, line := range strings.Split(raw, "\n") {
			fields := strings.Split(line, "\t")
			if len(fields) == 5 {
				types[fields[0]] = fields[1]
			}
		}
		for _, line := range strings.Split(raw, "\n") {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			fields := strings.Split(line, "\t")
			if len(fields) != 5 {
				return nil, fmt.Errorf("malformed dataset inventory")
			}
			if fields[0] == root {
				continue
			}
			if !strings.HasPrefix(fields[0], root+"/") && !strings.HasPrefix(fields[0], root+"@") {
				return nil, fmt.Errorf("dataset inventory escaped its namespace")
			}
			if len(result) >= MaxNamespaceEntries {
				return nil, fmt.Errorf("dataset inventory exceeds its entry budget")
			}
			size, err := strconv.ParseUint(fields[2], 10, 64)
			if err != nil {
				return nil, fmt.Errorf("malformed dataset inventory bytes")
			}
			logical, logicalErr := strconv.ParseUint(fields[3], 10, 64)
			known := logicalErr == nil
			if fields[4] != "-" {
				volume, volumeErr := strconv.ParseUint(fields[4], 10, 64)
				if volumeErr != nil {
					return nil, fmt.Errorf("malformed dataset logical capacity")
				}
				logical = max(logical, volume)
			} else if fields[1] == "snapshot" {
				base, _, _ := strings.Cut(fields[0], "@")
				if types[base] != "filesystem" {
					known = false
				}
			}
			name := strings.TrimPrefix(fields[0], root)
			name = strings.TrimPrefix(name, "/")
			result = append(result, NamespaceEntry{Name: name, Type: fields[1], Bytes: size, LogicalBytes: logical, CapacityKnown: known, Evidence: fmt.Sprintf("%x", sha256.Sum256([]byte(line)))})
		}
		return result, nil
	}
	if store.Name() != "file" {
		return nil, fmt.Errorf("physical namespace inventory unsupported")
	}
	return InventoryDirectory(ctx, root, false)
}
func InventoryDirectory(ctx context.Context, root string, optional bool) ([]NamespaceEntry, error) {
	directory, err := os.Open(root)
	if err != nil {
		if optional && os.IsNotExist(err) {
			return []NamespaceEntry{}, nil
		}
		return nil, err
	}
	defer directory.Close()
	result := []NamespaceEntry{}
	for {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		entries, readErr := directory.ReadDir(256)
		for _, entry := range entries {
			if len(result) >= MaxNamespaceEntries {
				return nil, fmt.Errorf("directory inventory exceeds its entry budget")
			}
			info, err := os.Lstat(filepath.Join(root, entry.Name()))
			if err != nil {
				return nil, err
			}
			kind := "other"
			switch {
			case info.Mode().IsRegular():
				kind = "file"
			case info.IsDir():
				kind = "directory"
			case info.Mode()&os.ModeSymlink != 0:
				kind = "symlink"
			}
			size := uint64(max(info.Size(), 0))
			evidence := fmt.Sprintf("%s/%d/%d/%d", entry.Name(), info.Mode(), size, info.ModTime().UnixNano())
			result = append(result, NamespaceEntry{Name: entry.Name(), Type: kind, Bytes: size, Evidence: fmt.Sprintf("%x", sha256.Sum256([]byte(evidence)))})
		}
		if readErr == io.EOF {
			return result, nil
		}
		if readErr != nil {
			return nil, readErr
		}
	}
}
