package logview

import (
	"encoding/json"
	"strings"
	"time"
	"unicode"
)

// Values are what is worth keeping at hand while the app runs: tokens,
// ids. They come from the app (FdevLog.value, a record with "l":"value"),
// and from network headers and bodies, by key: the token keys below and
// the project's own (Parser.ValueKeys). The newest of each name is kept,
// for this session only: nothing of it is written anywhere but the log.

// Value is one value found in a log.
type Value struct {
	Name, Value string
	From        string // where it was found: "POST /auth/login · response body"
}

// tokenKeys are the keys, folded (lower case, no _ or -), whose values are
// kept without asking.
var tokenKeys = map[string]bool{
	"authorization": true, "accesstoken": true, "refreshtoken": true, "idtoken": true,
	"token": true, "authtoken": true, "jwt": true, "sessionid": true, "apikey": true, "xapikey": true,
	"xauthtoken": true, "bearertoken": true,
}

// valueKey is a key as the lists compare it.
func valueKey(k string) string {
	return strings.Map(func(r rune) rune {
		if r == '_' || r == '-' || r == ' ' {
			return -1
		}
		return unicode.ToLower(r)
	}, k)
}

// findValues walks a JSON value for keys to keep, at any depth; a string
// that holds JSON is read too.
func (p *Parser) findValues(n *node, from string) []Value {
	var out []Value
	var walk func(n *node, depth int)
	walk = func(n *node, depth int) {
		if n == nil || depth > 12 {
			return
		}
		if !n.object && !n.array {
			var s string
			if json.Unmarshal([]byte(n.scalar), &s) == nil {
				if t := strings.TrimSpace(s); t != "" && (t[0] == '{' || t[0] == '[') {
					if inner, ok := parseJSON([]byte(t)); ok {
						walk(inner, depth+1)
					}
				}
			}
			return
		}
		for i, v := range n.values {
			if n.object && !v.object && !v.array && p.keeps(n.keys[i]) {
				value := v.scalar
				var s string
				if json.Unmarshal([]byte(value), &s) == nil {
					value = s
				}
				if !masked(value) && len(value) < 64*1024 {
					out = append(out, Value{Name: clean(n.keys[i]), Value: clean(value), From: from})
				}
				continue
			}
			walk(v, depth+1)
		}
	}
	walk(n, 0)
	return out
}

func (p *Parser) keeps(key string) bool {
	k := valueKey(key)
	if tokenKeys[k] {
		return true
	}
	for _, extra := range p.ValueKeys {
		if valueKey(extra) == k {
			return true
		}
	}
	return false
}

// value is a record the app sent to keep a value at hand.
func (p *Parser) value(r record, now time.Time) *Entry {
	v := string(r.Value)
	var s string
	if json.Unmarshal(r.Value, &s) == nil {
		v = s
	}
	name := clean(r.Key)
	if name == "" {
		name = "value"
	}
	e := p.entry("value", clean(r.Tag), name+" = "+preview(clean(v), 32), now, "")
	e.Values = []Value{{Name: name, Value: clean(v), From: "FdevLog.value"}}
	return e
}

// preview is the start of a long value and its length.
func preview(v string, n int) string {
	r := []rune(v)
	if len(r) <= n {
		return v
	}
	return string(r[:n]) + "… (" + plural(len(r), "char") + ")"
}
