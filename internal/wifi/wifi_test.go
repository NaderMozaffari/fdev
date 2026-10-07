package wifi

import (
	"errors"
	"net"
	"os"
	"strings"
	"syscall"
	"testing"
)

func TestParseServices(t *testing.T) {
	out := "List of discovered mdns services\n" +
		"adb-5d9c2e71-k8wqzt\t_adb-tls-connect._tcp\t192.168.1.95:39537\n" +
		"adb-5d9c2e71-k8wqzt (2)\t_adb-tls-pairing._tcp.\t192.168.1.95:41485\r\n" +
		"fdev-abc\t_adb-tls-pairing._tcp\t192.168.1.95:37000\n" +
		"other\t_printer._tcp\t192.168.1.2:631\n"
	got := parseServices(out)
	if len(got) != 3 {
		t.Fatalf("got %d services: %+v", len(got), got)
	}
	if !got[0].connect() || got[0].pairing() || got[0].Addr != "192.168.1.95:39537" || got[0].host() != "192.168.1.95" {
		t.Errorf("connect service: %+v", got[0])
	}
	if !got[1].pairing() || got[1].Name != "adb-5d9c2e71-k8wqzt (2)" || got[1].Addr != "192.168.1.95:41485" {
		t.Errorf("pairing service: %+v", got[1])
	}
}

func TestPairResult(t *testing.T) {
	cases := map[string]pairOutcome{
		"Successfully paired to 192.168.1.95:37099 [guid=adb-5d9c2e71-k8wqzt]":     paired,
		"Failed: Wrong password or connection was dropped.":                        wrongCode,
		"error: protocol fault (couldn't read status message): Undefined error: 0": unreachable,
		"Failed: Unable to start pairing client.":                                  unreachable,
		"error: unknown host service":                                              pairFailed,
	}
	for out, want := range cases {
		if got := pairResult(out, nil); got != want {
			t.Errorf("%q: got %v, want %v", out, got, want)
		}
	}
}

func TestConnectedAndWireless(t *testing.T) {
	for out, want := range map[string]bool{
		"connected to 192.168.1.95:39537":                                true,
		"already connected to 192.168.1.95:39537":                        true,
		"failed to connect to '192.168.1.95:39537': Operation timed out": false,
		"cannot connect to 192.168.1.95:39537: Connection refused":       false,
	} {
		if connected(out) != want {
			t.Errorf("connected(%q) != %v", out, want)
		}
	}
	for serial, want := range map[string]bool{
		"192.168.1.95:39537":                        true,
		"adb-5d9c2e71-k8wqzt._adb-tls-connect._tcp": true,
		"emulator-5554":                             false,
		"R58M123ABC":                                false,
	} {
		if wireless(serial) != want {
			t.Errorf("wireless(%q) != %v", serial, want)
		}
	}
}

func TestInput(t *testing.T) {
	if a, ok := hostPort(" 192.168.1.95:41485 "); !ok || a != "192.168.1.95:41485" {
		t.Errorf("hostPort: %q %v", a, ok)
	}
	for _, bad := range []string{"123456", "192.168.1.95", "phone:41485", ""} {
		if _, ok := hostPort(bad); ok {
			t.Errorf("hostPort(%q) took it", bad)
		}
	}
	if !isCode("905546") || isCode("90554") || isCode("90554a") {
		t.Error("isCode")
	}
}

func lan(iface, cidr string) localNet {
	ip, n, _ := net.ParseCIDR(cidr)
	return localNet{iface: iface, ip: ip, net: n}
}

var timeout = errors.New("i/o timeout")

func TestExplain(t *testing.T) {
	phone := net.ParseIP("192.168.1.95")
	wifiNet := []localNet{lan("en1", "192.168.1.170/24"), lan("utun4", "100.64.0.6/32")}

	// The phone is on the Wi-Fi, but its traffic goes into the VPN.
	p := netState{phone: phone, nets: wifiNet, route: "utun4", vpns: []string{"ExpressVPN"}, dial: timeout}.explain()
	if p == nil || !strings.Contains(p.title, "VPN is in the way") || !strings.Contains(p.title, "ExpressVPN") {
		t.Fatalf("vpn: %+v", p)
	}
	all := strings.Join(p.fixes, "\n")
	if !strings.Contains(all, "Allow access to devices on the local network") || !strings.Contains(all, "192.168.1.95") {
		t.Errorf("vpn fixes: %s", all)
	}

	// Another network: the computer moved to a hotspot.
	p = netState{phone: phone, nets: []localNet{lan("en1", "10.140.169.43/24")}, route: "en1", dial: timeout}.explain()
	if p == nil || !strings.Contains(p.title, "another network") || !strings.Contains(p.title, "10.140.169.43 on en1") {
		t.Errorf("other network: %+v", p)
	}
	// ...even when a VPN takes everything else.
	p = netState{phone: phone, nets: []localNet{lan("en1", "10.140.169.43/24")}, route: "utun4", dial: timeout}.explain()
	if p == nil || !strings.Contains(p.title, "another network") {
		t.Errorf("other network, vpn route: %+v", p)
	}

	// Same network, routed right, port closed.
	p = netState{phone: phone, nets: wifiNet, route: "en1", dial: syscall.ECONNREFUSED}.explain()
	if p == nil || !strings.Contains(p.title, "refused") {
		t.Errorf("refused: %+v", p)
	}
	// Same network, no answer.
	p = netState{phone: phone, nets: wifiNet, route: "en1", dial: timeout}.explain()
	if p == nil || !strings.Contains(p.title, "doesn't answer") {
		t.Errorf("no answer: %+v", p)
	}
	// Reachable.
	if p := (netState{phone: phone, nets: wifiNet, route: "en1"}).explain(); p != nil {
		t.Errorf("reachable: %+v", p)
	}
	// Route unknown (Windows), same network: nothing from the routes.
	if p := (netState{phone: phone, nets: wifiNet}).explainRoute(); p != nil {
		t.Errorf("no route info: %+v", p)
	}
}

func TestMatchVPNs(t *testing.T) {
	ps := "/Applications/ExpressVPN.app/Contents/MacOS/ExpressVPN\n" +
		"/Applications/ExpressVPN.app/Contents/MacOS/expressvpn-daemon\n" +
		"/Library/PrivilegedHelperTools/io.github.clash-verge-rev.clash-verge-rev.service.bundle/Contents/MacOS/clash-verge-service\n" +
		"/usr/sbin/mDNSResponder\n" +
		"\"WireGuard.exe\",\"1154\",\"Console\",\"1\",\"26,812 K\"\n"
	got := strings.Join(matchVPNs(ps), ", ")
	if got != "ExpressVPN, Clash, WireGuard" {
		t.Errorf("got %q", got)
	}
}

func TestQR(t *testing.T) {
	text := pairingQR("fdev-abc123", "p4ssw0rd22")
	if text != "WIFI:T:ADB;S:fdev-abc123;P:p4ssw0rd22;;" {
		t.Errorf("qr text %q", text)
	}
	out, err := renderQR(text)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) < 12 || !strings.Contains(out, "█") {
		t.Errorf("qr has %d lines", len(lines))
	}
	if os.Getenv("SHOW_QR") != "" {
		t.Log("\n" + out)
	}
	if r := randomText(10); len(r) != 10 || strings.ContainsAny(r, ";:,\\\"") {
		t.Errorf("randomText %q", r)
	}
}
