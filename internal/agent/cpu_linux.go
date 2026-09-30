package agent

import (
	"math"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"golang.org/x/sys/unix"
)

func availableCPU() uint32 {
	count := runtime.NumCPU()
	var set unix.CPUSet
	if unix.SchedGetaffinity(0, &set) == nil && set.Count() > 0 {
		count = set.Count()
	}
	if count < 1 {
		count = 4
	}
	quota := cgroupQuota(os.ReadFile, func(path string) bool { _, err := os.Stat(path); return err == nil })
	return uint32(min(uint64(count), max(uint64(1), quota)))
}

// Match Rust's available_parallelism: cgroup quotas use whole core equivalents
// rounded down, then a minimum of one and the process affinity limit.
// Reference: https://doc.rust-lang.org/src/std/sys/thread/unix.rs.html
func cgroupQuota(read func(string) ([]byte, error), exists func(string) bool) uint64 {
	data, err := read("/proc/self/cgroup")
	if err != nil {
		return math.MaxUint64
	}
	version := 0
	group := ""
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.SplitN(line, ":", 3)
		if len(fields) != 3 || len(fields[2]) == 0 {
			continue
		}
		if fields[1] == "" {
			if version == 0 {
				version = 2
				group = strings.TrimPrefix(fields[2], "/")
			}
		} else if utf8.ValidString(fields[1]) && slices.Contains(strings.Split(fields[1], ","), "cpu") {
			version = 1
			group = strings.TrimPrefix(fields[2], "/")
		}
	}
	if version == 0 {
		return math.MaxUint64
	}
	type hierarchy struct{ mount, group string }
	roots := []hierarchy{{"/sys/fs/cgroup", group}}
	if version == 1 {
		roots = []hierarchy{{"/sys/fs/cgroup/cpu", group}, {"/sys/fs/cgroup/cpu,cpuacct", group}}
		if !exists(filepath.Join(roots[0].mount, group)) && !exists(filepath.Join(roots[1].mount, group)) {
			mounts, err := read("/proc/self/mountinfo")
			if err == nil && utf8.Valid(mounts) {
				for _, line := range strings.Split(string(mounts), "\n") {
					fields := strings.Split(strings.TrimSpace(line), " ")
					if len(fields) < 10 {
						break
					}
					if fields[len(fields)-3] != "cgroup" || !slices.Contains(strings.Split(fields[len(fields)-1], ","), "cpu") {
						continue
					}
					prefix := strings.TrimPrefix(fields[3], "/")
					if prefix != "" && group != prefix && !strings.HasPrefix(group, prefix+"/") {
						continue
					}
					rest := strings.TrimPrefix(strings.TrimPrefix(group, prefix), "/")
					roots = append(roots, hierarchy{fields[4], rest})
					break
				}
			}
		}
	}
	quota := uint64(math.MaxUint64)
	parse := func(raw string) (uint64, bool) {
		value, err := strconv.ParseUint(strings.TrimPrefix(raw, "+"), 10, 64)
		return value, err == nil
	}
	for _, root := range roots {
		path := filepath.Join(root.mount, root.group)
		if version == 2 {
			if !exists(filepath.Join(path, "cgroup.controllers")) {
				continue
			}
		} else if !exists(path) {
			continue
		}
		for path == root.mount || strings.HasPrefix(path, root.mount+"/") {
			var limit, period uint64
			var okLimit, okPeriod bool
			if version == 2 {
				raw, err := read(filepath.Join(path, "cpu.max"))
				if err == nil && utf8.Valid(raw) {
					line := strings.SplitN(string(raw), "\n", 2)[0]
					fields := strings.Split(line, " ")
					if len(fields) < 2 {
						return quota
					}
					limit, okLimit = parse(fields[0])
					period, okPeriod = parse(fields[1])
				}
			} else {
				raw, err := read(filepath.Join(path, "cpu.cfs_quota_us"))
				if err == nil && utf8.Valid(raw) {
					limit, okLimit = parse(strings.TrimSpace(string(raw)))
				}
				raw, err = read(filepath.Join(path, "cpu.cfs_period_us"))
				if err == nil && utf8.Valid(raw) {
					period, okPeriod = parse(strings.TrimSpace(string(raw)))
				}
			}
			if okLimit && okPeriod && period > 0 {
				quota = min(quota, limit/period)
			}
			path = filepath.Dir(path)
		}
		break
	}
	return quota
}
