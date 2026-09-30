// Package netcfg implements TTstack's host networking contracts.
package netcfg

import (
	"encoding/binary"
	"fmt"
	"math/bits"
	"net/netip"
	"strings"
)

const (
	BridgeName = "tt0"
	BridgeAddr = "10.10.0.1"
	BridgeCIDR = "10.10.0.1/16"
	NFTTable   = "tt-nat"
)

func VMIP(index uint32) string {
	n := index + 1
	return fmt.Sprintf("10.10.%d.%d", (n/254)&255, n%254+1)
}
func MAC(address string) (string, error) {
	ip, err := netip.ParseAddr(address)
	if err != nil || !ip.Is4() {
		return "", fmt.Errorf("invalid guest IPv4 address")
	}
	b := ip.As4()
	return fmt.Sprintf("02:54:%02x:%02x:%02x:%02x", b[0], b[1], b[2], b[3]), nil
}

// TapName freezes SipHash-1-3 with zero keys, Rust str's 0xff sentinel,
// and the low 48 bits. It never uses Go's randomized map hash.
func TapName(id string) string {
	data := append([]byte(id), 0xff)
	v := [4]uint64{0x736f6d6570736575, 0x646f72616e646f6d, 0x6c7967656e657261, 0x7465646279746573}
	round := func() {
		v[0] += v[1]
		v[1] = bits.RotateLeft64(v[1], 13)
		v[1] ^= v[0]
		v[0] = bits.RotateLeft64(v[0], 32)
		v[2] += v[3]
		v[3] = bits.RotateLeft64(v[3], 16)
		v[3] ^= v[2]
		v[0] += v[3]
		v[3] = bits.RotateLeft64(v[3], 21)
		v[3] ^= v[0]
		v[2] += v[1]
		v[1] = bits.RotateLeft64(v[1], 17)
		v[1] ^= v[2]
		v[2] = bits.RotateLeft64(v[2], 32)
	}
	n := len(data) / 8 * 8
	for i := 0; i < n; i += 8 {
		m := binary.LittleEndian.Uint64(data[i : i+8])
		v[3] ^= m
		round()
		v[0] ^= m
	}
	last := uint64(len(data)) << 56
	for i, b := range data[n:] {
		last |= uint64(b) << (8 * i)
	}
	v[3] ^= last
	round()
	v[0] ^= last
	v[2] ^= 0xff
	round()
	round()
	round()
	return fmt.Sprintf("tt-%012x", (v[0]^v[1]^v[2]^v[3])&0xffffffffffff)
}
func isolationTable(id string) string { return strings.ReplaceAll(TapName(id), "-", "_") }
