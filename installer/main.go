// Command installer is fdev's installer for macOS and Windows: a small
// window that installs fdev, the stable or the beta channel, in a folder and
// on the PATH, and saves the theme picked for fdev. It carries its release's
// fdev, so it needs no internet; online, it downloads a newer release of the
// channel instead, so running it again updates fdev. The screens are
// frontend/index.html, drawn in a webview by Wails; scripts/installer.sh
// builds the .dmg and the .exe.
//
// It is a module of its own so that fdev itself doesn't depend on Wails.
package main

import (
	"embed"
	"io/fs"
	"os"
	"runtime"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed all:frontend
var frontend embed.FS

// bundled is the fdev release archive for the OS and CPU the installer is
// built for, which scripts/installer.sh puts in bundle/ before building.
//
//go:embed all:bundle
var bundled embed.FS

// version is the release the installer installs, and repo where it is;
// scripts/installer.sh sets both with -ldflags. A build without a version
// installs the newest beta.
var (
	version = ""
	repo    = "NaderMozaffari/fdev"
)

func main() {
	if code, ok := runElevated(os.Args[1:]); ok {
		os.Exit(code)
	}
	assets, _ := fs.Sub(frontend, "frontend")
	app := newApp()
	err := wails.Run(&options.App{
		Title:            "Install fdev",
		Width:            880,
		Height:           560,
		DisableResize:    true,
		Frameless:        runtime.GOOS == "windows", // the page draws its own title bar there
		BackgroundColour: &options.RGBA{R: 0x16, G: 0x16, B: 0x1E, A: 255},
		AssetServer:      &assetserver.Options{Assets: assets},
		OnStartup:        app.startup,
		Bind:             []any{app},
		Mac: &mac.Options{
			TitleBar:   mac.TitleBarHiddenInset(),
			Appearance: mac.NSAppearanceNameDarkAqua,
		},
		Windows: &windows.Options{Theme: windows.Dark},
	})
	if err != nil {
		println("fdev installer:", err.Error())
		os.Exit(1)
	}
}
