package dsmui

import (
	"net/http"
	"testing"
)

// The local network is what the NAS itself is on; the links containers,
// virtual machines and VPNs bring along carry private addresses too, and a
// name pointed at one of those leads nowhere a phone can go.
func TestVirtualLinksAreNotTheNetwork(t *testing.T) {
	for name, want := range map[string]bool{
		"ovs_eth0": false, "eth0": false, "bond0": false, "ovs_bond0": false,
		"docker0": true, "br-5f3a": true, "veth12ab": true, "tun0": true,
		"wg0": true, "tailscale0": true,
	} {
		if got := isVirtualLink(name); got != want {
			t.Errorf("%s: virtual %v, want %v", name, got, want)
		}
	}
}

// The screen gets the local addresses along with the rest of the address
// view, so the "home network only" way can say where the name has to lead.
func TestAddressCarriesLocalIPs(t *testing.T) {
	sc := newScreen(t)
	code, got := sc.do(t, http.MethodGet, "/dsm/admin/address", "")
	if code != http.StatusOK {
		t.Fatalf("got %d", code)
	}
	if _, ok := got["local_ips"]; !ok {
		t.Fatalf("no local_ips in %v", got)
	}
}
