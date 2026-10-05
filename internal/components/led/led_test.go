package led_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"parrot/internal/components/led"
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

func TestValidateOutputPin(t *testing.T) {
	if err := led.Validate(mustTarget(t, "esp32"), 4); err != nil {
		t.Errorf("Validate(esp32, 4) = %v, want nil", err)
	}
}

func TestValidateErrors(t *testing.T) {
	tests := []struct {
		target  string
		pin     int
		wantErr string
	}{
		{"esp32", 34, "GPIO34 cannot be used as an output on ESP32"},
		{"esp32-c3", 50, "GPIO50 is not available on ESP32-C3"},
		{"esp32", 6, "GPIO6 is reserved for the SPI flash on ESP32"},
	}
	for _, tt := range tests {
		err := led.Validate(mustTarget(t, tt.target), tt.pin)
		if err == nil || err.Error() != tt.wantErr {
			t.Errorf("Validate(%s, %d) = %v, want %q", tt.target, tt.pin, err, tt.wantErr)
		}
	}
}

func TestNewNormalizesName(t *testing.T) {
	l, err := led.New("Status-LED", 4)
	if err != nil {
		t.Fatal(err)
	}
	if l.Name != "status_led" || l.Pin != 4 {
		t.Errorf("New = %+v, want {Name: status_led, Pin: 4}", l)
	}
}

func TestCreate(t *testing.T) {
	dir := t.TempDir()
	created, err := led.Create(dir, led.LED{Name: "status_led", Config: led.Config{Pin: 4}})
	if err != nil {
		t.Fatal(err)
	}

	base := filepath.Join(dir, "components", "status_led")
	want := []string{
		base,
		filepath.Join(base, "status_led.c"),
		filepath.Join(base, "include", "status_led.h"),
		filepath.Join(base, "CMakeLists.txt"),
	}
	if strings.Join(created, "\n") != strings.Join(want, "\n") {
		t.Errorf("created = %v, want %v", created, want)
	}

	contains := map[string][]string{
		"status_led.c":         {"#define STATUS_LED_GPIO GPIO_NUM_4", "void status_led_toggle(void)"},
		"include/status_led.h": {"void status_led_init(void);", "void status_led_on(void);"},
		"CMakeLists.txt":       {`SRCS "status_led.c"`, "PRIV_REQUIRES esp_driver_gpio"},
	}
	for file, snippets := range contains {
		data, err := os.ReadFile(filepath.Join(base, file))
		if err != nil {
			t.Fatal(err)
		}
		for _, s := range snippets {
			if !strings.Contains(string(data), s) {
				t.Errorf("%s does not contain %q", file, s)
			}
		}
	}
}

func TestCreateExistingFolder(t *testing.T) {
	dir := t.TempDir()
	l := led.LED{Name: "status", Config: led.Config{Pin: 4}}
	if _, err := led.Create(dir, l); err != nil {
		t.Fatal(err)
	}
	if _, err := led.Create(dir, l); err == nil {
		t.Error("second Create succeeded, want error")
	}
}
