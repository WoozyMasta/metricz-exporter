//go:build windows

package service

import (
	"fmt"
	"os"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	k32                  = windows.NewLazySystemDLL("kernel32.dll")
	procGetConsoleWindow = k32.NewProc("GetConsoleWindow")
	procAttachConsole    = k32.NewProc("AttachConsole")
	procAllocConsole     = k32.NewProc("AllocConsole")
	procSetConsoleTitleW = k32.NewProc("SetConsoleTitleW")
)

const attachParentProcess = 0xFFFFFFFF

// EnsureInteractiveConsoleAttached attaches to a parent console or allocates a new one
// (for Explorer double-click scenarios). It rebinds STDIN/STDOUT/STDERR to the console.
func EnsureInteractiveConsoleAttached() {
	// already has a console?
	if getConsoleWindow() != 0 {
		return
	}

	// if stdout/stderr already point to a device/pipe, do nothing
	if stdioUsable() {
		return
	}

	// attach or allocate
	if err := attachConsole(attachParentProcess); err != nil {
		if err2 := allocConsole(); err2 != nil {
			return
		}
	}

	// rebind stdio (best effort)
	if f, err := os.OpenFile("CONOUT$", os.O_RDWR, 0); err == nil {
		os.Stdout = f
		os.Stderr = f
	}

	if f, err := os.OpenFile("CONIN$", os.O_RDWR, 0); err == nil {
		os.Stdin = f
	}
}

// IsLegacyConHost reports classic conhost without VT (ANSI) support.
// True → prefer WinAPI-only features (no ANSI).
func IsLegacyConHost() bool {
	if !stdioToConsole() {
		return false
	}

	return !vtEnabled()
}

// setConsoleTitleWin sets the console title via SetConsoleTitleW.
// Returns nil on success. On failure uses GetLastError value from the call.
func setConsoleTitleWin(title string) error {
	p, err := windows.UTF16PtrFromString(title)
	if err != nil {
		return err
	}

	r1, _, lastErr := procSetConsoleTitleW.Call(uintptr(unsafe.Pointer(p)))
	if r1 == 0 { // BOOL FALSE → failure
		// lastErr can be ERROR_SUCCESS (0), which is not nil as an error value.
		if lastErr != nil && lastErr != windows.ERROR_SUCCESS {
			return lastErr
		}
		return fmt.Errorf("SetConsoleTitleW failed")
	}

	return nil
}

func getConsoleWindow() uintptr {
	r1, _, _ := procGetConsoleWindow.Call()

	return r1
}

func attachConsole(pid uint32) error {
	r1, _, e1 := procAttachConsole.Call(uintptr(pid))
	if r1 == 0 {
		if e1 != nil {
			return e1
		}
		return fmt.Errorf("AttachConsole failed")
	}
	return nil
}

func allocConsole() error {
	r1, _, e1 := procAllocConsole.Call()
	if r1 == 0 {
		if e1 != nil {
			return e1
		}
		return fmt.Errorf("AllocConsole failed")
	}
	return nil
}

func stdioUsable() bool {
	// STDOUT
	if h, err := windows.GetStdHandle(windows.STD_OUTPUT_HANDLE); err == nil && h != 0 {
		if ft, err2 := windows.GetFileType(h); err2 == nil {
			if ft == windows.FILE_TYPE_CHAR {
				return true
			}
		}
	}

	// STDERR
	if h, err := windows.GetStdHandle(windows.STD_ERROR_HANDLE); err == nil && h != 0 {
		if ft, err2 := windows.GetFileType(h); err2 == nil {
			if ft == windows.FILE_TYPE_CHAR {
				return true
			}
		}
	}

	return false
}

func stdioToConsole() bool {
	h, err := windows.GetStdHandle(windows.STD_OUTPUT_HANDLE)
	if err != nil || h == 0 {
		return false
	}

	ft, err := windows.GetFileType(h)
	if err != nil {
		return false
	}

	// FILE_TYPE_CHAR → console
	return ft == windows.FILE_TYPE_CHAR
}

func vtEnabled() bool {
	h, err := windows.GetStdHandle(windows.STD_OUTPUT_HANDLE)
	if err != nil || h == 0 {
		return false
	}

	var mode uint32
	if err := windows.GetConsoleMode(h, &mode); err != nil {
		return false
	}

	return mode&0x0004 != 0
}
