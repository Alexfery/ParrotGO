package inspect_test

import (
	"bytes"
	"errors"
	"testing"

	"parrot/internal/inspect"
	"parrot/internal/project"
)

func render(t *testing.T, in inspect.Inspection) string {
	t.Helper()
	var out bytes.Buffer
	r := inspect.TextRenderer{Writer: &out}
	if err := r.Render(in); err != nil {
		t.Fatal(err)
	}
	return out.String()
}

func checkOutput(t *testing.T, got, want string) {
	t.Helper()
	if got != want {
		t.Errorf("output =\n%s\nwant\n%s", got, want)
	}
}

func TestRenderEmptyProject(t *testing.T) {
	checkOutput(t, render(t, resolve("esp32-c3")), `Parrot Project

Target
------

ESP32-C3
Parrot ID: esp32-c3
ESP-IDF target: esp32c3

Components
----------

No components yet. Add one with parrot add.
`)
}

func TestRenderProject(t *testing.T) {
	in := resolve("esp32",
		status,
		component("button", "user", `{"pin": 18}`),
		component("adc", "light", `{"pin": 34}`),
		motor,
		sensors,
		envDevice,
		display,
		environment,
	)
	checkOutput(t, render(t, in), `Parrot Project

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

user
└── Button
    └── GPIO18

light
└── ADC
    ├── GPIO34
    ├── ADC Unit: 1
    └── ADC Channel: 6

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
        ├── environment_device (I2C Device)
        │   ├── Address: 0x76
        │   ├── Frequency: 400000 Hz
        │   └── Sensor
        │       └── environment (BME280)
        └── display (I2C Device)
            ├── Address: 0x3C
            └── Frequency: 100000 Hz
`)
}

func TestRenderIssues(t *testing.T) {
	in := resolve("esp32",
		component("future-component", "something", `{}`),
		status,
		environment,
		component("sensor-bme280", "outdoor", `{"device": "status"}`),
		component("button", "status", `{"pin": 18}`),
	)
	checkOutput(t, render(t, in), `Parrot Project

Target
------

ESP32
Parrot ID: esp32
ESP-IDF target: esp32

Components
----------

something
└── future-component
    └── WARNING: unknown component type: this version of Parrot cannot inspect it

status
└── LED
    └── GPIO4

environment
└── BME280
    └── ERROR: dependency "environment_device" not found

outdoor
└── BME280
    └── ERROR: "status" is not an I2C device (its type is "led")

status
└── Button
    ├── GPIO18
    └── ERROR: duplicate component name "status": an earlier component in parrot.json has it
`)
}

func TestRenderUnsupportedTarget(t *testing.T) {
	checkOutput(t, render(t, resolve("esp99", status)), `Parrot Project

Target
------

ERROR: unsupported target "esp99" (supported: esp32, esp32-c3, esp32-s3, esp32-c6)

Components
----------

status
└── LED
    └── GPIO4
`)
}

// The renderer only needs an Inspection, not a parrot.json: here one built by
// hand, with an issue on a nested component and a vertical bar that must
// connect a sibling below a subtree.
func TestRenderInspectionModel(t *testing.T) {
	in := inspect.Inspection{
		Target: inspect.TargetInfo{ID: "esp32", DisplayName: "ESP32", IDFTarget: "esp32"},
		Components: []inspect.Component{{
			Name: "bus", Label: "I2C Bus",
			Properties: []inspect.Property{{Name: "SDA", Value: "GPIO21"}},
			Children: []inspect.Group{{Name: "Devices", Components: []inspect.Component{
				{Name: "a", Label: "I2C Device", Issues: []inspect.Issue{{Severity: inspect.SeverityWarning, Message: "check me"}}},
				{Name: "b", Type: "custom"},
			}}},
		}},
	}
	checkOutput(t, render(t, in), `Parrot Project

Target
------

ESP32
Parrot ID: esp32
ESP-IDF target: esp32

Components
----------

bus
└── I2C Bus
    ├── SDA: GPIO21
    └── Devices
        ├── a (I2C Device)
        │   └── WARNING: check me
        └── b (custom)
`)
}

// Resolving and rendering the same manifest always gives the same text.
func TestRenderDeterministic(t *testing.T) {
	cfg := project.Config{Target: "esp32", Components: []project.ComponentConfig{
		environment, display, component("pwm", "fan", `{"pin": 18, "frequency": 25000}`), sensors, envDevice, status,
	}}
	first := render(t, inspect.Resolve(cfg))
	for range 50 {
		if again := render(t, inspect.Resolve(cfg)); again != first {
			t.Fatalf("output changed:\n%s\nthen\n%s", first, again)
		}
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("disk full") }

func TestRenderWriteError(t *testing.T) {
	r := inspect.TextRenderer{Writer: failingWriter{}}
	if err := r.Render(resolve("esp32", status)); err == nil || err.Error() != "disk full" {
		t.Errorf("Render error = %v, want disk full", err)
	}
}

func TestRenderSPIBus(t *testing.T) {
	in := resolve("esp32",
		component("spi-bus", "main_bus", `{"mosi": 23, "miso": 19, "sclk": 18}`),
		component("spi-bus", "display_bus", `{"mosi": 13, "sclk": 14}`),
	)
	checkOutput(t, render(t, in), `Parrot Project

Target
------

ESP32
Parrot ID: esp32
ESP-IDF target: esp32

Components
----------

main_bus
└── SPI Bus
    ├── Host: SPI2_HOST
    ├── MOSI: GPIO23
    ├── MISO: GPIO19
    └── SCLK: GPIO18

display_bus
└── SPI Bus
    ├── Host: SPI3_HOST
    ├── MOSI: GPIO13
    ├── MISO: disabled
    └── SCLK: GPIO14
`)
}
