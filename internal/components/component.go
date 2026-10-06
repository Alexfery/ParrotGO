// Package components holds the hardware components Parrot can add to a
// project. Each component type lives in its own subpackage (e.g. led) and
// decides which target capabilities it needs; internal/targets only describes
// the hardware.
package components

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"parrot/internal/generator"
	"parrot/templates"
)

// Dir is the ESP-IDF folder for project components, relative to the project root.
const Dir = "components"

// ReservedPrefix starts the names of support components Parrot generates
// itself (e.g. parrot_adc); user components cannot use it.
const ReservedPrefix = "parrot_"

// mainComponent is the project's own component, the main folder with
// app_main. ESP-IDF adds the components folder after it, and a component
// added later replaces one of the same name, so components/main would
// silently replace the project's code.
const mainComponent = "main"

var validName = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]*$`)

// NormalizeName validates a user-supplied component name and returns its
// canonical form: lowercase, with '-' replaced by '_'. The canonical name is
// used everywhere (folder, files, C symbols, parrot.json), so it is always a
// valid C identifier and "status-led" and "status_led" are the same component.
func NormalizeName(name string) (string, error) {
	if !validName.MatchString(name) {
		return "", fmt.Errorf("invalid component name %q: start with a letter, then use letters, digits, '-' and '_'", name)
	}
	normalized := strings.ToLower(strings.ReplaceAll(name, "-", "_"))
	if strings.HasPrefix(normalized, ReservedPrefix) {
		return "", fmt.Errorf("invalid component name %q: the %q prefix is reserved for Parrot", name, ReservedPrefix)
	}
	if normalized == mainComponent {
		return "", fmt.Errorf("invalid component name %q: ESP-IDF projects already have a %q component (the main folder, with app_main)", name, mainComponent)
	}
	return normalized, nil
}

// Path returns the folder of the named component inside projectDir.
func Path(projectDir, name string) string {
	return filepath.Join(projectDir, Dir, name)
}

// Generate creates projectDir/components/<name> from the templates in
// templates/components/<kind>/ and returns the paths it created. kind is a
// folder, which can be nested (e.g. "sensor/bme280").
// Every component type provides the same three templates:
//
//	component.c.tmpl    -> <name>.c
//	component.h.tmpl    -> include/<name>.h
//	CMakeLists.txt.tmpl -> CMakeLists.txt
//
// It fails if the component folder already exists.
func Generate(projectDir, kind, name string, data any) ([]string, error) {
	dir := Path(projectDir, name)
	if err := generator.CreateDir(dir); err != nil {
		return nil, err
	}
	tmplDir := "components/" + kind + "/"
	files := []generator.File{
		{Template: tmplDir + "component.c.tmpl", Path: filepath.Join(dir, name+".c")},
		{Template: tmplDir + "component.h.tmpl", Path: filepath.Join(dir, "include", name+".h")},
		{Template: tmplDir + "CMakeLists.txt.tmpl", Path: filepath.Join(dir, "CMakeLists.txt")},
	}
	written, err := generator.RenderFiles(templates.FS, files, data)
	return append([]string{dir}, written...), err
}
