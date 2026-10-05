package espidf_test

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"parrot/internal/espidf"
)

// fakeCommand returns the path of the test binary, which acts as a fake
// command (see TestMain).
func fakeCommand(t *testing.T) string {
	t.Helper()
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("PARROT_FAKE_IDF", "1")
	return self
}

func TestCaptureReturnsStdoutAndStderr(t *testing.T) {
	out, err := espidf.Capture(context.Background(), fakeCommand(t), "--version")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"args=--version", "warning from fake idf.py"} {
		if !strings.Contains(out, want) {
			t.Errorf("output %q does not contain %q", out, want)
		}
	}
}

func TestCaptureReturnsExitStatusWithOutput(t *testing.T) {
	command := fakeCommand(t)
	t.Setenv("PARROT_FAKE_IDF_EXIT", "3")
	out, err := espidf.Capture(context.Background(), command, "check")
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 3 {
		t.Errorf("Capture error = %v, want exit status 3", err)
	}
	if !strings.Contains(out, "args=check") {
		t.Errorf("output %q, want what the command printed before failing", out)
	}
}

func TestCaptureStopsAtDeadline(t *testing.T) {
	command := fakeCommand(t)
	t.Setenv("PARROT_FAKE_IDF_SLEEP", "1m")
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := espidf.Capture(ctx, command, "--version")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Capture error = %v, want the deadline", err)
	}
	if elapsed := time.Since(start); elapsed > 20*time.Second {
		t.Errorf("Capture took %v after the deadline", elapsed)
	}
}

func TestVersionAndToolsCheckArgs(t *testing.T) {
	idfPath := filepath.Join("esp", "esp-idf")
	tools := filepath.Join(idfPath, "tools")
	if got, want := espidf.VersionArgs(idfPath), []string{filepath.Join(tools, "idf.py"), "--version"}; !slices.Equal(got, want) {
		t.Errorf("VersionArgs = %q, want %q", got, want)
	}
	if got, want := espidf.ToolsCheckArgs(idfPath), []string{filepath.Join(tools, "idf_tools.py"), "check"}; !slices.Equal(got, want) {
		t.Errorf("ToolsCheckArgs = %q, want %q", got, want)
	}
}

func TestVenvPython(t *testing.T) {
	want := filepath.Join("venv", "bin", "python")
	if runtime.GOOS == "windows" {
		want = filepath.Join("venv", "Scripts", "python.exe")
	}
	if got := espidf.VenvPython("venv"); got != want {
		t.Errorf("VenvPython = %q, want %q", got, want)
	}
}
