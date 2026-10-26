package netcfg

import "testing"

func TestFleetIPv4BoundsEnginesToTheManagedSubnet(t *testing.T) {
	for _, tc := range []struct {
		address string
		valid   bool
	}{
		{"10.10.0.2", true},
		{"10.10.255.254", true},
		{"10.10.0.1", false},   // the bridge address itself
		{"10.11.0.2", false},   // outside the managed subnet
		{"192.168.1.2", false}, // arbitrary IPv4
		{"1.2.3.4", false},
		{"::1", false},
		{"not-an-ip", false},
		{"", false},
	} {
		ip, err := FleetIPv4(tc.address)
		if (err == nil) != tc.valid {
			t.Errorf("%s: valid=%v err=%v", tc.address, err == nil, err)
		}
		if tc.valid && ip.String() != tc.address {
			t.Errorf("%s: normalized to %s", tc.address, ip)
		}
	}
}
