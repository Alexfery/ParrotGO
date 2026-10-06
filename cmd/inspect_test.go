package cmd

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"parrot/internal/espidf"
	"parrot/internal/inspect"
	"parrot/internal/project"
)

// newInspectDemo creates the project of the manual test in a temporary
// directory and moves into it.
func newInspectDemo(t *testing.T) {
	t.Helper()
	t.Chdir(t.TempDir())
	mustRun(t, "new", "inspect-demo", "--target", "esp32")
	t.Chdir("inspect-demo")
	mustRun(t, "add", "led", "status", "--pin", "4")
	mustRun(t, "add", "pwm", "motor", "--pin", "5", "--frequency", "5000")
	mustRun(t, "add", "i2c", "sensors", "--sda", "21", "--scl", "22")
	mustRun(t, "add", "i2c-device", "environment-device", "--bus", "sensors", "--address", "0x76", "--frequency", "400000")
	mustRun(t, "add", "sensor", "bme280", "environment", "--device", "environment-device")
}

const inspectDemoReport = `Parrot Project

Target
------

ESP32
Parrot ID: esp32
ESP-IDF target: esp32

Components
----------

status
└── LED
    └── GPIO4

motor
└── PWM
    ├── GPIO5
    ├── Frequency: 5000 Hz
    ├── Duty Resolution: 13 bits
    ├── LEDC Timer: 0
    └── LEDC Channel: 0

sensors
└── I2C Bus
    ├── SDA: GPIO21
    ├── SCL: GPIO22
    └── Devices
        └── environment_device (I2C Device)
            ├── Address: 0x76
            ├── Frequency: 400000 Hz
            └── Sensor
                └── environment (BME280)

No problems detected.
`

func TestInspect(t *testing.T) {
	newInspectDemo(t)
	out, err := runOutput(t, "inspect")
	if err != nil {
		t.Fatal(err)
	}
	if out != inspectDemoReport {
		t.Errorf("output =\n%s\nwant\n%s", out, inspectDemoReport)
	}
}

// The LEDC timer and channel inspect shows are the ones in the generated code.
func TestInspectMatchesGeneration(t *testing.T) {
	t.Chdir(t.TempDir())
	mustRun(t, "new", "pwm-demo", "--target", "esp32-c3")
	t.Chdir("pwm-demo")
	pwms := []struct{ name, pin, frequency string }{
		{"brightness", "4", "5000"},
		{"fan", "6", "25000"},
		{"buzzer", "7", "2000"},
		{"backlight", "8", "5000"},
	}
	for _, p := range pwms {
		mustRun(t, "add", "pwm", p.name, "--pin", p.pin, "--frequency", p.frequency)
	}

	cfg, err := project.LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range inspect.Resolve(cfg).Components {
		values := map[string]string{}
		for _, p := range c.Properties {
			values[p.Name] = p.Value
		}
		source, err := os.ReadFile(filepath.Join("components", c.Name, c.Name+".c"))
		if err != nil {
			t.Fatal(err)
		}
		for _, symbol := range []string{"LEDC_TIMER_" + values["LEDC Timer"], "LEDC_CHANNEL_" + values["LEDC Channel"]} {
			if !strings.Contains(string(source), symbol+"\n") {
				t.Errorf("inspect gives %s %s, which %s.c does not use", c.Name, symbol, c.Name)
			}
		}
	}
}

// snapshot records every file of the project: its content and when it was
// last written.
func snapshot(t *testing.T) map[string]string {
	t.Helper()
	files := map[string]string{}
	err := filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		files[path] = fmt.Sprintf("%s\n%s", info.ModTime().Format(time.RFC3339Nano), data)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

// inspect reads parrot.json and writes nothing, not even the migration of an
// old manifest, which parrot add would save.
func TestInspectIsReadOnly(t *testing.T) {
	newInspectDemo(t)
	before := snapshot(t)
	mustRun(t, "inspect")
	after := snapshot(t)
	if len(after) != len(before) {
		t.Errorf("inspect changed the files of the project: %d before, %d after", len(before), len(after))
	}
	for path, state := range before {
		if after[path] != state {
			t.Errorf("inspect changed %s", path)
		}
	}

	old := `{"target": "esp32", "components": [{"type": "led", "name": "status", "pin": 4}]}`
	if err := os.WriteFile(project.ConfigFile, []byte(old), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := runOutput(t, "inspect")
	if err != nil || !strings.Contains(out, "status\n└── LED\n    └── GPIO4\n") {
		t.Errorf("inspect of an old manifest: error = %v, output =\n%s", err, out)
	}
	if data, _ := os.ReadFile(project.ConfigFile); string(data) != old {
		t.Errorf("inspect rewrote the old manifest:\n%s", data)
	}
}

// inspect never runs idf.py: it works with ESP-IDF out of reach.
func TestInspectWithoutESPIDF(t *testing.T) {
	newInspectDemo(t)
	t.Setenv("PATH", t.TempDir())
	t.Setenv("IDF_PATH", "")
	if _, err := espidf.FindIDF(); err == nil {
		t.Fatal("idf.py is still on PATH")
	}
	out, err := runOutput(t, "inspect")
	if err != nil || out != inspectDemoReport {
		t.Errorf("inspect without ESP-IDF: error = %v, output =\n%s", err, out)
	}
}

func writeManifest(t *testing.T, manifest string) {
	t.Helper()
	t.Chdir(t.TempDir())
	if err := os.WriteFile(project.ConfigFile, []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
}

// A structurally invalid manifest is reported in full and makes the command
// fail; warnings alone do not.
func TestInspectInvalidManifest(t *testing.T) {
	writeManifest(t, `{"target": "esp32", "components": [
		{"type": "led", "name": "status", "config": {"pin": 4}},
		{"type": "sensor-bme280", "name": "environment", "config": {"device": "environment_device"}},
		{"type": "sensor-bme280", "name": "outdoor", "config": {"device": "status"}},
		{"type": "button", "name": "status", "config": {"pin": 18}}
	]}`)
	out, err := runOutput(t, "inspect")
	if err == nil || err.Error() != "3 problems detected in parrot.json" {
		t.Errorf("error = %v, want 3 problems", err)
	}
	for _, want := range []string{
		"environment\n└── BME280\n    └── ERROR: dependency \"environment_device\" not found\n",
		"outdoor\n└── BME280\n    └── ERROR: \"status\" is not an I2C device (its type is \"led\")\n",
		"    └── ERROR: duplicate component name \"status\": an earlier component in parrot.json has it\n",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output does not contain\n%s\nin\n%s", want, out)
		}
	}

	writeManifest(t, `{"target": "esp32", "components": [{"type": "future-component", "name": "something"}]}`)
	out, err = runOutput(t, "inspect")
	if err != nil {
		t.Errorf("unknown type only: error = %v, want none", err)
	}
	want := "something\n└── future-component\n    └── WARNING: unknown component type: this version of Parrot cannot inspect it\n\nNo problems detected (1 warning).\n"
	if !strings.HasSuffix(out, want) {
		t.Errorf("output =\n%s\nwant suffix\n%s", out, want)
	}
}

func TestInspectErrors(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := run(t, "inspect"); err == nil || err.Error() != "current directory is not a Parrot project" {
		t.Errorf("outside a project: error = %v", err)
	}
	writeManifest(t, `{"target": "esp32",`)
	if err := run(t, "inspect"); err == nil || !strings.HasPrefix(err.Error(), "invalid parrot.json") {
		t.Errorf("unreadable parrot.json: error = %v", err)
	}
	writeManifest(t, `{"target": "esp32"}`)
	if err := run(t, "inspect", "extra"); err == nil {
		t.Error("inspect accepted an argument")
	}
}

func TestInspectHelp(t *testing.T) {
	out, err := runOutput(t, "inspect", "--help")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(out, "Inspect the hardware structure of the current Parrot project.\n") ||
		!strings.Contains(out, "Usage:\n  parrot inspect") {
		t.Errorf("help =\n%s", out)
	}
}
