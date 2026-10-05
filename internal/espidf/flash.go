package espidf

import (
	"context"

	"parrot/internal/targets"
)

// FlashOptions describes what Flasher.Flash writes to a device, and how.
type FlashOptions struct {
	ProjectDir string
	Target     targets.Target

	// Port is the device's serial port, e.g. "COM5" or "/dev/ttyUSB0". It is
	// passed to idf.py as given, without checking that it exists. When empty,
	// idf.py looks for the device itself.
	Port string
}

// FlashArgs returns the idf.py arguments that flash the project for
// opts.Target, through opts.Port when it is set.
func FlashArgs(opts FlashOptions) []string {
	return deviceArgs(opts.Target, opts.Port, "flash")
}

// deviceArgs returns the idf.py arguments that run action on a device: the
// target, the port when it is set (idf.py looks for the device otherwise),
// then the action.
func deviceArgs(target targets.Target, port, action string) []string {
	args := []string{targetArg(target)}
	if port != "" {
		args = append(args, "-p", port)
	}
	return append(args, action)
}

// Flasher flashes ESP-IDF projects to a device through idf.py.
type Flasher struct {
	Runner CommandRunner
}

// Flash writes the project in opts.ProjectDir to the device. idf.py builds
// the project first when it is out of date, so there is no separate build
// step. Check the project with CheckProject first.
func (f Flasher) Flash(ctx context.Context, opts FlashOptions) error {
	return f.Runner.Run(ctx, RunOptions{Dir: opts.ProjectDir, Args: FlashArgs(opts)})
}
