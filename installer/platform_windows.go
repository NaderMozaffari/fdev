package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"

	"github.com/NaderMozaffari/fdev/internal/update"
)

func destinations() ([]Dest, error) {
	user, err := update.InstallDir()
	if err != nil {
		return nil, err
	}
	pf := os.Getenv("ProgramFiles")
	if pf == "" {
		pf = `C:\Program Files`
	}
	all := filepath.Join(pf, "fdev")
	return []Dest{
		{ID: "user", Path: user, Shown: `%LOCALAPPDATA%\Programs\fdev`, Note: "just for me · no admin"},
		{ID: "all", Path: all, Shown: all, Note: "everyone · needs admin", Admin: true},
	}, nil
}

// elevatedFlag starts the installer as the elevated helper that installs
// for everyone: --elevated-install <binary> <folder> <1 to add it to the PATH>.
const elevatedFlag = "--elevated-install"

// The helper's exit codes.
const (
	exitCopy = 2
	exitPath = 3
)

// installAdmin asks Windows for admin rights (the UAC prompt) and runs a
// second copy of the installer with them, which copies bin into dir and,
// when addPath, puts dir on the PATH for everyone.
func installAdmin(bin []byte, dir string, addPath bool) error {
	tmp, err := os.CreateTemp("", "fdev-*.exe")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(bin); err != nil {
		tmp.Close()
		return err
	}
	tmp.Close()
	self, err := os.Executable()
	if err != nil {
		return err
	}
	flag := "0"
	if addPath {
		flag = "1"
	}
	code, err := runAsAdmin(self, []string{elevatedFlag, tmp.Name(), dir, flag})
	switch {
	case errors.Is(err, windows.ERROR_CANCELLED):
		return errors.New("admin rights were refused, so nothing was installed")
	case err != nil:
		return err
	case code == exitCopy:
		return fmt.Errorf("couldn't copy fdev into %s", dir)
	case code == exitPath:
		return fmt.Errorf("installed in %s, but couldn't add it to the PATH", dir)
	case code != 0:
		return fmt.Errorf("the admin install stopped (code %d)", code)
	}
	return nil
}

func runElevated(args []string) (int, bool) {
	if len(args) != 4 || args[0] != elevatedFlag {
		return 0, false
	}
	bin, err := os.ReadFile(args[1])
	if err != nil {
		return exitCopy, true
	}
	dir := args[2]
	if os.MkdirAll(dir, 0o755) != nil || update.Replace(filepath.Join(dir, "fdev.exe"), bin) != nil {
		return exitCopy, true
	}
	if args[3] == "1" {
		if _, err := addToPath(registry.LOCAL_MACHINE, `SYSTEM\CurrentControlSet\Control\Session Manager\Environment`, dir); err != nil {
			return exitPath, true
		}
	}
	return 0, true
}

// onPath puts d on the user's PATH. Installing for everyone did it
// already, for the whole PC.
func onPath(d Dest) ([]string, error) {
	if d.Admin {
		return []string{"for everyone on this PC"}, nil
	}
	added, err := addToPath(registry.CURRENT_USER, "Environment", d.Path)
	if !added {
		return nil, err
	}
	return []string{"for your account"}, err
}

// addToPath adds dir to the Path in the registry key, unless it is there,
// and tells running programs (Explorer, so new terminals) that it changed.
func addToPath(root registry.Key, key, dir string) (bool, error) {
	k, err := registry.OpenKey(root, key, registry.QUERY_VALUE|registry.SET_VALUE)
	if err != nil {
		return false, err
	}
	defer k.Close()
	cur, _, err := k.GetStringValue("Path")
	if err != nil && !errors.Is(err, registry.ErrNotExist) {
		return false, err
	}
	for _, p := range strings.Split(cur, ";") {
		if strings.EqualFold(strings.TrimRight(p, `\`), strings.TrimRight(dir, `\`)) {
			return false, nil
		}
	}
	if cur != "" && !strings.HasSuffix(cur, ";") {
		cur += ";"
	}
	if err := k.SetExpandStringValue("Path", cur+dir); err != nil {
		return false, err
	}
	env, _ := windows.UTF16PtrFromString("Environment")
	var result uintptr
	// SendMessageTimeoutW(HWND_BROADCAST, WM_SETTINGCHANGE, 0, "Environment", SMTO_ABORTIFHUNG, 5s, &result)
	_, _, _ = sendMessageTimeout.Call(0xffff, 0x1a, 0, uintptr(unsafe.Pointer(env)), 2, 5000, uintptr(unsafe.Pointer(&result)))
	return true, nil
}

// tilde is for macOS, which writes paths in the home folder as ~/...;
// Windows paths stay as they are.
func tilde(p string) string { return p }

func openTerminal() error {
	home, _ := os.UserHomeDir()
	if wt, err := exec.LookPath("wt.exe"); err == nil {
		return exec.Command(wt, "-d", home).Start()
	}
	cmd := exec.Command("cmd", "/c", "start", "", "/D", home, "powershell")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	return cmd.Start()
}

var (
	user32             = windows.NewLazySystemDLL("user32.dll")
	shell32            = windows.NewLazySystemDLL("shell32.dll")
	sendMessageTimeout = user32.NewProc("SendMessageTimeoutW")
	shellExecuteEx     = shell32.NewProc("ShellExecuteExW")
)

// shellExecuteInfo is Windows' SHELLEXECUTEINFOW.
type shellExecuteInfo struct {
	size       uint32
	mask       uint32
	hwnd       windows.Handle
	verb       *uint16
	file       *uint16
	parameters *uint16
	directory  *uint16
	show       int32
	instApp    windows.Handle
	idList     uintptr
	class      *uint16
	keyClass   windows.Handle
	hotKey     uint32
	icon       windows.Handle
	process    windows.Handle
}

// runAsAdmin runs exe with args after the UAC prompt, waits for it and gives
// its exit code. A refused prompt is windows.ERROR_CANCELLED.
func runAsAdmin(exe string, args []string) (uint32, error) {
	verb, _ := windows.UTF16PtrFromString("runas")
	file, _ := windows.UTF16PtrFromString(exe)
	params, _ := windows.UTF16PtrFromString(windows.ComposeCommandLine(args))
	info := shellExecuteInfo{verb: verb, file: file, parameters: params, mask: 0x40 /* SEE_MASK_NOCLOSEPROCESS */}
	info.size = uint32(unsafe.Sizeof(info))
	if ok, _, err := shellExecuteEx.Call(uintptr(unsafe.Pointer(&info))); ok == 0 {
		return 0, err
	}
	defer windows.CloseHandle(info.process)
	if _, err := windows.WaitForSingleObject(info.process, windows.INFINITE); err != nil {
		return 0, err
	}
	var code uint32
	err := windows.GetExitCodeProcess(info.process, &code)
	return code, err
}
