// Package espidf isolates every interaction with the ESP-IDF toolchain: it
// is the SDK layer of the ESP32 platform (internal/platforms/esp32): build,
// flash and monitor reach it only through that platform, and parrot doctor
// uses it to check the environment. Parrot drives ESP-IDF
// through idf.py only; it never calls CMake, Ninja or the compilers directly,
// and it never installs anything.
package espidf

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
)

// RunOptions describes one run of idf.py.
type RunOptions struct {
	Dir  string   // working directory: the project
	Args []string // idf.py arguments, one per element

	// Interactive hands the terminal to idf.py, for sessions such as IDF
	// Monitor: idf.py reads the user's keys from Runner.Stdin, and Ctrl+C is
	// left to idf.py instead of stopping it. See Runner.Run.
	Interactive bool
}

// CommandRunner runs idf.py. Builder, Flasher and Monitor depend on this
// interface so tests can check the command without an ESP-IDF installation
// or a device.
type CommandRunner interface {
	Run(ctx context.Context, opts RunOptions) error
}

// Runner runs idf.py as a child process. Its output is streamed to Stdout and
// Stderr while it runs, not collected. Only interactive runs read Stdin.
type Runner struct {
	IDFPath string // idf.py executable, as returned by FindIDF
	Stdin   io.Reader
	Stdout  io.Writer
	Stderr  io.Writer
}

// NewRunner finds idf.py and returns a Runner using stdin, stdout and stderr.
func NewRunner(stdin io.Reader, stdout, stderr io.Writer) (*Runner, error) {
	path, err := FindIDF()
	if err != nil {
		return nil, err
	}
	return &Runner{IDFPath: path, Stdin: stdin, Stdout: stdout, Stderr: stderr}, nil
}

// Run starts idf.py with opts.Args in opts.Dir, without a shell, and waits
// for it. A non-zero exit status is returned as an error.
//
// Cancelling ctx (e.g. on Ctrl+C) kills a non-interactive run. An interactive
// one owns the terminal and gets Ctrl+C from it directly, so it is left to
// handle it: IDF Monitor sends Ctrl+C to the device and keeps running until
// the user leaves it with Ctrl+], and idf.py ignores SIGINT meanwhile.
//
// A non-interactive run gets no Stdin and reads from the null device: build
// and flash need no input.
func (r *Runner) Run(ctx context.Context, opts RunOptions) error {
	if opts.Interactive {
		ctx = context.WithoutCancel(ctx)
	}
	cmd := exec.CommandContext(ctx, r.IDFPath, opts.Args...)
	cmd.Dir = opts.Dir
	// When these are the terminal's *os.File, idf.py inherits them directly:
	// it keeps its colors and progress output, and an interactive run reads
	// the keys itself, so Parrot never sees or consumes them.
	if opts.Interactive {
		cmd.Stdin = r.Stdin
	}
	cmd.Stdout = r.Stdout
	cmd.Stderr = r.Stderr

	err := cmd.Run()
	if ctx.Err() != nil {
		return fmt.Errorf("idf.py %s interrupted", strings.Join(opts.Args, " "))
	}
	if err != nil {
		return fmt.Errorf("idf.py %s: %w", strings.Join(opts.Args, " "), err)
	}
	return nil
}
