package agent

import (
	"math"
	"os"
	"testing"
)

func TestCPUQuotaUsesAncestorLimitsAndRoundsDownWithoutChangingHost(t *testing.T) {
	files := map[string]string{"/proc/self/cgroup": "0::/group/child\n", "/sys/fs/cgroup/group/child/cgroup.controllers": "cpu memory pids", "/sys/fs/cgroup/group/child/cpu.max": "250000 100000\n", "/sys/fs/cgroup/group/cpu.max": "150000 100000\n", "/sys/fs/cgroup/cpu.max": "max 100000\n"}
	read := func(path string) ([]byte, error) {
		if value, ok := files[path]; ok {
			return []byte(value), nil
		}
		return nil, os.ErrNotExist
	}
	exists := func(path string) bool { _, ok := files[path]; return ok }
	if got := cgroupQuota(read, exists); got != 1 {
		t.Fatal(got)
	}
	files["/sys/fs/cgroup/group/cpu.max"] = "50000 100000\n"
	if got := cgroupQuota(read, exists); got != 0 {
		t.Fatal(got)
	}
	delete(files, "/sys/fs/cgroup/group/child/cgroup.controllers")
	if got := cgroupQuota(read, exists); got != math.MaxUint64 {
		t.Fatal(got)
	}
}
func TestCPUQuotaV1BindingTakesPrecedenceOverV2(t *testing.T) {
	files := map[string]string{"/proc/self/cgroup": "2:cpu,cpuacct:/tenant/guest\n0::/unified\n", "/proc/self/mountinfo": "42 41 0:25 /tenant /cgroup/bound rw - cgroup cgroup rw,cpu,cpuacct\n", "/cgroup/bound/guest": "", "/cgroup/bound/guest/cpu.cfs_quota_us": "300000\n", "/cgroup/bound/guest/cpu.cfs_period_us": "100000\n", "/cgroup/bound/cpu.cfs_quota_us": "200000\n", "/cgroup/bound/cpu.cfs_period_us": "100000\n"}
	read := func(path string) ([]byte, error) {
		if value, ok := files[path]; ok {
			return []byte(value), nil
		}
		return nil, os.ErrNotExist
	}
	exists := func(path string) bool { _, ok := files[path]; return ok }
	if got := cgroupQuota(read, exists); got != 2 {
		t.Fatal(got)
	}
}
