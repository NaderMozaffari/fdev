package main

import (
	_ "embed"
	"fmt"
	"os"
	"strings"

	"github.com/NaderMozaffari/fdev/internal/vscode"
)

// license goes into the extension's .vsix (the file is outside the package
// that builds it, and go:embed can't reach up a folder).
//
//go:embed LICENSE
var license []byte

// runVSCode is `fdev vscode [--editor <name> | --vsix <file>]`: install fdev's
// VS Code extension, or write its .vsix to a file.
func runVSCode(args []string) int {
	c := findCommand("vscode")
	editorName, vsixPath := "", ""
	value := func(i *int, a string) (string, bool) {
		if _, v, ok := strings.Cut(a, "="); ok {
			return v, true
		}
		if *i+1 < len(args) {
			*i++
			return args[*i], true
		}
		fmt.Fprintf(os.Stderr, "fdev vscode: %s needs a value\n", strings.SplitN(a, "=", 2)[0])
		return "", false
	}
	for i := 0; i < len(args); i++ {
		a := args[i]
		flag, _, _ := strings.Cut(a, "=")
		switch flag {
		case "--editor":
			v, ok := value(&i, a)
			if !ok {
				return 2
			}
			if _, known := vscode.Lookup(v); !known {
				return unknown("editor", v, vscode.EditorNames(), "fdev vscode --editor ")
			}
			editorName = v
		case "--vsix":
			v, ok := value(&i, a)
			if !ok {
				return 2
			}
			vsixPath = v
		case "-h", "--help":
			commandHelp(os.Stdout, c)
			return 0
		default:
			return unknown("option", a, optionFlags(c.options), "fdev vscode ")
		}
	}
	if editorName != "" && vsixPath != "" {
		fmt.Fprintln(os.Stderr, "fdev vscode: --vsix only writes the file, and --editor installs it: pick one")
		return 2
	}
	pkg, err := vscode.Info()
	if err != nil {
		fmt.Fprintln(os.Stderr, "fdev vscode:", err)
		return 1
	}
	vsix, err := vscode.VSIX(license)
	if err != nil {
		fmt.Fprintln(os.Stderr, "fdev vscode:", err)
		return 1
	}
	if vsixPath != "" {
		file, err := vscode.Write(vsix, vsixPath)
		if err != nil {
			fmt.Fprintln(os.Stderr, "fdev vscode:", err)
			return 1
		}
		fmt.Printf("Wrote %s (%s %s)\n", file, pkg.ID(), pkg.Version)
		fmt.Printf("Install it with: code --install-extension %q\n", file)
		fmt.Println("or in the editor: Extensions view, ..., Install from VSIX...")
		return 0
	}
	e, err := vscode.Install(vsix, editorName)
	if err != nil {
		fmt.Fprintln(os.Stderr, "fdev vscode:", err)
		return 1
	}
	fmt.Printf("Installed %s %s in %s.\n", pkg.ID(), pkg.Version, e.Title)
	fmt.Println("Reload the window to start it (Developer: Reload Window); Ctrl+Alt+L (Cmd+Alt+L on a Mac) then cycles show, dim and hide for the logs in your code.")
	return 0
}
