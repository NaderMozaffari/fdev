// rec records a program in a pseudo-terminal as an asciicast (v2), typing
// the keys a scene file gives it, for the README's GIFs and screenshots
// (see ../record.sh).
//
//	rec -scene hero.scene -o hero.cast -cols 120 -rows 32 -- fdev
//
// A scene is a line per step:
//
//	sleep 1.5s        wait
//	key down 3        press a key (enter, esc, tab, up, ctrl+s, a, ...), 3 times
//	type tag:Auth     type text, a key at a time
//	click 12 30       click the cell at column 12, row 30 (from 1)
//	mark name         a marker, to take a screenshot at (agg --select marker:name)
//	# a comment
//
// It also turns agg's GIFs into what the README shows:
//
//	rec png frame.gif shot.png                  a one-frame GIF as a PNG
//	rec join -delay 2.5s -o all.gif a.gif b.gif  one-frame GIFs as a slideshow
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/gif"
	"image/png"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/creack/pty"
)

var keys = map[string]string{
	"enter": "\r", "esc": "\x1b", "tab": "\t", "shift+tab": "\x1b[Z", "space": " ", "backspace": "\x7f",
	"up": "\x1b[A", "down": "\x1b[B", "right": "\x1b[C", "left": "\x1b[D",
	"shift+up": "\x1b[1;2A", "shift+down": "\x1b[1;2B",
	"home": "\x1b[H", "end": "\x1b[F", "pgup": "\x1b[5~", "pgdown": "\x1b[6~",
}

// The terminal's answers to what programs ask it: its colors, its
// attributes, the cursor, the modes it supports.
var (
	askBg     = regexp.MustCompile(`\x1b\]11;\?(\x07|\x1b\\)`)
	askFg     = regexp.MustCompile(`\x1b\]10;\?(\x07|\x1b\\)`)
	askDA1    = regexp.MustCompile(`\x1b\[0?c`)
	askCursor = regexp.MustCompile(`\x1b\[6n`)
	askMode   = regexp.MustCompile(`\x1b\[\?(\d+)\$p`)
)

type recorder struct {
	mu     sync.Mutex
	start  time.Time
	events [][3]any
	tty    *os.File
	bg, fg string
}

func (r *recorder) add(kind, data string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	t := float64(time.Since(r.start).Microseconds()) / 1e6
	r.events = append(r.events, [3]any{t, kind, data})
}

func (r *recorder) answer(out string) {
	for range askBg.FindAllString(out, -1) {
		r.tty.WriteString("\x1b]11;" + xcolor(r.bg) + "\x1b\\")
	}
	for range askFg.FindAllString(out, -1) {
		r.tty.WriteString("\x1b]10;" + xcolor(r.fg) + "\x1b\\")
	}
	for _, m := range askMode.FindAllStringSubmatch(out, -1) {
		r.tty.WriteString("\x1b[?" + m[1] + ";2$y") // known, reset
	}
	for range askCursor.FindAllString(out, -1) {
		r.tty.WriteString("\x1b[1;1R")
	}
	for range askDA1.FindAllString(out, -1) {
		r.tty.WriteString("\x1b[?62;22c")
	}
}

// xcolor is #rrggbb as X11's rgb:rrrr/gggg/bbbb.
func xcolor(hex string) string {
	h := strings.TrimPrefix(hex, "#")
	return fmt.Sprintf("rgb:%s%s/%s%s/%s%s", h[0:2], h[0:2], h[2:4], h[2:4], h[4:6], h[4:6])
}

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "png":
			check(toPNG(os.Args[2:]))
			return
		case "join":
			check(join(os.Args[2:]))
			return
		}
	}
	scene := flag.String("scene", "", "the scene file")
	out := flag.String("o", "out.cast", "the asciicast to write")
	cols := flag.Int("cols", 120, "terminal width")
	rows := flag.Int("rows", 32, "terminal height")
	bg := flag.String("bg", "#171717", "the terminal's background, told to the program")
	fg := flag.String("fg", "#dddddd", "the terminal's foreground")
	flag.Parse()
	if flag.NArg() == 0 || *scene == "" {
		fmt.Fprintln(os.Stderr, "usage: rec -scene file [-o out.cast] -- command [args]")
		os.Exit(2)
	}
	steps, err := os.ReadFile(*scene)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	cmd := exec.Command(flag.Arg(0), flag.Args()[1:]...)
	tty, err := pty.StartWithSize(cmd, &pty.Winsize{Cols: uint16(*cols), Rows: uint16(*rows)})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	r := &recorder{start: time.Now(), tty: tty, bg: *bg, fg: *fg}

	done := make(chan struct{})
	go func() {
		defer close(done)
		buf := make([]byte, 64<<10)
		var rest []byte
		for {
			n, err := tty.Read(buf)
			if n > 0 {
				data := append(rest, buf[:n]...)
				// Keep a rune cut in two for the next read.
				cut := len(data)
				for i := len(data) - 1; i >= 0 && i >= len(data)-utf8.UTFMax; i-- {
					if utf8.RuneStart(data[i]) {
						if !utf8.FullRune(data[i:]) {
							cut = i
						}
						break
					}
				}
				rest = append([]byte(nil), data[cut:]...)
				s := string(data[:cut])
				r.add("o", s)
				r.answer(s)
			}
			if err != nil {
				return
			}
		}
	}()

	sc := bufio.NewScanner(strings.NewReader(string(steps)))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		verb, arg, _ := strings.Cut(line, " ")
		switch verb {
		case "sleep":
			d, err := time.ParseDuration(arg)
			if err != nil {
				fail(line, err)
			}
			wait(d, done)
		case "key":
			name, count := arg, 1
			if f := strings.Fields(arg); len(f) == 2 {
				name = f[0]
				if count, err = strconv.Atoi(f[1]); err != nil {
					fail(line, err)
				}
			}
			seq, ok := keys[name]
			switch {
			case ok:
			case strings.HasPrefix(name, "ctrl+") && len(name) == 6:
				seq = string(rune(name[5] - 'a' + 1))
			case utf8.RuneCountInString(name) == 1:
				seq = name
			default:
				fail(line, fmt.Errorf("unknown key %q", name))
			}
			for i := 0; i < count; i++ {
				tty.WriteString(seq)
				wait(220*time.Millisecond, done)
			}
		case "type":
			for _, c := range arg {
				tty.WriteString(string(c))
				wait(110*time.Millisecond, done)
			}
		case "click":
			var x, y int
			if _, err := fmt.Sscan(arg, &x, &y); err != nil {
				fail(line, err)
			}
			tty.WriteString(fmt.Sprintf("\x1b[<35;%d;%dM", x, y)) // the pointer moves there first
			wait(120*time.Millisecond, done)
			tty.WriteString(fmt.Sprintf("\x1b[<0;%d;%dM\x1b[<0;%d;%dm", x, y, x, y))
			wait(150*time.Millisecond, done)
		case "move":
			var x, y int
			if _, err := fmt.Sscan(arg, &x, &y); err != nil {
				fail(line, err)
			}
			tty.WriteString(fmt.Sprintf("\x1b[<35;%d;%dM", x, y))
			wait(120*time.Millisecond, done)
		case "mark":
			r.add("m", arg)
		default:
			fail(line, fmt.Errorf("unknown step"))
		}
	}

	cmd.Process.Signal(os.Interrupt)
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		cmd.Process.Kill()
	}
	tty.Close()

	f, err := os.Create(*out)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.Encode(map[string]any{"version": 2, "width": *cols, "height": *rows, "timestamp": r.start.Unix(),
		"env": map[string]string{"TERM": "xterm-256color", "SHELL": "/bin/zsh"}})
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, e := range r.events {
		enc.Encode(e)
	}
}

// wait waits for d, or until the program ends.
func wait(d time.Duration, done <-chan struct{}) {
	select {
	case <-time.After(d):
	case <-done:
	}
}

func fail(line string, err error) {
	fmt.Fprintf(os.Stderr, "rec: %s: %v\n", line, err)
	os.Exit(2)
}

func check(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "rec:", err)
		os.Exit(1)
	}
}

// lastFrame is the last frame of a GIF.
func lastFrame(path string) (*image.Paletted, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	g, err := gif.DecodeAll(f)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return g.Image[len(g.Image)-1], nil
}

func toPNG(args []string) error {
	if len(args) != 2 {
		return fmt.Errorf("usage: rec png frame.gif shot.png")
	}
	img, err := lastFrame(args[0])
	if err != nil {
		return err
	}
	f, err := os.Create(args[1])
	if err != nil {
		return err
	}
	defer f.Close()
	return (&png.Encoder{CompressionLevel: png.BestCompression}).Encode(f, img)
}

func join(args []string) error {
	fs := flag.NewFlagSet("join", flag.ExitOnError)
	out := fs.String("o", "out.gif", "the GIF to write")
	delay := fs.Duration("delay", 2500*time.Millisecond, "how long each frame shows")
	fs.Parse(args)
	all := &gif.GIF{LoopCount: 0}
	for _, path := range fs.Args() {
		img, err := lastFrame(path)
		if err != nil {
			return err
		}
		all.Image = append(all.Image, img)
		all.Delay = append(all.Delay, int(delay.Milliseconds()/10))
		all.Disposal = append(all.Disposal, gif.DisposalNone)
	}
	if len(all.Image) == 0 {
		return fmt.Errorf("no frames")
	}
	f, err := os.Create(*out)
	if err != nil {
		return err
	}
	defer f.Close()
	return gif.EncodeAll(f, all)
}
