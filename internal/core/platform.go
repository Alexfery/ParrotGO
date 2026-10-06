// Package core is what Parrot does whatever the hardware: it finds the
// platform of a project and hands the project to it. It knows no SDK: ESP-IDF
// and idf.py belong to the ESP32 platform (internal/platforms/esp32), which
// core reaches through the Platform interface, as it will reach any other.
package core

import (
	"context"
	"io"

	"parrot/internal/hardware"
	"parrot/internal/project"
)

// Platform is an embedded ecosystem Parrot supports: the MCUs and boards it
// knows, and the SDK that creates, builds, flashes and monitors their
// projects. ESP32 with ESP-IDF is the first one.
//
// Commands get a project's platform from a Registry, by the ID its
// parrot.json records, and never run an SDK themselves.
type Platform interface {
	// ID names the platform in parrot.json and on the command line, e.g.
	// "esp32".
	ID() string

	// CreateProject generates a new project in a new directory called
	// opts.Name, relative to the current directory: its parrot.json, which
	// records the platform, and the files its SDK needs. It does not need
	// the SDK to be installed.
	CreateProject(ctx context.Context, opts CreateProjectOptions) (CreatedProject, error)

	// OpenProject checks that the project in dir, described by cfg, can be
	// handed to the platform's SDK, and returns it. Every problem it can find
	// before an SDK command runs is reported here: hardware the platform does
	// not know, missing project files, an SDK that is not installed. The
	// project's SDK commands use stdio.
	OpenProject(dir string, cfg project.Config, stdio IO) (Project, error)
}

// Project is a project its platform opened, ready for the platform's SDK.
type Project interface {
	// Hardware is what the project runs on.
	Hardware() Hardware

	// Build compiles the project.
	Build(ctx context.Context) error

	// Flash writes the project to the device on port; the platform may
	// build it first. An empty port lets the SDK find the device.
	Flash(ctx context.Context, port string) error

	// Monitor shows the serial output of the device on port until the user
	// leaves it. It neither builds nor flashes. An empty port lets the SDK
	// pick it.
	Monitor(ctx context.Context, port string) error
}

// Hardware is what a project runs on, as its platform resolved it from
// parrot.json.
type Hardware struct {
	MCU   hardware.MCU
	Board *hardware.Board // nil when the project names no board
}

// CreateProjectOptions describes the project Platform.CreateProject
// generates.
type CreateProjectOptions struct {
	Name string // the project's name, which is also its directory

	// Target is the ID of the MCU. When empty, it is the board's MCU, or the
	// platform's default MCU when there is no board either.
	Target string
	Board  string // the ID of the board; empty for none
}

// CreatedProject is what Platform.CreateProject generated.
type CreatedProject struct {
	Hardware Hardware
	Paths    []string // the directories and files created, in creation order
}

// IO is the terminal a platform's SDK commands use: the user's terminal,
// outside tests.
type IO struct {
	Stdin  io.Reader // read only by interactive commands, such as Monitor
	Stdout io.Writer
	Stderr io.Writer
}
