package espidf

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"parrot/internal/targets"
)

// CheckProject verifies that dir looks like an ESP-IDF project.
func CheckProject(dir string) error {
	_, err := os.Stat(filepath.Join(dir, "CMakeLists.txt"))
	if errors.Is(err, fs.ErrNotExist) {
		return errors.New("invalid ESP-IDF project: CMakeLists.txt not found")
	}
	return err
}

// targetArg returns the idf.py argument that selects target.
//
// The target is passed as a CMake cache entry instead of running
// `idf.py set-target`, which deletes the build directory and regenerates
// sdkconfig. If the build directory or sdkconfig were made for another
// target, idf.py stops and explains how to switch (set-target or fullclean).
func targetArg(target targets.Target) string {
	return "-DIDF_TARGET=" + target.IDFTarget
}

// BuildArgs returns the idf.py arguments that build for target.
func BuildArgs(target targets.Target) []string {
	return []string{targetArg(target), "build"}
}

// Builder builds ESP-IDF projects through idf.py.
type Builder struct {
	Runner CommandRunner
}

// Build builds the project in projectDir for target, in ESP-IDF's default
// build directory (projectDir/build). Check the project with CheckProject first.
func (b Builder) Build(ctx context.Context, projectDir string, target targets.Target) error {
	return b.Runner.Run(ctx, RunOptions{Dir: projectDir, Args: BuildArgs(target)})
}
