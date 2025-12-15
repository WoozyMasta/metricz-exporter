//go:build windows

package service

import (
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/windows/svc"
)

// serviceHandler is an SCM handler that starts the app in a goroutine and
// acknowledges Stop/Shutdown. The app must handle its own graceful shutdown.
type serviceHandler struct{ run func() }

// Execute implements svc.Handler. It starts run() and reports Running.
// On Stop/Shutdown it reports StopPending and returns.
func (h *serviceHandler) Execute(_ []string, r <-chan svc.ChangeRequest, s chan<- svc.Status) (bool, uint32) {
	s <- svc.Status{State: svc.StartPending}
	go h.run()
	s <- svc.Status{State: svc.Running, Accepts: svc.AcceptStop | svc.AcceptShutdown}

	for c := range r {
		switch c.Cmd {
		case svc.Interrogate:
			s <- svc.Status{State: svc.Running, Accepts: svc.AcceptStop | svc.AcceptShutdown}

		case svc.Stop, svc.Shutdown:
			s <- svc.Status{State: svc.StopPending}
			return false, 0
		}
	}

	return false, 0
}

// IsRunningUnderWindowsService reports whether the current process is managed by
// the Windows Service Control Manager. It is safe to call on Windows only.
func IsRunningUnderWindowsService() bool {
	ok, _ := svc.IsWindowsService()

	return ok
}

func defaultServiceName() string {
	exe, err := os.Executable()
	if err != nil {
		return "beyond-bounds"
	}

	return filepath.Base(exe)
}

// RunUnderWindowsService runs the provided function under SCM.
// The function must block until the app is done.
func RunUnderWindowsService(run func()) {
	if err := svc.Run(defaultServiceName(), &serviceHandler{run: run}); err != nil {
		fmt.Fprintf(os.Stderr, "Service failed: %v\n", err)
		os.Exit(1)
	}
}
