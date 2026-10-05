package espidf_test

import (
	"context"
	"errors"
	"path/filepath"
	"slices"
	"testing"

	"parrot/internal/espidf"
)

func TestMonitorArgs(t *testing.T) {
	tests := []struct {
		target string
		port   string
		want   []string
	}{
		{"esp32", "", []string{"-DIDF_TARGET=esp32", "monitor"}},
		{"esp32-c3", "", []string{"-DIDF_TARGET=esp32c3", "monitor"}},
		{"esp32-c3", "COM5", []string{"-DIDF_TARGET=esp32c3", "-p", "COM5", "monitor"}},
		{"esp32-s3", "/dev/ttyUSB0", []string{"-DIDF_TARGET=esp32s3", "-p", "/dev/ttyUSB0", "monitor"}},
	}
	for _, tt := range tests {
		opts := espidf.MonitorOptions{Target: mustTarget(t, tt.target), Port: tt.port}
		if got := espidf.MonitorArgs(opts); !slices.Equal(got, tt.want) {
			t.Errorf("MonitorArgs(%s, port %q) = %q, want %q", tt.target, tt.port, got, tt.want)
		}
	}
}

func TestMonitorArgsWithoutPortLetsIDFPickIt(t *testing.T) {
	args := espidf.MonitorArgs(espidf.MonitorOptions{Target: mustTarget(t, "esp32")})
	if slices.Contains(args, "-p") {
		t.Errorf("MonitorArgs without a port = %q, want no -p", args)
	}
}

func TestMonitorRunsInteractiveIDFInProjectDir(t *testing.T) {
	runner := &recordingRunner{}
	dir := filepath.Join("some", "project")
	opts := espidf.MonitorOptions{ProjectDir: dir, Target: mustTarget(t, "esp32-c3"), Port: "COM5"}
	if err := (espidf.Monitor{Runner: runner}).Run(context.Background(), opts); err != nil {
		t.Fatal(err)
	}
	if runner.dir != dir {
		t.Errorf("dir = %q, want %q", runner.dir, dir)
	}
	if want := []string{"-DIDF_TARGET=esp32c3", "-p", "COM5", "monitor"}; !slices.Equal(runner.args, want) {
		t.Errorf("args = %q, want %q", runner.args, want)
	}
	if !runner.interactive {
		t.Error("monitor ran without the terminal, want an interactive run")
	}
	// Only the monitor: no build or flash before it.
	if runner.calls != 1 {
		t.Errorf("idf.py ran %d times, want once", runner.calls)
	}
}

func TestMonitorPropagatesRunnerError(t *testing.T) {
	failure := errors.New("idf.py -DIDF_TARGET=esp32 monitor: exit status 1")
	opts := espidf.MonitorOptions{ProjectDir: ".", Target: mustTarget(t, "esp32")}
	err := (espidf.Monitor{Runner: &recordingRunner{err: failure}}).Run(context.Background(), opts)
	if !errors.Is(err, failure) {
		t.Errorf("Monitor error = %v, want %v", err, failure)
	}
}
