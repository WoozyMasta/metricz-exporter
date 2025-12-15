// Package service provides Windows service/console helpers.
package service

import (
	"fmt"
	"os"
	"strings"

	"golang.org/x/term"
)

// SetTerminalTitle sets terminal title across platforms.
// - On legacy Windows conhost (no VT): use WinAPI title.
// - Otherwise (modern terminals): send ANSI OSC escape to TTY stdout.
func SetTerminalTitle(title string) {
	if IsLegacyConHost() {
		_ = setConsoleTitleWin(title) //nolint:errcheck // best-effort on legacy conhost
		return
	}

	// Avoid polluting redirected output
	if term.IsTerminal(int(os.Stdout.Fd())) {
		fmt.Printf("\033]0;%s\a", title)
	}
}

// CanUseANSIColors decides whether ANSI colors should be enabled for the given file.
// Rules:
//  1. FORCE_COLOR=true → enable.
//  2. Legacy conhost → disable.
//  3. NO_COLOR set → disable. (https://no-color.org/)
//  4. Writer must be a TTY.
//  5. COLORTERM set → enable.
//  6. TERM=dumb → disable.
//  7. Common color-capable TERM substrings → enable.
//  8. Otherwise → disable.
func CanUseANSIColors(f *os.File) bool {
	if os.Getenv("FORCE_COLOR") == "true" {
		return true
	}

	if IsLegacyConHost() {
		return false
	}

	if os.Getenv("NO_COLOR") != "" {
		return false
	}

	if !term.IsTerminal(int(f.Fd())) {
		return false
	}

	if ct := strings.ToLower(os.Getenv("COLORTERM")); ct != "" {
		return true
	}

	termEnv := strings.ToLower(os.Getenv("TERM"))
	if termEnv == "dumb" {
		return false
	}

	if strings.Contains(termEnv, "xterm") ||
		strings.Contains(termEnv, "screen") ||
		strings.Contains(termEnv, "vt100") ||
		strings.Contains(termEnv, "color") {
		return true
	}

	return false
}
