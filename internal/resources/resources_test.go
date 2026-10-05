package resources_test

import (
	"fmt"
	"strings"
	"testing"

	"parrot/internal/resources"
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

func gpio(name string, pin int) resources.Claim {
	return resources.Claim{Component: name, Needs: resources.Needs{GPIOs: []int{pin}}}
}

func pwm(name string, pin, frequency, resolution int) resources.Claim {
	return resources.Claim{Component: name, Needs: resources.Needs{
		GPIOs: []int{pin},
		LEDC:  &resources.LEDCTimer{Frequency: frequency, Resolution: resolution},
	}}
}

func TestGPIOConflicts(t *testing.T) {
	esp32 := mustTarget(t, "esp32")
	tests := []struct {
		name   string
		claims []resources.Claim
	}{
		{"button on LED pin", []resources.Claim{gpio("status", 4), gpio("user", 4)}},
		{"two buttons", []resources.Claim{gpio("status", 4), gpio("other", 5), gpio("user", 4)}},
		{"PWM on LED pin", []resources.Claim{gpio("status", 4), pwm("fan", 4, 5000, 13)}},
	}
	for _, tt := range tests {
		_, err := resources.Allocate(esp32, tt.claims)
		if err == nil || err.Error() != `GPIO4 is already used by component "status"` {
			t.Errorf("%s: error = %v", tt.name, err)
		}
	}
}

func TestLEDCAllocation(t *testing.T) {
	alloc, err := resources.Allocate(mustTarget(t, "esp32"), []resources.Claim{
		pwm("a", 4, 5000, 13),
		gpio("led", 5),
		pwm("b", 18, 1000, 13),
		pwm("c", 19, 5000, 13), // same timer configuration as "a"
		pwm("d", 21, 5000, 10), // same frequency, different resolution
	})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string][2]int{ // timer, channel
		"a": {0, 0},
		"b": {1, 1},
		"c": {0, 2},
		"d": {2, 3},
	}
	if len(alloc.LEDC) != len(want) {
		t.Errorf("allocated %d LEDC users, want %d: %+v", len(alloc.LEDC), len(want), alloc.LEDC)
	}
	for name, tc := range want {
		got := alloc.LEDC[name]
		if got.Timer != tc[0] || got.Channel != tc[1] {
			t.Errorf("%s: timer %d channel %d, want timer %d channel %d", name, got.Timer, got.Channel, tc[0], tc[1])
		}
	}
	if alloc.LEDC["c"].Frequency != 5000 || alloc.LEDC["c"].Resolution != 13 {
		t.Errorf("c: assignment does not carry its timer configuration: %+v", alloc.LEDC["c"])
	}
}

// Appending a claim must not move the resources of earlier ones.
func TestLEDCAllocationIsStable(t *testing.T) {
	esp32 := mustTarget(t, "esp32")
	claims := []resources.Claim{pwm("a", 4, 5000, 13), pwm("b", 5, 1000, 13)}
	before, _ := resources.Allocate(esp32, claims)
	after, err := resources.Allocate(esp32, append(claims, pwm("c", 18, 2000, 13)))
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"a", "b"} {
		if before.LEDC[name] != after.LEDC[name] {
			t.Errorf("%s moved from %+v to %+v", name, before.LEDC[name], after.LEDC[name])
		}
	}
}

func TestLEDCChannelsExhausted(t *testing.T) {
	tests := []struct {
		target   string
		channels int
	}{
		{"esp32", 8},
		{"esp32-c3", 6},
		{"esp32-s3", 8},
		{"esp32-c6", 6},
	}
	for _, tt := range tests {
		target := mustTarget(t, tt.target)
		var claims []resources.Claim
		for i := 0; i < tt.channels; i++ {
			claims = append(claims, pwm(string(rune('a'+i)), i, 5000, 13))
		}
		if _, err := resources.Allocate(target, claims); err != nil {
			t.Errorf("%s: %d PWMs: %v", tt.target, tt.channels, err)
		}
		claims = append(claims, pwm("extra", 40, 5000, 13))
		_, err := resources.Allocate(target, claims)
		want := "no LEDC channels available on " + target.DisplayName
		if err == nil || err.Error() != want {
			t.Errorf("%s: %d PWMs: error = %v, want %q", tt.target, tt.channels+1, err, want)
		}
	}
}

func TestLEDCTimersExhausted(t *testing.T) {
	var claims []resources.Claim
	for i, freq := range []int{1000, 2000, 3000, 4000} {
		claims = append(claims, pwm(string(rune('a'+i)), i, freq, 13))
	}
	esp32 := mustTarget(t, "esp32")
	if _, err := resources.Allocate(esp32, claims); err != nil {
		t.Fatalf("4 frequencies: %v", err)
	}
	// A 5th PWM at an existing frequency still fits: it shares a timer.
	if _, err := resources.Allocate(esp32, append(claims, pwm("same", 10, 2000, 13))); err != nil {
		t.Errorf("5th PWM sharing a timer: %v", err)
	}
	_, err := resources.Allocate(esp32, append(claims, pwm("new", 10, 5000, 13)))
	if err == nil || !strings.HasPrefix(err.Error(), "no compatible LEDC timer available on ESP32") {
		t.Errorf("5th frequency: error = %v", err)
	}
}

func i2cBus(name string, sda, scl int) resources.Claim {
	return resources.Claim{Component: name, Needs: resources.Needs{GPIOs: []int{sda, scl}, I2CController: true}}
}

func TestI2CControllers(t *testing.T) {
	tests := []struct {
		target      string
		controllers int
	}{
		{"esp32", 2},
		{"esp32-c3", 1},
		{"esp32-s3", 2},
		{"esp32-c6", 1},
	}
	for _, tt := range tests {
		target := mustTarget(t, tt.target)
		claims := []resources.Claim{gpio("status", 2), pwm("fan", 3, 5000, 13)} // no I2C controller
		for i := 0; i < tt.controllers; i++ {
			claims = append(claims, i2cBus(fmt.Sprintf("bus%d", i), 4+2*i, 5+2*i))
		}
		alloc, err := resources.Allocate(target, claims)
		if err != nil || alloc.I2CControllers != tt.controllers {
			t.Errorf("%s: %d buses: %d controllers in use, error %v; want %d", tt.target, tt.controllers, alloc.I2CControllers, err, tt.controllers)
		}
		_, err = resources.Allocate(target, append(claims, i2cBus("extra", 20, 21)))
		want := "no I2C master controllers available on " + target.DisplayName
		if err == nil || err.Error() != want {
			t.Errorf("%s: %d buses: error = %v, want %q", tt.target, tt.controllers+1, err, want)
		}
	}
}

func TestNoI2CController(t *testing.T) {
	soc := targets.Target{DisplayName: "Test SoC"} // describes no I2C controller
	_, err := resources.Allocate(soc, []resources.Claim{i2cBus("sensors", 1, 2)})
	if want := "no I2C master controllers available on Test SoC"; err == nil || err.Error() != want {
		t.Errorf("error = %v, want %q", err, want)
	}
}

// The bus owns both of its GPIOs.
func TestI2CBusGPIOConflicts(t *testing.T) {
	esp32 := mustTarget(t, "esp32")
	tests := []struct {
		claims  []resources.Claim
		wantErr string
	}{
		{[]resources.Claim{gpio("status", 21), i2cBus("sensors", 21, 22)}, `GPIO21 is already used by component "status"`},
		{[]resources.Claim{gpio("status", 22), i2cBus("sensors", 21, 22)}, `GPIO22 is already used by component "status"`},
		{[]resources.Claim{i2cBus("sensors", 21, 22), gpio("status", 22)}, `GPIO22 is already used by component "sensors"`},
		{[]resources.Claim{i2cBus("sensors", 21, 22), i2cBus("display", 25, 21)}, `GPIO21 is already used by component "sensors"`},
	}
	for _, tt := range tests {
		_, err := resources.Allocate(esp32, tt.claims)
		if err == nil || err.Error() != tt.wantErr {
			t.Errorf("Allocate(%v) error = %v, want %q", tt.claims, err, tt.wantErr)
		}
	}
}

func i2cDevice(name, bus string, address uint16) resources.Claim {
	return resources.Claim{Component: name, Needs: resources.Needs{I2CAddress: &resources.I2CAddress{Bus: bus, Address: address}}}
}

func TestI2CAddresses(t *testing.T) {
	esp32 := mustTarget(t, "esp32")
	claims := []resources.Claim{
		i2cBus("sensors", 21, 22),
		i2cBus("aux", 25, 26),
		i2cDevice("display", "sensors", 0x3C),
		i2cDevice("imu", "sensors", 0x68),
		i2cDevice("display2", "aux", 0x3C), // same address, other bus: another device
	}
	alloc, err := resources.Allocate(esp32, claims)
	if err != nil {
		t.Fatal(err)
	}
	// Devices share their bus's controller and GPIOs.
	if alloc.I2CControllers != 2 {
		t.Errorf("%d I2C controllers in use, want 2 (one per bus)", alloc.I2CControllers)
	}
	_, err = resources.Allocate(esp32, append(claims, i2cDevice("conflict", "sensors", 0x3C)))
	if want := `I2C address 0x3C is already used on bus "sensors" by component "display"`; err == nil || err.Error() != want {
		t.Errorf("error = %v, want %q", err, want)
	}
	// A device claims no GPIO: the bus's pins stay the bus's.
	_, err = resources.Allocate(esp32, append(claims, gpio("status", 21)))
	if want := `GPIO21 is already used by component "sensors"`; err == nil || err.Error() != want {
		t.Errorf("error = %v, want %q", err, want)
	}
}

// A driver takes its device, and nothing the device or its bus already has.
func TestDrivenDevices(t *testing.T) {
	esp32 := mustTarget(t, "esp32")
	driver := func(name, device string) resources.Claim {
		return resources.Claim{Component: name, Needs: resources.Needs{Drives: device}}
	}
	claims := []resources.Claim{
		i2cBus("sensors", 21, 22),
		i2cDevice("environment_device", "sensors", 0x76),
		i2cDevice("outdoor_device", "sensors", 0x77),
		driver("environment", "environment_device"),
		driver("outdoor", "outdoor_device"),
	}
	alloc, err := resources.Allocate(esp32, claims)
	if err != nil {
		t.Fatal(err)
	}
	if alloc.I2CControllers != 1 {
		t.Errorf("%d I2C controllers in use, want 1", alloc.I2CControllers)
	}
	_, err = resources.Allocate(esp32, append(claims, driver("second", "environment_device")))
	if want := `component "environment_device" is already driven by component "environment"`; err == nil || err.Error() != want {
		t.Errorf("error = %v, want %q", err, want)
	}
}
