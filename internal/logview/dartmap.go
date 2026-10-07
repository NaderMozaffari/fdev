package logview

import (
	"regexp"
	"strings"
)

// Dart prints a Map as {id: 7, name: Ali, tags: [a, b]}: keys and strings
// without quotes. fdev reads that back into JSON, so a map that was logged
// with print() or in a string ('user: $map') is shown like a JSON body.

// prettyText lays out a message that is, or ends with, a JSON value or a
// Dart Map, List or Set: indented, as JSON. Text before the value stays as
// it is, on the first lines. ok is false for anything else, and for a value
// that fits on one line.
func prettyText(msg string) ([]Line, bool) {
	t := strings.TrimRight(msg, " \n\r")
	if t == "" || (t[len(t)-1] != '}' && t[len(t)-1] != ']') {
		return nil, false
	}
	tried := 0
	for i := 0; i < len(t) && tried < 6; i++ {
		if t[i] != '{' && t[i] != '[' {
			continue
		}
		tried++
		value := t[i:]
		lines, ok := jsonLines([]byte(value))
		if !ok {
			var n *node
			if n, ok = parseDart(value); ok {
				lines = prettyLines(n)
			}
		}
		if !ok || len(lines) < 2 {
			continue
		}
		var out []Line
		head := strings.Split(t[:i], "\n")
		for _, h := range head[:len(head)-1] {
			out = append(out, Line{h, LinePlain})
		}
		out = append(out, Line{head[len(head)-1] + lines[0], LineJSON})
		for _, l := range lines[1:] {
			out = append(out, Line{l, LineJSON})
		}
		return out, true
	}
	return nil, false
}

// dartKey is what follows ", " when a map's next key starts: a key, then
// ": ". A value with ", " in it is kept whole when what follows doesn't
// read as a key.
var dartKey = regexp.MustCompile(`^, [^,:{}\[\]\n]{1,80}?: `)

var dartNumber = regexp.MustCompile(`^-?\d+(?:\.\d+)?(?:[eE][+-]?\d+)?$`)

type dartParser struct {
	s string
	i int
}

// parseDart reads a Dart Map, List or Set as toString prints it.
func parseDart(s string) (*node, bool) {
	p := &dartParser{s: s}
	n, ok := p.value(0, false)
	if !ok || p.i != len(s) || !(n.object || n.array) {
		return nil, false
	}
	return n, true
}

// value reads one value; close is the bracket that ends the container it
// is in ('}' for a map or a set, ']' for a list, 0 at the top), inMap
// whether that is a map.
func (p *dartParser) value(close byte, inMap bool) (*node, bool) {
	if p.i >= len(p.s) {
		return nil, false
	}
	switch p.s[p.i] {
	case '{':
		return p.mapOrSet()
	case '[':
		return p.list(']')
	}
	return p.scalar(close, inMap), true
}

func (p *dartParser) mapOrSet() (*node, bool) {
	start := p.i
	p.i++ // {
	if p.i < len(p.s) && p.s[p.i] == '}' {
		p.i++
		return &node{object: true}, true
	}
	n := &node{object: true}
	for {
		key, ok := p.key()
		if !ok {
			// No "key: ": a Set, {a, b}.
			p.i = start
			return p.list('}')
		}
		v, ok := p.value('}', true)
		if !ok {
			return nil, false
		}
		n.keys = append(n.keys, key)
		n.values = append(n.values, v)
		switch {
		case strings.HasPrefix(p.s[p.i:], ", "):
			p.i += 2
		case p.i < len(p.s) && p.s[p.i] == '}':
			p.i++
			return n, true
		default:
			return nil, false
		}
	}
}

// key reads "key: ", up to the first ": " before any bracket or comma.
func (p *dartParser) key() (string, bool) {
	for j := p.i; j < len(p.s); j++ {
		switch p.s[j] {
		case ',', '{', '}', '[', ']', '\n':
			return "", false
		case ':':
			if j+1 < len(p.s) && p.s[j+1] == ' ' && j > p.i {
				key := p.s[p.i:j]
				p.i = j + 2
				return key, true
			}
		}
	}
	return "", false
}

func (p *dartParser) list(close byte) (*node, bool) {
	p.i++ // [ or {
	n := &node{array: true}
	if p.i < len(p.s) && p.s[p.i] == close {
		p.i++
		return n, true
	}
	for {
		v, ok := p.value(close, false)
		if !ok {
			return nil, false
		}
		n.values = append(n.values, v)
		switch {
		case strings.HasPrefix(p.s[p.i:], ", "):
			p.i += 2
		case p.i < len(p.s) && p.s[p.i] == close:
			p.i++
			return n, true
		default:
			return nil, false
		}
	}
}

// scalar reads a value up to where its container goes on: in a map, ", "
// before a key, or the '}'; in a list or a set, ", " or its bracket.
func (p *dartParser) scalar(close byte, inMap bool) *node {
	start := p.i
	for p.i < len(p.s) {
		rest := p.s[p.i:]
		if close != 0 && rest[0] == close {
			break
		}
		if strings.HasPrefix(rest, ", ") && close != 0 && (!inMap || dartKey.MatchString(rest)) {
			break
		}
		p.i++
	}
	return dartScalar(p.s[start:p.i])
}

func dartScalar(s string) *node {
	switch {
	case s == "null", s == "true", s == "false", dartNumber.MatchString(s):
		return &node{scalar: s}
	}
	return &node{scalar: encode(s)}
}
