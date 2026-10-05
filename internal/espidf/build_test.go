package espidf_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"parrot/internal/espidf"
	"parrot/internal/targets"
)

// recordingRunner records the last command instead of running it.
type recordingRunner struct {
	dir         string
	args        []string
	interactive bool
	calls       int
	err         error
}

func (r *recordingRunner) Run(_ context.Context, opts espidf.RunOptions) error {
	r.dir, r.args, r.interactive = opts.Dir, opts.Args, opts.Interactive
	r.calls++
	return r.err
}

func mustTarget(t *testing.T, id string) targets.Target {
	t.Helper()
	target, err := targets.Get(id)
	if err != nil {
		t.Fatal(err)
	}
	return target
}

func TestBuildArgs(t *testing.T) {
	tests := map[string]string{
		"esp32":    "-DIDF_TARGET=esp32",
		"esp32-c3": "-DIDF_TARGET=esp32c3",
		"esp32-s3": "-DIDF_TARGET=esp32s3",
		"esp32-c6": "-DIDF_TARGET=esp32c6",
	}
	for id, define := range tests {
		got := espidf.BuildArgs(mustTarget(t, id))
		if want := []string{define, "build"}; !slices.Equal(got, want) {
			t.Errorf("BuildArgs(%s) = %q, want %q", id, got, want)
		}
	}
}

func TestBuildRunsIDFInProjectDir(t *testing.T) {
	runner := &recordingRunner{}
	dir := filepath.Join("some", "project")
	if err := (espidf.Builder{Runner: runner}).Build(context.Background(), dir, mustTarget(t, "esp32-c3")); err != nil {
		t.Fatal(err)
	}
	if runner.dir != dir {
		t.Errorf("dir = %q, want %q", runner.dir, dir)
	}
	if want := []string{"-DIDF_TARGET=esp32c3", "build"}; !slices.Equal(runner.args, want) {
		t.Errorf("args = %q, want %q", runner.args, want)
	}
	if runner.interactive {
		t.Error("build ran interactively, want no terminal input")
	}
}

func TestBuildPropagatesRunnerError(t *testing.T) {
	failure := errors.New("idf.py -DIDF_TARGET=esp32 build: exit status 2")
	err := (espidf.Builder{Runner: &recordingRunner{err: failure}}).Build(context.Background(), ".", mustTarget(t, "esp32"))
	if !errors.Is(err, failure) {
		t.Errorf("Build error = %v, want %v", err, failure)
	}
}

func TestCheckProject(t *testing.T) {
	dir := t.TempDir()
	err := espidf.CheckProject(dir)
	if err == nil || err.Error() != "invalid ESP-IDF project: CMakeLists.txt not found" {
		t.Errorf("CheckProject without CMakeLists.txt = %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "CMakeLists.txt"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := espidf.CheckProject(dir); err != nil {
		t.Errorf("CheckProject with CMakeLists.txt = %v", err)
	}
}
