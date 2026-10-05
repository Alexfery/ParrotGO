package espidf

import (
	"context"

	"parrot/internal/targets"
)

// MonitorOptions describes the session Monitor.Run opens.
type MonitorOptions struct {
	ProjectDir string
	Target     targets.Target

	// Port is the device's serial port, e.g. "COM5" or "/dev/ttyUSB0". It is
	// passed to idf.py as given. When empty, idf.py picks the port itself.
	Port string
}

// MonitorArgs returns the idf.py arguments that open IDF Monitor on the
// device, through opts.Port when it is set.
//
// The target is passed as for build and flash: idf.py monitor configures the
// project with CMake when it has never been built, and IDF Monitor reuses
// these arguments when it builds or flashes from its menu (Ctrl+T).
func MonitorArgs(opts MonitorOptions) []string {
	return deviceArgs(opts.Target, opts.Port, "monitor")
}

// Monitor opens IDF Monitor, ESP-IDF's serial monitor, through idf.py.
type Monitor struct {
	Runner CommandRunner
}

// Run hands the terminal to IDF Monitor and returns when the user leaves it
// (Ctrl+]). It neither builds nor flashes the project.
func (m Monitor) Run(ctx context.Context, opts MonitorOptions) error {
	return m.Runner.Run(ctx, RunOptions{Dir: opts.ProjectDir, Args: MonitorArgs(opts), Interactive: true})
}
