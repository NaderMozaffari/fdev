package logview

import (
	"bytes"
	"encoding/json"
	"sort"
	"strings"
)

// node is a parsed JSON value that keeps the key order.
type node struct {
	keys   []string
	values []*node
	object bool
	array  bool
	scalar string // encoded, for anything else
}

func parseJSON(data []byte) (*node, bool) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	n, err := readNode(dec)
	if err != nil || dec.More() {
		return nil, false
	}
	return n, true
}

func readNode(dec *json.Decoder) (*node, error) {
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	switch t := tok.(type) {
	case json.Delim:
		n := &node{object: t == '{', array: t == '['}
		for dec.More() {
			if n.object {
				keyTok, err := dec.Token()
				if err != nil {
					return nil, err
				}
				key, _ := keyTok.(string)
				n.keys = append(n.keys, key)
			}
			child, err := readNode(dec)
			if err != nil {
				return nil, err
			}
			n.values = append(n.values, child)
		}
		if _, err := dec.Token(); err != nil { // the closing delimiter
			return nil, err
		}
		return n, nil
	default:
		return &node{scalar: encode(t)}, nil
	}
}

// encode is json.Marshal without escaping <, > and & (as \u003c...).
func encode(v any) string {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
	return strings.TrimSuffix(b.String(), "\n")
}

// prettyLines indents n, keeping arrays of plain values on one line.
func prettyLines(n *node) []string {
	return strings.Split(pretty(n, ""), "\n")
}

func pretty(n *node, indent string) string {
	inner := indent + "  "
	switch {
	case n.object && len(n.values) > 0:
		parts := make([]string, 0, len(n.values))
		for _, i := range keyOrder(n.keys) {
			parts = append(parts, inner+encode(n.keys[i])+": "+pretty(n.values[i], inner))
		}
		return "{\n" + strings.Join(parts, ",\n") + "\n" + indent + "}"
	case n.object:
		return "{}"
	case n.array && len(n.values) > 0 && hasContainer(n.values):
		parts := make([]string, len(n.values))
		for i, v := range n.values {
			parts[i] = inner + pretty(v, inner)
		}
		return "[\n" + strings.Join(parts, ",\n") + "\n" + indent + "]"
	case n.array:
		parts := make([]string, len(n.values))
		for i, v := range n.values {
			parts[i] = v.scalar
		}
		return "[" + strings.Join(parts, ", ") + "]"
	}
	return n.scalar
}

func hasContainer(values []*node) bool {
	for _, v := range values {
		if v.object || v.array {
			return true
		}
	}
	return false
}

// jsonLines pretty-prints data if it is a JSON object or array.
func jsonLines(data []byte) ([]string, bool) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || (trimmed[0] != '{' && trimmed[0] != '[') {
		return nil, false
	}
	n, ok := parseJSON(trimmed)
	if !ok {
		return nil, false
	}
	return prettyLines(n), true
}

// looseLines indents text that starts like JSON but does not parse, such as
// a body the app cut short ("… (44352 chars)"): not checked, only made
// readable. Line breaks follow {, [ and the commas between values.
func looseLines(s string) ([]string, bool) {
	t := strings.TrimSpace(s)
	if t == "" || (t[0] != '{' && t[0] != '[') {
		return nil, false
	}
	var lines []string
	var cur strings.Builder
	depth := 0
	inString, escaped := false, false
	flush := func() {
		if l := strings.TrimRight(cur.String(), " "); strings.TrimSpace(l) != "" {
			lines = append(lines, l)
		}
		cur.Reset()
		cur.WriteString(strings.Repeat("  ", depth))
	}
	for _, r := range t {
		if inString {
			cur.WriteRune(r)
			switch {
			case escaped:
				escaped = false
			case r == '\\':
				escaped = true
			case r == '"':
				inString = false
			}
			continue
		}
		switch r {
		case '"':
			inString = true
			cur.WriteRune(r)
		case '{', '[':
			cur.WriteRune(r)
			depth++
			flush()
		case '}', ']':
			depth = max(depth-1, 0)
			flush()
			cur.WriteRune(r)
		case ',':
			cur.WriteRune(r)
			flush()
		case ':':
			cur.WriteString(": ")
		case '\n', '\r', '\t', ' ':
			if l := cur.String(); strings.TrimSpace(l) != "" && !strings.HasSuffix(l, " ") {
				cur.WriteRune(' ')
			}
		default:
			cur.WriteRune(r)
		}
	}
	flush()
	return lines, len(lines) > 1
}

// A server sends keys in any order, so an object is shown with the keys
// that say what it is first and its dates last, the others between them in
// the order they came:
//
//	id  uuid  name  title  type  status  code  message  …the rest…  created_at  updated_at  deleted_at
var (
	firstKeys = []string{"id", "uuid", "name", "title", "type", "status", "code", "message"}
	lastKeys  = []string{"createdat", "updatedat", "deletedat"}
)

const plainRank = 100

// keyRank is where a key goes: its place among firstKeys, plainRank for the
// others, and after them the dates (created_at, createdAt and created-at
// are the same).
func keyRank(key string) int {
	k := strings.ToLower(strings.NewReplacer("_", "", "-", "").Replace(key))
	for i, f := range firstKeys {
		if k == f {
			return i
		}
	}
	for i, l := range lastKeys {
		if k == l {
			return plainRank + 1 + i
		}
	}
	return plainRank
}

// keyOrder is the indexes of keys in the order they are shown.
func keyOrder(keys []string) []int {
	order := make([]int, len(keys))
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(a, b int) bool { return keyRank(keys[order[a]]) < keyRank(keys[order[b]]) })
	return order
}

// importantKey and dateKey are what render.go draws apart from the rest.
func importantKey(key string) bool { return keyRank(key) < plainRank }
func dateKey(key string) bool      { return keyRank(key) > plainRank }
