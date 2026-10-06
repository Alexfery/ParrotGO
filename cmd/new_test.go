package cmd

import (
	"os"
	"strings"
	"testing"
)

func TestNew(t *testing.T) {
	t.Chdir(t.TempDir())
	out, err := runOutput(t, "new", "app")
	if err != nil {
		t.Fatal(err)
	}
	want := "Creating Parrot project: app\nTarget: ESP32\n\n" +
		"✓ Created app\n✓ Created app/parrot.json\n✓ Created app/CMakeLists.txt\n" +
		"✓ Created app/main/CMakeLists.txt\n✓ Created app/main/main.c\n✓ Created app/.gitignore\n\n" +
		"Project created successfully.\n\nNext:\n\n  cd app\n  parrot build\n"
	if out != want {
		t.Errorf("output\n%q\nwant\n%q", out, want)
	}
	data, _ := os.ReadFile("app/parrot.json")
	if want := "{\n  \"platform\": \"esp32\",\n  \"target\": \"esp32\"\n}\n"; string(data) != want {
		t.Errorf("parrot.json = %q, want %q", data, want)
	}
}

// A board sets the target: the commands then build for the board's chip and
// show the board.
func TestNewWithBoard(t *testing.T) {
	t.Chdir(t.TempDir())
	out, err := runOutput(t, "new", "app", "--board", "esp32-c3-devkitm-1")
	if err != nil {
		t.Fatal(err)
	}
	if want := "Creating Parrot project: app\nTarget: ESP32-C3\nBoard: ESP32-C3-DevKitM-1\n\n"; !strings.HasPrefix(out, want) {
		t.Errorf("output %q does not start with %q", out, want)
	}
	data, _ := os.ReadFile("app/parrot.json")
	want := "{\n  \"platform\": \"esp32\",\n  \"target\": \"esp32-c3\",\n  \"board\": \"esp32-c3-devkitm-1\"\n}\n"
	if string(data) != want {
		t.Errorf("parrot.json = %q, want %q", data, want)
	}

	t.Chdir("app")
	record := installFakeIDF(t)
	out, err = runOutput(t, "build")
	if err != nil {
		t.Fatal(err)
	}
	if got := readInvocation(t, record); got.args != "-DIDF_TARGET=esp32c3 build" {
		t.Errorf("idf.py %s, want idf.py -DIDF_TARGET=esp32c3 build", got.args)
	}
	if want := "Target: ESP32-C3\nBoard: ESP32-C3-DevKitM-1\n\nBuilding project...\n"; !strings.Contains(out, want) {
		t.Errorf("output %q does not contain %q", out, want)
	}
}

// Every error is reported before the project directory is created.
func TestNewErrors(t *testing.T) {
	tests := []struct {
		flags   []string
		wantErr string
	}{
		{[]string{"--platform", "stm32"}, "unsupported platform \"stm32\"\n\nSupported platforms:\n  esp32"},
		{[]string{"--target", "banana"}, `unsupported target "banana"`},
		{[]string{"--board", "nucleo-f401re"}, `unsupported board "nucleo-f401re"`},
		{[]string{"--target", "esp32-s3", "--board", "esp32-c3-devkitm-1"},
			`target "esp32-s3" does not match board "esp32-c3-devkitm-1", whose target is "esp32-c3"`},
	}
	for _, tt := range tests {
		t.Chdir(t.TempDir())
		err := run(t, append([]string{"new", "app"}, tt.flags...)...)
		if err == nil || !strings.HasPrefix(err.Error(), tt.wantErr) {
			t.Errorf("%q: error = %v, want %q", tt.flags, err, tt.wantErr)
		}
		if _, err := os.Stat("app"); err == nil {
			t.Errorf("%q: the project directory was created", tt.flags)
		}
	}
}

// A parrot.json written before Parrot had platforms names none: it is an
// ESP32 project, and building it does not rewrite it.
func TestManifestWithoutPlatform(t *testing.T) {
	newProject(t, "esp32-c3")
	old := []byte(`{"target": "esp32-c3"}`)
	if err := os.WriteFile("parrot.json", old, 0o644); err != nil {
		t.Fatal(err)
	}
	record := installFakeIDF(t)
	mustRun(t, "build")
	if got := readInvocation(t, record); got.args != "-DIDF_TARGET=esp32c3 build" {
		t.Errorf("idf.py %s, want idf.py -DIDF_TARGET=esp32c3 build", got.args)
	}
	if data, _ := os.ReadFile("parrot.json"); string(data) != string(old) {
		t.Errorf("parrot.json rewritten to %q", data)
	}
}
