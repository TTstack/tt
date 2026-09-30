package netcfg

import (
	"net/netip"
	"strings"
	"testing"
)

func TestTapNamesPreservePreUpgradeHostObjects(t *testing.T) {
	for id, want := range map[string]string{"": "tt-6ea523c53def", "vm1": "tt-e9e453b4d5a5", "vm2": "tt-fc7c4d3fbec3", "83b7e95-test-vm": "tt-ab2cb9caf2f1", "long-id-abcdefghijklmnopqrstuvwxyz": "tt-75e5d7552606"} {
		if got := TapName(id); got != want {
			t.Errorf("%q: %s want %s", id, got, want)
		}
	}
	if len(TapName(strings.Repeat("a", 200))) > 15 {
		t.Fatal("TAP name too long")
	}
}
func TestVMIPSequentialAndWrapsToNextOctet(t *testing.T) {
	for i, want := range map[uint32]string{0: "10.10.0.2", 1: "10.10.0.3", 252: "10.10.0.254", 253: "10.10.1.1"} {
		if got := VMIP(i); got != want {
			t.Errorf("%d: %s", i, got)
		}
	}
}
func TestVMIPUniqueAndValid(t *testing.T) {
	seen := map[string]bool{}
	for i := uint32(0); i < 1000; i++ {
		ip := VMIP(i)
		parsed, err := netip.ParseAddr(ip)
		if err != nil || seen[ip] {
			t.Fatal(ip, err)
		}
		seen[ip] = true
		last := parsed.As4()[3]
		if last == 0 || last == 255 {
			t.Fatal(ip)
		}
	}
}
func TestMACAddressesFollowIPv4Octets(t *testing.T) {
	got, err := MAC("10.10.0.2")
	if err != nil || got != "02:54:0a:0a:00:02" {
		t.Fatal(got, err)
	}
	if _, err := MAC("not-an-ip"); err == nil {
		t.Fatal("invalid address accepted")
	}
}
func TestIsolationRejectsUnallocatedAddresses(t *testing.T) {
	for _, ip := range []string{"::1", "1.2.3.4", "10.10.0.1", "10.10.0.2; flush ruleset"} {
		if _, err := IsolationRules("test", ip); err == nil {
			t.Fatal(ip)
		}
	}
	if _, err := IsolationRules("test", "10.10.0.2"); err != nil {
		t.Fatal(err)
	}
}
func TestPFPreflightRejectsQuickWildcardThatSkipsSiblingAnchors(t *testing.T) {
	info := "Status: Enabled for 0 days"
	nat := `rdr-anchor "ttstack/*" all`
	good := "anchor \"ttstack/*\" all\npass all flags S/SA keep state"
	old := `anchor "ttstack/*" quick all`
	if !PFHooksReady(info, nat, good) || PFHooksReady(info, nat, old) || PFHooksReady(info, nat, old+"\n"+good) || PFHooksReady("Status: Disabled", nat, good) || PFHooksReady(info, "", good) {
		t.Fatal("PF preflight")
	}
}
func TestCleanupAcceptsNativeQualifiedNamesWithoutMatchingOtherGuests(t *testing.T) {
	prefix := "forward-10-10-0-2-"
	for _, name := range []string{"  ttstack/forward-10-10-0-2-20000", "forward-10-10-0-2-20001"} {
		if _, ok := ownedForwardAnchor(name, prefix); !ok {
			t.Fatal(name)
		}
	}
	for _, name := range []string{"ttstack/forward-10-10-0-20-20000", "ttstack/deny-10-10-0-2", "elsewhere/forward-10-10-0-2-20000", "forward-10-10-0-2-20000/foreign"} {
		if _, ok := ownedForwardAnchor(name, prefix); ok {
			t.Fatal(name)
		}
	}
}
