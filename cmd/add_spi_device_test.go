package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"parrot/internal/project"
)

// newSPIDeviceDemo creates the project of the manual test, with its bus, in a
// temporary directory and moves into it.
func newSPIDeviceDemo(t *testing.T) {
	t.Helper()
	t.Chdir(t.TempDir())
	mustRun(t, "new", "spi-device-demo", "--target", "esp32")
	t.Chdir("spi-device-demo")
	mustRun(t, "add", "spi", "main-bus", "--mosi", "23", "--miso", "19", "--sclk", "18")
}

func TestAddSPIDevice(t *testing.T) {
	newSPIDeviceDemo(t)
	busFiles := snapshot(t)

	out, err := runOutput(t, "add", "spi-device", "display", "--bus", "main-bus", "--cs", "5", "--frequency", "10000000", "--mode", "0")
	if err != nil {
		t.Fatal(err)
	}
	want := `Adding SPI device: display
Target: ESP32
Bus: main_bus (SPI2_HOST: MOSI GPIO23, MISO GPIO19, SCLK GPIO18)
CS: GPIO5
Frequency: 10000000 Hz
Mode: 0 (CPOL 0, CPHA 0)

✓ Created components/display
✓ Created components/display/display.c
✓ Created components/display/include/display.h
✓ Created components/display/CMakeLists.txt
✓ Updated parrot.json

SPI device component added successfully.
`
	if out != want {
		t.Errorf("output =\n%s\nwant\n%s", out, want)
	}
	// A second device shares the bus, with its own CS, frequency and mode.
	mustRun(t, "add", "spi-device", "sensor", "--bus", "main_bus", "--cs", "17", "--frequency", "1000000", "--mode", "3")

	data, err := os.ReadFile(project.ConfigFile)
	if err != nil {
		t.Fatal(err)
	}
	wantManifest := `{
  "platform": "esp32",
  "target": "esp32",
  "components": [
    {
      "type": "spi-bus",
      "name": "main_bus",
      "config": {
        "mosi": 23,
        "miso": 19,
        "sclk": 18
      }
    },
    {
      "type": "spi-device",
      "name": "display",
      "config": {
        "bus": "main_bus",
        "cs": 5,
        "frequency": 10000000,
        "mode": 0
      }
    },
    {
      "type": "spi-device",
      "name": "sensor",
      "config": {
        "bus": "main_bus",
        "cs": 17,
        "frequency": 1000000,
        "mode": 3
      }
    }
  ]
}
`
	if string(data) != wantManifest {
		t.Errorf("parrot.json =\n%s\nwant\n%s", data, wantManifest)
	}

	// Each device depends on the bus in CMake, and asks it for its host.
	for _, name := range []string{"display", "sensor"} {
		cmake, err := os.ReadFile(filepath.Join("components", name, "CMakeLists.txt"))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(cmake), "    PRIV_REQUIRES main_bus\n") {
			t.Errorf("%s/CMakeLists.txt does not require main_bus:\n%s", name, cmake)
		}
		source, err := os.ReadFile(filepath.Join("components", name, name+".c"))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(source), "spi_bus_add_device(main_bus_get_host(), &config, &handle)") {
			t.Errorf("%s.c does not add the device to main_bus", name)
		}
	}
	// The bus is not regenerated or changed by its devices.
	after := snapshot(t)
	for path, content := range busFiles {
		if path != project.ConfigFile && after[path] != content {
			t.Errorf("%s changed when the devices were added", path)
		}
	}
}

// Every SPI mode is accepted, and devices on one bus can differ in mode and
// frequency.
func TestAddSPIDeviceModes(t *testing.T) {
	newSPIDeviceDemo(t)
	for mode, cs := range []int{5, 16, 17, 21} {
		name := fmt.Sprintf("device%d", mode)
		frequency := fmt.Sprint(1_000_000 * (mode + 1))
		out, err := runOutput(t, "add", "spi-device", name, "--bus", "main-bus", "--cs", fmt.Sprint(cs), "--frequency", frequency, "--mode", fmt.Sprint(mode))
		if err != nil {
			t.Fatalf("mode %d: %v", mode, err)
		}
		if want := fmt.Sprintf("Mode: %d (CPOL %d, CPHA %d)\n", mode, mode>>1, mode&1); !strings.Contains(out, want) {
			t.Errorf("mode %d: output does not contain %q:\n%s", mode, want, out)
		}
		source, err := os.ReadFile(filepath.Join("components", name, name+".c"))
		if err != nil {
			t.Fatal(err)
		}
		for _, define := range []string{
			fmt.Sprintf("#define DEVICE%d_MODE %d\n", mode, mode),
			fmt.Sprintf("#define DEVICE%d_FREQUENCY_HZ %s\n", mode, frequency),
			fmt.Sprintf("#define DEVICE%d_CS_GPIO %d\n", mode, cs),
		} {
			if !strings.Contains(string(source), define) {
				t.Errorf("%s.c does not contain %q", name, define)
			}
		}
	}
}

// Rejected commands must leave parrot.json and components/ untouched.
func TestAddSPIDeviceRejected(t *testing.T) {
	newSPIDeviceDemo(t)
	mustRun(t, "add", "led", "status", "--pin", "4")
	mustRun(t, "add", "i2c", "sensors", "--sda", "21", "--scl", "22")
	mustRun(t, "add", "spi-device", "display", "--bus", "main-bus", "--cs", "5", "--frequency", "10000000", "--mode", "0")
	before, err := os.ReadFile(project.ConfigFile)
	if err != nil {
		t.Fatal(err)
	}

	flags := func(bus, cs, frequency, mode string) []string {
		return []string{"--bus", bus, "--cs", cs, "--frequency", frequency, "--mode", mode}
	}
	rejected := []struct {
		args    []string
		wantErr string
	}{
		{flags("nobus", "16", "1000000", "0"), `SPI bus "nobus" does not exist`},
		{flags("status", "16", "1000000", "0"), `component "status" is not a SPI bus`},
		{flags("sensors", "16", "1000000", "0"), `component "sensors" is not a SPI bus`},
		{flags("display", "16", "1000000", "0"), `component "display" is not a SPI bus`},
		{flags("main-bus", "50", "1000000", "0"), "GPIO50 is not available on ESP32"},
		{flags("main-bus", "34", "1000000", "0"), "GPIO34 cannot be used for SPI CS on ESP32: the master drives CS, so it needs an output GPIO"},
		{flags("main-bus", "6", "1000000", "0"), "GPIO6 is reserved for the SPI flash on ESP32"},
		{flags("main-bus", "4", "1000000", "0"), `GPIO4 is already used by component "status"`},
		{flags("main-bus", "21", "1000000", "0"), `GPIO21 is already used by component "sensors"`},
		{flags("main-bus", "23", "1000000", "0"), `GPIO23 is already used by SPI bus "main_bus"`},
		{flags("main-bus", "19", "1000000", "0"), `GPIO19 is already used by SPI bus "main_bus"`},
		{flags("main-bus", "18", "1000000", "0"), `GPIO18 is already used by SPI bus "main_bus"`},
		{flags("main-bus", "5", "1000000", "3"), `GPIO5 is already used by SPI device "display"`},
		{flags("main-bus", "16", "0", "0"), "frequency must be positive, got 0 Hz"},
		{[]string{"--bus", "main-bus", "--cs", "16", "--frequency=-1000000", "--mode", "0"}, "frequency must be positive, got -1000000 Hz"},
		{[]string{"--bus", "main-bus", "--cs", "16", "--frequency", "1000000", "--mode=-1"}, "SPI mode must be between 0 and 3"},
		{flags("main-bus", "16", "1000000", "4"), "SPI mode must be between 0 and 3"},
		{flags("main-bus", "16", "1000000", "10"), "SPI mode must be between 0 and 3"},
		{[]string{"--cs", "16", "--frequency", "1000000", "--mode", "0"}, `required flag(s) "bus" not set`},
		{[]string{"--bus", "main-bus", "--frequency", "1000000", "--mode", "0"}, `required flag(s) "cs" not set`},
		{[]string{"--bus", "main-bus", "--cs", "16", "--mode", "0"}, `required flag(s) "frequency" not set`},
		{[]string{"--bus", "main-bus", "--cs", "16", "--frequency", "1000000"}, `required flag(s) "mode" not set`},
		// MOSI, MISO and SCLK belong to the bus.
		{append(flags("main-bus", "16", "1000000", "0"), "--mosi", "13"), "unknown flag: --mosi"},
	}
	for _, tt := range rejected {
		args := append([]string{"add", "spi-device", "device"}, tt.args...)
		if err := run(t, args...); err == nil || err.Error() != tt.wantErr {
			t.Errorf("parrot %v: error = %v, want %q", args, err, tt.wantErr)
		}
	}
	if err := run(t, append([]string{"add", "spi-device", "display"}, flags("main-bus", "16", "1000000", "0")...)...); err == nil || err.Error() != `component "display" already exists` {
		t.Errorf("duplicate name: error = %v", err)
	}
	if err := run(t, "add", "spi-device", "device"); err == nil || !strings.HasPrefix(err.Error(), "required flag(s)") {
		t.Errorf("no flags: error = %v", err)
	}

	after, _ := os.ReadFile(project.ConfigFile)
	if string(after) != string(before) {
		t.Errorf("parrot.json changed after rejected commands:\n%s", after)
	}
	if _, err := os.Stat("components/device"); err == nil {
		t.Error("components/device was created by a rejected command")
	}
}

// When generation fails, the device is not recorded in parrot.json.
func TestAddSPIDeviceGenerationFailure(t *testing.T) {
	newSPIDeviceDemo(t)
	before, err := os.ReadFile(project.ConfigFile)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join("components", "display"), 0o755); err != nil {
		t.Fatal(err)
	}
	err = run(t, "add", "spi-device", "display", "--bus", "main-bus", "--cs", "5", "--frequency", "10000000", "--mode", "0")
	if err == nil || err.Error() != `directory "components/display" already exists` {
		t.Errorf("error = %v, want the existing directory", err)
	}
	if after, _ := os.ReadFile(project.ConfigFile); string(after) != string(before) {
		t.Errorf("parrot.json changed after a failed generation:\n%s", after)
	}
}

const spiDeviceInspectReport = `Parrot Project

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
    ├── SCLK: GPIO18
    └── Devices
        ├── display (SPI Device)
        │   ├── CS: GPIO5
        │   ├── Frequency: 10000000 Hz
        │   └── Mode: 0 (CPOL 0, CPHA 0)
        └── sensor (SPI Device)
            ├── CS: GPIO17
            ├── Frequency: 1000000 Hz
            └── Mode: 3 (CPOL 1, CPHA 1)

No problems detected.
`

// The devices are listed under their bus, once.
func TestInspectSPIDevices(t *testing.T) {
	newSPIDeviceDemo(t)
	mustRun(t, "add", "spi-device", "display", "--bus", "main-bus", "--cs", "5", "--frequency", "10000000", "--mode", "0")
	mustRun(t, "add", "spi-device", "sensor", "--bus", "main-bus", "--cs", "17", "--frequency", "1000000", "--mode", "3")
	out, err := runOutput(t, "inspect")
	if err != nil {
		t.Fatal(err)
	}
	if out != spiDeviceInspectReport {
		t.Errorf("output =\n%s\nwant\n%s", out, spiDeviceInspectReport)
	}
}

// Components are ESP-IDF components: parrot add needs the ESP32 platform.
func TestAddNeedsESP32Platform(t *testing.T) {
	newSPIDeviceDemo(t)
	manifest := []byte(`{"platform": "stm32", "target": "stm32f401re"}`)
	if err := os.WriteFile(project.ConfigFile, manifest, 0o644); err != nil {
		t.Fatal(err)
	}
	err := run(t, "add", "spi-device", "display", "--bus", "main-bus", "--cs", "5", "--frequency", "10000000", "--mode", "0")
	if want := `parrot.json names platform "stm32", but this command only supports "esp32"`; err == nil || err.Error() != want {
		t.Errorf("error = %v, want %q", err, want)
	}
	if after, _ := os.ReadFile(project.ConfigFile); string(after) != string(manifest) {
		t.Errorf("parrot.json changed:\n%s", after)
	}
}
