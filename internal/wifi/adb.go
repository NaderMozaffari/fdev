package wifi

import (
	"context"
	"os/exec"
	"strings"
	"time"
)

// service is a phone's adb service found over mDNS: pairing (while a
// pairing dialog is open) or connect (while Wireless debugging is on).
type service struct {
	Name, Type, Addr string // Addr is ip:port
}

func (s service) pairing() bool { return strings.Contains(s.Type, "_adb-tls-pairing") }
func (s service) connect() bool { return strings.Contains(s.Type, "_adb-tls-connect") }

func (s service) host() string {
	host, _, _ := strings.Cut(s.Addr, ":")
	return host
}

// parseServices reads `adb mdns services`: name, type and ip:port, by tabs.
func parseServices(out string) []service {
	var list []service
	for _, line := range strings.Split(out, "\n") {
		f := strings.Split(strings.TrimRight(line, "\r"), "\t")
		if len(f) < 3 || !strings.Contains(f[1], "_adb-tls-") {
			continue
		}
		list = append(list, service{Name: strings.TrimSpace(f[0]), Type: strings.TrimSpace(f[1]), Addr: strings.TrimSpace(f[2])})
	}
	return list
}

type adb string

func (a adb) run(timeout time.Duration, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, string(a), args...).CombinedOutput()
	if ctx.Err() != nil {
		err = ctx.Err()
	}
	return strings.TrimSpace(string(out)), err
}

func (a adb) services() []service {
	out, _ := a.run(5*time.Second, "mdns", "services")
	return parseServices(out)
}

// pairOutcome is how `adb pair` went.
type pairOutcome int

const (
	paired pairOutcome = iota
	wrongCode
	unreachable // the network, or the dialog closed
	pairFailed
)

func pairResult(out string, err error) pairOutcome {
	low := strings.ToLower(out)
	switch {
	case strings.Contains(low, "successfully paired"):
		return paired
	case strings.Contains(low, "wrong password"):
		return wrongCode
	case strings.Contains(low, "protocol fault"), strings.Contains(low, "unable to start pairing"),
		strings.Contains(low, "connection refused"), strings.Contains(low, "timed out"),
		strings.Contains(low, "no route"), strings.Contains(low, "unreachable"),
		strings.Contains(low, "failed to connect"), err == context.DeadlineExceeded:
		return unreachable
	}
	return pairFailed
}

func (a adb) pair(addr, code string) (pairOutcome, string) {
	out, err := a.run(30*time.Second, "pair", addr, code)
	return pairResult(out, err), out
}

// connected reports whether `adb connect` got through.
func connected(out string) bool {
	low := strings.ToLower(out)
	return strings.Contains(low, "connected to") && !strings.Contains(low, "failed") && !strings.Contains(low, "cannot")
}

func (a adb) connect(addr string) (bool, string) {
	out, _ := a.run(10*time.Second, "connect", addr)
	ok := connected(out)
	if !ok {
		_, _ = a.run(3*time.Second, "disconnect", addr) // no offline leftover
	}
	return ok, out
}

// wireless is whether an adb serial is a Wi-Fi connection: ip:port, or
// the name adb gives a phone it connected to by itself over mDNS.
func wireless(serial string) bool {
	if strings.Contains(serial, "._adb-tls-connect.") {
		return true
	}
	host, port, ok := strings.Cut(serial, ":")
	return ok && port != "" && strings.Count(host, ".") == 3
}
