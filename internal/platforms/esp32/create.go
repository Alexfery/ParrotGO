package esp32

import (
	"context"
	"path/filepath"

	"parrot/internal/core"
	"parrot/internal/generator"
	"parrot/internal/project"
	"parrot/internal/targets"
	"parrot/templates"
)

// templateData is what the project templates get, so they can use {{.Name}}
// or {{.Target.IDFTarget}}.
type templateData struct {
	Name   string
	Target targets.Target
}

// projectFiles maps each embedded template to its path inside the new project.
func projectFiles(dir string) []generator.File {
	return []generator.File{
		{Template: "project/CMakeLists.txt.tmpl", Path: filepath.Join(dir, "CMakeLists.txt")},
		{Template: "project/main.CMakeLists.txt.tmpl", Path: filepath.Join(dir, "main", "CMakeLists.txt")},
		{Template: "project/main.c.tmpl", Path: filepath.Join(dir, "main", "main.c")},
		{Template: "project/gitignore.tmpl", Path: filepath.Join(dir, ".gitignore")},
	}
}

// CreateProject generates a minimal ESP-IDF project. Without a target or a
// board, the target is targets.DefaultID. The hardware is checked before
// anything is created.
func (Platform) CreateProject(_ context.Context, opts core.CreateProjectOptions) (core.CreatedProject, error) {
	targetID := opts.Target
	if targetID == "" && opts.Board == "" {
		targetID = targets.DefaultID
	}
	target, hw, err := resolve(targetID, opts.Board)
	if err != nil {
		return core.CreatedProject{}, err
	}

	cfg := project.Config{Platform: ID, Target: target.ID, Board: opts.Board}
	created, err := project.Create(opts.Name, cfg)
	if err != nil {
		return core.CreatedProject{Hardware: hw, Paths: created}, err
	}
	written, err := generator.RenderFiles(templates.FS, projectFiles(opts.Name), templateData{Name: opts.Name, Target: target})
	return core.CreatedProject{Hardware: hw, Paths: append(created, written...)}, err
}
