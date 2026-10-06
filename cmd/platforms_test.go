package cmd

import (
	"context"
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"

	"parrot/internal/core"
	"parrot/internal/hardware"
	"parrot/internal/project"
)

// fakePlatform stands for a platform other than ESP32: it records what the
// commands ask of it instead of running an SDK.
type fakePlatform struct {
	id      string
	created []core.CreateProjectOptions
	opened  *fakeProject // the last project opened
}

func (p *fakePlatform) ID() string { return p.id }

func (p *fakePlatform) CreateProject(_ context.Context, opts core.CreateProjectOptions) (core.CreatedProject, error) {
	p.created = append(p.created, opts)
	return core.CreatedProject{Hardware: fakeHardware(opts.Target, opts.Board), Paths: []string{opts.Name}}, nil
}

func (p *fakePlatform) OpenProject(dir string, cfg project.Config, stdio core.IO) (core.Project, error) {
	p.opened = &fakeProject{dir: dir, hardware: fakeHardware(cfg.Target, cfg.Board), stdio: stdio}
	return p.opened, nil
}

// fakeHardware names the MCU and the board in capitals, as a platform's
// catalog would give them names of their own.
func fakeHardware(target, board string) core.Hardware {
	hw := core.Hardware{MCU: hardware.MCU{ID: target, Name: strings.ToUpper(target)}}
	if board != "" {
		hw.Board = &hardware.Board{ID: board, Name: strings.ToUpper(board), MCUID: target}
	}
	return hw
}

// fakeProject records the SDK commands run on it, and prints on the
// terminal it was given, as an SDK would.
type fakeProject struct {
	dir      string
	hardware core.Hardware
	stdio    core.IO
	calls    []string // e.g. "build()", "flash(COM5)"
}

func (p *fakeProject) Hardware() core.Hardware     { return p.hardware }
func (p *fakeProject) Build(context.Context) error { return p.run("build()") }
func (p *fakeProject) Flash(_ context.Context, port string) error {
	return p.run("flash(" + port + ")")
}
func (p *fakeProject) Monitor(_ context.Context, port string) error {
	return p.run("monitor(" + port + ")")
}

func (p *fakeProject) run(call string) error {
	p.calls = append(p.calls, call)
	fmt.Fprintf(p.stdio.Stdout, "fake SDK: %s\n", call)
	return nil
}

// usePlatforms makes the commands see only ps until the test ends.
func usePlatforms(t *testing.T, ps ...core.Platform) {
	t.Helper()
	saved := platforms
	platforms = core.NewRegistry(ps...)
	t.Cleanup(func() { platforms = saved })
}

// The lifecycle commands hand the project to the platform its parrot.json
// names, and to no other, without knowing what that platform's SDK does.
func TestCommandsUseProjectPlatform(t *testing.T) {
	alpha, beta := &fakePlatform{id: "alpha"}, &fakePlatform{id: "beta"}
	usePlatforms(t, alpha, beta)
	t.Chdir(t.TempDir())
	if err := os.WriteFile("parrot.json", []byte(`{"platform": "beta", "target": "beta-1", "board": "beta-board"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		args     []string
		wantCall string
		wantOut  string
	}{
		{
			[]string{"build"}, "build()",
			"Parrot build\n\nTarget: BETA-1\nBoard: BETA-BOARD\n\nBuilding project...\n\n" +
				"fake SDK: build()\n\n✓ Build completed successfully.\n",
		},
		{
			[]string{"flash", "--port", "COM7"}, "flash(COM7)",
			"Parrot flash\n\nTarget: BETA-1\nBoard: BETA-BOARD\nPort: COM7\n\nFlashing device...\n\n" +
				"fake SDK: flash(COM7)\n\n✓ Flash completed successfully.\n",
		},
		{
			[]string{"monitor"}, "monitor()",
			"Parrot monitor\n\nTarget: BETA-1\nBoard: BETA-BOARD\nPort: auto\n\nStarting serial monitor...\n\n" +
				"fake SDK: monitor()\n",
		},
	}
	for _, tt := range tests {
		out, err := runOutput(t, tt.args...)
		if err != nil {
			t.Fatalf("%v: %v", tt.args, err)
		}
		if out != tt.wantOut {
			t.Errorf("%v: output\n%q\nwant\n%q", tt.args, out, tt.wantOut)
		}
		if beta.opened == nil || beta.opened.dir != "." || !slices.Equal(beta.opened.calls, []string{tt.wantCall}) {
			t.Errorf("%v: beta's project = %+v, want %s on .", tt.args, beta.opened, tt.wantCall)
		}
	}
	if alpha.opened != nil {
		t.Error("alpha opened beta's project")
	}
}

func TestNewUsesPlatform(t *testing.T) {
	alpha, beta := &fakePlatform{id: "alpha"}, &fakePlatform{id: "beta"}
	usePlatforms(t, alpha, beta)
	t.Chdir(t.TempDir())

	out, err := runOutput(t, "new", "app", "--platform", "beta", "--target", "beta-1", "--board", "beta-board")
	if err != nil {
		t.Fatal(err)
	}
	if want := []core.CreateProjectOptions{{Name: "app", Target: "beta-1", Board: "beta-board"}}; !slices.Equal(beta.created, want) {
		t.Errorf("beta created %+v, want %+v", beta.created, want)
	}
	if len(alpha.created) != 0 {
		t.Errorf("alpha created %+v, want nothing", alpha.created)
	}
	want := "Creating Parrot project: app\nTarget: BETA-1\nBoard: BETA-BOARD\n\n✓ Created app\n\n" +
		"Project created successfully.\n\nNext:\n\n  cd app\n  parrot build\n"
	if out != want {
		t.Errorf("output\n%q\nwant\n%q", out, want)
	}
}

func TestNewUsesDefaultPlatform(t *testing.T) {
	alpha, esp32 := &fakePlatform{id: "alpha"}, &fakePlatform{id: project.DefaultPlatform}
	usePlatforms(t, alpha, esp32)
	t.Chdir(t.TempDir())

	mustRun(t, "new", "app")
	if want := []core.CreateProjectOptions{{Name: "app"}}; !slices.Equal(esp32.created, want) {
		t.Errorf("%s created %+v, want %+v", project.DefaultPlatform, esp32.created, want)
	}
	if len(alpha.created) != 0 {
		t.Errorf("alpha created %+v, want nothing", alpha.created)
	}
}

// Projects whose parrot.json names no platform, as every one did before
// Parrot had platforms, must keep opening with the ESP32 platform.
func TestDefaultPlatformIsRegistered(t *testing.T) {
	p, err := platforms.Get(project.DefaultPlatform)
	if err != nil || p.ID() != project.DefaultPlatform {
		t.Errorf("Get(%q) = %v, %v", project.DefaultPlatform, p, err)
	}
}
