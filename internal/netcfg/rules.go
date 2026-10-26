package netcfg

import (
	_ "embed"
	"fmt"
	"net/netip"
	"strconv"
	"strings"
)

//go:embed isolation.nft
var isolationTemplate string

//go:embed nat.nft
var natTemplate string

// FleetIPv4 requires an address inside the managed bridge subnet and rejects
// the bridge's own address. Engines and network setup share it so every
// consumer of Vm.IP enforces the same boundary.
func FleetIPv4(address string) (netip.Addr, error) {
	ip, err := netip.ParseAddr(address)
	if err != nil || !ip.Is4() {
		return netip.Addr{}, fmt.Errorf("invalid guest IPv4 address")
	}
	b := ip.As4()
	if b[0] != 10 || b[1] != 10 || ip.String() == BridgeAddr {
		return netip.Addr{}, fmt.Errorf("guest IPv4 address must be inside " + BridgeCIDR + ", excluding " + BridgeAddr)
	}
	return ip, nil
}
func IsolationRules(id, address string) (string, error) {
	ip, err := FleetIPv4(address)
	if err != nil {
		return "", err
	}
	mac, _ := MAC(address)
	return strings.NewReplacer("{t}", isolationTable(id), "{tap}", TapName(id), "{mac}", mac, "{ip}", ip.String(), "{BRIDGE_ADDR}", BridgeAddr, "{BRIDGE_NAME}", BridgeName).Replace(isolationTemplate), nil
}
func NATRules() string { return strings.ReplaceAll(natTemplate, "{NFT_TABLE}", NFTTable) }
func PFHooksReady(info, nat, filter string) bool {
	if !strings.Contains(info, "Status: Enabled") {
		return false
	}
	rdr := false
	for _, line := range strings.Split(nat, "\n") {
		if strings.HasPrefix(line, `rdr-anchor "ttstack/*"`) {
			rdr = true
		}
	}
	hooks := false
	for _, line := range strings.Split(filter, "\n") {
		if strings.HasPrefix(line, `anchor "ttstack/*"`) {
			hooks = true
			for _, word := range strings.Fields(line) {
				if word == "quick" {
					return false
				}
			}
		}
	}
	return rdr && hooks
}
func addressKey(address string) (string, error) {
	ip, err := netip.ParseAddr(address)
	if err != nil || !ip.Is4() {
		return "", fmt.Errorf("invalid guest IPv4 address")
	}
	b := ip.As4()
	if b[0] != 10 || b[1] != 10 {
		return "", fmt.Errorf("guest address is outside the TTstack subnet")
	}
	return strings.ReplaceAll(ip.String(), ".", "-"), nil
}
func ownedForwardAnchor(name, prefix string) (string, bool) {
	name = strings.TrimPrefix(strings.TrimSpace(name), "ttstack/")
	rest, ok := strings.CutPrefix(name, prefix)
	if !ok {
		return "", false
	}
	// Rust u16::from_str accepts a leading plus, unlike JSON numeric map keys.
	if _, err := strconv.ParseUint(strings.TrimPrefix(rest, "+"), 10, 16); err != nil {
		return "", false
	}
	return name, true
}
func PFForwardRule(hostPort uint16, ip string, guestPort uint16) (string, string, error) {
	key, err := addressKey(ip)
	if err != nil {
		return "", "", err
	}
	return fmt.Sprintf("ttstack/forward-%s-%d", key, hostPort), fmt.Sprintf("rdr inet proto tcp from any to self port %d -> %s port %d\npass in quick inet proto tcp from any to %s port %d flags S/SA keep state (if-bound)\npass out quick inet proto tcp from any to %s port %d flags S/SA keep state (if-bound)\n", hostPort, ip, guestPort, ip, guestPort, ip, guestPort), nil
}
