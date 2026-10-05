// Package templates embeds the template files into the parrot binary,
// so the executable does not depend on this folder at runtime.
package templates

import "embed"

// FS holds every template, addressed by its path relative to this folder
// (for example "project/main.c.tmpl").
//
//go:embed project/*.tmpl components/*/*.tmpl components/*/*/*.tmpl
var FS embed.FS
