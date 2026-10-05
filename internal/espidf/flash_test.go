package espidf_test

import (
	"context"
	"errors"
	"path/filepath"
	"slices"
	"testing"

	"parrot/internal/espidf"
)

func TestFlashArgs(t *testing.T) {
	tests := []struct {
		target string
		port   string
		want   []string
	}{
		{"esp32", "", []string{"-DIDF_TARGET=esp32", "flash"}},
		{"esp32-c3", "", []string{"-DIDF_TARGET=esp32c3", "flash"}},
		{"esp32-s3", "", []string{"-DIDF_TARGET=esp32s3", "flash"}},
		{"esp32-c3", "COM5", []string{"-DIDF_TARGET=esp32c3", "-p", "COM5", "flash"}},
		{"esp32", "/dev/ttyUSB0", []string{"-DIDF_TARGET=esp32", "-p", "/dev/ttyUSB0", "flash"}},
		{"esp32-s3", "/dev/cu.usbserial-1420", []string{"-DIDF_TARGET=esp32s3", "-p", "/dev/cu.usbserial-1420", "flash"}},
	}
	for _, tt := range tests {
		opts := espidf.FlashOptions{Target: mustTarget(t, tt.target), Port: tt.port}
		if got := espidf.FlashArgs(opts); !slices.Equal(got, tt.want) {
			t.Errorf("FlashArgs(%s, port %q) = %q, want %q", tt.target, tt.port, got, tt.want)
		}
	}
}

func TestFlashArgsWithoutPortLetsIDFDetectIt(t *testing.T) {
	args := espidf.FlashArgs(espidf.FlashOptions{Target: mustTarget(t, "esp32")})
	if slices.Contains(args, "-p") {
		t.Errorf("FlashArgs without a port = %q, want no -p", args)
	}
}

func TestFlashRunsIDFInProjectDir(t *testing.T) {
	runner := &recordingRunner{}
	dir := filepath.Join("some", "project")
	opts := espidf.FlashOptions{ProjectDir: dir, Target: mustTarget(t, "esp32-c3"), Port: "COM5"}
	if err := (espidf.Flasher{Runner: runner}).Flash(context.Background(), opts); err != nil {
		t.Fatal(err)
	}
	if runner.dir != dir {
		t.Errorf("dir = %q, want %q", runner.dir, dir)
	}
	if want := []string{"-DIDF_TARGET=esp32c3", "-p", "COM5", "flash"}; !slices.Equal(runner.args, want) {
		t.Errorf("args = %q, want %q", runner.args, want)
	}
	if runner.interactive {
		t.Error("flash ran interactively, want no terminal input")
	}
}

func TestFlashPropagatesRunnerError(t *testing.T) {
	failure := errors.New("idf.py -DIDF_TARGET=esp32 -p COM5 flash: exit status 2")
	opts := espidf.FlashOptions{ProjectDir: ".", Target: mustTarget(t, "esp32"), Port: "COM5"}
	err := (espidf.Flasher{Runner: &recordingRunner{err: failure}}).Flash(context.Background(), opts)
	if !errors.Is(err, failure) {
		t.Errorf("Flash error = %v, want %v", err, failure)
	}
}
