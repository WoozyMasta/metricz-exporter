//go:build !windows

package service

import (
	"fmt"
	"os"
)

// IsRunningUnderWindowsService always returns false on non-Windows platforms.
func IsRunningUnderWindowsService() bool { return false }

// RunUnderWindowsService is unsupported on non-Windows platforms.
func RunUnderWindowsService(_ func()) {
	fmt.Fprintln(os.Stderr, "Windows SCM is not supported on this platform")
	os.Exit(1)
}
