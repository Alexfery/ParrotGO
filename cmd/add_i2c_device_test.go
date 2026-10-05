package cmd

import (
	"os"
	"strings"
	"testing"
)

func TestAddI2CDevice(t *testing.T) {
	t.Chdir(t.TempDir())
	mustRun(t, "new", "i2c-devices", "--target", "esp32")
	t.Chdir("i2c-devices")

	mustRun(t, "add", "i2c", "sensors", "--sda", "21", "--scl", "22")
	mustRun(t, "add", "i2c", "aux", "--sda", "25", "--scl", "26")
	mustRun(t, "add", "led", "status", "--pin", "4")

	out, err := runOutput(t, "add", "i2c-device", "display", "--bus", "sensors", "--address", "0x3C", "--frequency", "400000")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"Adding I2C device: display\n",
		"Bus: sensors (SDA GPIO21, SCL GPIO22)\n",
		"Address: 0x3C\n",
		"Frequency: 400000 Hz\n",
		"✓ Created components/display/display.c\n",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output does not contain %q:\n%s", want, out)
		}
	}
	// Decimal address; bus name as the user may type it.
	mustRun(t, "add", "i2c-device", "imu", "--bus", "Sensors", "--address", "104", "--frequency", "400000")
	// The same address on another bus is another device.
	mustRun(t, "add", "i2c-device", "display2", "--bus", "aux", "--address", "60", "--frequency", "100000")
	// Devices take no GPIO: other components can still use free pins.
	mustRun(t, "add", "button", "user", "--pin", "18")

	rejected := []struct {
		args    []string
		wantErr string
	}{
		{[]string{"add", "i2c-device", "conflict", "--bus", "sensors", "--address", "60", "--frequency", "100000"},
			`I2C address 0x3C is already used on bus "sensors" by component "display"`},
		{[]string{"add", "i2c-device", "lost", "--bus", "nobus", "--address", "0x10", "--frequency", "100000"},
			`I2C bus "nobus" does not exist`},
		{[]string{"add", "i2c-device", "on_led", "--bus", "status", "--address", "0x10", "--frequency", "100000"},
			`component "status" is not an I2C bus`},
		{[]string{"add", "i2c-device", "display", "--bus", "aux", "--address", "0x10", "--frequency", "100000"},
			`component "display" already exists`},
		{[]string{"add", "i2c-device", "wide", "--bus", "sensors", "--address", "0x3FF", "--frequency", "100000"},
			"I2C address 0x3FF does not fit in 7 bits (10-bit addresses are not supported)"},
		{[]string{"add", "i2c-device", "eight", "--bus", "sensors", "--address", "0x78", "--frequency", "100000"},
			"I2C address 0x78 is reserved by the I2C specification (0x00-0x07 and 0x78-0x7F); if 0x78 is an 8-bit address that includes the R/W bit, use 0x3C"},
		{[]string{"add", "i2c-device", "text", "--bus", "sensors", "--address", "display", "--frequency", "100000"},
			`invalid I2C address "display": write it in hexadecimal (0x3C) or decimal (60)`},
		{[]string{"add", "i2c-device", "still", "--bus", "sensors", "--address", "0x10", "--frequency", "0"},
			"frequency must be positive, got 0 Hz"},
		{[]string{"add", "i2c-device", "half", "--bus", "sensors", "--address", "0x10"},
			`required flag(s) "frequency" not set`},
		{[]string{"add", "led", "on_bus", "--pin", "21"},
			`GPIO21 is already used by component "sensors"`},
	}
	for _, tt := range rejected {
		err := run(t, tt.args...)
		if err == nil || err.Error() != tt.wantErr {
			t.Errorf("parrot %v: error = %v, want %q", tt.args, err, tt.wantErr)
		}
	}
	for _, name := range []string{"conflict", "lost", "on_led", "wide", "eight", "text", "still", "half", "on_bus"} {
		if _, err := os.Stat("components/" + name); err == nil {
			t.Errorf("components/%s was created by a rejected command", name)
		}
	}

	cmake, err := os.ReadFile("components/imu/CMakeLists.txt")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(cmake), "PRIV_REQUIRES sensors\n") {
		t.Errorf("imu does not depend on its bus:\n%s", cmake)
	}
	source, err := os.ReadFile("components/display2/display2.c")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(source), "aux_get_handle()") || !strings.Contains(string(source), ".device_address = 0x3C,") {
		t.Errorf("display2.c does not use 0x3C on the aux bus:\n%s", source)
	}

	data, err := os.ReadFile("parrot.json")
	if err != nil {
		t.Fatal(err)
	}
	want := `{
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
      "type": "i2c-bus",
      "name": "aux",
      "config": {
        "sda": 25,
        "scl": 26
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
      "type": "i2c-device",
      "name": "display",
      "config": {
        "bus": "sensors",
        "address": 60,
        "frequency": 400000
      }
    },
    {
      "type": "i2c-device",
      "name": "imu",
      "config": {
        "bus": "sensors",
        "address": 104,
        "frequency": 400000
      }
    },
    {
      "type": "i2c-device",
      "name": "display2",
      "config": {
        "bus": "aux",
        "address": 60,
        "frequency": 100000
      }
    },
    {
      "type": "button",
      "name": "user",
      "config": {
        "pin": 18
      }
    }
  ]
}
`
	if string(data) != want {
		t.Errorf("parrot.json =\n%s\nwant\n%s", data, want)
	}
}
