package bme280_test

import (
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"parrot/internal/components/i2cdevice"
	"parrot/internal/components/sensor/bme280"
	"parrot/internal/project"
	"parrot/internal/targets"
)

func TestNew(t *testing.T) {
	s, err := bme280.New("Environment", "environment-device")
	if err != nil {
		t.Fatal(err)
	}
	want := bme280.Sensor{Name: "environment", Config: bme280.Config{Device: "environment_device"}}
	if s != want {
		t.Errorf("New = %+v, want %+v", s, want)
	}
	if _, err := bme280.New("2env", "environment_device"); err == nil {
		t.Error("New accepted an invalid sensor name")
	}
	if _, err := bme280.New("environment", "2dev"); err == nil {
		t.Error("New accepted an invalid device name")
	}
}

// The sensor takes its device, and none of the resources the device and its
// bus already hold.
func TestNeeds(t *testing.T) {
	esp32, _ := targets.Get("esp32")
	needs, err := bme280.Config{Device: "environment_device"}.Needs(esp32)
	if err != nil {
		t.Fatal(err)
	}
	if len(needs.GPIOs) != 0 {
		t.Errorf("Needs.GPIOs = %v, want none: SDA and SCL are the bus's", needs.GPIOs)
	}
	if needs.I2CAddress != nil {
		t.Errorf("Needs.I2CAddress = %+v, want none: the address is the device's", needs.I2CAddress)
	}
	if needs.I2CController || needs.LEDC != nil {
		t.Errorf("Needs = %+v, want no controller and no LEDC", needs)
	}
	if needs.Drives != "environment_device" {
		t.Errorf("Needs.Drives = %q, want environment_device", needs.Drives)
	}
}

func manifest(t *testing.T, devices ...project.ComponentConfig) project.Config {
	t.Helper()
	cfg := project.Config{Target: "esp32", Components: []project.ComponentConfig{
		{Type: "i2c-bus", Name: "sensors", Config: []byte(`{"sda": 21, "scl": 22}`)},
		{Type: "led", Name: "status", Config: []byte(`{"pin": 4}`)},
	}}
	cfg.Components = append(cfg.Components, devices...)
	return cfg
}

func device(name string, address int) project.ComponentConfig {
	return project.ComponentConfig{Type: "i2c-device", Name: name,
		Config: []byte(`{"bus": "sensors", "address": ` + strconv.Itoa(address) + `, "frequency": 400000}`)}
}

func TestResolveDevice(t *testing.T) {
	cfg := manifest(t, device("environment_device", 0x76), device("outdoor_device", 0x77), device("display", 0x3C))
	sensor := func(dev string) bme280.Sensor {
		return bme280.Sensor{Name: "environment", Config: bme280.Config{Device: dev}}
	}

	// Both addresses of a BME280; the device comes back with its own config.
	for name, address := range map[string]uint16{"environment_device": 0x76, "outdoor_device": 0x77} {
		d, err := sensor(name).ResolveDevice(cfg)
		if err != nil {
			t.Errorf("ResolveDevice(%s): %v", name, err)
			continue
		}
		want := i2cdevice.Device{Name: name, Config: i2cdevice.Config{Bus: "sensors", Address: address, Frequency: 400000}}
		if d != want {
			t.Errorf("ResolveDevice(%s) = %+v, want %+v", name, d, want)
		}
	}

	for dev, wantErr := range map[string]string{
		"missing": `I2C device "missing" does not exist`,
		"status":  `component "status" is not an I2C device`,
		"sensors": `component "sensors" is not an I2C device`, // the bus itself
		"display": `I2C device "display" uses address 0x3C; BME280 expects 0x76 or 0x77`,
	} {
		if _, err := sensor(dev).ResolveDevice(cfg); err == nil || err.Error() != wantErr {
			t.Errorf("ResolveDevice(%s) error = %v, want %q", dev, err, wantErr)
		}
	}
}

func TestCreate(t *testing.T) {
	dir := t.TempDir()
	s := bme280.Sensor{Name: "environment", Config: bme280.Config{Device: "environment_device"}}
	created, err := bme280.Create(dir, s)
	if err != nil {
		t.Fatal(err)
	}
	base := filepath.Join(dir, "components", "environment")
	want := []string{
		base,
		filepath.Join(base, "environment.c"),
		filepath.Join(base, "include", "environment.h"),
		filepath.Join(base, "CMakeLists.txt"),
	}
	if !slices.Equal(created, want) {
		t.Errorf("created %q, want %q", created, want)
	}
	source := read(t, want[1])
	header := read(t, want[2])
	cmake := read(t, want[3])

	// The driver reaches the sensor through the device's functions only:
	// a register read is one write-then-read transaction.
	for _, s := range []string{
		`#include "environment_device.h"`,
		"environment_device_transmit_receive(&reg, 1, data, length, I2C_TIMEOUT_MS)",
		"environment_device_transmit(bytes, sizeof(bytes), I2C_TIMEOUT_MS)",
		"esp_err_t environment_init(void)",
		"esp_err_t environment_read(environment_reading_t *reading)",
	} {
		if !strings.Contains(source, s) {
			t.Errorf("environment.c does not contain %q", s)
		}
	}
	// Never the ESP-IDF driver, the bus, or the device's lifecycle; comments
	// may name them, the code may not.
	code := withoutComments(source)
	for _, s := range []string{
		"i2c_master_", "driver/i2c", "sensors", "environment_device_init", "environment_device_deinit",
		"environment_device_receive(", "environment_device_get_handle", "malloc",
	} {
		if strings.Contains(code, s) {
			t.Errorf("environment.c uses %q", s)
		}
	}
	// The generated files know neither the address nor the GPIOs.
	for name, text := range map[string]string{"environment.c": code, "environment.h": header, "CMakeLists.txt": cmake} {
		for _, s := range []string{"0x76", "0x77", "400000", "gpio", "sda", "scl"} {
			if strings.Contains(strings.ToLower(text), s) {
				t.Errorf("%s contains %q", name, s)
			}
		}
	}

	for _, s := range []string{
		"#pragma once",
		`#include "esp_err.h"`,
		"} environment_reading_t;",
		"float temperature_c;",
		"float pressure_pa;",
		"float humidity_percent;",
		"esp_err_t environment_init(void);",
		"esp_err_t environment_read(environment_reading_t *reading);",
	} {
		if !strings.Contains(header, s) {
			t.Errorf("environment.h does not contain %q", s)
		}
	}
	// The register helpers and the device stay private to environment.c.
	for _, s := range []string{"environment_device.h", "i2c_master", "read_registers", "write_register", "calibration"} {
		if strings.Contains(withoutComments(header), s) {
			t.Errorf("environment.h exposes %q", s)
		}
	}

	// environment depends on its device only, privately: the device brings
	// the bus and esp_driver_i2c.
	lines := strings.Split(withoutCMakeComments(cmake), "\n")
	if !slices.Contains(lines, "    PRIV_REQUIRES environment_device") {
		t.Errorf("CMakeLists.txt does not privately require environment_device:\n%s", cmake)
	}
	for _, s := range []string{"sensors", "esp_driver_i2c", "    REQUIRES"} {
		if strings.Contains(withoutCMakeComments(cmake), s) {
			t.Errorf("CMakeLists.txt requires %q:\n%s", s, cmake)
		}
	}

	// The component folder is never overwritten.
	if _, err := bme280.Create(dir, s); err == nil {
		t.Error("Create succeeded on an existing component")
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

// withoutComments drops // comments from C source.
func withoutComments(source string) string {
	var code []string
	for _, line := range strings.Split(source, "\n") {
		code = append(code, strings.SplitN(line, "//", 2)[0])
	}
	return strings.Join(code, "\n")
}

func withoutCMakeComments(source string) string {
	var code []string
	for _, line := range strings.Split(source, "\n") {
		if !strings.HasPrefix(strings.TrimSpace(line), "#") {
			code = append(code, line)
		}
	}
	return strings.Join(code, "\n")
}
