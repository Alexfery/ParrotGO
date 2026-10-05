package i2cdevice_test

import (
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"parrot/internal/components/i2cdevice"
	"parrot/internal/project"
	"parrot/internal/targets"
)

func TestParseAddress(t *testing.T) {
	// Hexadecimal and decimal forms of the same address give the same number.
	for _, s := range []string{"0x3C", "0x3c", "0X3C", "60", "060"} {
		got, err := i2cdevice.ParseAddress(s)
		if err != nil || got != 0x3C {
			t.Errorf("ParseAddress(%q) = 0x%X, %v; want 0x3C", s, got, err)
		}
	}
	for _, s := range []string{"", "0x", "3C", "abc", "-1", "0x3C ", "70000", "0o74"} {
		if got, err := i2cdevice.ParseAddress(s); err == nil {
			t.Errorf("ParseAddress(%q) = 0x%X, want an error", s, got)
		}
	}
}

func TestFormatAddress(t *testing.T) {
	for address, want := range map[uint16]string{0x3C: "0x3C", 0x08: "0x08", 0x68: "0x68"} {
		if got := i2cdevice.FormatAddress(address); got != want {
			t.Errorf("FormatAddress(%d) = %q, want %q", address, got, want)
		}
	}
}

func TestNew(t *testing.T) {
	d, err := i2cdevice.New("Main-Display", "Sensors", 0x3C, 400000)
	if err != nil {
		t.Fatal(err)
	}
	want := i2cdevice.Device{Name: "main_display", Config: i2cdevice.Config{Bus: "sensors", Address: 0x3C, Frequency: 400000}}
	if d != want {
		t.Errorf("New = %+v, want %+v", d, want)
	}
	// The usable 7-bit range.
	for _, address := range []uint16{0x08, 0x77} {
		if _, err := i2cdevice.New("dev", "sensors", address, 100000); err != nil {
			t.Errorf("address 0x%02X: %v", address, err)
		}
	}
}

func TestNewErrors(t *testing.T) {
	tests := []struct {
		address   uint16
		frequency int
		wantErr   string
	}{
		{0x80, 100000, "I2C address 0x80 does not fit in 7 bits (10-bit addresses are not supported); if 0x80 is an 8-bit address that includes the R/W bit, use 0x40"},
		{0x200, 100000, "I2C address 0x200 does not fit in 7 bits (10-bit addresses are not supported)"},
		{0x78, 400000, "I2C address 0x78 is reserved by the I2C specification (0x00-0x07 and 0x78-0x7F); if 0x78 is an 8-bit address that includes the R/W bit, use 0x3C"},
		{0x00, 100000, "I2C address 0x00 is reserved by the I2C specification (0x00-0x07 and 0x78-0x7F)"},
		{0x07, 100000, "I2C address 0x07 is reserved by the I2C specification (0x00-0x07 and 0x78-0x7F)"},
		{0x3C, 0, "frequency must be positive, got 0 Hz"},
		{0x3C, -100000, "frequency must be positive, got -100000 Hz"},
	}
	for _, tt := range tests {
		_, err := i2cdevice.New("display", "sensors", tt.address, tt.frequency)
		if err == nil || err.Error() != tt.wantErr {
			t.Errorf("New(0x%02X, %d Hz) = %v, want %q", tt.address, tt.frequency, err, tt.wantErr)
		}
	}
	// scl_speed_hz is a uint32_t; only a 64-bit int can go beyond it.
	if strconv.IntSize == 64 {
		shift := 32
		tooHigh := 1 << shift
		if _, err := i2cdevice.New("display", "sensors", 0x3C, tooHigh); err == nil || err.Error() != "frequency 4294967296 Hz is too high" {
			t.Errorf("New(%d Hz) = %v", tooHigh, err)
		}
	}
	if _, err := i2cdevice.New("display", "2bus", 0x3C, 400000); err == nil {
		t.Error("New with an invalid bus name succeeded")
	}
}

func TestNeeds(t *testing.T) {
	esp32, _ := targets.Get("esp32")
	needs, err := i2cdevice.Config{Bus: "sensors", Address: 0x3C, Frequency: 400000}.Needs(esp32)
	if err != nil {
		t.Fatal(err)
	}
	if needs.I2CAddress == nil || needs.I2CAddress.Bus != "sensors" || needs.I2CAddress.Address != 0x3C {
		t.Errorf("Needs = %+v, want address 0x3C on sensors", needs)
	}
	// The bus owns the GPIOs and the controller.
	if len(needs.GPIOs) != 0 || needs.I2CController || needs.LEDC != nil {
		t.Errorf("Needs = %+v, want no GPIO, controller or LEDC", needs)
	}
	if _, err := (i2cdevice.Config{Bus: "sensors", Address: 0x80, Frequency: 400000}).Needs(esp32); err == nil {
		t.Error("Needs accepted an address that does not fit in 7 bits")
	}
}

func TestResolveBus(t *testing.T) {
	cfg := project.Config{Components: []project.ComponentConfig{
		{Type: "i2c-bus", Name: "sensors", Config: []byte(`{"sda": 21, "scl": 22}`)},
		{Type: "led", Name: "status", Config: []byte(`{"pin": 4}`)},
	}}
	device := func(bus string) i2cdevice.Device {
		return i2cdevice.Device{Name: "display", Config: i2cdevice.Config{Bus: bus, Address: 0x3C, Frequency: 400000}}
	}
	bus, err := device("sensors").ResolveBus(cfg)
	if err != nil || bus.Name != "sensors" || bus.SDA != 21 || bus.SCL != 22 {
		t.Errorf("ResolveBus(sensors) = %+v, %v", bus, err)
	}
	if _, err := device("missing").ResolveBus(cfg); err == nil || err.Error() != `I2C bus "missing" does not exist` {
		t.Errorf("ResolveBus(missing) error = %v", err)
	}
	if _, err := device("status").ResolveBus(cfg); err == nil || err.Error() != `component "status" is not an I2C bus` {
		t.Errorf("ResolveBus(status) error = %v", err)
	}
}

func TestCreate(t *testing.T) {
	dir := t.TempDir()
	d := i2cdevice.Device{Name: "display", Config: i2cdevice.Config{Bus: "sensors", Address: 0x3C, Frequency: 400000}}
	created, err := i2cdevice.Create(dir, d)
	if err != nil {
		t.Fatal(err)
	}

	base := filepath.Join(dir, "components", "display")
	want := []string{
		base,
		filepath.Join(base, "display.c"),
		filepath.Join(base, "include", "display.h"),
		filepath.Join(base, "CMakeLists.txt"),
	}
	if !slices.Equal(created, want) {
		t.Errorf("created %q, want %q", created, want)
	}
	read := func(path string) string {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return strings.ReplaceAll(string(data), "\r\n", "\n")
	}

	source := read(want[1])
	for _, s := range []string{
		`#include "sensors.h"`,
		"i2c_master_bus_handle_t bus = sensors_get_handle();",
		"if (bus == NULL) {\n        return ESP_ERR_INVALID_STATE;",
		".dev_addr_length = I2C_ADDR_BIT_LEN_7,",
		".device_address = 0x3C,",
		".scl_speed_hz = 400000,",
		"i2c_master_bus_add_device(bus, &config, &handle)",
		"i2c_master_bus_rm_device(display_handle)",
		"i2c_master_transmit(display_handle, data, length, timeout_ms)",
		"i2c_master_receive(display_handle, data, length, timeout_ms)",
		"i2c_master_transmit_receive(display_handle, write_data, write_length,\n                                       read_data, read_length, timeout_ms)",
	} {
		if !strings.Contains(source, s) {
			t.Errorf("display.c does not contain %q", s)
		}
	}
	// The device never creates or deletes its bus, and passes the 7-bit
	// address as is. Comments may name the bus functions; the code may not.
	var code []string
	for _, line := range strings.Split(source, "\n") {
		code = append(code, strings.SplitN(line, "//", 2)[0])
	}
	for _, s := range []string{"sensors_init", "sensors_deinit", "i2c_new_master_bus", "i2c_del_master_bus", "<<"} {
		if strings.Contains(strings.Join(code, "\n"), s) {
			t.Errorf("display.c calls %q", s)
		}
	}

	header := read(want[2])
	for _, s := range []string{
		`#include "driver/i2c_master.h"`,
		"esp_err_t display_init(void);",
		"esp_err_t display_deinit(void);",
		"i2c_master_dev_handle_t display_get_handle(void);",
		"esp_err_t display_transmit(const uint8_t *data, size_t length, int timeout_ms);",
		"esp_err_t display_receive(uint8_t *data, size_t length, int timeout_ms);",
		"esp_err_t display_transmit_receive(const uint8_t *write_data, size_t write_length,",
	} {
		if !strings.Contains(header, s) {
			t.Errorf("display.h does not contain %q", s)
		}
	}
	// The header does not expose the bus: users of the device do not need it.
	if strings.Contains(header, `#include "sensors.h"`) {
		t.Error("display.h includes sensors.h")
	}

	// esp_driver_i2c is public (the header exposes its handle type); the bus
	// is private (only display.c includes sensors.h).
	lines := strings.Split(read(want[3]), "\n")
	for _, line := range []string{"    REQUIRES esp_driver_i2c", "    PRIV_REQUIRES sensors"} {
		if !slices.Contains(lines, line) {
			t.Errorf("CMakeLists.txt has no line %q:\n%s", line, strings.Join(lines, "\n"))
		}
	}
}
