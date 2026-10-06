package project_test

import (
	"os"
	"testing"

	"parrot/internal/project"
)

type pinConfig struct {
	Pin int `json:"pin"`
}

type pwmConfig struct {
	Pin       int `json:"pin"`
	Frequency int `json:"frequency"`
}

func mustEntry(t *testing.T, typ, name string, config any) project.ComponentConfig {
	t.Helper()
	c, err := project.NewComponentConfig(typ, name, config)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestConfigRoundTrip(t *testing.T) {
	t.Chdir(t.TempDir())
	cfg := project.Config{
		Target: "esp32-c3",
		Components: []project.ComponentConfig{
			mustEntry(t, "led", "status", pinConfig{Pin: 4}),
			mustEntry(t, "pwm", "fan", pwmConfig{Pin: 5, Frequency: 25000}),
		},
	}
	if err := project.SaveConfig(cfg); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(project.ConfigFile)
	if err != nil {
		t.Fatal(err)
	}
	wantJSON := `{
  "target": "esp32-c3",
  "components": [
    {
      "type": "led",
      "name": "status",
      "config": {
        "pin": 4
      }
    },
    {
      "type": "pwm",
      "name": "fan",
      "config": {
        "pin": 5,
        "frequency": 25000
      }
    }
  ]
}
`
	if string(data) != wantJSON {
		t.Errorf("parrot.json =\n%s\nwant\n%s", data, wantJSON)
	}

	got, err := project.LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if got.Target != "esp32-c3" || len(got.Components) != 2 {
		t.Fatalf("LoadConfig = %+v", got)
	}
	var fan pwmConfig
	if err := got.Components[1].Decode(&fan); err != nil {
		t.Fatal(err)
	}
	if fan != (pwmConfig{Pin: 5, Frequency: 25000}) {
		t.Errorf("decoded fan config = %+v", fan)
	}
}

// Manifests written before the "config" object had a top-level pin.
func TestLoadConfigMigratesTopLevelPin(t *testing.T) {
	t.Chdir(t.TempDir())
	old := `{"target": "esp32", "components": [{"type": "led", "name": "status", "pin": 4}]}`
	if err := os.WriteFile(project.ConfigFile, []byte(old), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := project.LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	var led pinConfig
	if err := cfg.Components[0].Decode(&led); err != nil || led.Pin != 4 {
		t.Fatalf("decoded config = %+v, %v; want pin 4", led, err)
	}

	if err := project.SaveConfig(cfg); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(project.ConfigFile)
	want := `{
  "target": "esp32",
  "components": [
    {
      "type": "led",
      "name": "status",
      "config": {
        "pin": 4
      }
    }
  ]
}
`
	if string(data) != want {
		t.Errorf("migrated parrot.json =\n%s\nwant\n%s", data, want)
	}
}

func TestDecodeRejectsUnknownFields(t *testing.T) {
	c := mustEntry(t, "pwm", "fan", pwmConfig{Pin: 5, Frequency: 25000})
	var led pinConfig
	if err := c.Decode(&led); err == nil {
		t.Error("Decode of a PWM config into an LED config succeeded, want error")
	}
}

func TestConfigWithoutComponents(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := project.SaveConfig(project.Config{Target: "esp32"}); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(project.ConfigFile)
	if want := "{\n  \"target\": \"esp32\"\n}\n"; string(data) != want {
		t.Errorf("parrot.json = %q, want %q", data, want)
	}
}

func TestConfigWithPlatformAndBoard(t *testing.T) {
	t.Chdir(t.TempDir())
	cfg := project.Config{Platform: "esp32", Target: "esp32-c3", Board: "esp32-c3-devkitm-1"}
	if err := project.SaveConfig(cfg); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(project.ConfigFile)
	want := `{
  "platform": "esp32",
  "target": "esp32-c3",
  "board": "esp32-c3-devkitm-1"
}
`
	if string(data) != want {
		t.Errorf("parrot.json =\n%s\nwant\n%s", data, want)
	}
	got, err := project.LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if got.Platform != cfg.Platform || got.Target != cfg.Target || got.Board != cfg.Board {
		t.Errorf("LoadConfig = %+v, want %+v", got, cfg)
	}
}

// Manifests written before Parrot had platforms are ESP32 projects, and are
// not rewritten to say so.
func TestPlatformID(t *testing.T) {
	if got := (project.Config{Target: "esp32"}).PlatformID(); got != "esp32" {
		t.Errorf("PlatformID without a platform = %q, want esp32", got)
	}
	if got := (project.Config{Platform: "stm32", Target: "stm32f401re"}).PlatformID(); got != "stm32" {
		t.Errorf("PlatformID = %q, want stm32", got)
	}
}

func TestLoadConfigNotProject(t *testing.T) {
	t.Chdir(t.TempDir())
	_, err := project.LoadConfig()
	if err == nil || err.Error() != "current directory is not a Parrot project" {
		t.Errorf("LoadConfig error = %v", err)
	}
}

func TestAddComponentDuplicate(t *testing.T) {
	cfg := project.Config{Target: "esp32"}
	if err := cfg.AddComponent(mustEntry(t, "led", "status", pinConfig{Pin: 4})); err != nil {
		t.Fatal(err)
	}
	err := cfg.AddComponent(mustEntry(t, "button", "status", pinConfig{Pin: 5}))
	if err == nil || err.Error() != `component "status" already exists` {
		t.Errorf("duplicate AddComponent error = %v", err)
	}
	if len(cfg.Components) != 1 {
		t.Errorf("components = %+v, want only the original", cfg.Components)
	}
}

func TestComponent(t *testing.T) {
	cfg := project.Config{Components: []project.ComponentConfig{
		{Type: "i2c-bus", Name: "sensors"},
		{Type: "led", Name: "status"},
	}}
	if c, found := cfg.Component("status"); !found || c.Type != "led" {
		t.Errorf(`Component("status") = %+v, %v; want the LED`, c, found)
	}
	if _, found := cfg.Component("display"); found {
		t.Error(`Component("display") found a component that does not exist`)
	}
}
