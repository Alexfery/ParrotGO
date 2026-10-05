package components_test

import (
	"testing"

	"parrot/internal/components"
)

func TestNormalizeName(t *testing.T) {
	tests := map[string]string{
		"status":     "status",
		"status-led": "status_led",
		"status_led": "status_led",
		"Status-LED": "status_led",
		"led2":       "led2",
	}
	for in, want := range tests {
		got, err := components.NormalizeName(in)
		if err != nil || got != want {
			t.Errorf("NormalizeName(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
}

func TestNormalizeNameInvalid(t *testing.T) {
	for _, in := range []string{"", "2led", "-led", "_led", "status led", "status.led", "../led", "parrot_adc", "Parrot-X"} {
		if got, err := components.NormalizeName(in); err == nil {
			t.Errorf("NormalizeName(%q) = %q, want error", in, got)
		}
	}
}
