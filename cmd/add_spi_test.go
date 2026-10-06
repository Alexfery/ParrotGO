package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"parrot/internal/inspect"
	"parrot/internal/project"
)

func TestAddSPI(t *testing.T) {
	t.Chdir(t.TempDir())
	mustRun(t, "new", "spi-demo", "--target", "esp32")
	t.Chdir("spi-demo")

	out, err := runOutput(t, "add", "spi", "main-bus", "--mosi", "23", "--miso", "19", "--sclk", "18")
	if err != nil {
		t.Fatal(err)
	}
	want := `Adding SPI bus: main_bus
Target: ESP32
MOSI: GPIO23
MISO: GPIO19
SCLK: GPIO18
Host: SPI2_HOST (1 of 2 SPI hosts in use)

✓ Created components/main_bus
✓ Created components/main_bus/main_bus.c
✓ Created components/main_bus/include/main_bus.h
✓ Created components/main_bus/CMakeLists.txt
✓ Updated parrot.json

SPI bus component added successfully.
`
	if out != want {
		t.Errorf("output =\n%s\nwant\n%s", out, want)
	}

	// A write-only bus: no MISO, and the ESP32's second host.
	out, err = runOutput(t, "add", "spi", "display-bus", "--mosi", "13", "--sclk", "14")
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range []string{"MISO: disabled\n", "Host: SPI3_HOST (2 of 2 SPI hosts in use)\n"} {
		if !strings.Contains(out, line) {
			t.Errorf("output does not contain %q:\n%s", line, out)
		}
	}

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
      "type": "spi-bus",
      "name": "display_bus",
      "config": {
        "mosi": 13,
        "sclk": 14
      }
    }
  ]
}
`
	if string(data) != wantManifest {
		t.Errorf("parrot.json =\n%s\nwant\n%s", data, wantManifest)
	}

	generated := map[string][]string{
		"components/main_bus/main_bus.c":         {"#define MAIN_BUS_HOST SPI2_HOST\n", ".miso_io_num = MAIN_BUS_MISO_GPIO,"},
		"components/display_bus/display_bus.c":   {"#define DISPLAY_BUS_HOST SPI3_HOST\n", ".miso_io_num = -1,"},
		"components/main_bus/CMakeLists.txt":     {"REQUIRES esp_driver_spi\n"},
		"components/main_bus/include/main_bus.h": {"spi_host_device_t main_bus_get_host(void);"},
	}
	for path, snippets := range generated {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		for _, s := range snippets {
			if !strings.Contains(string(content), s) {
				t.Errorf("%s does not contain %q", path, s)
			}
		}
	}
	// parrot add never touches the project's own code.
	if mainC, _ := os.ReadFile("main/main.c"); strings.Contains(string(mainC), "main_bus") {
		t.Errorf("main/main.c was changed:\n%s", mainC)
	}

	// Both hosts are taken.
	err = run(t, "add", "spi", "third", "--mosi", "25", "--sclk", "26")
	if err == nil || err.Error() != "no SPI hosts available on ESP32" {
		t.Errorf("third bus: error = %v", err)
	}
	if _, err := os.Stat("components/third"); err == nil {
		t.Error("rejected bus generated files")
	}
}

// inspect shows the host the generated code uses.
func TestAddSPIMatchesInspect(t *testing.T) {
	t.Chdir(t.TempDir())
	mustRun(t, "new", "s3", "--target", "esp32-s3")
	t.Chdir("s3")
	mustRun(t, "add", "spi", "fast", "--mosi", "11", "--miso", "13", "--sclk", "12")
	mustRun(t, "add", "led", "status", "--pin", "2")
	mustRun(t, "add", "spi", "slow", "--mosi", "35", "--sclk", "36")

	cfg, err := project.LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	buses := 0
	for _, c := range inspect.Resolve(cfg).Components {
		if c.Type != "spi-bus" {
			continue
		}
		buses++
		if c.Properties[0].Name != "Host" {
			t.Fatalf("%s properties = %+v, want its host first", c.Name, c.Properties)
		}
		source, err := os.ReadFile(filepath.Join("components", c.Name, c.Name+".c"))
		if err != nil {
			t.Fatal(err)
		}
		define := "#define " + strings.ToUpper(c.Name) + "_HOST " + c.Properties[0].Value + "\n"
		if !strings.Contains(string(source), define) {
			t.Errorf("inspect gives %s %s, which %s.c does not use", c.Name, c.Properties[0].Value, c.Name)
		}
	}
	if buses != 2 {
		t.Errorf("inspect shows %d SPI buses, want 2", buses)
	}

	out, err := runOutput(t, "inspect")
	if err != nil {
		t.Fatal(err)
	}
	for _, tree := range []string{
		"fast\n└── SPI Bus\n    ├── Host: SPI2_HOST\n    ├── MOSI: GPIO11\n    ├── MISO: GPIO13\n    └── SCLK: GPIO12\n",
		"slow\n└── SPI Bus\n    ├── Host: SPI3_HOST\n    ├── MOSI: GPIO35\n    ├── MISO: disabled\n    └── SCLK: GPIO36\n",
	} {
		if !strings.Contains(out, tree) {
			t.Errorf("inspect output does not contain\n%s\nin\n%s", tree, out)
		}
	}
}

// ESP32-C3 has a single SPI host for the application; GPIO0 is a valid MISO.
func TestAddSPIOnESP32C3(t *testing.T) {
	t.Chdir(t.TempDir())
	mustRun(t, "new", "c3", "--target", "esp32-c3")
	t.Chdir("c3")
	out, err := runOutput(t, "add", "spi", "main_bus", "--mosi", "7", "--miso", "0", "--sclk", "6")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "MISO: GPIO0\n") || !strings.Contains(out, "Host: SPI2_HOST (1 of 1 SPI hosts in use)\n") {
		t.Errorf("output =\n%s", out)
	}
	if data, _ := os.ReadFile(project.ConfigFile); !strings.Contains(string(data), `"miso": 0,`) {
		t.Errorf("parrot.json does not record MISO on GPIO0:\n%s", data)
	}
	err = run(t, "add", "spi", "second", "--mosi", "4", "--sclk", "5")
	if err == nil || err.Error() != "no SPI hosts available on ESP32-C3" {
		t.Errorf("second bus on ESP32-C3: error = %v", err)
	}
}

// Rejected commands must leave parrot.json and components/ untouched.
func TestAddSPIRejected(t *testing.T) {
	t.Chdir(t.TempDir())
	mustRun(t, "new", "spi-errors", "--target", "esp32")
	t.Chdir("spi-errors")
	mustRun(t, "add", "led", "status", "--pin", "23")
	mustRun(t, "add", "i2c", "sensors", "--sda", "21", "--scl", "22")
	before, err := os.ReadFile(project.ConfigFile)
	if err != nil {
		t.Fatal(err)
	}

	rejected := []struct {
		args    []string
		wantErr string
	}{
		{[]string{"--mosi", "23", "--miso", "19", "--sclk", "18"}, `GPIO23 is already used by component "status"`},
		{[]string{"--mosi", "13", "--miso", "21", "--sclk", "18"}, `GPIO21 is already used by component "sensors"`},
		{[]string{"--mosi", "13", "--miso", "19", "--sclk", "22"}, `GPIO22 is already used by component "sensors"`},
		{[]string{"--mosi", "13", "--miso", "13", "--sclk", "18"}, "MOSI and MISO cannot use the same GPIO"},
		{[]string{"--mosi", "13", "--miso", "19", "--sclk", "13"}, "MOSI and SCLK cannot use the same GPIO"},
		{[]string{"--mosi", "13", "--miso", "18", "--sclk", "18"}, "MISO and SCLK cannot use the same GPIO"},
		{[]string{"--mosi", "13", "--sclk", "13"}, "MOSI and SCLK cannot use the same GPIO"},
		{[]string{"--mosi", "34", "--miso", "19", "--sclk", "18"},
			"GPIO34 cannot be used for SPI MOSI on ESP32: the master drives MOSI, so it needs an output GPIO"},
		{[]string{"--mosi", "13", "--miso", "6", "--sclk", "18"}, "GPIO6 is reserved for the SPI flash on ESP32"},
		{[]string{"--mosi", "13", "--miso", "50", "--sclk", "18"}, "GPIO50 is not available on ESP32"},
		{[]string{"--mosi", "13", "--miso", "19", "--sclk", "39"},
			"GPIO39 cannot be used for SPI SCLK on ESP32: the master drives SCLK, so it needs an output GPIO"},
		// CS, frequency and mode belong to the devices.
		{[]string{"--mosi", "13", "--sclk", "18", "--cs", "5"}, "unknown flag: --cs"},
		{[]string{"--mosi", "13", "--sclk", "18", "--frequency", "1000000"}, "unknown flag: --frequency"},
		{[]string{"--mosi", "13", "--sclk", "18", "--mode", "0"}, "unknown flag: --mode"},
		{[]string{"--miso", "19", "--sclk", "18"}, `required flag(s) "mosi" not set`},
		{[]string{"--mosi", "13", "--miso", "19"}, `required flag(s) "sclk" not set`},
	}
	for _, tt := range rejected {
		args := append([]string{"add", "spi", "bus"}, tt.args...)
		if err := run(t, args...); err == nil || err.Error() != tt.wantErr {
			t.Errorf("parrot %v: error = %v, want %q", args, err, tt.wantErr)
		}
	}

	// The project's main component already has this name.
	err = run(t, "add", "spi", "main", "--mosi", "13", "--miso", "19", "--sclk", "18")
	if want := `invalid component name "main": ESP-IDF projects already have a "main" component (the main folder, with app_main)`; err == nil || err.Error() != want {
		t.Errorf("bus named main: error = %v, want %q", err, want)
	}
	if err := run(t, "add", "spi", "sensors", "--mosi", "13", "--sclk", "18"); err == nil || err.Error() != `component "sensors" already exists` {
		t.Errorf("duplicate name: error = %v", err)
	}

	after, _ := os.ReadFile(project.ConfigFile)
	if string(after) != string(before) {
		t.Errorf("parrot.json changed after rejected commands:\n%s", after)
	}
	for _, dir := range []string{"components/bus", "components/main"} {
		if _, err := os.Stat(dir); err == nil {
			t.Errorf("%s was created by a rejected command", dir)
		}
	}
}
