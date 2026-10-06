package spi_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"parrot/internal/components/spi"
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

func pin(n int) *int { return &n }

func TestNew(t *testing.T) {
	b, err := spi.New("Display-Bus", 23, nil, 18)
	if err != nil {
		t.Fatal(err)
	}
	if b.Name != "display_bus" || b.MOSI != 23 || b.MISO != nil || b.SCLK != 18 {
		t.Errorf("New = %+v, want display_bus on MOSI 23, SCLK 18, no MISO", b)
	}
	if _, err := spi.New("main", 23, pin(19), 18); err == nil || !strings.Contains(err.Error(), `invalid component name "main"`) {
		t.Errorf("New(main) error = %v, want the main component's name rejected", err)
	}
}

// Two signals on one GPIO are rejected before any project check. Without
// MISO, only MOSI and SCLK are compared.
func TestNewSamePin(t *testing.T) {
	tests := []struct {
		mosi       int
		miso       *int
		sclk       int
		wantErr    string
		comparison string
	}{
		{23, pin(23), 18, "MOSI and MISO cannot use the same GPIO", "MOSI == MISO"},
		{23, pin(19), 23, "MOSI and SCLK cannot use the same GPIO", "MOSI == SCLK"},
		{23, pin(18), 18, "MISO and SCLK cannot use the same GPIO", "MISO == SCLK"},
		{23, nil, 23, "MOSI and SCLK cannot use the same GPIO", "MOSI == SCLK without MISO"},
	}
	for _, tt := range tests {
		_, err := spi.New("main_bus", tt.mosi, tt.miso, tt.sclk)
		if err == nil || err.Error() != tt.wantErr {
			t.Errorf("%s: error = %v, want %q", tt.comparison, err, tt.wantErr)
		}
		if err := spi.Validate(mustTarget(t, "esp32"), spi.Config{MOSI: tt.mosi, MISO: tt.miso, SCLK: tt.sclk}); err == nil || err.Error() != tt.wantErr {
			t.Errorf("Validate %s: error = %v, want %q", tt.comparison, err, tt.wantErr)
		}
	}
}

func TestValidate(t *testing.T) {
	esp32 := mustTarget(t, "esp32")
	valid := []spi.Config{
		{MOSI: 23, MISO: pin(19), SCLK: 18},
		{MOSI: 23, SCLK: 18},                // no MISO
		{MOSI: 13, MISO: pin(34), SCLK: 14}, // MISO is only read: an input-only GPIO is fine
		{MOSI: 13, MISO: pin(0), SCLK: 14},  // GPIO0 is a GPIO like any other
	}
	for _, c := range valid {
		if err := spi.Validate(esp32, c); err != nil {
			t.Errorf("Validate(%+v) = %v, want nil", c, err)
		}
	}
	if err := spi.Validate(mustTarget(t, "esp32-c3"), spi.Config{MOSI: 7, MISO: pin(2), SCLK: 6}); err != nil {
		t.Errorf("Validate on ESP32-C3 = %v, want nil", err)
	}
}

func TestValidateErrors(t *testing.T) {
	tests := []struct {
		target  string
		config  spi.Config
		wantErr string
	}{
		// MOSI and SCLK are driven by the master: input-only GPIOs cannot carry them.
		{"esp32", spi.Config{MOSI: 34, MISO: pin(19), SCLK: 18},
			"GPIO34 cannot be used for SPI MOSI on ESP32: the master drives MOSI, so it needs an output GPIO"},
		{"esp32", spi.Config{MOSI: 23, MISO: pin(19), SCLK: 39},
			"GPIO39 cannot be used for SPI SCLK on ESP32: the master drives SCLK, so it needs an output GPIO"},
		{"esp32", spi.Config{MOSI: 50, SCLK: 18}, "GPIO50 is not available on ESP32"},
		{"esp32", spi.Config{MOSI: 23, MISO: pin(6), SCLK: 18}, "GPIO6 is reserved for the SPI flash on ESP32"},
		{"esp32", spi.Config{MOSI: 23, MISO: pin(20), SCLK: 18}, "GPIO20 is not available on ESP32"},
		{"esp32", spi.Config{MOSI: 23, MISO: pin(-1), SCLK: 18}, "GPIO-1 is not available on ESP32"},
		{"esp32", spi.Config{MOSI: 23, MISO: pin(19), SCLK: 11}, "GPIO11 is reserved for the SPI flash on ESP32"},
		{"esp32-c3", spi.Config{MOSI: 23, SCLK: 6}, "GPIO23 is not available on ESP32-C3"},
	}
	for _, tt := range tests {
		err := spi.Validate(mustTarget(t, tt.target), tt.config)
		if err == nil || err.Error() != tt.wantErr {
			t.Errorf("Validate(%s, %+v) = %v, want %q", tt.target, tt.config, err, tt.wantErr)
		}
	}
}

// A bus needs the GPIOs of its signals and an SPI host; MISO only if it has one.
func TestNeeds(t *testing.T) {
	esp32 := mustTarget(t, "esp32")
	n, err := spi.Config{MOSI: 23, MISO: pin(19), SCLK: 18}.Needs(esp32)
	if err != nil || !slices.Equal(n.GPIOs, []int{23, 19, 18}) || !n.SPIHost || n.LEDC != nil || n.I2CController {
		t.Errorf("Needs = %+v, %v; want GPIO23, GPIO19, GPIO18 and an SPI host", n, err)
	}
	n, err = spi.Config{MOSI: 23, SCLK: 18}.Needs(esp32)
	if err != nil || !slices.Equal(n.GPIOs, []int{23, 18}) || !n.SPIHost {
		t.Errorf("Needs without MISO = %+v, %v; want GPIO23, GPIO18 and an SPI host", n, err)
	}
}

// parrot.json stores the user's lines, MISO only when there is one, and no host.
func TestConfigJSON(t *testing.T) {
	tests := []struct {
		config spi.Config
		want   string
	}{
		{spi.Config{MOSI: 23, MISO: pin(19), SCLK: 18}, `{"mosi":23,"miso":19,"sclk":18}`},
		{spi.Config{MOSI: 23, SCLK: 18}, `{"mosi":23,"sclk":18}`},
		{spi.Config{MOSI: 7, MISO: pin(0), SCLK: 6}, `{"mosi":7,"miso":0,"sclk":6}`},
	}
	for _, tt := range tests {
		data, err := json.Marshal(tt.config)
		if err != nil || string(data) != tt.want {
			t.Errorf("Marshal(%+v) = %s, %v; want %s", tt.config, data, err, tt.want)
		}
		var back spi.Config
		if err := json.Unmarshal(data, &back); err != nil || back.MOSI != tt.config.MOSI || back.SCLK != tt.config.SCLK ||
			(back.MISO == nil) != (tt.config.MISO == nil) || (back.MISO != nil && *back.MISO != *tt.config.MISO) {
			t.Errorf("Unmarshal(%s) = %+v, %v", data, back, err)
		}
	}
}

func readGenerated(t *testing.T, dir, name string) map[string]string {
	t.Helper()
	files := map[string]string{}
	for _, file := range []string{name + ".c", filepath.Join("include", name+".h"), "CMakeLists.txt"} {
		data, err := os.ReadFile(filepath.Join(dir, "components", name, file))
		if err != nil {
			t.Fatal(err)
		}
		files[filepath.ToSlash(file)] = string(data)
	}
	return files
}

func checkContains(t *testing.T, files map[string]string, want map[string][]string) {
	t.Helper()
	for file, snippets := range want {
		for _, s := range snippets {
			if !strings.Contains(files[file], s) {
				t.Errorf("%s does not contain %q:\n%s", file, s, files[file])
			}
		}
	}
}

func TestCreate(t *testing.T) {
	dir := t.TempDir()
	b := spi.Bus{Name: "main_bus", Config: spi.Config{MOSI: 23, MISO: pin(19), SCLK: 18}}
	created, err := spi.Create(dir, b, mustTarget(t, "esp32"), targets.SPIHost{Number: 2, Symbol: "SPI2_HOST"})
	if err != nil {
		t.Fatal(err)
	}
	base := filepath.Join(dir, "components", "main_bus")
	want := []string{
		base,
		filepath.Join(base, "main_bus.c"),
		filepath.Join(base, "include", "main_bus.h"),
		filepath.Join(base, "CMakeLists.txt"),
	}
	if !slices.Equal(created, want) {
		t.Errorf("created = %v, want %v", created, want)
	}
	files := readGenerated(t, dir, "main_bus")
	checkContains(t, files, map[string][]string{
		"main_bus.c": {
			"#define MAIN_BUS_MOSI_GPIO 23\n",
			"#define MAIN_BUS_MISO_GPIO 19\n",
			"#define MAIN_BUS_SCLK_GPIO 18\n",
			"#define MAIN_BUS_HOST SPI2_HOST\n",
			"static bool main_bus_initialized = false;",
			".mosi_io_num = MAIN_BUS_MOSI_GPIO,",
			".miso_io_num = MAIN_BUS_MISO_GPIO,",
			".sclk_io_num = MAIN_BUS_SCLK_GPIO,",
			".quadwp_io_num = -1,",
			".quadhd_io_num = -1,",
			"spi_bus_initialize(MAIN_BUS_HOST, &config, SPI_DMA_CH_AUTO);",
			"spi_bus_free(MAIN_BUS_HOST);",
			"return ESP_ERR_INVALID_STATE;",
		},
		"include/main_bus.h": {
			"#pragma once",
			`#include "driver/spi_master.h"`,
			`#include "esp_err.h"`,
			"esp_err_t main_bus_init(void);",
			"esp_err_t main_bus_deinit(void);",
			"spi_host_device_t main_bus_get_host(void);",
			"MOSI GPIO23, MISO GPIO19, SCLK GPIO18",
		},
		"CMakeLists.txt": {`SRCS "main_bus.c"`, "REQUIRES esp_driver_spi\n"},
	})
	// The bus owns no device: no transactions, no CS, no ESP_ERROR_CHECK.
	for file, content := range files {
		for _, absent := range []string{"ESP_ERROR_CHECK", "spi_device_transmit", "spics_io_num", "clock_speed_hz"} {
			if strings.Contains(content, absent) {
				t.Errorf("%s contains %q", file, absent)
			}
		}
	}
}

// Without MISO the line is disabled with -1, and no GPIO is made up for it.
func TestCreateWithoutMISO(t *testing.T) {
	dir := t.TempDir()
	b := spi.Bus{Name: "display_bus", Config: spi.Config{MOSI: 23, SCLK: 18}}
	if _, err := spi.Create(dir, b, mustTarget(t, "esp32"), targets.SPIHost{Number: 3, Symbol: "SPI3_HOST"}); err != nil {
		t.Fatal(err)
	}
	files := readGenerated(t, dir, "display_bus")
	checkContains(t, files, map[string][]string{
		"display_bus.c":         {".miso_io_num = -1,", "#define DISPLAY_BUS_HOST SPI3_HOST\n"},
		"include/display_bus.h": {"MOSI GPIO23, no MISO, SCLK GPIO18", "it can only send"},
	})
	if strings.Contains(files["display_bus.c"], "MISO_GPIO") {
		t.Errorf("display_bus.c defines a MISO GPIO:\n%s", files["display_bus.c"])
	}
}

func TestCreateExistingFolder(t *testing.T) {
	dir := t.TempDir()
	b := spi.Bus{Name: "main_bus", Config: spi.Config{MOSI: 23, SCLK: 18}}
	host := targets.SPIHost{Number: 2, Symbol: "SPI2_HOST"}
	if _, err := spi.Create(dir, b, mustTarget(t, "esp32"), host); err != nil {
		t.Fatal(err)
	}
	if _, err := spi.Create(dir, b, mustTarget(t, "esp32"), host); err == nil {
		t.Error("second Create succeeded, want error")
	}
}
