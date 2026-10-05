package adc_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"parrot/internal/components/adc"
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

func TestValidateMapping(t *testing.T) {
	tests := []struct {
		target string
		pin    int
		want   targets.ADCInfo
	}{
		{"esp32", 34, targets.ADCInfo{Unit: 1, Channel: 6}},
		{"esp32", 36, targets.ADCInfo{Unit: 1, Channel: 0}},
		{"esp32", 25, targets.ADCInfo{Unit: 2, Channel: 8}},
		{"esp32-c3", 2, targets.ADCInfo{Unit: 1, Channel: 2}},
		{"esp32-s3", 2, targets.ADCInfo{Unit: 1, Channel: 1}},
		{"esp32-s3", 11, targets.ADCInfo{Unit: 2, Channel: 0}},
		{"esp32-c6", 6, targets.ADCInfo{Unit: 1, Channel: 6}},
	}
	for _, tt := range tests {
		got, err := adc.Validate(mustTarget(t, tt.target), tt.pin)
		if err != nil || got != tt.want {
			t.Errorf("Validate(%s, %d) = %+v, %v; want %+v", tt.target, tt.pin, got, err, tt.want)
		}
	}
}

func TestValidateErrors(t *testing.T) {
	tests := []struct {
		target  string
		pin     int
		wantErr string
	}{
		{"esp32", 18, "GPIO18 does not support ADC on ESP32"},
		{"esp32-c3", 5, "GPIO5 does not support ADC on ESP32-C3"},
		{"esp32-c6", 7, "GPIO7 does not support ADC on ESP32-C6"},
		{"esp32-c3", 50, "GPIO50 is not available on ESP32-C3"},
		{"esp32", 6, "GPIO6 is reserved for the SPI flash on ESP32"},
	}
	for _, tt := range tests {
		_, err := adc.Validate(mustTarget(t, tt.target), tt.pin)
		if err == nil || err.Error() != tt.wantErr {
			t.Errorf("Validate(%s, %d) = %v, want %q", tt.target, tt.pin, err, tt.wantErr)
		}
	}
}

func TestNewNormalizesName(t *testing.T) {
	a, err := adc.New("Light-Sensor", 34)
	if err != nil {
		t.Fatal(err)
	}
	if a.Name != "light_sensor" || a.Pin != 34 {
		t.Errorf("New = %+v, want {Name: light_sensor, Pin: 34}", a)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestCreate(t *testing.T) {
	dir := t.TempDir()
	esp32 := mustTarget(t, "esp32")

	created, err := adc.Create(dir, adc.ADC{Name: "light", Config: adc.Config{Pin: 34}}, esp32)
	if err != nil {
		t.Fatal(err)
	}
	// parrot_adc (folder + 3 files) and light (folder + 3 files).
	if len(created) != 8 {
		t.Fatalf("created %d paths, want 8: %v", len(created), created)
	}

	base := filepath.Join(dir, "components", "light")
	for _, s := range []string{
		"#define LIGHT_ADC_UNIT    ADC_UNIT_1",
		"#define LIGHT_ADC_CHANNEL ADC_CHANNEL_6",
		"parrot_adc_unit(LIGHT_ADC_UNIT, &light_adc)",
		".atten = ADC_ATTEN_DB_12",
		"adc_oneshot_read(light_adc, LIGHT_ADC_CHANNEL, &raw)",
	} {
		if !strings.Contains(readFile(t, filepath.Join(base, "light.c")), s) {
			t.Errorf("light.c does not contain %q", s)
		}
	}
	if h := readFile(t, filepath.Join(base, "include", "light.h")); !strings.Contains(h, "int light_read_raw(void);") {
		t.Errorf("light.h does not declare light_read_raw:\n%s", h)
	}
	if c := readFile(t, filepath.Join(base, "CMakeLists.txt")); !strings.Contains(c, "PRIV_REQUIRES esp_adc parrot_adc") {
		t.Errorf("CMakeLists.txt has wrong dependencies:\n%s", c)
	}
	if c := readFile(t, filepath.Join(dir, "components", "parrot_adc", "CMakeLists.txt")); !strings.Contains(c, "REQUIRES esp_adc") {
		t.Errorf("parrot_adc CMakeLists.txt has wrong dependencies:\n%s", c)
	}

	// A second ADC reuses parrot_adc instead of generating it again.
	created, err = adc.Create(dir, adc.ADC{Name: "pot", Config: adc.Config{Pin: 35}}, esp32)
	if err != nil {
		t.Fatal(err)
	}
	if len(created) != 4 {
		t.Errorf("second Create created %d paths, want 4: %v", len(created), created)
	}
}

func TestCreateRejectsNonADCPin(t *testing.T) {
	dir := t.TempDir()
	if _, err := adc.Create(dir, adc.ADC{Name: "light", Config: adc.Config{Pin: 18}}, mustTarget(t, "esp32")); err == nil {
		t.Fatal("Create on GPIO18 succeeded, want error")
	}
	if _, err := os.Stat(filepath.Join(dir, "components")); err == nil {
		t.Error("Create on an invalid pin generated files")
	}
}
