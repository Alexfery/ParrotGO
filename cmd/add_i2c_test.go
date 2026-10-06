package cmd

import (
	"os"
	"strings"
	"testing"
)

func TestAddI2C(t *testing.T) {
	t.Chdir(t.TempDir())
	mustRun(t, "new", "i2c-demo", "--target", "esp32")
	t.Chdir("i2c-demo")

	mustRun(t, "add", "led", "status", "--pin", "2")
	out, err := runOutput(t, "add", "i2c", "sensors", "--sda", "21", "--scl", "22")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Adding I2C bus: sensors\n", "SDA: GPIO21\n", "SCL: GPIO22\n", "I2C controllers: 1 of 2 in use"} {
		if !strings.Contains(out, want) {
			t.Errorf("output does not contain %q:\n%s", want, out)
		}
	}
	mustRun(t, "add", "pwm", "fan", "--pin", "18", "--frequency", "5000")

	// Rejected while one of the two controllers is still free, so that each
	// error comes from the check under test.
	rejected := []struct {
		args    []string
		wantErr string
	}{
		{[]string{"add", "i2c", "same", "--sda", "19", "--scl", "19"}, "SDA and SCL cannot use the same GPIO"},
		{[]string{"add", "i2c", "on_led", "--sda", "2", "--scl", "19"}, `GPIO2 is already used by component "status"`},
		{[]string{"add", "i2c", "on_fan", "--sda", "19", "--scl", "18"}, `GPIO18 is already used by component "fan"`},
		{[]string{"add", "led", "on_bus", "--pin", "22"}, `GPIO22 is already used by component "sensors"`},
		{[]string{"add", "i2c", "input", "--sda", "34", "--scl", "19"}, "GPIO34 cannot be used for I2C SDA on ESP32: I2C lines need a GPIO that is both input and output"},
		{[]string{"add", "i2c", "flash", "--sda", "19", "--scl", "6"}, "GPIO6 is reserved for the SPI flash on ESP32"},
		{[]string{"add", "i2c", "far", "--sda", "19", "--scl", "50"}, "GPIO50 is not available on ESP32"},
		{[]string{"add", "i2c", "sensors", "--sda", "19", "--scl", "23"}, `component "sensors" already exists`},
		{[]string{"add", "i2c", "half", "--sda", "19"}, `required flag(s) "scl" not set`},
	}
	for _, tt := range rejected {
		err := run(t, tt.args...)
		if err == nil || err.Error() != tt.wantErr {
			t.Errorf("parrot %v: error = %v, want %q", tt.args, err, tt.wantErr)
		}
	}

	// ESP32 has two HP I2C controllers: a second bus fits, a third does not.
	mustRun(t, "add", "i2c", "display", "--sda", "25", "--scl", "26")
	if err := run(t, "add", "i2c", "third", "--sda", "19", "--scl", "23"); err == nil || err.Error() != "no I2C master controllers available on ESP32" {
		t.Errorf("third bus: error = %v", err)
	}

	for _, path := range []string{
		"components/sensors/sensors.c",
		"components/sensors/include/sensors.h",
		"components/sensors/CMakeLists.txt",
		"components/display/display.c",
	} {
		if _, err := os.Stat(path); err != nil {
			t.Errorf("missing %s: %v", path, err)
		}
	}
	for _, name := range []string{"same", "on_led", "on_fan", "on_bus", "input", "flash", "far", "half", "third"} {
		if _, err := os.Stat("components/" + name); err == nil {
			t.Errorf("components/%s was created by a rejected command", name)
		}
	}

	data, err := os.ReadFile("parrot.json")
	if err != nil {
		t.Fatal(err)
	}
	want := `{
  "platform": "esp32",
  "target": "esp32",
  "components": [
    {
      "type": "led",
      "name": "status",
      "config": {
        "pin": 2
      }
    },
    {
      "type": "i2c-bus",
      "name": "sensors",
      "config": {
        "sda": 21,
        "scl": 22
      }
    },
    {
      "type": "pwm",
      "name": "fan",
      "config": {
        "pin": 18,
        "frequency": 5000
      }
    },
    {
      "type": "i2c-bus",
      "name": "display",
      "config": {
        "sda": 25,
        "scl": 26
      }
    }
  ]
}
`
	if string(data) != want {
		t.Errorf("parrot.json =\n%s\nwant\n%s", data, want)
	}
}

// ESP32-C3 and ESP32-C6 have one HP I2C controller (the C6's LP I2C is not used).
func TestAddI2CSingleController(t *testing.T) {
	for target, name := range map[string]string{"esp32-c3": "ESP32-C3", "esp32-c6": "ESP32-C6"} {
		newProject(t, target)
		mustRun(t, "add", "i2c", "sensors", "--sda", "4", "--scl", "5")
		err := run(t, "add", "i2c", "second", "--sda", "6", "--scl", "7")
		if want := "no I2C master controllers available on " + name; err == nil || err.Error() != want {
			t.Errorf("%s: second bus: error = %v, want %q", target, err, want)
		}
	}
}
