package espidf_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"parrot/internal/espidf"
)

// When PARROT_FAKE_IDF is set, the test binary acts as idf.py: it prints its
// working directory, arguments and standard input, sleeps for
// PARROT_FAKE_IDF_SLEEP if set, then exits with PARROT_FAKE_IDF_EXIT.
func TestMain(m *testing.M) {
	if os.Getenv("PARROT_FAKE_IDF") != "" {
		fakeIDF()
	}
	os.Exit(m.Run())
}

func fakeIDF() {
	wd, _ := os.Getwd()
	input, _ := io.ReadAll(os.Stdin)
	fmt.Printf("cwd=%s\nargs=%s\nstdin=%s\n", wd, strings.Join(os.Args[1:], " "), input)
	fmt.Fprintln(os.Stderr, "warning from fake idf.py")
	if sleep, err := time.ParseDuration(os.Getenv("PARROT_FAKE_IDF_SLEEP")); err == nil {
		time.Sleep(sleep)
	}
	code, _ := strconv.Atoi(os.Getenv("PARROT_FAKE_IDF_EXIT"))
	os.Exit(code)
}

// fakeRunner returns a Runner that starts the fake idf.py.
func fakeRunner(t *testing.T) (*espidf.Runner, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	self, err := os.Executable() // absolute, unlike os.Args[0]
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("PARROT_FAKE_IDF", "1")
	var stdout, stderr bytes.Buffer
	return &espidf.Runner{IDFPath: self, Stdout: &stdout, Stderr: &stderr}, &stdout, &stderr
}

func TestRunnerUsesDirAndStreamsOutput(t *testing.T) {
	runner, stdout, stderr := fakeRunner(t)
	dir := t.TempDir()
	if err := runner.Run(context.Background(), espidf.RunOptions{Dir: dir, Args: []string{"-DIDF_TARGET=esp32c3", "build"}}); err != nil {
		t.Fatal(err)
	}
	wantDir, _ := filepath.EvalSymlinks(dir)
	out := stdout.String()
	if !strings.Contains(out, "args=-DIDF_TARGET=esp32c3 build") {
		t.Errorf("stdout = %q, want the arguments", out)
	}
	gotDir := strings.TrimPrefix(strings.SplitN(out, "\n", 2)[0], "cwd=")
	if gotDir, _ = filepath.EvalSymlinks(gotDir); gotDir != wantDir {
		t.Errorf("idf.py ran in %q, want %q", gotDir, wantDir)
	}
	if !strings.Contains(stderr.String(), "warning from fake idf.py") {
		t.Errorf("stderr = %q, want the child's stderr", stderr.String())
	}
}

func TestRunnerPropagatesExitStatus(t *testing.T) {
	runner, _, _ := fakeRunner(t)
	t.Setenv("PARROT_FAKE_IDF_EXIT", "2")
	err := runner.Run(context.Background(), espidf.RunOptions{Dir: t.TempDir(), Args: []string{"build"}})
	if err == nil || err.Error() != "idf.py build: exit status 2" {
		t.Errorf("Run error = %v, want exit status 2", err)
	}
}

func TestRunnerStopsOnCancel(t *testing.T) {
	runner, _, _ := fakeRunner(t)
	t.Setenv("PARROT_FAKE_IDF_SLEEP", "1m")
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	start := time.Now()
	err := runner.Run(ctx, espidf.RunOptions{Dir: t.TempDir(), Args: []string{"build"}})
	if err == nil || err.Error() != "idf.py build interrupted" {
		t.Errorf("Run error = %v, want interrupted", err)
	}
	if elapsed := time.Since(start); elapsed > 20*time.Second {
		t.Errorf("Run took %v after cancellation", elapsed)
	}
}

// An interactive run owns the terminal: Ctrl+C is for idf.py (IDF Monitor
// sends it to the device), so cancelling ctx must not kill it.
func TestRunnerLeavesInteractiveRunOnCancel(t *testing.T) {
	runner, stdout, _ := fakeRunner(t)
	t.Setenv("PARROT_FAKE_IDF_SLEEP", "500ms")
	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(100*time.Millisecond, cancel)
	defer cancel()

	err := runner.Run(ctx, espidf.RunOptions{Dir: t.TempDir(), Args: []string{"monitor"}, Interactive: true})
	if err != nil {
		t.Errorf("Run error = %v, want the interactive run to finish on its own", err)
	}
	if !strings.Contains(stdout.String(), "args=monitor") {
		t.Errorf("stdout = %q, want the child's output", stdout.String())
	}
}

func TestRunnerGivesStdinOnlyToInteractiveRuns(t *testing.T) {
	for _, interactive := range []bool{false, true} {
		runner, stdout, _ := fakeRunner(t)
		runner.Stdin = strings.NewReader("keys typed by the user")
		opts := espidf.RunOptions{Dir: t.TempDir(), Args: []string{"monitor"}, Interactive: interactive}
		if err := runner.Run(context.Background(), opts); err != nil {
			t.Fatal(err)
		}
		want := "stdin=\n" // the null device
		if interactive {
			want = "stdin=keys typed by the user\n"
		}
		if !strings.Contains(stdout.String(), want) {
			t.Errorf("interactive=%v: stdout = %q, want %q", interactive, stdout.String(), want)
		}
	}
}

func TestFindIDF(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PATH", dir)
	if _, err := espidf.FindIDF(); err != espidf.ErrIDFNotFound {
		t.Errorf("FindIDF with empty PATH = %v, want ErrIDFNotFound", err)
	}

	name := "idf.py"
	if runtime.GOOS == "windows" {
		name = "idf.py.exe" // the launcher ESP-IDF puts on PATH
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	path, err := espidf.FindIDF()
	if err != nil || filepath.Base(path) != name {
		t.Errorf("FindIDF = %q, %v; want %s on PATH", path, err, name)
	}
}
