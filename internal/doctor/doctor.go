// Package doctor diagnoses the machine and the current project before Parrot
// hands them to ESP-IDF. It only looks: it runs read-only commands (versions,
// ESP-IDF's tool check) and never installs, activates or changes anything.
// The checks return results; presenting them is up to the caller.
package doctor

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"parrot/internal/espidf"
)

// System is what the checks need from the machine. Host is the real one;
// tests replace it. Files are read directly from the file system.
type System interface {
	FindIDF() (string, error) // idf.py on PATH
	LookPath(name string) (string, error)
	Getenv(key string) string
	// Capture runs a short read-only command and returns its output.
	Capture(ctx context.Context, name string, args ...string) (string, error)
}

// Host is the System of the machine Parrot runs on.
type Host struct{}

func (Host) FindIDF() (string, error)             { return espidf.FindIDF() }
func (Host) LookPath(name string) (string, error) { return exec.LookPath(name) }
func (Host) Getenv(key string) string             { return os.Getenv(key) }
func (Host) Capture(ctx context.Context, name string, args ...string) (string, error) {
	return espidf.Capture(ctx, name, args...)
}

// Default time limits of the commands the checks run, so that a stuck tool
// cannot hang Parrot. idf_tools.py check runs the version command of every
// ESP-IDF tool: about 10 s, but close to a minute on a cold start on Windows.
const (
	DefaultCommandTimeout = 20 * time.Second
	DefaultToolsTimeout   = 2 * time.Minute
)

// Doctor runs the checks.
type Doctor struct {
	System     System
	ProjectDir string // where to look for a Parrot project

	// Time limits of the commands; zero means the defaults above.
	CommandTimeout time.Duration // version commands
	ToolsTimeout   time.Duration // idf_tools.py check
}

// Run runs every check and returns the results. A failed check does not stop
// the others: only the checks that need its result are skipped.
func (d Doctor) Run(ctx context.Context) Report {
	return Report{Sections: []Section{
		{Name: "Environment", Results: d.environment(ctx)},
		{Name: "Project", Results: d.project()},
	}}
}

func (d Doctor) commandTimeout() time.Duration {
	if d.CommandTimeout > 0 {
		return d.CommandTimeout
	}
	return DefaultCommandTimeout
}

func (d Doctor) toolsTimeout() time.Duration {
	if d.ToolsTimeout > 0 {
		return d.ToolsTimeout
	}
	return DefaultToolsTimeout
}

// capture runs name with args within limit. Its error is meant for the
// report: it names the command and ends with what the command printed about
// the failure.
func (d Doctor) capture(ctx context.Context, limit time.Duration, name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, limit)
	defer cancel()
	out, err := d.System.Capture(ctx, name, args...)
	if err == nil {
		return out, nil
	}
	command := commandLine(name, args)
	switch {
	case errors.Is(ctx.Err(), context.DeadlineExceeded):
		return out, fmt.Errorf("%s did not finish within %v.", command, limit)
	case ctx.Err() != nil:
		return out, fmt.Errorf("%s was interrupted.", command)
	}
	message := fmt.Sprintf("%s failed (%v).", command, err)
	if summary := outputSummary(out); summary != "" {
		message += "\n" + summary
	}
	return out, errors.New(message)
}

// version runs `executable --version` and returns the first line it printed.
func (d Doctor) version(ctx context.Context, executable string) (string, error) {
	out, err := d.capture(ctx, d.commandTimeout(), executable, "--version")
	if err != nil {
		return "", err
	}
	if lines := nonEmptyLines(out); len(lines) > 0 {
		return lines[0], nil
	}
	return "", fmt.Errorf("%s printed no version.", commandLine(executable, []string{"--version"}))
}

// commandLine shortens a command for the report: paths are reduced to their
// last element, e.g. "python.exe idf_tools.py check".
func commandLine(name string, args []string) string {
	parts := []string{filepath.Base(name)}
	for _, arg := range args {
		parts = append(parts, filepath.Base(arg))
	}
	return strings.Join(parts, " ")
}

// maxSummaryLines bounds how much of a failed command's output the report shows.
const maxSummaryLines = 5

// outputSummary returns the lines of a failed command's output that explain
// the failure: the ones ESP-IDF's tools mark as errors or warnings if any,
// otherwise the last lines, where errors usually are.
func outputSummary(out string) string {
	lines := nonEmptyLines(out)
	var marked []string
	for _, line := range lines {
		if strings.HasPrefix(line, "ERROR") || strings.HasPrefix(line, "FATAL") || strings.HasPrefix(line, "WARNING") {
			marked = append(marked, line)
		}
	}
	if len(marked) > 0 {
		lines = marked
	}
	if len(lines) > maxSummaryLines {
		lines = lines[len(lines)-maxSummaryLines:]
	}
	return strings.Join(lines, "\n")
}

func nonEmptyLines(out string) []string {
	var lines []string
	for _, line := range strings.Split(out, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}
