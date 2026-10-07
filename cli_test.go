package main

import (
	"slices"
	"testing"
)

func TestSuggest(t *testing.T) {
	known := []string{"logs", "wifi", "update", "version", "help", "dev", "prod", "build-prod", "ios-dev", "ios-prod", "run dev"}
	for word, want := range map[string][]string{
		"versoin":    {"version"},
		"updte":      {"update"},
		"dve":        {"dev"},
		"build-pord": {"build-prod"},
		"ios":        {"ios-dev", "ios-prod"},
		"run dve":    {"run dev"},
		"flutter":    nil,
	} {
		if got := suggest(word, known); !slices.Equal(got, want) {
			t.Errorf("suggest(%q) = %q, want %q", word, got, want)
		}
	}
}

func TestEveryCommandIsFound(t *testing.T) {
	for _, n := range commandNames() {
		if findCommand(n) == nil {
			t.Errorf("findCommand(%q) = nil", n)
		}
	}
}
