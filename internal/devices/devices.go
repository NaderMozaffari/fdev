// Package devices lists the devices `flutter devices --machine` reports.
package devices

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

type Device struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	TargetPlatform string `json:"targetPlatform"`
	Emulator       bool   `json:"emulator"`
	SDK            string `json:"sdk"`
	IsSupported    *bool  `json:"isSupported"`
}

// Desc is the second column of the device picker.
func (d Device) Desc() string {
	kind := "device"
	if d.Emulator {
		kind = "emulator"
	}
	parts := []string{}
	for _, p := range []string{d.SDK, kind, d.ID} {
		if p != "" {
			parts = append(parts, p)
		}
	}
	return strings.Join(parts, " · ")
}

// List lists the devices of platform ("android", "ios", "web", "darwin",
// ... or "" for all). Android devices come from adb when it is installed,
// which answers at once where `flutter devices` takes seconds; the rest
// from `flutter devices --machine`, run in dir.
func List(dir, platform string) ([]Device, error) {
	if platform == "android" {
		if list, ok := adbDevices(); ok {
			return list, nil
		}
	}
	return flutterDevices(dir, platform)
}

func flutterDevices(dir, platform string) ([]Device, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "flutter", "devices", "--machine")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil && len(out) == 0 {
		return nil, err
	}
	// flutter may print other text (e.g. "Waiting for another flutter
	// command...") before the JSON.
	start := strings.IndexByte(string(out), '[')
	if start < 0 {
		return nil, nil
	}
	var all []Device
	if err := json.Unmarshal(out[start:], &all); err != nil {
		return nil, err
	}
	var matching []Device
	for _, d := range all {
		if d.IsSupported != nil && !*d.IsSupported {
			continue
		}
		if platform == "" || strings.HasPrefix(d.TargetPlatform, platform) {
			matching = append(matching, d)
		}
	}
	return matching, nil
}

// adbDevices is `adb devices -l`; false when adb is missing or fails.
func adbDevices() ([]Device, bool) {
	adb := adbPath()
	if adb == "" {
		return nil, false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, adb, "devices", "-l").Output()
	if err != nil {
		return nil, false
	}
	return parseADB(string(out)), true
}

// parseADB reads `adb devices -l`: the ready devices (not offline or
// unauthorized), with the serial as the id, as flutter has it.
func parseADB(out string) []Device {
	var list []Device
	for _, line := range strings.Split(out, "\n") {
		f := strings.Fields(line)
		if len(f) < 2 || f[1] != "device" { // the header, daemon notes, offline, unauthorized
			continue
		}
		d := Device{ID: f[0], TargetPlatform: "android", Emulator: strings.HasPrefix(f[0], "emulator-")}
		for _, kv := range f[2:] {
			if v, ok := strings.CutPrefix(kv, "model:"); ok {
				d.Name = strings.ReplaceAll(v, "_", " ")
			}
		}
		if d.Name == "" {
			d.Name = d.ID
		}
		list = append(list, d)
	}
	return list
}

// ADB is adb's path: on PATH, else in the usual Android SDK places; "" when
// it isn't installed.
func ADB() string { return adbPath() }

func adbPath() string {
	if p, err := exec.LookPath("adb"); err == nil {
		return p
	}
	var sdks []string
	for _, env := range []string{"ANDROID_HOME", "ANDROID_SDK_ROOT"} {
		if v := os.Getenv(env); v != "" {
			sdks = append(sdks, v)
		}
	}
	if home, err := os.UserHomeDir(); err == nil {
		sdks = append(sdks, filepath.Join(home, "Library", "Android", "sdk"),
			filepath.Join(home, "Android", "Sdk"), filepath.Join(home, "AppData", "Local", "Android", "Sdk"))
	}
	name := "adb"
	if runtime.GOOS == "windows" {
		name = "adb.exe"
	}
	for _, sdk := range sdks {
		if p := filepath.Join(sdk, "platform-tools", name); fileExists(p) {
			return p
		}
	}
	return ""
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
