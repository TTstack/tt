package netcfg

import (
	"encoding/json"
	"os"
	"testing"
)

func TestRustNetworkGoldenFixtures(t *testing.T) {
	data, err := os.ReadFile("testdata/network.json")
	if err != nil {
		t.Fatal(err)
	}
	var f struct {
		Revision    string
		Identifiers []struct{ ID, Tap string }
		IPs         []struct {
			Index uint32
			IP    string
		}
		IsolationRules string `json:"isolation_rules"`
	}
	if err := json.Unmarshal(data, &f); err != nil {
		t.Fatal(err)
	}
	if f.Revision != "1ad83a9feed5fac820d74b5e7ef752e50a552740" {
		t.Fatal("unexpected oracle revision")
	}
	if len(f.Identifiers) != 6 || len(f.IPs) != 6 {
		t.Fatal("incomplete identifier fixtures")
	}
	for _, v := range f.Identifiers {
		if got := TapName(v.ID); got != v.Tap {
			t.Errorf("%q: %s want %s", v.ID, got, v.Tap)
		}
	}
	for _, v := range f.IPs {
		if got := VMIP(v.Index); got != v.IP {
			t.Errorf("%d: %s want %s", v.Index, got, v.IP)
		}
	}
	rules, err := IsolationRules("vm1", "10.10.0.2")
	if err != nil {
		t.Fatal(err)
	}
	if rules != f.IsolationRules {
		t.Fatal("isolation rule bytes differ from Rust")
	}
}
