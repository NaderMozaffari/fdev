package main

import (
	"slices"
	"testing"
)

func TestVSCodeCommand(t *testing.T) {
	if c := findCommand("vscode"); c == nil || c.name != "vscode" {
		t.Errorf("findCommand(vscode) = %v", c)
	}
	// Other names stay free: a project can have a Makefile target called
	// "code" or "extension", and fdev <target> must still run it.
	for _, n := range []string{"code", "extension"} {
		if c := findCommand(n); c != nil {
			t.Errorf("%q must not be a command (it would hide a project's target), got %v", n, c.name)
		}
	}
	if got := suggest("vscod", commandNames()); !slices.Contains(got, "vscode") {
		t.Errorf("suggest(vscod) = %q", got)
	}
	flags := optionFlags(findCommand("vscode").options)
	if !slices.Equal(flags, []string{"--editor", "--vsix"}) {
		t.Errorf("flags = %q", flags)
	}
	// Mistakes are refused before anything is built or installed.
	for _, args := range [][]string{{"--editor"}, {"--editor", "emacs"}, {"--bogus"}, {"--editor", "code", "--vsix", "x"}} {
		if got := runVSCode(args); got != 2 {
			t.Errorf("runVSCode(%q) = %d, want 2", args, got)
		}
	}
}
