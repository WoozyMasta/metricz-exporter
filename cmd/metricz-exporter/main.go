// Package main is the MetricZ Exporter entrypoint.
package main

import (
	"fmt"
	"os"

	"github.com/woozymasta/metricz-exporter/internal/entrypoint"
	"github.com/woozymasta/metricz-exporter/internal/service"
	"github.com/woozymasta/metricz-exporter/internal/vars"
)

func main() {
	// Windows Service mode: run CLI inside service handler (NO os.Exit here)
	if service.IsRunningUnderWindowsService() {
		service.RunUnderWindowsService(func() {
			_ = entrypoint.Execute() // do not os.Exit() inside the service goroutine
		})
		return
	}

	// Double-click (Explorer) / no args -> ensure console
	if len(os.Args) == 1 {
		service.EnsureInteractiveConsoleAttached()
	}

	service.SetTerminalTitle(fmt.Sprintf("%s %s [%s.%d]",
		vars.Name, vars.CommitShort(), vars.Version, vars.Revision))

	// Normal CLI execution path
	os.Exit(entrypoint.Execute())
}
