package esp32

import (
	"context"

	"parrot/internal/core"
	"parrot/internal/espidf"
	"parrot/internal/project"
	"parrot/internal/targets"
)

// OpenProject checks, in this order, the target and board of cfg, that dir
// is an ESP-IDF project and that idf.py is on PATH. idf.py then uses stdio.
func (Platform) OpenProject(dir string, cfg project.Config, stdio core.IO) (core.Project, error) {
	target, hw, err := resolve(cfg.Target, cfg.Board)
	if err != nil {
		return nil, err
	}
	if err := espidf.CheckProject(dir); err != nil {
		return nil, err
	}
	runner, err := espidf.NewRunner(stdio.Stdin, stdio.Stdout, stdio.Stderr)
	if err != nil {
		return nil, err
	}
	return idfProject{dir: dir, target: target, hardware: hw, runner: runner}, nil
}

// idfProject is an ESP-IDF project, opened by OpenProject. Each command runs
// idf.py in dir, for target.
type idfProject struct {
	dir      string
	target   targets.Target
	hardware core.Hardware
	runner   espidf.CommandRunner
}

func (p idfProject) Hardware() core.Hardware { return p.hardware }

// Build runs idf.py build.
func (p idfProject) Build(ctx context.Context) error {
	return espidf.Builder{Runner: p.runner}.Build(ctx, p.dir, p.target)
}

// Flash runs idf.py flash, which rebuilds the project first when the sources
// changed.
func (p idfProject) Flash(ctx context.Context, port string) error {
	return espidf.Flasher{Runner: p.runner}.Flash(ctx, espidf.FlashOptions{ProjectDir: p.dir, Target: p.target, Port: port})
}

// Monitor runs IDF Monitor (idf.py monitor), which owns the terminal until
// the user leaves it with Ctrl+].
func (p idfProject) Monitor(ctx context.Context, port string) error {
	return espidf.Monitor{Runner: p.runner}.Run(ctx, espidf.MonitorOptions{ProjectDir: p.dir, Target: p.target, Port: port})
}
