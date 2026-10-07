package wifi

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"syscall"
	"time"
)

// localNet is an IPv4 network this computer is on.
type localNet struct {
	iface string
	ip    net.IP
	net   *net.IPNet
}

// netState is what the diagnosis looks at: this computer's networks, the
// interface the system routes the phone's address through, the VPN apps
// running, and how dialing the phone went.
type netState struct {
	phone net.IP
	nets  []localNet
	route string // "" when unknown
	vpns  []string
	dial  error
}

// problem is why the computer can't reach the phone, with what to do.
type problem struct {
	title string
	fixes []string
	stale bool // the port is closed: the phone moved on to another
}

// check dials the phone and, when that fails, works out why.
func check(addr string) *problem {
	s, ok := state(addr)
	if !ok {
		return nil
	}
	if s.dial = dial(addr); s.dial == nil {
		return nil
	}
	s.nets, s.route, s.vpns = localNets(), routeInterface(s.phone.String()), runningVPNs()
	return s.explain()
}

// precheck is check without dialing: only what the routes tell (a VPN, another
// network). A pairing port is not dialed before pairing, for the phone
// may take a stray connection as a failed pairing.
func precheck(addr string) *problem {
	s, ok := state(addr)
	if !ok {
		return nil
	}
	s.nets, s.route = localNets(), routeInterface(s.phone.String())
	if s.explainRoute() == nil {
		return nil
	}
	s.vpns = runningVPNs() // only to name them
	return s.explainRoute()
}

func state(addr string) (netState, bool) {
	host, _, err := net.SplitHostPort(addr)
	if err != nil || net.ParseIP(host) == nil {
		return netState{}, false
	}
	return netState{phone: net.ParseIP(host)}, true
}

func dial(addr string) error {
	c, err := net.DialTimeout("tcp", addr, 4*time.Second)
	if err == nil {
		c.Close()
	}
	return err
}

var tunnelName = regexp.MustCompile(`^(utun|tun|tap|wg|ppp|ipsec|gpd|zt|tailscale|nordlynx)`)

// explain turns the state into a problem; nil when the phone answers.
func (s netState) explain() *problem {
	if s.dial == nil {
		return nil
	}
	if p := s.explainRoute(); p != nil {
		return p
	}
	if errors.Is(s.dial, syscall.ECONNREFUSED) {
		return &problem{
			stale: true,
			title: "The phone refused the connection: the port is closed",
			fixes: []string{"the dialog closed or Wireless debugging restarted; open it again (it has a new port)"},
		}
	}
	p := &problem{title: fmt.Sprintf("The phone (%s) doesn't answer, though it is on this network (%s)", s.phone, s.lanFor(s.phone).iface)}
	if len(s.vpns) > 0 {
		p.title += ". VPN apps running: " + strings.Join(s.vpns, ", ")
		p.fixes = append(p.fixes, "the VPN may block the local network (a firewall or kill switch): "+vpnFixes(s.vpns)[0])
	}
	p.fixes = append(p.fixes,
		"keep the phone unlocked with the Wireless debugging screen open",
		"guest Wi-Fi and some routers keep devices apart (client/AP isolation); try another Wi-Fi or a phone hotspot")
	return p
}

// explainRoute is the problem the routes show: the phone's traffic going
// into a VPN's tunnel, or the phone on another network; nil when neither.
func (s netState) explainRoute() *problem {
	lan := s.lanFor(s.phone)
	if lan != nil && s.route != "" && s.route != lan.iface {
		p := &problem{title: fmt.Sprintf("The VPN is in the way: traffic to the phone (%s) goes into the tunnel (%s) instead of the Wi-Fi", s.phone, s.route)}
		if len(s.vpns) > 0 {
			p.title += ". VPN apps running: " + strings.Join(s.vpns, ", ")
		}
		p.fixes = append(p.fixes, vpnFixes(s.vpns)...)
		if lan != nil {
			if c := routeCommand(s.phone.String(), lan.iface); c != "" {
				p.fixes = append(p.fixes, "or keep the VPN as it is and send just the phone around it (until the next reboot or network change):\n      "+c)
			}
		}
		p.fixes = append(p.fixes, "or turn the VPN off while you debug")
		return p
	}
	if lan == nil {
		var mine []string
		for _, n := range s.nets {
			if !tunnelName.MatchString(n.iface) {
				mine = append(mine, fmt.Sprintf("%s on %s", n.ip, n.iface))
			}
		}
		where := "no network"
		if len(mine) > 0 {
			where = strings.Join(mine, ", ")
		}
		return &problem{
			title: fmt.Sprintf("The phone (%s) is on another network than this computer (%s)", s.phone, where),
			fixes: []string{"connect the phone and the computer to the same Wi-Fi (a phone hotspot works too, with the computer on it)"},
		}
	}
	return nil
}

func (s netState) lanFor(ip net.IP) *localNet {
	for i := range s.nets {
		if s.nets[i].net.Contains(ip) && !tunnelName.MatchString(s.nets[i].iface) {
			return &s.nets[i]
		}
	}
	return nil
}

func localNets() []localNet {
	ifaces, _ := net.Interfaces()
	var out []localNet
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, _ := iface.Addrs()
		for _, a := range addrs {
			if n, ok := a.(*net.IPNet); ok && n.IP.To4() != nil {
				out = append(out, localNet{iface: iface.Name, ip: n.IP, net: n})
			}
		}
	}
	return out
}

// routeInterface is the interface the system sends ip's traffic through.
func routeInterface(ip string) string {
	switch runtime.GOOS {
	case "darwin":
		return field(run("route", "-n", "get", ip), "interface:")
	case "linux":
		return field(run("ip", "route", "get", ip), "dev")
	}
	return ""
}

func routeCommand(ip, iface string) string {
	switch runtime.GOOS {
	case "darwin":
		return fmt.Sprintf("sudo route -n add -host %s -interface %s", ip, iface)
	case "linux":
		return fmt.Sprintf("sudo ip route add %s dev %s", ip, iface)
	}
	return ""
}

// field is the word after key in out.
func field(out, key string) string {
	f := strings.Fields(out)
	for i := 0; i+1 < len(f); i++ {
		if f[i] == key {
			return f[i+1]
		}
	}
	return ""
}

func run(name string, args ...string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	out, _ := exec.CommandContext(ctx, name, args...).Output()
	return string(out)
}

// vpnApps are VPN apps by a word in their process name, with how to let
// local traffic past them.
var vpnApps = []struct{ word, name, fix string }{
	{"expressvpn", "ExpressVPN", "in ExpressVPN, turn on Settings → General → \"Allow access to devices on the local network\""},
	{"clash", "Clash", "in Clash, send the LAN direct (rule IP-CIDR,192.168.0.0/16,DIRECT and 10.0.0.0/8) or turn TUN mode off"},
	{"v2ray", "v2ray", "in v2ray, route private IPs direct (\"bypass LAN\")"},
	{"xray", "Xray", "in Xray, route private IPs direct (\"bypass LAN\")"},
	{"sing-box", "sing-box", "in sing-box, route private IPs direct (ip_is_private → direct)"},
	{"hiddify", "Hiddify", "in Hiddify, turn on \"Bypass LAN\" (or use proxy mode instead of VPN)"},
	{"karing", "Karing", "in Karing, let private/LAN addresses go direct"},
	{"nekoray", "NekoRay", "in NekoRay, route private IPs direct (\"bypass LAN\")"},
	{"nekobox", "NekoBox", "in NekoBox, route private IPs direct (\"bypass LAN\")"},
	{"v2box", "V2Box", "in V2Box, turn on \"Bypass LAN\""},
	{"foxray", "FoXray", "in FoXray, route private IPs direct"},
	{"streisand", "Streisand", "in Streisand, route private IPs direct"},
	{"outline", "Outline", "Outline sends everything through the tunnel; disconnect it while you debug"},
	{"psiphon", "Psiphon", "Psiphon's VPN mode takes all traffic; use its proxy mode or disconnect"},
	{"wireguard", "WireGuard", "in WireGuard, leave the LAN out of AllowedIPs (\"Exclude private IPs\")"},
	{"openvpn", "OpenVPN", "in OpenVPN, allow LAN access (or add route-nopull / a route for the LAN)"},
	{"tunnelblick", "Tunnelblick", "in Tunnelblick, let the LAN go outside the VPN"},
	{"nordvpn", "NordVPN", "in NordVPN, turn on \"Allow LAN\" (Settings → Connection)"},
	{"surfshark", "Surfshark", "in Surfshark, turn on \"Bypass VPN for local network\""},
	{"protonvpn", "Proton VPN", "in Proton VPN, turn on \"Allow LAN connections\""},
	{"windscribe", "Windscribe", "in Windscribe, turn on \"Allow LAN traffic\""},
	{"mullvad", "Mullvad", "in Mullvad, turn on \"Local network sharing\""},
	{"cyberghost", "CyberGhost", "in CyberGhost, allow local network access"},
	{"tailscale", "Tailscale", "in Tailscale, turn on \"Allow local network access\" when using an exit node"},
}

func vpnFixes(running []string) []string {
	var out []string
	for _, name := range running {
		for _, a := range vpnApps {
			if a.name == name {
				out = append(out, a.fix)
			}
		}
	}
	if len(out) == 0 {
		out = append(out, "in the VPN app, turn on its LAN option (\"Allow LAN\", \"Bypass LAN\", \"local network access\")")
	}
	return out
}

// runningVPNs names the VPN apps running.
func runningVPNs() []string {
	var procs string
	if runtime.GOOS == "windows" {
		procs = run("tasklist", "/fo", "csv", "/nh")
	} else {
		procs = run("ps", "-axo", "comm=")
	}
	return matchVPNs(procs)
}

func matchVPNs(procs string) []string {
	seen := map[string]bool{}
	for _, line := range strings.Split(procs, "\n") {
		base := strings.ToLower(filepath.Base(strings.Trim(strings.TrimSpace(line), `"`)))
		if base == "" || base == "." {
			continue
		}
		for _, a := range vpnApps {
			if strings.Contains(base, a.word) {
				seen[a.name] = true
			}
		}
	}
	var out []string
	for _, a := range vpnApps { // in the list's order
		if seen[a.name] {
			out = append(out, a.name)
			delete(seen, a.name)
		}
	}
	return out
}
