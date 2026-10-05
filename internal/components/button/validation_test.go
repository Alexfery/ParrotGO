package button

import (
	"testing"

	"parrot/internal/targets"
)

// No supported SoC has an output-only GPIO, so the input rule is tested on a
// hand-built pin.
func TestCheckPinNoInput(t *testing.T) {
	err := checkPin(targets.Pin{Number: 5, Output: true, Pull: true}, "ESP32")
	want := "GPIO5 cannot be used as an input on ESP32"
	if err == nil || err.Error() != want {
		t.Errorf("checkPin = %v, want %q", err, want)
	}
}
