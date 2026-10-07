package wifi

import (
	"crypto/rand"
	"strings"

	"rsc.io/qr"
)

// pairingQR is the text Android's "Pair device with QR code" scans: the
// phone then announces a pairing service by that name, and adb pairs with
// the password.
func pairingQR(name, password string) string {
	return "WIFI:T:ADB;S:" + name + ";P:" + password + ";;"
}

func randomText(n int) string {
	const letters = "abcdefghijkmnpqrstuvwxyz23456789"
	b := make([]byte, n)
	_, _ = rand.Read(b)
	for i := range b {
		b[i] = letters[int(b[i])%len(letters)]
	}
	return string(b)
}

// renderQR draws text as a QR code, two modules a line with half blocks,
// black on white whatever the terminal's colors, with a quiet zone.
func renderQR(text string) (string, error) {
	code, err := qr.Encode(text, qr.M)
	if err != nil {
		return "", err
	}
	const quiet = 2
	size := code.Size + 2*quiet
	black := func(x, y int) bool {
		x, y = x-quiet, y-quiet
		return x >= 0 && y >= 0 && x < code.Size && y < code.Size && code.Black(x, y)
	}
	var b strings.Builder
	for y := 0; y < size; y += 2 {
		b.WriteString("  \x1b[30;107m")
		for x := 0; x < size; x++ {
			top, bottom := black(x, y), black(x, y+1)
			switch {
			case top && bottom:
				b.WriteString("█")
			case top:
				b.WriteString("▀")
			case bottom:
				b.WriteString("▄")
			default:
				b.WriteString(" ")
			}
		}
		b.WriteString("\x1b[0m\n")
	}
	return b.String(), nil
}
