package engine

import (
	"encoding/json"
	"os"
	"testing"
)

type engineGolden struct {
	Revision              string          `json:"revision"`
	CargoLock             string          `json:"cargo_lock_sha256"`
	Qemu                  []string        `json:"qemu"`
	QemuNetwork           string          `json:"qemu_network"`
	Docker                []string        `json:"docker"`
	Bhyve                 []string        `json:"bhyve"`
	Jail                  string          `json:"jail"`
	Jailer                []string        `json:"jailer"`
	Firecracker           json.RawMessage `json:"firecracker"`
	FirecrackerConfigured json.RawMessage `json:"firecracker_configured"`
	FirecrackerBytes      string          `json:"firecracker_bytes"`
}

func readGolden(t *testing.T) engineGolden {
	t.Helper()
	data, err := os.ReadFile("testdata/engines.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture engineGolden
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	if fixture.Revision != "1ad83a9feed5fac820d74b5e7ef752e50a552740" || fixture.CargoLock != "791f251f410e56db0ba247d5c0a8c82cdd1f97e26f8b5d74742042209192f62c" {
		t.Fatal("unexpected oracle provenance")
	}
	return fixture
}
