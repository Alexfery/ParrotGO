// Package generator renders templates into files on disk.
// It knows nothing about ESP-IDF; callers decide what to generate and where.
package generator

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"text/template"
)

const (
	dirPerm  = 0o755
	filePerm = 0o644
)

// File pairs a template with the path of the file it produces.
type File struct {
	Template string // path inside the template FS
	Path     string // destination on disk
}

// CreateDir creates a directory, creating its parents as needed.
// It fails if the directory itself already exists.
func CreateDir(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), dirPerm); err != nil {
		return err
	}
	err := os.Mkdir(path, dirPerm)
	if errors.Is(err, fs.ErrExist) {
		return fmt.Errorf("directory %q already exists", filepath.ToSlash(path))
	}
	return err
}

// RenderFiles renders each file in order and returns the paths written so far,
// stopping at the first error.
func RenderFiles(fsys fs.FS, files []File, data any) ([]string, error) {
	written := make([]string, 0, len(files))
	for _, f := range files {
		if err := RenderFile(fsys, f.Template, f.Path, data); err != nil {
			return written, err
		}
		written = append(written, f.Path)
	}
	return written, nil
}

// RenderFile executes the template at tmplPath in fsys with data and writes the
// result to dest, creating parent directories as needed. An existing dest is
// never overwritten.
func RenderFile(fsys fs.FS, tmplPath, dest string, data any) error {
	tmpl, err := template.ParseFS(fsys, tmplPath)
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return err
	}
	return WriteFile(dest, buf.Bytes())
}

// WriteFile writes content to a new file at path, creating parent directories
// as needed. It fails if the file already exists.
func WriteFile(path string, content []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), dirPerm); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, filePerm)
	if err != nil {
		return err
	}
	if _, err := f.Write(content); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}
