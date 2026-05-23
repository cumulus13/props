//go:build windows

// props - Show the Windows Properties dialog for any file, dir, drive, etc.
// Author  : Hadi Cahyadi <cumulus13@gmail.com>
// Homepage: https://github.com/cumulus13/props
// License : MIT
//
// The Properties dialog requires the calling process to stay alive.
// We run a Windows message loop so the process keeps running while
// the dialog is open, then exits cleanly when the dialog is closed.
// The terminal prompt returns immediately because we re-launch ourselves
// as a detached background process.

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"unsafe"
)

// ── Win32 types ──────────────────────────────────────────────────────────────

type shellExecuteInfo struct {
	cbSize         uint32
	fMask          uint32
	hwnd           uintptr
	lpVerb         *uint16
	lpFile         *uint16
	lpParameters   *uint16
	lpDirectory    *uint16
	nShow          int32
	hInstApp       uintptr
	lpIDList       uintptr
	lpClass        *uint16
	hkeyClass      uintptr
	dwHotKey       uint32
	hMonitor       uintptr
	hProcess       uintptr
}

type msg struct {
	hwnd    uintptr
	message uint32
	wParam  uintptr
	lParam  uintptr
	time    uint32
	pt      [2]int32
}

const (
	seeMaskInvokeIDList  uint32 = 0x0000000C
	seeMaskNoCloseProcess uint32 = 0x00000040
	swShow               int32  = 1
	wmQuit               uint32 = 0x0012
)

var (
	shell32         = syscall.NewLazyDLL("shell32.dll")
	user32          = syscall.NewLazyDLL("user32.dll")
	procShellExecEx = shell32.NewProc("ShellExecuteExW")
	procGetMessage  = user32.NewProc("GetMessageW")
	procDispatch    = user32.NewProc("DispatchMessageW")
	procTranslate   = user32.NewProc("TranslateMessage")
	procWaitForSingleObject = syscall.NewLazyDLL("kernel32.dll").NewProc("WaitForSingleObject")
	procCloseHandle         = syscall.NewLazyDLL("kernel32.dll").NewProc("CloseHandle")
)

// ── showProperties ────────────────────────────────────────────────────────────

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
		return fmt.Errorf("ShellExecuteEx failed: %w", lastErr)
	}

	// hProcess is set because of SEE_MASK_NOCLOSEPROCESS.
	// Wait for the Properties dialog process/thread to finish,
	// then close the handle.
	if sei.hProcess != 0 {
		procWaitForSingleObject.Call(sei.hProcess, 0xFFFFFFFF) // INFINITE
		procCloseHandle.Call(sei.hProcess)
	} else {
		// Fallback: pump a message loop until WM_QUIT
		// (handles cases where hProcess is not returned)
		var m msg
		for {
			r, _, _ := procGetMessage.Call(
				uintptr(unsafe.Pointer(&m)), 0, 0, 0,
			)
			if r == 0 || r == ^uintptr(0) { // 0=WM_QUIT, -1=error
				break
			}
			procTranslate.Call(uintptr(unsafe.Pointer(&m)))
			procDispatch.Call(uintptr(unsafe.Pointer(&m)))
		}
	}
	return nil
}

// ── relaunch as detached background process ───────────────────────────────────
// This gives the terminal its prompt back immediately while our process
// keeps running in the background waiting for the dialog to close.

func relaunchDetached() {
	// Build arg list with a sentinel so the child knows not to re-relaunch
	args := append([]string{"--worker"}, os.Args[1:]...)
	exe, _ := os.Executable()

	cmd := &syscall.StartupInfo{}
	pi := &syscall.ProcessInformation{}
	cmdLine, _ := syscall.UTF16PtrFromString(
		syscall.EscapeArg(exe) + " " + joinArgs(args),
	)
	exePtr, _ := syscall.UTF16PtrFromString(exe)

	syscall.CreateProcess(
		exePtr,
		cmdLine,
		nil, nil, false,
		// DETACHED_PROCESS | CREATE_NO_WINDOW
		0x00000008|0x08000000,
		nil, nil,
		cmd, pi,
	)
	// Parent exits immediately → terminal gets prompt back
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

// ── usage ─────────────────────────────────────────────────────────────────────

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

// ── main ──────────────────────────────────────────────────────────────────────

func main() {
	args := os.Args[1:]

	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" || args[0] == "/?" {
		usage()
		if len(args) == 0 {
			os.Exit(1)
		}
		os.Exit(0)
	}

	// Worker mode: we are the background process — open dialogs and wait.
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

	// Launcher mode: re-launch as detached worker, then exit immediately.
	relaunchDetached()
}
