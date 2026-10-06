package esp32

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"parrot/internal/core"
	"parrot/internal/espidf"
	"parrot/internal/project"
	"parrot/internal/targets"
)

func TestBoards(t *testing.T) {
	tests := []struct {
		board, target, mcuName string
	}{
		{"esp32-devkit-v1", "esp32", "ESP32"},
		{"esp32-c3-devkitm-1", "esp32-c3", "ESP32-C3"},
		{"esp32-s3-devkitc-1", "esp32-s3", "ESP32-S3"},
		{"esp32-c6-devkitc-1", "esp32-c6", "ESP32-C6"},
	}
	for _, tt := range tests {
		target, hw, err := resolve("", tt.board)
		if err != nil {
			t.Errorf("%s: %v", tt.board, err)
			continue
		}
		if target.ID != tt.target || hw.MCU.ID != tt.target || hw.MCU.Name != tt.mcuName {
			t.Errorf("%s: target %q, MCU %+v; want %s (%s)", tt.board, target.ID, hw.MCU, tt.target, tt.mcuName)
		}
		if hw.Board == nil || hw.Board.ID != tt.board {
			t.Errorf("%s: board %+v", tt.board, hw.Board)
		}
	}
}

func TestLookupBoardUnknown(t *testing.T) {
	_, err := lookupBoard("nucleo-f401re")
	want := "unsupported board \"nucleo-f401re\"\n\nSupported boards:\n" +
		"  esp32-devkit-v1\n  esp32-c3-devkitm-1\n  esp32-s3-devkitc-1\n  esp32-c6-devkitc-1"
	if err == nil || err.Error() != want {
		t.Errorf("lookupBoard error = %v, want %q", err, want)
	}
}

// TestCatalog checks that every level of the hardware links to the one above
// it: board -> MCU (a target of the registry) -> family -> vendor.
func TestCatalog(t *testing.T) {
	ids := make(map[string]bool)
	for _, b := range boards {
		if ids[b.ID] {
			t.Errorf("board %q is listed twice", b.ID)
		}
		ids[b.ID] = true
		if _, err := targets.Get(b.MCUID); err != nil {
			t.Errorf("board %q: %v", b.ID, err)
		}
	}
	for _, target := range targets.All() {
		m, f := mcu(target), family(target)
		if m.ID != target.ID || m.Name != target.DisplayName {
			t.Errorf("MCU of %s = %+v", target.ID, m)
		}
		if m.FamilyID != f.ID || f.VendorID != espressif.ID {
			t.Errorf("%s: MCU %+v, family %+v; want them linked up to %s", target.ID, m, f, espressif.Name)
		}
	}
}

func TestResolve(t *testing.T) {
	tests := []struct {
		target, board string
		wantTarget    string // "" when an error is expected
		wantErr       string
	}{
		{"esp32-c3", "", "esp32-c3", ""},
		{"", "esp32-s3-devkitc-1", "esp32-s3", ""},
		{"esp32-s3", "esp32-s3-devkitc-1", "esp32-s3", ""},
		{"esp32", "esp32-c3-devkitm-1", "", `target "esp32" does not match board "esp32-c3-devkitm-1", whose target is "esp32-c3"`},
		{"banana", "", "", `unsupported target "banana"`},
		{"", "", "", `unsupported target ""`}, // a parrot.json needs a target or a board
		{"esp32", "banana-board", "", `unsupported board "banana-board"`},
	}
	for _, tt := range tests {
		target, hw, err := resolve(tt.target, tt.board)
		if tt.wantErr != "" {
			if err == nil || !strings.HasPrefix(err.Error(), tt.wantErr) {
				t.Errorf("resolve(%q, %q) error = %v, want %q", tt.target, tt.board, err, tt.wantErr)
			}
			continue
		}
		if err != nil || target.ID != tt.wantTarget || hw.MCU.ID != tt.wantTarget {
			t.Errorf("resolve(%q, %q) = %q, %+v, %v; want %s", tt.target, tt.board, target.ID, hw, err, tt.wantTarget)
		}
		if (tt.board == "") != (hw.Board == nil) {
			t.Errorf("resolve(%q, %q) board = %+v", tt.target, tt.board, hw.Board)
		}
	}
}

func TestCreateProject(t *testing.T) {
	tests := []struct {
		opts      core.CreateProjectOptions
		wantMCU   string
		wantBoard string
		wantJSON  string
	}{
		{
			core.CreateProjectOptions{Name: "plain"}, "ESP32", "",
			"{\n  \"platform\": \"esp32\",\n  \"target\": \"esp32\"\n}\n",
		},
		{
			core.CreateProjectOptions{Name: "chip", Target: "esp32-s3"}, "ESP32-S3", "",
			"{\n  \"platform\": \"esp32\",\n  \"target\": \"esp32-s3\"\n}\n",
		},
		{
			core.CreateProjectOptions{Name: "board", Board: "esp32-c3-devkitm-1"}, "ESP32-C3", "ESP32-C3-DevKitM-1",
			"{\n  \"platform\": \"esp32\",\n  \"target\": \"esp32-c3\",\n  \"board\": \"esp32-c3-devkitm-1\"\n}\n",
		},
	}
	t.Chdir(t.TempDir())
	for _, tt := range tests {
		created, err := Platform{}.CreateProject(context.Background(), tt.opts)
		if err != nil {
			t.Fatalf("%s: %v", tt.opts.Name, err)
		}
		name := tt.opts.Name
		wantPaths := []string{
			name,
			filepath.Join(name, "parrot.json"),
			filepath.Join(name, "CMakeLists.txt"),
			filepath.Join(name, "main", "CMakeLists.txt"),
			filepath.Join(name, "main", "main.c"),
			filepath.Join(name, ".gitignore"),
		}
		if !slices.Equal(created.Paths, wantPaths) {
			t.Errorf("%s: created %q, want %q", name, created.Paths, wantPaths)
		}
		if created.Hardware.MCU.Name != tt.wantMCU {
			t.Errorf("%s: MCU %+v, want %s", name, created.Hardware.MCU, tt.wantMCU)
		}
		if gotBoard := boardName(created.Hardware); gotBoard != tt.wantBoard {
			t.Errorf("%s: board %q, want %q", name, gotBoard, tt.wantBoard)
		}
		if data, _ := os.ReadFile(filepath.Join(name, "parrot.json")); string(data) != tt.wantJSON {
			t.Errorf("%s: parrot.json =\n%s\nwant\n%s", name, data, tt.wantJSON)
		}
		if data, _ := os.ReadFile(filepath.Join(name, "CMakeLists.txt")); !strings.Contains(string(data), "project("+name+")") {
			t.Errorf("%s: CMakeLists.txt =\n%s", name, data)
		}
	}
}

func boardName(hw core.Hardware) string {
	if hw.Board == nil {
		return ""
	}
	return hw.Board.Name
}

// Hardware errors are reported before anything is created.
func TestCreateProjectChecksHardwareFirst(t *testing.T) {
	t.Chdir(t.TempDir())
	for _, opts := range []core.CreateProjectOptions{
		{Name: "app", Target: "banana"},
		{Name: "app", Board: "banana-board"},
		{Name: "app", Target: "esp32-s3", Board: "esp32-c3-devkitm-1"},
	} {
		if _, err := (Platform{}).CreateProject(context.Background(), opts); err == nil {
			t.Errorf("%+v: CreateProject succeeded, want error", opts)
		}
		if _, err := os.Stat("app"); err == nil {
			t.Fatalf("%+v: the project directory was created", opts)
		}
	}
}

// putIDFOnPath makes PATH hold only an idf.py that is never run.
func putIDFOnPath(t *testing.T) {
	t.Helper()
	bin := t.TempDir()
	name := "idf.py"
	if runtime.GOOS == "windows" {
		name = "idf.py.exe"
	}
	if err := os.WriteFile(filepath.Join(bin, name), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
}

// espIDFProject returns a directory that looks like an ESP-IDF project.
func espIDFProject(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "CMakeLists.txt"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestOpenProject(t *testing.T) {
	putIDFOnPath(t)
	dir := espIDFProject(t)
	cfg := project.Config{Platform: ID, Target: "esp32-c3", Board: "esp32-c3-devkitm-1"}
	opened, err := Platform{}.OpenProject(dir, cfg, core.IO{})
	if err != nil {
		t.Fatal(err)
	}
	hw := opened.Hardware()
	if hw.MCU.ID != "esp32-c3" || boardName(hw) != "ESP32-C3-DevKitM-1" {
		t.Errorf("Hardware = %+v", hw)
	}
	p := opened.(idfProject)
	if p.dir != dir || p.target.IDFTarget != "esp32c3" {
		t.Errorf("opened %+v, want %s for esp32c3", p, dir)
	}
}

// The checks run in the order the commands always reported them: the
// hardware, the project, then idf.py.
func TestOpenProjectErrors(t *testing.T) {
	t.Setenv("PATH", t.TempDir()) // no idf.py
	open := func(dir, target string) error {
		_, err := Platform{}.OpenProject(dir, project.Config{Target: target}, core.IO{})
		return err
	}
	if err := open(t.TempDir(), "banana"); err == nil || !strings.HasPrefix(err.Error(), `unsupported target "banana"`) {
		t.Errorf("unsupported target: error = %v", err)
	}
	if err := open(t.TempDir(), "esp32"); err == nil || err.Error() != "invalid ESP-IDF project: CMakeLists.txt not found" {
		t.Errorf("no CMakeLists.txt: error = %v", err)
	}
	if err := open(espIDFProject(t), "esp32"); !errors.Is(err, espidf.ErrIDFNotFound) {
		t.Errorf("no idf.py: error = %v", err)
	}
}

// recordingRunner records the idf.py runs instead of running them.
type recordingRunner struct{ runs []espidf.RunOptions }

func (r *recordingRunner) Run(_ context.Context, opts espidf.RunOptions) error {
	r.runs = append(r.runs, opts)
	return nil
}

// An opened project hands each command to idf.py, in its directory and for
// its target.
func TestProjectRunsIDF(t *testing.T) {
	target, err := targets.Get("esp32-c3")
	if err != nil {
		t.Fatal(err)
	}
	runner := &recordingRunner{}
	dir := filepath.Join("some", "project")
	p := idfProject{dir: dir, target: target, runner: runner}
	ctx := context.Background()
	if err := p.Build(ctx); err != nil {
		t.Fatal(err)
	}
	if err := p.Flash(ctx, "COM5"); err != nil {
		t.Fatal(err)
	}
	if err := p.Monitor(ctx, ""); err != nil {
		t.Fatal(err)
	}

	want := []espidf.RunOptions{
		{Dir: dir, Args: []string{"-DIDF_TARGET=esp32c3", "build"}},
		{Dir: dir, Args: []string{"-DIDF_TARGET=esp32c3", "-p", "COM5", "flash"}},
		{Dir: dir, Args: []string{"-DIDF_TARGET=esp32c3", "monitor"}, Interactive: true},
	}
	if len(runner.runs) != len(want) {
		t.Fatalf("idf.py ran %d times, want %d: %+v", len(runner.runs), len(want), runner.runs)
	}
	for i, got := range runner.runs {
		if got.Dir != want[i].Dir || !slices.Equal(got.Args, want[i].Args) || got.Interactive != want[i].Interactive {
			t.Errorf("run %d = %+v, want %+v", i, got, want[i])
		}
	}
}

// Target lets the ESP32-only commands into the platform: projects without a
// platform are ESP32 projects, others are rejected, and the board must match.
func TestTarget(t *testing.T) {
	for _, cfg := range []project.Config{
		{Target: "esp32-c3"},
		{Platform: ID, Target: "esp32-c3", Board: "esp32-c3-devkitm-1"},
		{Platform: ID, Board: "esp32-c3-devkitm-1"},
	} {
		if target, err := Target(cfg); err != nil || target.ID != "esp32-c3" {
			t.Errorf("Target(%+v) = %q, %v; want esp32-c3", cfg, target.ID, err)
		}
	}
	tests := []struct {
		cfg     project.Config
		wantErr string
	}{
		{project.Config{Platform: "stm32", Target: "stm32f401re"}, `parrot.json names platform "stm32", but this command only supports "esp32"`},
		{project.Config{Target: "esp32", Board: "esp32-c3-devkitm-1"}, `target "esp32" does not match board "esp32-c3-devkitm-1", whose target is "esp32-c3"`},
		{project.Config{Target: "banana"}, `unsupported target "banana"`},
	}
	for _, tt := range tests {
		if _, err := Target(tt.cfg); err == nil || !strings.HasPrefix(err.Error(), tt.wantErr) {
			t.Errorf("Target(%+v) error = %v, want %q", tt.cfg, err, tt.wantErr)
		}
	}
}
