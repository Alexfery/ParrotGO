// Package project manages the layout of ESP-IDF projects.
package project

import (
	"fmt"
	"path/filepath"
	"regexp"

	"parrot/internal/generator"
	"parrot/internal/targets"
	"parrot/templates"
)

// CreateOptions describes the project to create. It is also the data passed
// to the templates, so they can use {{.Name}} or {{.Target.IDFTarget}}.
type CreateOptions struct {
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

// validName keeps names usable both as a directory and as a CMake project name.
var validName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]*$`)

// Create generates a minimal ESP-IDF project in a new directory called
// opts.Name, relative to the current working directory. It returns the paths
// it created, in creation order. It fails if the directory already exists.
func Create(opts CreateOptions) ([]string, error) {
	if !validName.MatchString(opts.Name) {
		return nil, fmt.Errorf("invalid project name %q: use letters, digits, '-' and '_'", opts.Name)
	}
	if err := generator.CreateDir(opts.Name); err != nil {
		return nil, err
	}
	created := []string{opts.Name}

	cfgPath, err := writeConfig(opts.Name, Config{Target: opts.Target.ID})
	if err != nil {
		return created, err
	}
	created = append(created, cfgPath)

	written, err := generator.RenderFiles(templates.FS, projectFiles(opts.Name), opts)
	return append(created, written...), err
}
