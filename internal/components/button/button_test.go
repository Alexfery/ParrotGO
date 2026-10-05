package button_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"parrot/internal/components/button"
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

func TestValidateInputPin(t *testing.T) {
	if err := button.Validate(mustTarget(t, "esp32"), 18); err != nil {
		t.Errorf("Validate(esp32, 18) = %v, want nil", err)
	}
}

func TestValidateErrors(t *testing.T) {
	tests := []struct {
		target  string
		pin     int
		wantErr string
	}{
		{"esp32-c3", 50, "GPIO50 is not available on ESP32-C3"},
		{"esp32", 34, "GPIO34 has no internal pull-up on ESP32, which the button needs"},
		{"esp32", 6, "GPIO6 is reserved for the SPI flash on ESP32"},
	}
	for _, tt := range tests {
		err := button.Validate(mustTarget(t, tt.target), tt.pin)
		if err == nil || err.Error() != tt.wantErr {
			t.Errorf("Validate(%s, %d) = %v, want %q", tt.target, tt.pin, err, tt.wantErr)
		}
	}
}

func TestNewNormalizesName(t *testing.T) {
	b, err := button.New("User-Button", 18)
	if err != nil {
		t.Fatal(err)
	}
	if b.Name != "user_button" || b.Pin != 18 {
		t.Errorf("New = %+v, want {Name: user_button, Pin: 18}", b)
	}
}

func TestCreate(t *testing.T) {
	dir := t.TempDir()
	created, err := button.Create(dir, button.Button{Name: "user_button", Config: button.Config{Pin: 18}})
	if err != nil {
		t.Fatal(err)
	}
	if len(created) != 4 {
		t.Fatalf("created %d paths, want 4: %v", len(created), created)
	}

	base := filepath.Join(dir, "components", "user_button")
	contains := map[string][]string{
		"user_button.c": {
			"#define USER_BUTTON_GPIO GPIO_NUM_18",
			"gpio_set_pull_mode(USER_BUTTON_GPIO, GPIO_PULLUP_ONLY);",
			"return gpio_get_level(USER_BUTTON_GPIO) == 0;",
		},
		"include/user_button.h": {"#include <stdbool.h>", "void user_button_init(void);", "bool user_button_is_pressed(void);"},
		"CMakeLists.txt":        {`SRCS "user_button.c"`, "PRIV_REQUIRES esp_driver_gpio"},
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
