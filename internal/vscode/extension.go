package vscode

import (
	"archive/zip"
	"bytes"
	"embed"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/NaderMozaffari/fdev/internal/editor"
)

// The extension is plain JavaScript with no build step, so the folder is
// what a .vsix holds. Its tests (extension/test) are listed out of the
// patterns on purpose: they stay in the repository, not in the binary.
//
//go:embed extension/package.json extension/README.md extension/src
var extensionFiles embed.FS

// Editor is one of the editors built on VS Code that can run the extension.
type Editor struct {
	Name    string   // what --editor takes: code, cursor, ...
	Title   string   // how it is called to a person
	Product string   // the folder it keeps its settings in
	CLI     string   // its command line, which installs extensions
	Aliases []string // other names --editor takes
}

// editors is in the order fdev tries them when it doesn't run inside one.
var editors = []Editor{
	{"code", "Visual Studio Code", "Code", "code", []string{"vscode", "vs-code"}},
	{"code-insiders", "VS Code Insiders", "Code - Insiders", "code-insiders", []string{"insiders"}},
	{"cursor", "Cursor", "Cursor", "cursor", nil},
	{"windsurf", "Windsurf", "Windsurf", "windsurf", nil},
	{"codium", "VSCodium", "VSCodium", "codium", []string{"vscodium"}},
}

func named(name string) Editor {
	e, _ := Lookup(name)
	return e
}

// Lookup is the editor --editor <name> means (its name or an alias, in any case).
func Lookup(name string) (Editor, bool) {
	name = strings.ToLower(strings.TrimSpace(name))
	for _, e := range editors {
		if e.Name == name {
			return e, true
		}
		for _, a := range e.Aliases {
			if a == name {
				return e, true
			}
		}
	}
	return Editor{}, false
}

// EditorNames is every name --editor takes, for the help and the "did you
// mean" suggestions.
func EditorNames() []string {
	var out []string
	for _, e := range editors {
		out = append(out, e.Name)
		out = append(out, e.Aliases...)
	}
	return out
}

// Package is what the extension's package.json says about it.
type Package struct {
	Name, DisplayName, Description, Version, Publisher string
	Engine                                             string // engines.vscode, e.g. ^1.85.0
	License, Homepage, Repository                      string
	Categories, Keywords                               []string
	ExtensionKind                                      []string
}

// ID is how VS Code names the extension: publisher.name.
func (p Package) ID() string { return p.Publisher + "." + p.Name }

// Info reads the embedded extension's package.json.
func Info() (Package, error) {
	data, err := extensionFiles.ReadFile("extension/package.json")
	if err != nil {
		return Package{}, err
	}
	var raw struct {
		Name          string            `json:"name"`
		DisplayName   string            `json:"displayName"`
		Description   string            `json:"description"`
		Version       string            `json:"version"`
		Publisher     string            `json:"publisher"`
		License       string            `json:"license"`
		Homepage      string            `json:"homepage"`
		Categories    []string          `json:"categories"`
		Keywords      []string          `json:"keywords"`
		ExtensionKind []string          `json:"extensionKind"`
		Engines       map[string]string `json:"engines"`
		Repository    struct {
			URL string `json:"url"`
		} `json:"repository"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return Package{}, fmt.Errorf("extension/package.json: %w", err)
	}
	p := Package{
		Name: raw.Name, DisplayName: raw.DisplayName, Description: raw.Description,
		Version: raw.Version, Publisher: raw.Publisher, Engine: raw.Engines["vscode"],
		License: raw.License, Homepage: raw.Homepage, Repository: raw.Repository.URL,
		Categories: raw.Categories, Keywords: raw.Keywords, ExtensionKind: raw.ExtensionKind,
	}
	if p.DisplayName == "" {
		p.DisplayName = p.Name
	}
	for what, v := range map[string]string{"name": p.Name, "version": p.Version, "publisher": p.Publisher, "engines.vscode": p.Engine} {
		if v == "" {
			return Package{}, fmt.Errorf("extension/package.json has no %s", what)
		}
	}
	return p, nil
}

// FileName is what the .vsix is called: fdev-0.1.0.vsix.
func (p Package) FileName() string { return p.Name + "-" + p.Version + ".vsix" }

// licenseEntry is where the license goes in the package, as vsce puts it.
const licenseEntry = "extension/LICENSE.txt"

// VSIX builds the extension's .vsix in memory: a zip with the manifest
// VS Code reads before it unpacks anything, and the extension's files under
// extension/. license is fdev's LICENSE (the file is outside this package, so
// the caller embeds it); it is left out when empty.
func VSIX(license []byte) ([]byte, error) {
	pkg, err := Info()
	if err != nil {
		return nil, err
	}
	type entry struct {
		name string
		data []byte
	}
	var files []entry
	err = fs.WalkDir(extensionFiles, "extension", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == "test" || d.Name() == "node_modules" {
				return fs.SkipDir
			}
			return nil
		}
		data, err := extensionFiles.ReadFile(p)
		files = append(files, entry{p, data})
		return err
	})
	if err != nil {
		return nil, err
	}
	if len(license) > 0 {
		files = append(files, entry{licenseEntry, license})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].name < files[j].name })

	var names []string
	for _, f := range files {
		names = append(names, f.name)
	}
	out := []entry{
		{"[Content_Types].xml", contentTypes(names)},
		{"extension.vsixmanifest", manifest(pkg, len(license) > 0)},
	}
	out = append(out, files...)

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	// One fixed time, so the same fdev always makes the same bytes.
	stamp := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	for _, f := range out {
		w, err := zw.CreateHeader(&zip.FileHeader{Name: f.name, Method: zip.Deflate, Modified: stamp})
		if err != nil {
			return nil, err
		}
		if _, err := w.Write(f.data); err != nil {
			return nil, err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

var contentTypeOf = map[string]string{
	".json":         "application/json",
	".js":           "application/javascript",
	".md":           "text/markdown",
	".txt":          "text/plain",
	".svg":          "image/svg+xml",
	".png":          "image/png",
	".vsixmanifest": "text/xml",
}

// contentTypes is [Content_Types].xml: a type for every extension in the
// package, as the Open Packaging Conventions ask.
func contentTypes(names []string) []byte {
	seen := map[string]bool{".vsixmanifest": true}
	for _, n := range names {
		seen[strings.ToLower(path.Ext(n))] = true
	}
	var exts []string
	for e := range seen {
		exts = append(exts, e)
	}
	sort.Strings(exts)
	var b strings.Builder
	b.WriteString(xml.Header)
	b.WriteString(`<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">` + "\n")
	for _, e := range exts {
		typ := contentTypeOf[e]
		if typ == "" {
			typ = "application/octet-stream"
		}
		fmt.Fprintf(&b, "  <Default Extension=%q ContentType=%q/>\n", e, typ)
	}
	b.WriteString("</Types>\n")
	return []byte(b.String())
}

func esc(s string) string {
	var b bytes.Buffer
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}

// manifest is extension.vsixmanifest, which says what is in the package.
func manifest(p Package, hasLicense bool) []byte {
	var b strings.Builder
	b.WriteString(xml.Header)
	b.WriteString(`<PackageManifest Version="2.0.0" xmlns="http://schemas.microsoft.com/developer/vsx-schema/2011" xmlns:d="http://schemas.microsoft.com/developer/vsx-schema-design/2011">` + "\n")
	b.WriteString("  <Metadata>\n")
	fmt.Fprintf(&b, "    <Identity Language=\"en-US\" Id=\"%s\" Version=\"%s\" Publisher=\"%s\" />\n", esc(p.Name), esc(p.Version), esc(p.Publisher))
	fmt.Fprintf(&b, "    <DisplayName>%s</DisplayName>\n", esc(p.DisplayName))
	fmt.Fprintf(&b, "    <Description xml:space=\"preserve\">%s</Description>\n", esc(p.Description))
	if len(p.Keywords) > 0 {
		fmt.Fprintf(&b, "    <Tags>%s</Tags>\n", esc(strings.Join(p.Keywords, ",")))
	}
	if len(p.Categories) > 0 {
		fmt.Fprintf(&b, "    <Categories>%s</Categories>\n", esc(strings.Join(p.Categories, ",")))
	}
	b.WriteString("    <GalleryFlags>Public</GalleryFlags>\n")
	b.WriteString("    <Properties>\n")
	prop := func(id, value string) {
		fmt.Fprintf(&b, "      <Property Id=\"%s\" Value=\"%s\" />\n", id, esc(value))
	}
	prop("Microsoft.VisualStudio.Code.Engine", p.Engine)
	prop("Microsoft.VisualStudio.Code.ExtensionDependencies", "")
	prop("Microsoft.VisualStudio.Code.ExtensionPack", "")
	prop("Microsoft.VisualStudio.Code.ExtensionKind", strings.Join(p.ExtensionKind, ","))
	prop("Microsoft.VisualStudio.Code.LocalizedLanguages", "")
	if p.Repository != "" {
		prop("Microsoft.VisualStudio.Services.Links.Source", p.Repository)
		prop("Microsoft.VisualStudio.Services.Links.Getstarted", p.Repository)
		prop("Microsoft.VisualStudio.Services.Links.Repository", p.Repository)
	}
	if p.Homepage != "" {
		prop("Microsoft.VisualStudio.Services.Links.Learn", p.Homepage)
	}
	b.WriteString("    </Properties>\n")
	if hasLicense {
		b.WriteString("    <License>" + licenseEntry + "</License>\n")
	}
	b.WriteString("  </Metadata>\n")
	b.WriteString("  <Installation>\n    <InstallationTarget Id=\"Microsoft.VisualStudio.Code\"/>\n  </Installation>\n")
	b.WriteString("  <Dependencies/>\n")
	b.WriteString("  <Assets>\n")
	b.WriteString("    <Asset Type=\"Microsoft.VisualStudio.Code.Manifest\" Path=\"extension/package.json\" Addressable=\"true\" />\n")
	b.WriteString("    <Asset Type=\"Microsoft.VisualStudio.Services.Content.Details\" Path=\"extension/README.md\" Addressable=\"true\" />\n")
	if hasLicense {
		b.WriteString("    <Asset Type=\"Microsoft.VisualStudio.Services.Content.License\" Path=\"" + licenseEntry + "\" Addressable=\"true\" />\n")
	}
	b.WriteString("  </Assets>\n")
	b.WriteString("</PackageManifest>\n")
	return []byte(b.String())
}

// Write puts the .vsix at dest, or in dest when that is a folder, and
// returns the file it wrote.
func Write(vsix []byte, dest string) (string, error) {
	if info, err := os.Stat(dest); err == nil && info.IsDir() {
		pkg, err := Info()
		if err != nil {
			return "", err
		}
		dest = filepath.Join(dest, pkg.FileName())
	}
	return dest, os.WriteFile(dest, vsix, 0o644)
}

// How Install finds an editor's command line and runs it; tests swap them.
var (
	findCLI = editor.Find
	runCLI  = func(bin string, args ...string) ([]byte, error) {
		return exec.Command(bin, args...).CombinedOutput()
	}
)

// Choose is the editor to install into, and its command line: the one named
// (want), else the one fdev runs inside, else the first found. An editor
// that isn't installed is an error when it was asked for by name, and
// skipped otherwise.
func Choose(want string) (Editor, string, error) {
	if want != "" {
		e, ok := Lookup(want)
		if !ok {
			return Editor{}, "", fmt.Errorf("no editor %q (the choices are %s)", want, strings.Join(editorNames(), ", "))
		}
		bin := findCLI(e.CLI)
		if bin == "" {
			return Editor{}, "", fmt.Errorf("can't find %s's command line (%s): put it on PATH (in VS Code: Shell Command: Install 'code' command in PATH)", e.Title, e.CLI)
		}
		return e, bin, nil
	}
	if os.Getenv("TERM_PROGRAM") == "vscode" {
		if e := current(); findCLI(e.CLI) != "" {
			return e, findCLI(e.CLI), nil
		}
	}
	for _, e := range editors {
		if bin := findCLI(e.CLI); bin != "" {
			return e, bin, nil
		}
	}
	return Editor{}, "", errors.New("no VS Code, Cursor, Windsurf or VSCodium command line found: fdev vscode --vsix <file> writes the extension to install it by hand (Extensions view, ..., Install from VSIX)")
}

func editorNames() []string {
	var out []string
	for _, e := range editors {
		out = append(out, e.Name)
	}
	return out
}

// Install installs the .vsix in the editor Choose picks, through the editor's
// own command line (--install-extension, run without a shell), and returns
// that editor. --force makes it replace an older fdev.
func Install(vsix []byte, want string) (Editor, error) {
	e, bin, err := Choose(want)
	if err != nil {
		return e, err
	}
	pkg, err := Info()
	if err != nil {
		return e, err
	}
	dir, err := os.MkdirTemp("", "fdev-vsix-")
	if err != nil {
		return e, err
	}
	defer os.RemoveAll(dir)
	file, err := Write(vsix, filepath.Join(dir, pkg.FileName()))
	if err != nil {
		return e, err
	}
	if out, err := runCLI(bin, "--install-extension", file, "--force"); err != nil {
		return e, fmt.Errorf("%s --install-extension failed: %w\n%s", filepath.Base(bin), err, strings.TrimSpace(string(out)))
	}
	return e, nil
}
