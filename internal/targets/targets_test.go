package targets_test

import (
	"testing"

	"parrot/internal/targets"
)

func mustGet(t *testing.T, id string) targets.Target {
	t.Helper()
	target, err := targets.Get(id)
	if err != nil {
		t.Fatalf("Get(%q): %v", id, err)
	}
	return target
}

func TestGet(t *testing.T) {
	tests := []struct {
		id, idfTarget string
	}{
		{"esp32", "esp32"},
		{"esp32-c3", "esp32c3"},
		{"esp32-s3", "esp32s3"},
		{"esp32-c6", "esp32c6"},
	}
	for _, tt := range tests {
		target := mustGet(t, tt.id)
		if target.ID != tt.id || target.IDFTarget != tt.idfTarget {
			t.Errorf("Get(%q) = {ID: %q, IDFTarget: %q}, want {%q, %q}",
				tt.id, target.ID, target.IDFTarget, tt.id, tt.idfTarget)
		}
	}
}

func TestGetDefault(t *testing.T) {
	mustGet(t, targets.DefaultID)
}

func TestGetUnsupported(t *testing.T) {
	if _, err := targets.Get("banana"); err == nil {
		t.Fatal(`Get("banana") succeeded, want error`)
	}
}

// TestPinCount guards the target definitions against missing or duplicated GPIOs.
func TestPinCount(t *testing.T) {
	want := map[string]int{"esp32": 34, "esp32-c3": 22, "esp32-s3": 45, "esp32-c6": 31}
	for _, target := range targets.All() {
		if got := len(target.Pins()); got != want[target.ID] {
			t.Errorf("%s has %d GPIOs, want %d", target.ID, got, want[target.ID])
		}
	}
}

func TestGPIOExists(t *testing.T) {
	pin, err := mustGet(t, "esp32").GPIO(4)
	if err != nil {
		t.Fatal(err)
	}
	if pin.Number != 4 || !pin.Input || !pin.Output || !pin.Pull || pin.Reserved {
		t.Errorf("GPIO4 = %+v, want input/output with pull resistors", pin)
	}
	if pin.ADC == nil || *pin.ADC != (targets.ADCInfo{Unit: 2, Channel: 0}) {
		t.Errorf("GPIO4 ADC = %v, want ADC2 channel 0", pin.ADC)
	}
}

// TestADCMapping checks every GPIO -> ADC unit/channel pair against
// ESP-IDF's soc/<target>/include/soc/adc_channel.h. Pins not listed must
// have no ADC.
func TestADCMapping(t *testing.T) {
	type adc = targets.ADCInfo
	want := map[string]map[int]adc{
		"esp32": {
			36: {1, 0}, 37: {1, 1}, 38: {1, 2}, 39: {1, 3}, 32: {1, 4}, 33: {1, 5}, 34: {1, 6}, 35: {1, 7},
			4: {2, 0}, 0: {2, 1}, 2: {2, 2}, 15: {2, 3}, 13: {2, 4}, 12: {2, 5}, 14: {2, 6}, 27: {2, 7}, 25: {2, 8}, 26: {2, 9},
		},
		// GPIO5 (ADC2 CH0) is left out on purpose: ADC2 oneshot is unsupported on ESP32-C3.
		"esp32-c3": {0: {1, 0}, 1: {1, 1}, 2: {1, 2}, 3: {1, 3}, 4: {1, 4}},
		"esp32-s3": {
			1: {1, 0}, 2: {1, 1}, 3: {1, 2}, 4: {1, 3}, 5: {1, 4}, 6: {1, 5}, 7: {1, 6}, 8: {1, 7}, 9: {1, 8}, 10: {1, 9},
			11: {2, 0}, 12: {2, 1}, 13: {2, 2}, 14: {2, 3}, 15: {2, 4}, 16: {2, 5}, 17: {2, 6}, 18: {2, 7}, 19: {2, 8}, 20: {2, 9},
		},
		"esp32-c6": {0: {1, 0}, 1: {1, 1}, 2: {1, 2}, 3: {1, 3}, 4: {1, 4}, 5: {1, 5}, 6: {1, 6}},
	}
	for _, target := range targets.All() {
		for _, pin := range target.Pins() {
			wantADC, hasADC := want[target.ID][pin.Number]
			switch {
			case hasADC && pin.ADC == nil:
				t.Errorf("%s GPIO%d: no ADC, want %+v", target.ID, pin.Number, wantADC)
			case !hasADC && pin.ADC != nil:
				t.Errorf("%s GPIO%d: ADC %+v, want none", target.ID, pin.Number, *pin.ADC)
			case hasADC && *pin.ADC != wantADC:
				t.Errorf("%s GPIO%d: ADC %+v, want %+v", target.ID, pin.Number, *pin.ADC, wantADC)
			}
		}
	}
}

func TestPinCannotModifyRegistry(t *testing.T) {
	esp32 := mustGet(t, "esp32")
	pin, _ := esp32.Pin(34)
	pin.ADC.Channel = 99
	again, _ := esp32.Pin(34)
	if again.ADC.Channel != 6 {
		t.Errorf("registry modified through a returned pin: channel = %d", again.ADC.Channel)
	}
}

func TestGPIOMissing(t *testing.T) {
	tests := []struct {
		id      string
		number  int
		wantErr string
	}{
		{"esp32-c3", 50, "GPIO50 is not available on ESP32-C3"},
		{"esp32", 20, "GPIO20 is not available on ESP32"},
		{"esp32-s3", 22, "GPIO22 is not available on ESP32-S3"},
	}
	for _, tt := range tests {
		target := mustGet(t, tt.id)
		if _, ok := target.Pin(tt.number); ok {
			t.Errorf("%s: Pin(%d) found, want missing", tt.id, tt.number)
		}
		_, err := target.GPIO(tt.number)
		if err == nil || err.Error() != tt.wantErr {
			t.Errorf("%s: GPIO(%d) error = %v, want %q", tt.id, tt.number, err, tt.wantErr)
		}
	}
}

func TestESP32InputOnly(t *testing.T) {
	esp32 := mustGet(t, "esp32")
	for _, number := range []int{34, 35, 36, 37, 38, 39} {
		pin, _ := esp32.Pin(number)
		if !pin.Input || pin.Output || pin.Pull {
			t.Errorf("GPIO%d: Input=%v Output=%v Pull=%v, want input only without pull resistors",
				number, pin.Input, pin.Output, pin.Pull)
		}
	}
	if pin, _ := esp32.Pin(33); !pin.Output {
		t.Error("GPIO33 should support output")
	}
}

func TestReservedFlashPins(t *testing.T) {
	esp32 := mustGet(t, "esp32")
	if pin, _ := esp32.Pin(6); !pin.Reserved {
		t.Error("ESP32 GPIO6 should be reserved")
	}
	if pin, _ := esp32.Pin(5); pin.Reserved {
		t.Error("ESP32 GPIO5 should not be reserved")
	}
}

func TestPinsSorted(t *testing.T) {
	pins := mustGet(t, "esp32").Pins()
	for i := 1; i < len(pins); i++ {
		if pins[i-1].Number >= pins[i].Number {
			t.Fatalf("Pins() not sorted at index %d: %d then %d", i, pins[i-1].Number, pins[i].Number)
		}
	}
}

// TestLEDC checks the LEDC data against ESP-IDF's soc_caps.h and clk_tree_defs.h.
func TestLEDC(t *testing.T) {
	want := map[string]targets.LEDC{
		"esp32":    {Timers: 4, Channels: 8, HighSpeedMode: true, MaxResolution: 20, ClockSource: "LEDC_USE_APB_CLK", ClockHz: 80_000_000},
		"esp32-c3": {Timers: 4, Channels: 6, MaxResolution: 14, ClockSource: "LEDC_USE_APB_CLK", ClockHz: 80_000_000},
		"esp32-s3": {Timers: 4, Channels: 8, MaxResolution: 14, ClockSource: "LEDC_USE_APB_CLK", ClockHz: 80_000_000},
		"esp32-c6": {Timers: 4, Channels: 6, MaxResolution: 20, ClockSource: "LEDC_USE_PLL_DIV_CLK", ClockHz: 80_000_000},
	}
	for _, target := range targets.All() {
		if target.LEDC != want[target.ID] {
			t.Errorf("%s LEDC = %+v, want %+v", target.ID, target.LEDC, want[target.ID])
		}
	}
}

func TestLEDCTimerFits(t *testing.T) {
	ledc := mustGet(t, "esp32").LEDC
	tests := []struct {
		frequency, resolution int
		want                  bool
	}{
		{5000, 13, true},       // divider 1.95
		{5000, 14, false},      // 5 kHz * 2^14 = 81.92 MHz, more than the 80 MHz clock
		{40_000_000, 1, true},  // divider exactly 1
		{40_000_000, 2, false}, // divider below 1
		{1, 17, true},          // divider 610, within the 10-bit integer part
		{1, 16, false},         // divider 1220, too large
		{5000, 21, false},      // above ESP32's 20-bit timers
		{0, 13, false},
	}
	for _, tt := range tests {
		if got := ledc.TimerFits(tt.frequency, tt.resolution); got != tt.want {
			t.Errorf("TimerFits(%d, %d) = %v, want %v", tt.frequency, tt.resolution, got, tt.want)
		}
	}
}

// TestI2C checks the HP I2C controller counts against SOC_HP_I2C_NUM in
// ESP-IDF's soc/<target>/include/soc/soc_caps.h.
func TestI2C(t *testing.T) {
	want := map[string]int{"esp32": 2, "esp32-c3": 1, "esp32-s3": 2, "esp32-c6": 1}
	for _, target := range targets.All() {
		if got := target.I2C.HPControllers; got != want[target.ID] {
			t.Errorf("%s has %d HP I2C controllers, want %d", target.ID, got, want[target.ID])
		}
	}
}
