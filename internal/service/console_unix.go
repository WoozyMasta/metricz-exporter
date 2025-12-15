//go:build !windows

package service

// EnsureInteractiveConsoleAttached is a no-op on non-Windows platforms.
func EnsureInteractiveConsoleAttached() {}

// IsLegacyConHost is always false on non-Windows.
func IsLegacyConHost() bool {
	return false
}

// setConsoleTitleWin is a Windows-only primitive; keep stub to satisfy calls.
func setConsoleTitleWin(_ string) error {
	return nil
}
