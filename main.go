//go:build windows

// props - Show the Windows Properties dialog for any file, dir, drive, etc.
// Author  : Hadi Cahyadi <cumulus13@gmail.com>
// Homepage: https://github.com/cumulus13/props
// License : MIT

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"
	"unsafe"
)

type shellExecuteInfo struct {
	cbSize       uint32
	fMask        uint32
	hwnd         uintptr
	lpVerb       *uint16
	lpFile       *uint16
	lpParameters *uint16
	lpDirectory  *uint16
	nShow        int32
	hInstApp     uintptr
	lpIDList     uintptr
	lpClass      *uint16
	hkeyClass    uintptr
	dwHotKey     uint32
	hMonitor     uintptr
	hProcess     uintptr
}

type winMsg struct {
	hwnd    uintptr
	message uint32
	wParam  uintptr
	lParam  uintptr
	time    uint32
	pt      [2]int32
}

const (
	// This exact combination (INVOKEIDLIST | NOCLOSEPROCESS, lpFile set,
	// no lpIDList, no NOASYNC) is the one that actually showed the
	// Properties dialog.
	seeMaskInvokeIDList   uint32 = 0x0000000C
	seeMaskNoCloseProcess uint32 = 0x00000040
	swShow                int32  = 1
	pmRemove              uint32 = 0x0001
)

var (
	kernel32 = syscall.NewLazyDLL("kernel32.dll")
	shell32  = syscall.NewLazyDLL("shell32.dll")
	user32   = syscall.NewLazyDLL("user32.dll")

	procShellExecEx        = shell32.NewProc("ShellExecuteExW")
	procEnumWindows        = user32.NewProc("EnumWindows")
	procGetClassName       = user32.NewProc("GetClassNameW")
	procGetWindowThreadPid = user32.NewProc("GetWindowThreadProcessId")
	procIsWindowVisible    = user32.NewProc("IsWindowVisible")
	procIsWindow           = user32.NewProc("IsWindow")
	procPeekMessage        = user32.NewProc("PeekMessageW")
	procTranslateMessage   = user32.NewProc("TranslateMessage")
	procDispatchMessage    = user32.NewProc("DispatchMessageW")
	procGetCurrentProcId   = kernel32.NewProc("GetCurrentProcessId")
	procCloseHandle        = kernel32.NewProc("CloseHandle")
)

func pumpMessages() {
	var m winMsg
	for {
		ret, _, _ := procPeekMessage.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0, uintptr(pmRemove))
		if ret == 0 {
			break
		}
		_, _, _ = procTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
		_, _, _ = procDispatchMessage.Call(uintptr(unsafe.Pointer(&m)))
	}
}

// findOwnDialog returns the HWND of a visible "#32770" dialog window
// owned by the given PID, or 0 if none exists yet.
func findOwnDialog(pid uint32) uintptr {
	var found uintptr
	cb := syscall.NewCallback(func(hwnd uintptr, _ uintptr) uintptr {
		var winPid uint32
		_, _, _ = procGetWindowThreadPid.Call(hwnd, uintptr(unsafe.Pointer(&winPid)))
		if winPid != pid {
			return 1
		}
		visible, _, _ := procIsWindowVisible.Call(hwnd)
		if visible == 0 {
			return 1
		}
		buf := make([]uint16, 256)
		_, _, _ = procGetClassName.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
		if syscall.UTF16ToString(buf) == "#32770" {
			found = hwnd
			return 0
		}
		return 1
	})
	_, _, _ = procEnumWindows.Call(cb, 0)
	return found
}

func showProperties(target string) error {
	abs, err := filepath.Abs(target)
	if err != nil {
		return fmt.Errorf("cannot resolve path: %w", err)
	}

	verb, _ := syscall.UTF16PtrFromString("properties")
	file, _ := syscall.UTF16PtrFromString(abs)

	sei := shellExecuteInfo{
		fMask:  seeMaskInvokeIDList | seeMaskNoCloseProcess,
		lpVerb: verb,
		lpFile: file,
		nShow:  swShow,
	}
	sei.cbSize = uint32(unsafe.Sizeof(sei))

	r, _, lastErr := procShellExecEx.Call(uintptr(unsafe.Pointer(&sei)))
	if r == 0 {
		return fmt.Errorf("ShellExecuteEx failed (hInstApp=%d): %w", sei.hInstApp, lastErr)
	}

	// Close hProcess handle if we got one — we don't use it for waiting
	// since it doesn't correspond to the dialog's lifetime.
	if sei.hProcess != 0 {
		_, _, _ = procCloseHandle.Call(sei.hProcess)
	}

	pid, _, _ := procGetCurrentProcId.Call()

	// Poll for our own "#32770" dialog window for up to 5s, pumping
	// messages so it can actually be created.
	var dlg uintptr
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		pumpMessages()
		dlg = findOwnDialog(uint32(pid))
		if dlg != 0 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	if dlg == 0 {
		return nil
	}

	// Wait until the dialog window is closed.
	for {
		exists, _, _ := procIsWindow.Call(dlg)
		if exists == 0 {
			break
		}
		pumpMessages()
		time.Sleep(50 * time.Millisecond)
	}

	return nil
}

func relaunchDetached() {
	args := append([]string{"--worker"}, os.Args[1:]...)
	exe, _ := os.Executable()
	si := &syscall.StartupInfo{}
	pi := &syscall.ProcessInformation{}
	cmdLine, _ := syscall.UTF16PtrFromString(syscall.EscapeArg(exe) + " " + joinArgs(args))
	exePtr, _ := syscall.UTF16PtrFromString(exe)
	_ = syscall.CreateProcess(exePtr, cmdLine, nil, nil, false,
		0x00000008|0x08000000, nil, nil, si, pi)
}

func joinArgs(args []string) string {
	out := ""
	for i, a := range args {
		if i > 0 {
			out += " "
		}
		out += syscall.EscapeArg(a)
	}
	return out
}

func usage() {
	name := filepath.Base(os.Args[0])
	fmt.Fprintf(os.Stderr, "props  -  Windows Properties dialog from the CLI\n")
	fmt.Fprintf(os.Stderr, "Author : Hadi Cahyadi <cumulus13@gmail.com>\n")
	fmt.Fprintf(os.Stderr, "Home   : https://github.com/cumulus13/props\n\n")
	fmt.Fprintf(os.Stderr, "Usage  : %s <path> [path2 path3 ...]\n\n", name)
	fmt.Fprintf(os.Stderr, "Examples:\n")
	fmt.Fprintf(os.Stderr, "  %s C:\\Windows\n", name)
	fmt.Fprintf(os.Stderr, "  %s myfile.txt\n", name)
	fmt.Fprintf(os.Stderr, "  %s C:\\\n", name)
	fmt.Fprintf(os.Stderr, "  %s . ..\n", name)
}

func main() {
	args := os.Args[1:]

	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" || args[0] == "/?" {
		usage()
		if len(args) == 0 {
			os.Exit(1)
		}
		os.Exit(0)
	}

	if args[0] == "--worker" {
		args = args[1:]
		exitCode := 0
		for _, arg := range args {
			if err := showProperties(arg); err != nil {
				fmt.Fprintf(os.Stderr, "ERROR [%s]: %v\n", arg, err)
				exitCode = 1
			}
		}
		os.Exit(exitCode)
	}

	relaunchDetached()
}
