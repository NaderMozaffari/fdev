package vscode

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"encoding/xml"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func readVSIX(t *testing.T, license []byte) map[string]string {
	t.Helper()
	data, err := VSIX(license)
	if err != nil {
		t.Fatal(err)
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("not a zip: %v", err)
	}
	files := map[string]string{}
	for _, f := range zr.File {
		r, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		b, err := io.ReadAll(r)
		r.Close()
		if err != nil {
			t.Fatalf("%s: %v", f.Name, err)
		}
		files[f.Name] = string(b)
	}
	return files
}

func TestVSIXHasTheFilesVSCodeReads(t *testing.T) {
	files := readVSIX(t, []byte("MIT License"))
	for _, name := range []string{
		"[Content_Types].xml", "extension.vsixmanifest",
		"extension/package.json", "extension/README.md", "extension/LICENSE.txt",
		"extension/src/extension.js", "extension/src/scan.js",
	} {
		if _, ok := files[name]; !ok {
			t.Errorf("%s is not in the .vsix", name)
		}
	}
	for name := range files {
		if strings.Contains(name, "/test/") || strings.HasPrefix(name, "test/") || strings.Contains(name, "node_modules") {
			t.Errorf("%s must not be in the .vsix", name)
		}
		if name != "[Content_Types].xml" && name != "extension.vsixmanifest" && !strings.HasPrefix(name, "extension/") {
			t.Errorf("%s is outside extension/", name)
		}
	}
	if files["extension/LICENSE.txt"] != "MIT License" {
		t.Errorf("license = %q", files["extension/LICENSE.txt"])
	}
}

func TestVSIXWithoutLicense(t *testing.T) {
	files := readVSIX(t, nil)
	if _, ok := files["extension/LICENSE.txt"]; ok {
		t.Error("a LICENSE appeared out of nothing")
	}
	if strings.Contains(files["extension.vsixmanifest"], "License") {
		t.Error("the manifest names a license that isn't there")
	}
}

func TestManifestMatchesPackageJSON(t *testing.T) {
	files := readVSIX(t, []byte("MIT"))
	var pkg struct {
		Name, DisplayName, Description, Version, Publisher string
		Engines                                            map[string]string
	}
	if err := json.Unmarshal([]byte(files["extension/package.json"]), &pkg); err != nil {
		t.Fatal(err)
	}
	var m struct {
		Identity struct {
			Language  string `xml:"Language,attr"`
			ID        string `xml:"Id,attr"`
			Version   string `xml:"Version,attr"`
			Publisher string `xml:"Publisher,attr"`
		} `xml:"Metadata>Identity"`
		DisplayName string `xml:"Metadata>DisplayName"`
		Description string `xml:"Metadata>Description"`
		Properties  []struct {
			ID    string `xml:"Id,attr"`
			Value string `xml:"Value,attr"`
		} `xml:"Metadata>Properties>Property"`
		Assets []struct {
			Type string `xml:"Type,attr"`
			Path string `xml:"Path,attr"`
		} `xml:"Assets>Asset"`
	}
	if err := xml.Unmarshal([]byte(files["extension.vsixmanifest"]), &m); err != nil {
		t.Fatalf("manifest is not XML: %v", err)
	}
	if m.Identity.Language != "en-US" || m.Identity.ID != pkg.Name || m.Identity.Version != pkg.Version || m.Identity.Publisher != pkg.Publisher {
		t.Errorf("identity = %+v, package.json = %+v", m.Identity, pkg)
	}
	if m.DisplayName != pkg.DisplayName || m.Description != pkg.Description {
		t.Errorf("display name/description = %q / %q", m.DisplayName, m.Description)
	}
	engine := ""
	for _, p := range m.Properties {
		if p.ID == "Microsoft.VisualStudio.Code.Engine" {
			engine = p.Value
		}
	}
	if engine == "" || engine != pkg.Engines["vscode"] {
		t.Errorf("engine = %q, package.json says %q", engine, pkg.Engines["vscode"])
	}
	assets := map[string]string{}
	for _, a := range m.Assets {
		assets[a.Type] = a.Path
		if _, ok := files[a.Path]; !ok {
			t.Errorf("asset %s names %s, which is not in the .vsix", a.Type, a.Path)
		}
	}
	if assets["Microsoft.VisualStudio.Code.Manifest"] != "extension/package.json" ||
		assets["Microsoft.VisualStudio.Services.Content.Details"] != "extension/README.md" {
		t.Errorf("assets = %v", assets)
	}
}

func TestContentTypesCoverEveryExtension(t *testing.T) {
	files := readVSIX(t, []byte("MIT"))
	var types struct {
		Defaults []struct {
			Extension string `xml:"Extension,attr"`
			Type      string `xml:"ContentType,attr"`
		} `xml:"Default"`
	}
	if err := xml.Unmarshal([]byte(files["[Content_Types].xml"]), &types); err != nil {
		t.Fatal(err)
	}
	have := map[string]bool{}
	for _, d := range types.Defaults {
		have[d.Extension] = d.Type != ""
	}
	for name := range files {
		if name == "[Content_Types].xml" {
			continue
		}
		if ext := strings.ToLower(filepath.Ext(name)); !have[ext] {
			t.Errorf("no content type for %s (%s)", ext, name)
		}
	}
}

func TestInfoMatchesTheEmbeddedPackage(t *testing.T) {
	p, err := Info()
	if err != nil {
		t.Fatal(err)
	}
	if p.ID() != "nadermozaffari.fdev" || p.FileName() != "fdev-"+p.Version+".vsix" || p.Engine == "" {
		t.Errorf("Info = %+v", p)
	}
}

func TestVSIXIsTheSameEveryTime(t *testing.T) {
	a, _ := VSIX([]byte("x"))
	b, _ := VSIX([]byte("x"))
	if !bytes.Equal(a, b) {
		t.Error("two builds differ")
	}
}

func TestLookup(t *testing.T) {
	for in, want := range map[string]string{"code": "code", "VSCode": "code", "insiders": "code-insiders", "cursor": "cursor", "vscodium": "codium", "windsurf": "windsurf"} {
		if e, ok := Lookup(in); !ok || e.Name != want {
			t.Errorf("Lookup(%q) = %q, %v; want %q", in, e.Name, ok, want)
		}
	}
	if _, ok := Lookup("emacs"); ok {
		t.Error("emacs is not a VS Code")
	}
}

// fakeCLIs makes findCLI know only the commands in have (cli -> path).
func fakeCLIs(t *testing.T, have map[string]string) {
	t.Helper()
	old := findCLI
	findCLI = func(name string) string { return have[name] }
	t.Cleanup(func() { findCLI = old })
}

func TestChoose(t *testing.T) {
	all := map[string]string{"code": "/bin/code", "cursor": "/bin/cursor", "codium": "/bin/codium"}

	t.Run("the editor fdev runs in", func(t *testing.T) {
		fakeCLIs(t, all)
		t.Setenv("TERM_PROGRAM", "vscode")
		t.Setenv("__CFBundleIdentifier", "com.todesktop.230313mzl4w4u92")
		t.Setenv("VSCODE_GIT_ASKPASS_NODE", "")
		t.Setenv("TERM_PROGRAM_VERSION", "")
		if e, bin, err := Choose(""); err != nil || e.Name != "cursor" || bin != "/bin/cursor" {
			t.Errorf("Choose = %v, %q, %v", e.Name, bin, err)
		}
	})
	t.Run("the first found outside an editor", func(t *testing.T) {
		fakeCLIs(t, map[string]string{"cursor": "/bin/cursor", "codium": "/bin/codium"})
		t.Setenv("TERM_PROGRAM", "Apple_Terminal")
		if e, _, err := Choose(""); err != nil || e.Name != "cursor" {
			t.Errorf("Choose = %v, %v", e.Name, err)
		}
	})
	t.Run("the editor it runs in is not installed", func(t *testing.T) {
		fakeCLIs(t, map[string]string{"codium": "/bin/codium"})
		t.Setenv("TERM_PROGRAM", "vscode")
		t.Setenv("__CFBundleIdentifier", "com.microsoft.VSCode")
		t.Setenv("VSCODE_GIT_ASKPASS_NODE", "")
		t.Setenv("TERM_PROGRAM_VERSION", "")
		if e, _, err := Choose(""); err != nil || e.Name != "codium" {
			t.Errorf("Choose = %v, %v", e.Name, err)
		}
	})
	t.Run("--editor wins", func(t *testing.T) {
		fakeCLIs(t, all)
		t.Setenv("TERM_PROGRAM", "vscode")
		if e, bin, err := Choose("vscodium"); err != nil || e.Name != "codium" || bin != "/bin/codium" {
			t.Errorf("Choose = %v, %q, %v", e.Name, bin, err)
		}
	})
	t.Run("--editor that is not installed is an error", func(t *testing.T) {
		fakeCLIs(t, all)
		if _, _, err := Choose("windsurf"); err == nil || !strings.Contains(err.Error(), "Windsurf") {
			t.Errorf("err = %v", err)
		}
	})
	t.Run("none found", func(t *testing.T) {
		fakeCLIs(t, nil)
		t.Setenv("TERM_PROGRAM", "")
		if _, _, err := Choose(""); err == nil || !strings.Contains(err.Error(), "--vsix") {
			t.Errorf("err = %v", err)
		}
	})
}

func TestInstallRunsTheCLIWithArgv(t *testing.T) {
	fakeCLIs(t, map[string]string{"code": "/bin/code"})
	t.Setenv("TERM_PROGRAM", "")
	oldRun := runCLI
	t.Cleanup(func() { runCLI = oldRun })
	var gotBin string
	var gotArgs []string
	runCLI = func(bin string, args ...string) ([]byte, error) {
		gotBin, gotArgs = bin, args
		// The file has to be there, and be a zip, while the editor reads it.
		data, err := os.ReadFile(args[1])
		if err != nil {
			t.Errorf("the .vsix is not there when the CLI runs: %v", err)
			return nil, nil
		}
		if _, err := zip.NewReader(bytes.NewReader(data), int64(len(data))); err != nil {
			t.Errorf("the file is not a zip: %v", err)
		}
		return []byte("ok"), nil
	}
	vsix, err := VSIX(nil)
	if err != nil {
		t.Fatal(err)
	}
	e, err := Install(vsix, "")
	if err != nil || e.Name != "code" {
		t.Fatalf("Install = %v, %v", e.Name, err)
	}
	if gotBin != "/bin/code" || len(gotArgs) != 3 || gotArgs[0] != "--install-extension" ||
		!strings.HasSuffix(gotArgs[1], ".vsix") || gotArgs[2] != "--force" {
		t.Errorf("ran %s %q", gotBin, gotArgs)
	}
	if _, err := os.Stat(gotArgs[1]); err == nil {
		t.Error("the temporary .vsix is left behind")
	}
}

func TestInstallReportsTheCLIsOutput(t *testing.T) {
	fakeCLIs(t, map[string]string{"code": "/bin/code"})
	old := runCLI
	t.Cleanup(func() { runCLI = old })
	runCLI = func(string, ...string) ([]byte, error) {
		return []byte("Extension is not compatible"), io.ErrUnexpectedEOF
	}
	if _, err := Install([]byte("x"), "code"); err == nil || !strings.Contains(err.Error(), "not compatible") {
		t.Errorf("err = %v", err)
	}
}

func TestWriteIntoAFolderOrAFile(t *testing.T) {
	dir := t.TempDir()
	got, err := Write([]byte("x"), dir)
	if err != nil || filepath.Dir(got) != dir || !strings.HasSuffix(got, ".vsix") {
		t.Fatalf("Write(dir) = %q, %v", got, err)
	}
	file := filepath.Join(dir, "mine.vsix")
	if got, err := Write([]byte("x"), file); err != nil || got != file {
		t.Errorf("Write(file) = %q, %v", got, err)
	}
}
