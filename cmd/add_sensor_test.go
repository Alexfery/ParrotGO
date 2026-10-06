package cmd

import (
	"os"
	"strings"
	"testing"
)

func TestAddSensorBME280(t *testing.T) {
	t.Chdir(t.TempDir())
	mustRun(t, "new", "weather", "--target", "esp32")
	t.Chdir("weather")

	mustRun(t, "add", "i2c", "sensors", "--sda", "21", "--scl", "22")
	mustRun(t, "add", "i2c-device", "environment-device", "--bus", "sensors", "--address", "0x76", "--frequency", "400000")
	mustRun(t, "add", "i2c-device", "outdoor-device", "--bus", "sensors", "--address", "0x77", "--frequency", "100000")
	mustRun(t, "add", "i2c-device", "display", "--bus", "sensors", "--address", "0x3C", "--frequency", "400000")
	mustRun(t, "add", "led", "status", "--pin", "4")
	devices, err := os.ReadFile("parrot.json")
	if err != nil {
		t.Fatal(err)
	}

	out, err := runOutput(t, "add", "sensor", "bme280", "environment", "--device", "environment-device")
	if err != nil {
		t.Fatal(err)
	}
	want := `Adding BME280 sensor: environment
Target: ESP32
Device: environment_device
Bus: sensors
Address: 0x76

✓ Validated BME280 transport
✓ Created components/environment
✓ Created components/environment/environment.c
✓ Created components/environment/include/environment.h
✓ Created components/environment/CMakeLists.txt
✓ Updated parrot.json

BME280 sensor component added successfully.
`
	if out != want {
		t.Errorf("output =\n%s\nwant\n%s", out, want)
	}
	// The other address of a BME280.
	mustRun(t, "add", "sensor", "bme280", "outdoor", "--device", "outdoor_device")

	rejected := []struct {
		args    []string
		wantErr string
	}{
		{[]string{"add", "sensor", "bme280", "lost", "--device", "nodevice"},
			`I2C device "nodevice" does not exist`},
		{[]string{"add", "sensor", "bme280", "on_led", "--device", "status"},
			`component "status" is not an I2C device`},
		{[]string{"add", "sensor", "bme280", "on_bus", "--device", "sensors"},
			`component "sensors" is not an I2C device`},
		{[]string{"add", "sensor", "bme280", "on_display", "--device", "display"},
			`I2C device "display" uses address 0x3C; BME280 expects 0x76 or 0x77`},
		{[]string{"add", "sensor", "bme280", "environment", "--device", "outdoor_device"},
			`component "environment" already exists`},
		{[]string{"add", "sensor", "bme280", "second", "--device", "environment-device"},
			`component "environment_device" is already driven by component "environment"`},
		{[]string{"add", "sensor", "bme280", "unplugged"},
			`required flag(s) "device" not set`},
		{[]string{"add", "sensor", "bmp280", "pressure", "--device", "environment_device"},
			`unknown sensor "bmp280" (supported: bme280)`},
		// The sensor took neither the bus's GPIOs nor the device's address.
		{[]string{"add", "led", "on_sda", "--pin", "21"},
			`GPIO21 is already used by component "sensors"`},
		{[]string{"add", "i2c-device", "clash", "--bus", "sensors", "--address", "0x76", "--frequency", "100000"},
			`I2C address 0x76 is already used on bus "sensors" by component "environment_device"`},
	}
	for _, tt := range rejected {
		err := run(t, tt.args...)
		if err == nil || err.Error() != tt.wantErr {
			t.Errorf("parrot %v: error = %v, want %q", tt.args, err, tt.wantErr)
		}
	}
	for _, name := range []string{"lost", "on_led", "on_bus", "on_display", "second", "unplugged", "pressure", "on_sda", "clash"} {
		if _, err := os.Stat("components/" + name); err == nil {
			t.Errorf("components/%s was created by a rejected command", name)
		}
	}

	// environment depends on its device only; the device on its bus.
	for path, want := range map[string]string{
		"components/environment/CMakeLists.txt":        "    PRIV_REQUIRES environment_device\n",
		"components/outdoor/CMakeLists.txt":            "    PRIV_REQUIRES outdoor_device\n",
		"components/environment_device/CMakeLists.txt": "    PRIV_REQUIRES sensors\n",
	} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(data), want) {
			t.Errorf("%s does not contain %q:\n%s", path, want, data)
		}
	}

	// The sensors only name their devices, whose entries are unchanged.
	data, err := os.ReadFile("parrot.json")
	if err != nil {
		t.Fatal(err)
	}
	wantManifest := `{
  "platform": "esp32",
  "target": "esp32",
  "components": [
    {
      "type": "i2c-bus",
      "name": "sensors",
      "config": {
        "sda": 21,
        "scl": 22
      }
    },
    {
      "type": "i2c-device",
      "name": "environment_device",
      "config": {
        "bus": "sensors",
        "address": 118,
        "frequency": 400000
      }
    },
    {
      "type": "i2c-device",
      "name": "outdoor_device",
      "config": {
        "bus": "sensors",
        "address": 119,
        "frequency": 100000
      }
    },
    {
      "type": "i2c-device",
      "name": "display",
      "config": {
        "bus": "sensors",
        "address": 60,
        "frequency": 400000
      }
    },
    {
      "type": "led",
      "name": "status",
      "config": {
        "pin": 4
      }
    },
    {
      "type": "sensor-bme280",
      "name": "environment",
      "config": {
        "device": "environment_device"
      }
    },
    {
      "type": "sensor-bme280",
      "name": "outdoor",
      "config": {
        "device": "outdoor_device"
      }
    }
  ]
}
`
	if string(data) != wantManifest {
		t.Errorf("parrot.json =\n%s\nwant\n%s", data, wantManifest)
	}
	if !strings.HasPrefix(string(data), strings.TrimSuffix(string(devices), "\n  ]\n}\n")) {
		t.Error("adding the sensors changed the entries before them")
	}
}

func TestAddSensorHelp(t *testing.T) {
	t.Chdir(t.TempDir())
	out, err := runOutput(t, "add", "sensor")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "bme280") {
		t.Errorf("parrot add sensor does not list bme280:\n%s", out)
	}
}
