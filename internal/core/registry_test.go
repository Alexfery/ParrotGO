package core_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"parrot/internal/core"
	"parrot/internal/hardware"
	"parrot/internal/project"
)

// fakePlatform records the projects it opens instead of checking them.
type fakePlatform struct {
	id     string
	err    error // returned by OpenProject
	opened []openCall
}

type openCall struct {
	dir string
	cfg project.Config
}

func (p *fakePlatform) ID() string { return p.id }

func (p *fakePlatform) CreateProject(context.Context, core.CreateProjectOptions) (core.CreatedProject, error) {
	return core.CreatedProject{}, errors.New("not used")
}

func (p *fakePlatform) OpenProject(dir string, cfg project.Config, _ core.IO) (core.Project, error) {
	p.opened = append(p.opened, openCall{dir, cfg})
	if p.err != nil {
		return nil, p.err
	}
	return fakeProject{hardware.MCU{ID: cfg.Target}}, nil
}

type fakeProject struct{ mcu hardware.MCU }

func (p fakeProject) Hardware() core.Hardware             { return core.Hardware{MCU: p.mcu} }
func (fakeProject) Build(context.Context) error           { return nil }
func (fakeProject) Flash(context.Context, string) error   { return nil }
func (fakeProject) Monitor(context.Context, string) error { return nil }

func TestRegistryGet(t *testing.T) {
	alpha, beta := &fakePlatform{id: "alpha"}, &fakePlatform{id: "beta"}
	r := core.NewRegistry(alpha, beta)
	for id, want := range map[string]*fakePlatform{"alpha": alpha, "beta": beta} {
		got, err := r.Get(id)
		if err != nil || got != want {
			t.Errorf("Get(%q) = %v, %v; want the %s platform", id, got, err, id)
		}
	}
}

func TestRegistryGetUnknown(t *testing.T) {
	r := core.NewRegistry(&fakePlatform{id: "esp32"}, &fakePlatform{id: "alpha"})
	_, err := r.Get("stm32")
	want := "unsupported platform \"stm32\"\n\nSupported platforms:\n  alpha\n  esp32"
	if err == nil || err.Error() != want {
		t.Errorf("Get(stm32) error = %v, want %q", err, want)
	}
}

func TestZeroRegistry(t *testing.T) {
	var r core.Registry
	if _, err := r.Get("esp32"); err == nil {
		t.Error("Get on an empty registry succeeded, want error")
	}
	r.Register(&fakePlatform{id: "esp32"})
	if _, err := r.Get("esp32"); err != nil {
		t.Errorf("Get after Register: %v", err)
	}
}

func TestRegisterRejectsBadIDs(t *testing.T) {
	for name, ids := range map[string][]string{
		"empty ID":     {""},
		"duplicate ID": {"esp32", "esp32"},
	} {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Error("Register did not panic")
				}
			}()
			var r core.Registry
			for _, id := range ids {
				r.Register(&fakePlatform{id: id})
			}
		})
	}
}

// writeManifest makes dir a Parrot project with manifest as its parrot.json.
func writeManifest(t *testing.T, manifest string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, project.ConfigFile), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// OpenProject hands the project to the platform its parrot.json names, and
// to no other.
func TestOpenProjectUsesPlatformOfManifest(t *testing.T) {
	alpha, beta := &fakePlatform{id: "alpha"}, &fakePlatform{id: "beta"}
	r := core.NewRegistry(alpha, beta)
	dir := writeManifest(t, `{"platform": "beta", "target": "beta-1", "board": "beta-board"}`)

	p, err := r.OpenProject(dir, core.IO{})
	if err != nil {
		t.Fatal(err)
	}
	if len(alpha.opened) != 0 || len(beta.opened) != 1 {
		t.Fatalf("alpha opened %d projects, beta %d; want only beta, once", len(alpha.opened), len(beta.opened))
	}
	got := beta.opened[0]
	if got.dir != dir || got.cfg.Target != "beta-1" || got.cfg.Board != "beta-board" {
		t.Errorf("beta opened %+v, want %s with its parrot.json", got, dir)
	}
	if p.Hardware().MCU.ID != "beta-1" {
		t.Errorf("project hardware = %+v, want the one beta opened", p.Hardware())
	}
}

// Manifests written before Parrot had platforms name none: they are ESP32
// projects.
func TestOpenProjectWithoutPlatform(t *testing.T) {
	esp32 := &fakePlatform{id: project.DefaultPlatform}
	r := core.NewRegistry(&fakePlatform{id: "alpha"}, esp32)
	if _, err := r.OpenProject(writeManifest(t, `{"target": "esp32-c3"}`), core.IO{}); err != nil {
		t.Fatal(err)
	}
	if len(esp32.opened) != 1 {
		t.Errorf("%s opened %d projects, want 1", project.DefaultPlatform, len(esp32.opened))
	}
}

func TestOpenProjectErrors(t *testing.T) {
	failure := errors.New("CMakeLists.txt not found")
	r := core.NewRegistry(&fakePlatform{id: "alpha", err: failure})

	if _, err := r.OpenProject(t.TempDir(), core.IO{}); !errors.Is(err, project.ErrNotProject) {
		t.Errorf("without parrot.json: error = %v, want ErrNotProject", err)
	}
	_, err := r.OpenProject(writeManifest(t, `{"platform": "stm32", "target": "stm32f401re"}`), core.IO{})
	if err == nil || !strings.HasPrefix(err.Error(), `unsupported platform "stm32"`) {
		t.Errorf("unknown platform: error = %v", err)
	}
	if _, err := r.OpenProject(writeManifest(t, `{"platform": "alpha", "target": "a"}`), core.IO{}); !errors.Is(err, failure) {
		t.Errorf("platform failure: error = %v, want %v", err, failure)
	}
}
