package spidevice_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"parrot/internal/components/spi"
	"parrot/internal/components/spidevice"
	"parrot/internal/project"
	"parrot/internal/targets"
)

func mustTarget(t *testing.T, id string) targets.Target {
	t.Helper()
	target, err := targets.Get(id)
	if err != nil {
		t.Fatal(err)
	}
	return target
}

func TestNew(t *testing.T) {
	d, err := spidevice.New("Main-Display", "Main-Bus", 5, 10_000_000, 0)
	if err != nil {
		t.Fatal(err)
	}
	want := spidevice.Device{Name: "main_display", Config: spidevice.Config{Bus: "main_bus", CS: 5, Frequency: 10_000_000, Mode: 0}}
	if d != want {
		t.Errorf("New = %+v, want %+v", d, want)
	}
}

// Every mode is accepted, and stands for its (CPOL, CPHA) pair, as in
// spi_device_interface_config_t.mode.
func TestModes(t *testing.T) {
	for mode, want := range [][2]int{{0, 0}, {0, 1}, {1, 0}, {1, 1}} {
		d, err := spidevice.New("display", "main_bus", 5, 1_000_000, mode)
		if err != nil {
			t.Errorf("mode %d: %v", mode, err)
			continue
		}
		if got := [2]int{d.CPOL(), d.CPHA()}; got != want {
			t.Errorf("mode %d: (CPOL, CPHA) = %v, want %v", mode, got, want)
		}
	}
}

func TestNewErrors(t *testing.T) {
	tests := []struct {
		frequency, mode int
		wantErr         string
	}{
		{0, 0, "frequency must be positive, got 0 Hz"},
		{-1_000_000, 0, "frequency must be positive, got -1000000 Hz"},
		{1_000_000, -1, "SPI mode must be between 0 and 3"},
		{1_000_000, 4, "SPI mode must be between 0 and 3"},
		{1_000_000, 10, "SPI mode must be between 0 and 3"},
	}
	for _, tt := range tests {
		_, err := spidevice.New("display", "main_bus", 5, tt.frequency, tt.mode)
		if err == nil || err.Error() != tt.wantErr {
			t.Errorf("New(%d Hz, mode %d) = %v, want %q", tt.frequency, tt.mode, err, tt.wantErr)
		}
	}
	// clock_speed_hz is a C int; only a 64-bit Go int can go beyond it.
	if strconv.IntSize == 64 {
		shift := 31
		tooHigh := 1 << shift
		if _, err := spidevice.New("display", "main_bus", 5, tooHigh, 0); err == nil || err.Error() != "frequency 2147483648 Hz is too high" {
			t.Errorf("New(%d Hz) = %v", tooHigh, err)
		}
	}
	if _, err := spidevice.New("display", "2bus", 5, 1_000_000, 0); err == nil {
		t.Error("New with an invalid bus name succeeded")
	}
}

// A device claims its CS GPIO only: the bus owns MOSI, MISO, SCLK and the host.
func TestNeeds(t *testing.T) {
	needs, err := spidevice.Config{Bus: "main_bus", CS: 5, Frequency: 10_000_000, Mode: 0}.Needs(mustTarget(t, "esp32"))
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(needs.GPIOs, []int{5}) || needs.SPIHost || needs.LEDC != nil || needs.I2CController {
		t.Errorf("Needs = %+v, want GPIO5 only", needs)
	}
	if _, err := (spidevice.Config{Bus: "main_bus", CS: 5, Frequency: 1, Mode: 4}).Needs(mustTarget(t, "esp32")); err == nil {
		t.Error("Needs accepted mode 4")
	}
}

func TestValidate(t *testing.T) {
	config := func(cs int) spidevice.Config {
		return spidevice.Config{Bus: "main_bus", CS: cs, Frequency: 10_000_000, Mode: 0}
	}
	esp32 := mustTarget(t, "esp32")
	if err := spidevice.Validate(esp32, config(5)); err != nil {
		t.Errorf("CS GPIO5: %v", err)
	}
	tests := []struct {
		target  string
		cs      int
		wantErr string
	}{
		{"esp32", 34, "GPIO34 cannot be used for SPI CS on ESP32: the master drives CS, so it needs an output GPIO"},
		{"esp32", 39, "GPIO39 cannot be used for SPI CS on ESP32: the master drives CS, so it needs an output GPIO"},
		{"esp32", 6, "GPIO6 is reserved for the SPI flash on ESP32"},
		{"esp32", 50, "GPIO50 is not available on ESP32"},
		{"esp32", -1, "GPIO-1 is not available on ESP32"},
		{"esp32-c3", 22, "GPIO22 is not available on ESP32-C3"},
	}
	for _, tt := range tests {
		err := spidevice.Validate(mustTarget(t, tt.target), config(tt.cs))
		if err == nil || err.Error() != tt.wantErr {
			t.Errorf("%s CS GPIO%d: error = %v, want %q", tt.target, tt.cs, err, tt.wantErr)
		}
	}
}

func TestResolveBus(t *testing.T) {
	cfg := project.Config{Components: []project.ComponentConfig{
		{Type: "spi-bus", Name: "main_bus", Config: []byte(`{"mosi": 23, "miso": 19, "sclk": 18}`)},
		{Type: "led", Name: "status", Config: []byte(`{"pin": 4}`)},
	}}
	device := func(bus string) spidevice.Device {
		return spidevice.Device{Name: "display", Config: spidevice.Config{Bus: bus, CS: 5, Frequency: 10_000_000}}
	}
	bus, err := device("main_bus").ResolveBus(cfg)
	if err != nil || bus.Name != "main_bus" || bus.MOSI != 23 || bus.MISO == nil || *bus.MISO != 19 || bus.SCLK != 18 {
		t.Errorf("ResolveBus(main_bus) = %+v, %v", bus, err)
	}
	if _, err := device("missing").ResolveBus(cfg); err == nil || err.Error() != `SPI bus "missing" does not exist` {
		t.Errorf("ResolveBus(missing) error = %v", err)
	}
	if _, err := device("status").ResolveBus(cfg); err == nil || err.Error() != `component "status" is not a SPI bus` {
		t.Errorf("ResolveBus(status) error = %v", err)
	}
}

// The manifest entry names the bus and holds the device's own settings only.
func TestConfigJSON(t *testing.T) {
	data, err := json.Marshal(spidevice.Config{Bus: "main_bus", CS: 5, Frequency: 10_000_000, Mode: 0})
	if err != nil {
		t.Fatal(err)
	}
	if want := `{"bus":"main_bus","cs":5,"frequency":10000000,"mode":0}`; string(data) != want {
		t.Errorf("JSON = %s, want %s", data, want)
	}
}

func read(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return strings.ReplaceAll(string(data), "\r\n", "\n")
}

// code returns source without its comments, which may name functions the
// code must not call.
func code(source string) string {
	var lines []string
	for _, line := range strings.Split(source, "\n") {
		lines = append(lines, strings.SplitN(line, "//", 2)[0])
	}
	return strings.Join(lines, "\n")
}

func mainBus(t *testing.T) spi.Bus {
	t.Helper()
	miso := 19
	bus, err := spi.New("main_bus", 23, &miso, 18)
	if err != nil {
		t.Fatal(err)
	}
	return bus
}

func TestCreate(t *testing.T) {
	dir := t.TempDir()
	d := spidevice.Device{Name: "sensor", Config: spidevice.Config{Bus: "main_bus", CS: 17, Frequency: 1_000_000, Mode: 3}}
	created, err := spidevice.Create(dir, d, mainBus(t), mustTarget(t, "esp32"))
	if err != nil {
		t.Fatal(err)
	}

	base := filepath.Join(dir, "components", "sensor")
	want := []string{
		base,
		filepath.Join(base, "sensor.c"),
		filepath.Join(base, "include", "sensor.h"),
		filepath.Join(base, "CMakeLists.txt"),
	}
	if !slices.Equal(created, want) {
		t.Errorf("created %q, want %q", created, want)
	}

	source := read(t, want[1])
	for _, s := range []string{
		`#include "main_bus.h"`,
		"#define SENSOR_CS_GPIO 17\n",
		"#define SENSOR_FREQUENCY_HZ 1000000\n",
		"// SPI mode 3: CPOL 1 (SCLK idles high), CPHA 1 (data sampled on the second edge).",
		"#define SENSOR_MODE 3\n",
		"#define SENSOR_QUEUE_SIZE 1\n",
		"static spi_device_handle_t sensor_handle = NULL;",
		".clock_speed_hz = SENSOR_FREQUENCY_HZ,",
		".mode = SENSOR_MODE,",
		".spics_io_num = SENSOR_CS_GPIO,",
		".queue_size = SENSOR_QUEUE_SIZE,",
		"spi_bus_add_device(main_bus_get_host(), &config, &handle)",
		"spi_bus_remove_device(sensor_handle)",
		"return sensor_transfer(data, NULL, length);",
		"if (length > SIZE_MAX / 8) {\n        return ESP_ERR_INVALID_SIZE;",
		".length = length * 8,",
		"spi_device_transmit(sensor_handle, &transaction)",
	} {
		if !strings.Contains(source, s) {
			t.Errorf("sensor.c does not contain %q", s)
		}
	}
	// The device neither creates nor frees its bus, and keeps to synchronous
	// transactions on fixed settings: no queue, bus lock, callbacks or
	// protocol phases. A full-duplex bus can receive.
	for _, s := range []string{
		"main_bus_init", "main_bus_deinit", "spi_bus_initialize", "spi_bus_free",
		"spi_device_queue_trans", "spi_device_get_trans_result", "spi_device_acquire_bus", "spi_device_release_bus",
		"pre_cb", "post_cb", "command_bits", "address_bits", "dummy_bits", "ESP_ERR_NOT_SUPPORTED",
		"mosi", "miso", "sclk", "MOSI_GPIO", "SPI2_HOST",
	} {
		if strings.Contains(code(source), s) {
			t.Errorf("sensor.c uses %q", s)
		}
	}

	header := read(t, want[2])
	for _, s := range []string{
		`#include "driver/spi_master.h"`,
		"// SPI device on the \"main_bus\" bus: CS GPIO17, SCLK at 1000000 Hz, SPI mode 3\n// (CPOL 1, CPHA 1).",
		"esp_err_t sensor_init(void);",
		"esp_err_t sensor_deinit(void);",
		"spi_device_handle_t sensor_get_handle(void);",
		"esp_err_t sensor_transmit(const void *data, size_t length);",
		"esp_err_t sensor_transfer(const void *tx_data, void *rx_data, size_t length);",
	} {
		if !strings.Contains(header, s) {
			t.Errorf("sensor.h does not contain %q", s)
		}
	}
	// Users of the device do not need its bus.
	if strings.Contains(header, `#include "main_bus.h"`) {
		t.Error("sensor.h includes main_bus.h")
	}

	// esp_driver_spi is public (the header exposes spi_device_handle_t); the
	// bus is private (only sensor.c includes main_bus.h).
	lines := strings.Split(read(t, want[3]), "\n")
	for _, line := range []string{`    SRCS "sensor.c"`, "    REQUIRES esp_driver_spi", "    PRIV_REQUIRES main_bus"} {
		if !slices.Contains(lines, line) {
			t.Errorf("CMakeLists.txt has no line %q:\n%s", line, strings.Join(lines, "\n"))
		}
	}
}

// On a bus without MISO, the device cannot receive, and says so.
func TestCreateOnWriteOnlyBus(t *testing.T) {
	dir := t.TempDir()
	bus, err := spi.New("display_bus", 13, nil, 14)
	if err != nil {
		t.Fatal(err)
	}
	d := spidevice.Device{Name: "panel", Config: spidevice.Config{Bus: "display_bus", CS: 15, Frequency: 20_000_000, Mode: 0}}
	if _, err := spidevice.Create(dir, d, bus, mustTarget(t, "esp32")); err != nil {
		t.Fatal(err)
	}
	base := filepath.Join(dir, "components", "panel")
	if source := read(t, filepath.Join(base, "panel.c")); !strings.Contains(source, "if (rx_data != NULL) {\n        return ESP_ERR_NOT_SUPPORTED;") {
		t.Errorf("panel.c does not reject rx_data:\n%s", source)
	}
	if header := read(t, filepath.Join(base, "include", "panel.h")); !strings.Contains(header, "// The bus has no MISO line, so this device can only send.") {
		t.Errorf("panel.h does not say the device can only send:\n%s", header)
	}
}

func TestCreateExistingFolder(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "components", "display"), 0o755); err != nil {
		t.Fatal(err)
	}
	d := spidevice.Device{Name: "display", Config: spidevice.Config{Bus: "main_bus", CS: 5, Frequency: 10_000_000}}
	if _, err := spidevice.Create(dir, d, mainBus(t), mustTarget(t, "esp32")); err == nil {
		t.Error("Create over an existing folder succeeded")
	}
}
