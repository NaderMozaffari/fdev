// Package wifi is `fdev wifi`: debugging an Android phone over Wi-Fi. It
// connects phones paired before, and pairs new ones with a QR code shown
// in the terminal or with the phone's pairing code. When the phone can't be
// reached it says why: a VPN taking the traffic, another network, a closed
// dialog.
package wifi

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"strings"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/NaderMozaffari/fdev/internal/devices"
)

type Options struct {
	In  io.Reader
	Out io.Writer
	// Wait asks for enter before returning, so the output stays on screen
	// when fdev's launcher ran this.
	Wait bool
	// Pair skips connecting phones paired before.
	Pair bool
}

var (
	bold  = lipgloss.NewStyle().Bold(true)
	muted = lipgloss.NewStyle().Foreground(lipgloss.BrightBlack)
	green = lipgloss.NewStyle().Foreground(lipgloss.Green)
	red   = lipgloss.NewStyle().Foreground(lipgloss.Red)
	amber = lipgloss.NewStyle().Foreground(lipgloss.Yellow)
)

type session struct {
	adb   adb
	out   io.Writer
	lines chan string // what the user types, a line at a time
	// failed is the pairing services tried, so they're not tried again
	// until the phone shows a new one (a new dialog has a new port).
	failed map[string]bool
}

func Run(o Options) int {
	s := &session{out: o.Out, lines: make(chan string), failed: map[string]bool{}}
	go func() {
		sc := bufio.NewScanner(o.In)
		for sc.Scan() {
			s.lines <- strings.TrimSpace(sc.Text())
		}
		close(s.lines)
	}()
	code := s.run(o)
	if o.Wait {
		s.say(muted.Render("\npress enter to go back"))
		<-s.lines
	}
	return code
}

func (s *session) say(a ...any) { lipgloss.Fprintln(s.out, a...) }

func (s *session) run(o Options) int {
	path := devices.ADB()
	if path == "" {
		s.say(red.Render("✘ adb not found."), "Install Android SDK Platform-Tools (Android Studio → SDK Manager), or put adb on PATH.")
		return 1
	}
	s.adb = adb(path)
	_, _ = s.adb.run(15*time.Second, "start-server")
	s.say(bold.Render("Wi-Fi debugging"))

	before := s.wirelessDevices()
	for _, d := range before {
		s.say(green.Render("✓ connected:"), d.Name, muted.Render(d.ID))
	}
	if !o.Pair {
		if s.reconnect(before) {
			return 0
		}
	}
	return s.pair(before)
}

func (s *session) wirelessDevices() []devices.Device {
	list, _ := devices.List("", "android")
	var out []devices.Device
	for _, d := range list {
		if wireless(d.ID) {
			out = append(out, d)
		}
	}
	return out
}

// reconnect connects the phones that announce themselves and were paired
// before; true when one connected.
func (s *session) reconnect(have []devices.Device) bool {
	unpaired := false
	for _, svc := range s.adb.services() {
		if !svc.connect() || isConnected(have, svc) {
			continue
		}
		s.say(muted.Render("connecting to " + svc.Addr + " ..."))
		if ok, _ := s.adb.connect(svc.Addr); ok {
			s.done(have)
			return true
		}
		switch p := check(svc.Addr); {
		case p == nil: // it answers, but won't take this computer
			unpaired = true
		case p.stale: // an old announcement
		default:
			s.report(p)
			s.say(muted.Render("Once that's fixed, run fdev wifi again: a phone paired before connects without pairing."))
			s.say(muted.Render("Or pair below; fdev checks the network again when the phone shows up."))
		}
	}
	if unpaired {
		s.say(muted.Render("The phone answers but isn't paired with this computer: pair it below."))
	}
	return false
}

func isConnected(have []devices.Device, svc service) bool {
	for _, d := range have {
		if d.ID == svc.Addr || strings.HasPrefix(d.ID, svc.Name+".") {
			return true
		}
	}
	return false
}

// pair shows the QR code and waits for the phone: scanning it, opening the
// pairing-code dialog, or the user typing the dialog's address.
func (s *session) pair(before []devices.Device) int {
	name, password := "fdev-"+randomText(6), randomText(10)
	qr, err := renderQR(pairingQR(name, password))
	if err != nil {
		s.say(red.Render("✘ " + err.Error()))
		return 1
	}
	s.say("")
	s.say("On the phone: " + bold.Render("Settings → Developer options → Wireless debugging") + " (on the same Wi-Fi as this computer), then")
	s.say("  " + bold.Render("1  Pair device with QR code") + " and scan this:")
	s.say("")
	fmt.Fprint(s.out, qr)
	s.say("")
	s.say("  " + bold.Render("2  Pair device with pairing code") + ": fdev finds the phone and asks for the code.")
	s.say(muted.Render("     If it doesn't, type the IP address & Port the dialog shows (e.g. 192.168.1.95:41485)."))
	s.say(muted.Render("  q to quit"))
	s.say("")

	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	deadline := time.After(10 * time.Minute)
	hinted, start := false, time.Now()
	for {
		select {
		case line, ok := <-s.lines:
			if !ok || line == "q" || line == "quit" {
				return 1
			}
			if addr, ok := hostPort(line); ok {
				if s.withCode(addr) {
					return s.connectAfter(addr, before)
				}
			} else if line != "" {
				s.say(muted.Render("type the dialog's IP address & Port, like 192.168.1.95:41485, or q"))
			}
		case <-tick.C:
			for _, svc := range s.adb.services() {
				if !svc.pairing() || s.failed[svc.Addr] {
					continue
				}
				ok := false
				if svc.Name == name {
					s.say(green.Render("● QR code scanned") + muted.Render(" · pairing with "+svc.Addr+" ..."))
					ok = s.pairWith(svc.Addr, password, true)
				} else {
					s.say(green.Render("● phone found") + muted.Render(" ("+svc.Addr+")"))
					ok = s.withCode(svc.Addr)
				}
				if ok {
					return s.connectAfter(svc.Addr, before)
				}
				break // the services may have changed meanwhile
			}
			if !hinted && time.Since(start) > 45*time.Second {
				hinted = true
				s.say(muted.Render("Nothing yet. Check the phone is on this Wi-Fi with its pairing dialog open; or type the IP address & Port the dialog shows."))
			}
		case <-deadline:
			s.say(red.Render("✘ gave up waiting for the phone"))
			return 1
		}
	}
}

// withCode asks for the pairing code shown with addr and pairs.
func (s *session) withCode(addr string) bool {
	if p := precheck(addr); p != nil {
		s.failed[addr] = true
		s.report(p)
		s.say(muted.Render("Then open the pairing dialog again; fdev keeps waiting."))
		return false
	}
	for {
		fmt.Fprint(s.out, bold.Render("Pairing code")+muted.Render(" (6 digits, on the phone; enter to skip): "))
		code, ok := <-s.lines
		if !ok {
			return false
		}
		code = strings.ReplaceAll(code, " ", "")
		if code == "" || code == "q" {
			s.failed[addr] = true
			return false
		}
		if !isCode(code) {
			s.say(amber.Render("the code is the 6 digits under \"Wi-Fi pairing code\""))
			continue
		}
		return s.pairWith(addr, code, false)
	}
}

func (s *session) pairWith(addr, secret string, qr bool) bool {
	if p := precheck(addr); p != nil {
		s.failed[addr] = true
		s.report(p)
		s.say(muted.Render("Then pair again; fdev keeps waiting."))
		return false
	}
	outcome, out := s.adb.pair(addr, secret)
	switch outcome {
	case paired:
		s.say(green.Render("✓ paired"))
		return true
	case wrongCode:
		if qr {
			s.say(red.Render("✘ the phone turned the pairing down") + muted.Render(" (connection dropped); scan the code again"))
		} else {
			s.say(red.Render("✘ wrong code") + muted.Render(", or the dialog closed; open it again for a new code"))
		}
	case unreachable:
		if p := check(addr); p != nil {
			s.report(p)
		} else {
			s.say(red.Render("✘ couldn't pair: ") + out)
			s.say(muted.Render("  the network looks fine, so the dialog most likely closed (it does when the screen locks); open it again"))
		}
	default:
		s.say(red.Render("✘ couldn't pair: ") + out)
	}
	s.failed[addr] = true
	return false
}

// connectAfter connects the phone just paired at pairAddr: adb usually does
// it by itself; otherwise its connect service, or the address typed.
func (s *session) connectAfter(pairAddr string, before []devices.Device) int {
	host, _, _ := net.SplitHostPort(pairAddr)
	s.say(muted.Render("connecting ..."))
	tried := map[string]bool{}
	for i := 0; i < 10; i++ {
		if len(s.newDevices(before)) > 0 {
			return s.done(before)
		}
		for _, svc := range s.adb.services() {
			if svc.connect() && svc.host() == host && !tried[svc.Addr] {
				tried[svc.Addr] = true
				if ok, _ := s.adb.connect(svc.Addr); ok {
					return s.done(before)
				}
			}
		}
		time.Sleep(time.Second)
	}
	for {
		fmt.Fprint(s.out, bold.Render("IP address & Port")+muted.Render(" on the Wireless debugging screen (not the pairing one; enter to quit): "))
		line, ok := <-s.lines
		if !ok || line == "" || line == "q" {
			return 1
		}
		addr, ok := hostPort(line)
		if !ok {
			continue
		}
		if ok, out := s.adb.connect(addr); ok {
			return s.done(before)
		} else if p := check(addr); p != nil {
			s.report(p)
		} else {
			s.say(red.Render("✘ " + out))
		}
	}
}

func (s *session) newDevices(before []devices.Device) []devices.Device {
	var out []devices.Device
	for _, d := range s.wirelessDevices() {
		known := false
		for _, b := range before {
			known = known || b.ID == d.ID
		}
		if !known {
			out = append(out, d)
		}
	}
	return out
}

func (s *session) done(before []devices.Device) int {
	list := s.newDevices(before)
	for i := 0; len(list) == 0 && i < 5; i++ {
		time.Sleep(500 * time.Millisecond)
		list = s.newDevices(before)
	}
	for _, d := range list {
		s.say(green.Render("✓ connected over Wi-Fi:"), bold.Render(d.Name), muted.Render(d.ID))
	}
	if len(list) == 0 {
		s.say(green.Render("✓ connected over Wi-Fi"))
	}
	s.say(muted.Render("Run your app on it from fdev (it's in the device list) or with flutter run."))
	s.say(muted.Render("Next time, with Wireless debugging on, fdev wifi connects it again without pairing."))
	return 0
}

func (s *session) report(p *problem) {
	s.say(red.Render("✘ " + p.title))
	for _, f := range p.fixes {
		s.say("  → " + f)
	}
}

func hostPort(s string) (string, bool) {
	host, port, err := net.SplitHostPort(strings.TrimSpace(s))
	if err != nil || net.ParseIP(host) == nil || port == "" {
		return "", false
	}
	return net.JoinHostPort(host, port), true
}

func isCode(s string) bool {
	if len(s) != 6 {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
