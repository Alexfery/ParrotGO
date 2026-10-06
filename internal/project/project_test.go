package project_test

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"parrot/internal/project"
)

func TestCreate(t *testing.T) {
	t.Chdir(t.TempDir())
	created, err := project.Create("app", project.Config{Platform: "esp32", Target: "esp32"})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"app", filepath.Join("app", project.ConfigFile)}; !slices.Equal(created, want) {
		t.Errorf("Create = %q, want %q", created, want)
	}
	cfg, err := project.LoadConfigFrom("app")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Platform != "esp32" || cfg.Target != "esp32" {
		t.Errorf("parrot.json = %+v", cfg)
	}
}

func TestCreateErrors(t *testing.T) {
	t.Chdir(t.TempDir())
	if _, err := project.Create("-app", project.Config{}); err == nil || err.Error() != `invalid project name "-app": use letters, digits, '-' and '_'` {
		t.Errorf("invalid name: error = %v", err)
	}
	if _, err := os.Stat("-app"); err == nil {
		t.Error("a directory was created for an invalid name")
	}

	if err := os.Mkdir("app", 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := project.Create("app", project.Config{}); err == nil || err.Error() != `directory "app" already exists` {
		t.Errorf("existing directory: error = %v", err)
	}
	if _, err := os.Stat(filepath.Join("app", project.ConfigFile)); err == nil {
		t.Error("parrot.json was written into an existing directory")
	}
}
