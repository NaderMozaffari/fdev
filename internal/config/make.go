package config

import (
	"regexp"
	"strings"
)

// makefile is what fdev reads from a Makefile: its rules with their
// recipes and descriptions, and its variables, which it can expand the way
// make would with nothing set on the command line (no $(shell ...)).
type makefile struct {
	vars map[string]string
	// optional is the `NAME ?=` variables with an empty default: what the
	// caller may set (DEVICE=..., STORE=...).
	optional map[string]bool
	// tested is the values a variable is compared to, by $(filter v,$(V))
	// or ifeq ($(V),v): the answers it takes.
	tested map[string][]string
	rules  []makeRule
	help   map[string]string // from `make help`'s echo lines
}

type makeRule struct {
	name, desc string
	recipe     []string
}

var (
	makeAssign  = regexp.MustCompile(`^([A-Za-z_][A-Za-z0-9_]*)\s*(\?=|::=|:=|\+=|=)\s*(.*)$`)
	makeRuleRe  = regexp.MustCompile(`^([A-Za-z0-9][A-Za-z0-9_.-]*)\s*:([^=]|$)`)
	makeHelp    = regexp.MustCompile(`^@?echo\s+["']\s{1,4}([A-Za-z0-9][A-Za-z0-9_.-]*)\s{2,}(.+?)["']\s*$`)
	makeFilter  = regexp.MustCompile(`\$\(filter\s+([^,()$]+),\s*\$[({]([A-Za-z_][A-Za-z0-9_]*)[)}]\s*\)`)
	makeIfeq    = regexp.MustCompile(`if(?:n)?eq\s*\(\s*\$[({]([A-Za-z_][A-Za-z0-9_]*)[)}]\s*,\s*([^)$]+)\)`)
	makeVarRef  = regexp.MustCompile(`\$[({]([A-Za-z_][A-Za-z0-9_]*)[)}]`)
	makeDoubleH = regexp.MustCompile(`##\s*(.+)$`)
)

func parseMakefile(text string) *makefile {
	mk := &makefile{vars: map[string]string{}, optional: map[string]bool{}, tested: map[string][]string{}, help: map[string]string{}}
	// Join continued lines first.
	var lines []string
	cur := ""
	for _, l := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		if strings.HasSuffix(l, "\\") {
			cur += strings.TrimSuffix(l, "\\") + " "
			continue
		}
		lines = append(lines, cur+l)
		cur = ""
	}
	addTested := func(name, values string) {
		for _, v := range strings.Fields(values) {
			if !contains(mk.tested[name], v) {
				mk.tested[name] = append(mk.tested[name], v)
			}
		}
	}
	for _, m := range makeFilter.FindAllStringSubmatch(text, -1) {
		addTested(m[2], m[1])
	}
	for _, m := range makeIfeq.FindAllStringSubmatch(text, -1) {
		addTested(m[1], m[2])
	}

	var rule *makeRule
	comment := ""
	seen := map[string]bool{}
	for _, line := range lines {
		if strings.HasPrefix(line, "\t") {
			if rule != nil {
				r := strings.TrimSpace(line)
				rule.recipe = append(rule.recipe, r)
				if m := makeHelp.FindStringSubmatch(r); m != nil {
					mk.help[m[1]] = m[2]
				}
			}
			continue
		}
		if rule != nil && len(rule.recipe) > 0 {
			mk.rules = append(mk.rules, *rule)
		}
		rule = nil
		trimmed := strings.TrimSpace(line)
		switch {
		case trimmed == "":
			comment = ""
			continue
		case strings.HasPrefix(trimmed, "#"):
			comment = strings.TrimSpace(strings.TrimLeft(trimmed, "# "))
			continue
		}
		if m := makeAssign.FindStringSubmatch(line); m != nil {
			name, op, value := m[1], m[2], strings.TrimSpace(m[3])
			switch op {
			case "?=":
				if _, ok := mk.vars[name]; !ok {
					mk.vars[name] = value
					mk.optional[name] = value == ""
				}
			case "+=":
				mk.vars[name] = strings.TrimSpace(mk.vars[name] + " " + value)
			default:
				mk.vars[name] = value
				delete(mk.optional, name)
			}
			comment = ""
			continue
		}
		if m := makeRuleRe.FindStringSubmatch(line); m != nil && !seen[m[1]] {
			seen[m[1]] = true
			desc := comment
			if d := makeDoubleH.FindStringSubmatch(line); d != nil {
				desc = strings.TrimSpace(d[1])
			}
			rule = &makeRule{name: m[1], desc: desc}
		}
		comment = ""
	}
	if rule != nil && len(rule.recipe) > 0 {
		mk.rules = append(mk.rules, *rule)
	}
	return mk
}

// refs is every variable s uses, and the ones those use.
func (mk *makefile) refs(s string) map[string]bool {
	out := map[string]bool{}
	var walk func(string, int)
	walk = func(s string, depth int) {
		if depth > 8 {
			return
		}
		for _, m := range makeVarRef.FindAllStringSubmatch(s, -1) {
			if !out[m[1]] {
				out[m[1]] = true
				walk(mk.vars[m[1]], depth+1)
			}
		}
	}
	walk(s, 0)
	return out
}

// expand is s as make would run it with nothing set: variables replaced,
// a few functions worked out, $(shell ...) and the rest empty.
func (mk *makefile) expand(s string) string { return mk.expandAt(s, 0) }

func (mk *makefile) expandAt(s string, depth int) string {
	if depth > 10 || !strings.Contains(s, "$") {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] != '$' || i+1 >= len(s) {
			b.WriteByte(s[i])
			continue
		}
		next := s[i+1]
		if next == '$' {
			b.WriteByte('$')
			i++
			continue
		}
		if next != '(' && next != '{' {
			i++ // $@ and the like
			continue
		}
		end := matchParen(s, i+1)
		if end < 0 {
			b.WriteString(s[i:])
			break
		}
		b.WriteString(mk.call(s[i+2:end], depth))
		i = end
	}
	return b.String()
}

// matchParen is the index of the bracket closing the one at open.
func matchParen(s string, open int) int {
	o, c := s[open], byte(')')
	if o == '{' {
		c = '}'
	}
	depth := 0
	for i := open; i < len(s); i++ {
		switch s[i] {
		case o:
			depth++
		case c:
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

// call works out what is inside $( ): a variable or a function.
func (mk *makefile) call(inner string, depth int) string {
	name, rest, isFunc := strings.Cut(inner, " ")
	if !isFunc || strings.ContainsAny(name, "$,") {
		return mk.expandAt(mk.vars[mk.expandAt(inner, depth+1)], depth+1)
	}
	args := splitArgs(rest)
	arg := func(i int) string {
		if i < len(args) {
			return strings.TrimSpace(mk.expandAt(args[i], depth+1))
		}
		return ""
	}
	switch name {
	case "if":
		if arg(0) != "" {
			return arg(1)
		}
		return arg(2)
	case "or":
		for i := range args {
			if v := arg(i); v != "" {
				return v
			}
		}
		return ""
	case "and":
		v := ""
		for i := range args {
			if v = arg(i); v == "" {
				return ""
			}
		}
		return v
	case "filter", "filter-out":
		patterns := strings.Fields(arg(0))
		var out []string
		for _, w := range strings.Fields(arg(1)) {
			if contains(patterns, w) == (name == "filter") {
				out = append(out, w)
			}
		}
		return strings.Join(out, " ")
	case "subst":
		return strings.ReplaceAll(arg(2), arg(0), arg(1))
	case "strip":
		return strings.Join(strings.Fields(arg(0)), " ")
	case "firstword":
		if f := strings.Fields(arg(0)); len(f) > 0 {
			return f[0]
		}
	}
	return "" // shell, wildcard, and what fdev doesn't need
}

// splitArgs splits a function's arguments on the commas outside brackets.
func splitArgs(s string) []string {
	var out []string
	depth, start := 0, 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '(', '{':
			depth++
		case ')', '}':
			depth--
		case ',':
			if depth == 0 {
				out = append(out, s[start:i])
				start = i + 1
			}
		}
	}
	return append(out, s[start:])
}
