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
	seeMaskInvokeIDList uint32 = 0x0000000C
	swShow              int32  = 1
	pmNoRemove          uint32 = 0x0000
)

var (
	shell32        = syscall.NewLazyDLL("shell32.dll")
	user32         = syscall.NewLazyDLL("user32.dll")
	procShellExecEx = shell32.NewProc("ShellExecuteExW")
	procPeekMessage = user32.NewProc("PeekMessageW")
	procDispatch    = user32.NewProc("DispatchMessageW")
	procTranslate   = user32.NewProc("TranslateMessage")
	procIsWindow    = user32.NewProc("IsWindow")
	procFindWindow  = user32.NewProc("FindWindowW")
)

func showProperties(target string) error {
	abs, err := filepath.Abs(target)
	if err != nil {
		return fmt.Errorf("cannot resolve path: %w", err)
	}

	verb, _ := syscall.UTF16PtrFromString("properties")
	file, _ := syscall.UTF16PtrFromString(abs)

	sei := shellExecuteInfo{
		fMask:  seeMaskInvokeIDList,
		lpVerb: verb,
		lpFile: file,
		nShow:  swShow,
	}
	sei.cbSize = uint32(unsafe.Sizeof(sei))

	r, _, lastErr := procShellExecEx.Call(uintptr(unsafe.Pointer(&sei)))
	if r == 0 {
		return fmt.Errorf("ShellExecuteEx failed: %w", lastErr)
	}

	// Pump messages briefly to let the shell open the dialog,
	// then keep pumping while the properties window is alive.
	// We detect the dialog by watching for the shell's property sheet
	// class name "#32770" (standard dialog box).
	var m winMsg

	// Give shell time to create the window
	time.Sleep(500 * time.Millisecond)

	// Find the properties dialog window — class "#32770"
	cls, _ := syscall.UTF16PtrFromString("#32770")
	hwnd, _, _ := procFindWindow.Call(uintptr(unsafe.Pointer(cls)), 0)

	if hwnd == 0 {
		// Fallback: pump for 30 seconds max
		deadline := time.Now().Add(30 * time.Second)
		for time.Now().Before(deadline) {
			ret, _, _ := procPeekMessage.Call(
				uintptr(unsafe.Pointer(&m)), 0, 0, 0, 1,
			)
			if ret != 0 {
				_, _, _ = procTranslate.Call(uintptr(unsafe.Pointer(&m)))
				_, _, _ = procDispatch.Call(uintptr(unsafe.Pointer(&m)))
			}
			time.Sleep(100 * time.Millisecond)
		}
		return nil
	}

	// Wait until that window is destroyed
	for {
		r, _, _ := procIsWindow.Call(hwnd)
		if r == 0 {
			break
		}
		ret, _, _ := procPeekMessage.Call(
			uintptr(unsafe.Pointer(&m)), 0, 0, 0, 1,
		)
		if ret != 0 {
			_, _, _ = procTranslate.Call(uintptr(unsafe.Pointer(&m)))
			_, _, _ = procDispatch.Call(uintptr(unsafe.Pointer(&m)))
		}
		time.Sleep(100 * time.Millisecond)
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
