package i2c_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"parrot/internal/components/i2c"
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
	b, err := i2c.New("Sensor-Bus", 21, 22)
	if err != nil {
		t.Fatal(err)
	}
	if b.Name != "sensor_bus" || b.SDA != 21 || b.SCL != 22 {
		t.Errorf("New = %+v, want {Name: sensor_bus, SDA: 21, SCL: 22}", b)
	}
	if _, err := i2c.New("sensors", 21, 21); err == nil || err.Error() != "SDA and SCL cannot use the same GPIO" {
		t.Errorf("New with SDA == SCL: error = %v", err)
	}
	if _, err := i2c.New("2bus", 21, 22); err == nil {
		t.Error("New with an invalid name succeeded")
	}
}

func TestValidate(t *testing.T) {
	valid := map[string]i2c.Config{
		"esp32":    {SDA: 21, SCL: 22},
		"esp32-c3": {SDA: 4, SCL: 5},
		"esp32-s3": {SDA: 8, SCL: 9},
		"esp32-c6": {SDA: 6, SCL: 7},
	}
	for id, c := range valid {
		if err := i2c.Validate(mustTarget(t, id), c); err != nil {
			t.Errorf("Validate(%s, %+v) = %v, want nil", id, c, err)
		}
	}
}

func TestValidateErrors(t *testing.T) {
	tests := []struct {
		target   string
		sda, scl int
		wantErr  string
	}{
		{"esp32", 34, 22, "GPIO34 cannot be used for I2C SDA on ESP32: I2C lines need a GPIO that is both input and output"},
		{"esp32", 21, 39, "GPIO39 cannot be used for I2C SCL on ESP32: I2C lines need a GPIO that is both input and output"},
		{"esp32", 6, 22, "GPIO6 is reserved for the SPI flash on ESP32"},
		{"esp32-c3", 4, 22, "GPIO22 is not available on ESP32-C3"},
		{"esp32-s3", 23, 9, "GPIO23 is not available on ESP32-S3"},
		{"esp32", 21, 21, "SDA and SCL cannot use the same GPIO"},
	}
	for _, tt := range tests {
		err := i2c.Validate(mustTarget(t, tt.target), i2c.Config{SDA: tt.sda, SCL: tt.scl})
		if err == nil || err.Error() != tt.wantErr {
			t.Errorf("Validate(%s, SDA %d, SCL %d) = %v, want %q", tt.target, tt.sda, tt.scl, err, tt.wantErr)
		}
	}
}

func TestValidateTargetWithoutI2C(t *testing.T) {
	soc := targets.Target{DisplayName: "Test SoC"} // describes no I2C controller
	err := i2c.Validate(soc, i2c.Config{SDA: 1, SCL: 2})
	if want := "Test SoC has no I2C controller"; err == nil || err.Error() != want {
		t.Errorf("error = %v, want %q", err, want)
	}
}

func TestNeeds(t *testing.T) {
	esp32 := mustTarget(t, "esp32")
	needs, err := i2c.Config{SDA: 21, SCL: 22}.Needs(esp32)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(needs.GPIOs, []int{21, 22}) || !needs.I2CController || needs.LEDC != nil {
		t.Errorf("Needs = %+v, want GPIO21, GPIO22 and an I2C controller", needs)
	}
	if _, err := (i2c.Config{SDA: 21, SCL: 21}).Needs(esp32); err == nil {
		t.Error("Needs with SDA == SCL succeeded")
	}
}

func TestCreate(t *testing.T) {
	dir := t.TempDir()
	bus := i2c.Bus{Name: "sensors", Config: i2c.Config{SDA: 21, SCL: 22}}
	created, err := i2c.Create(dir, bus, mustTarget(t, "esp32"))
	if err != nil {
		t.Fatal(err)
	}

	base := filepath.Join(dir, "components", "sensors")
	want := []string{
		base,
		filepath.Join(base, "sensors.c"),
		filepath.Join(base, "include", "sensors.h"),
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
		return string(data)
	}
	source := read(want[1])
	for _, s := range []string{
		`#include "sensors.h"`,
		"#define SENSORS_SDA_GPIO GPIO_NUM_21",
		"#define SENSORS_SCL_GPIO GPIO_NUM_22",
		".i2c_port = -1,",
		".clk_source = I2C_CLK_SRC_DEFAULT,",
		".flags.enable_internal_pullup = true,",
		"i2c_new_master_bus(&config, &bus)",
		"i2c_del_master_bus(sensors_bus)",
	} {
		if !strings.Contains(source, s) {
			t.Errorf("sensors.c does not contain %q", s)
		}
	}
	// The SCL frequency belongs to the devices.
	if strings.Contains(source, "scl_speed_hz") {
		t.Error("sensors.c sets an SCL frequency on the bus")
	}

	header := read(want[2])
	for _, s := range []string{
		`#include "driver/i2c_master.h"`,
		"esp_err_t sensors_init(void);",
		"esp_err_t sensors_deinit(void);",
		"i2c_master_bus_handle_t sensors_get_handle(void);",
		"external pull-up resistors",
	} {
		if !strings.Contains(header, s) {
			t.Errorf("sensors.h does not contain %q", s)
		}
	}

	// The public header includes driver/i2c_master.h, so the dependency must
	// be public (REQUIRES) for the components that will use the bus.
	cmake := read(want[3])
	lines := strings.Split(strings.ReplaceAll(cmake, "\r\n", "\n"), "\n")
	if !slices.Contains(lines, "    REQUIRES esp_driver_i2c") || slices.Contains(lines, "    PRIV_REQUIRES esp_driver_i2c") {
		t.Errorf("CMakeLists.txt does not require esp_driver_i2c publicly:\n%s", cmake)
	}
}

func TestLookup(t *testing.T) {
	cfg := project.Config{Components: []project.ComponentConfig{
		{Type: "i2c-bus", Name: "sensors", Config: []byte(`{"sda": 21, "scl": 22}`)},
		{Type: "led", Name: "status", Config: []byte(`{"pin": 4}`)},
	}}
	bus, err := i2c.Lookup(cfg, "sensors")
	if err != nil || bus.Name != "sensors" || bus.SDA != 21 || bus.SCL != 22 {
		t.Errorf("Lookup(sensors) = %+v, %v; want the bus on GPIO21 and GPIO22", bus, err)
	}
	errors := map[string]string{
		"missing": `I2C bus "missing" does not exist`,
		"status":  `component "status" is not an I2C bus`,
	}
	for name, want := range errors {
		if _, err := i2c.Lookup(cfg, name); err == nil || err.Error() != want {
			t.Errorf("Lookup(%s) error = %v, want %q", name, err, want)
		}
	}
}
