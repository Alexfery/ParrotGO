package resources_test

import (
	"errors"
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

// The error names the claim that failed and keeps the cause's message.
func TestAllocateClaimError(t *testing.T) {
	_, err := resources.Allocate(mustTarget(t, "esp32"), []resources.Claim{
		gpio("status", 4), pwm("fan", 5, 5000, 13), gpio("user", 4),
	})
	var claimErr *resources.ClaimError
	if !errors.As(err, &claimErr) || claimErr.Component != "user" {
		t.Fatalf("error = %#v, want a ClaimError on user", err)
	}
	if err.Error() != `GPIO4 is already used by component "status"` {
		t.Errorf("message = %q", err.Error())
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

func gpTimer(name string) resources.Claim {
	return resources.Claim{Component: name, Needs: resources.Needs{GPTimer: true}}
}

// Timers are counted, like I2C controllers: ESP-IDF picks which one each
// gets. They take no GPIO, so they never conflict with other components.
func TestGPTimers(t *testing.T) {
	tests := []struct {
		target string
		timers int
	}{
		{"esp32", 4},
		{"esp32-c3", 2},
		{"esp32-s3", 4},
		{"esp32-c6", 2},
	}
	for _, tt := range tests {
		target := mustTarget(t, tt.target)
		claims := []resources.Claim{gpio("status", 2), pwm("fan", 3, 5000, 13), i2cBus("sensors", 4, 5)} // no timer
		for i := 0; i < tt.timers; i++ {
			claims = append(claims, gpTimer(fmt.Sprintf("tick%d", i)))
		}
		alloc, err := resources.Allocate(target, claims)
		if err != nil || alloc.GPTimers != tt.timers {
			t.Errorf("%s: %d timers: %d in use, error %v; want %d", tt.target, tt.timers, alloc.GPTimers, err, tt.timers)
		}
		_, err = resources.Allocate(target, append(claims, gpTimer("extra")))
		want := "no general purpose timers available on " + target.DisplayName
		var claimErr *resources.ClaimError
		if err == nil || err.Error() != want || !errors.As(err, &claimErr) || claimErr.Component != "extra" {
			t.Errorf("%s: %d timers: error = %v, want %q on extra", tt.target, tt.timers+1, err, want)
		}
	}
}

func TestNoGPTimer(t *testing.T) {
	soc := targets.Target{DisplayName: "Test SoC"} // describes no timer
	_, err := resources.Allocate(soc, []resources.Claim{gpTimer("heartbeat")})
	if want := "no general purpose timers available on Test SoC"; err == nil || err.Error() != want {
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

func spiBus(name string, pins ...int) resources.Claim {
	return resources.Claim{Component: name, Needs: resources.Needs{GPIOs: pins, SPIHost: true}}
}

// Each bus gets the target's next SPI host, in manifest order, until there
// are none left.
func TestSPIHosts(t *testing.T) {
	for _, target := range targets.All() {
		claims := []resources.Claim{gpio("status", 2), i2cBus("sensors", 3, 4)} // no SPI host
		for i := range target.SPI.Hosts {
			claims = append(claims, spiBus(fmt.Sprintf("bus%d", i), 5+3*i, 6+3*i, 7+3*i))
		}
		alloc, err := resources.Allocate(target, claims)
		if err != nil {
			t.Fatalf("%s: %v", target.ID, err)
		}
		for i, host := range target.SPI.Hosts {
			if got := alloc.SPIHosts[fmt.Sprintf("bus%d", i)]; got != host {
				t.Errorf("%s: bus%d host = %v, want %v", target.ID, i, got, host)
			}
		}
		if len(alloc.SPIHosts) != len(target.SPI.Hosts) {
			t.Errorf("%s: hosts = %v, want one per bus", target.ID, alloc.SPIHosts)
		}

		_, err = resources.Allocate(target, append(claims, spiBus("extra", 20, 21)))
		want := "no SPI hosts available on " + target.DisplayName
		if err == nil || err.Error() != want {
			t.Errorf("%s: one bus too many: error = %v, want %q", target.ID, err, want)
		}
	}
}

func TestSPIHostsFirstAndSecond(t *testing.T) {
	alloc, err := resources.Allocate(mustTarget(t, "esp32"), []resources.Claim{
		spiBus("main_bus", 23, 19, 18),
		gpio("status", 4),
		spiBus("display_bus", 13, 14), // no MISO
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := alloc.SPIHosts["main_bus"].Symbol; got != "SPI2_HOST" {
		t.Errorf("first bus host = %s, want SPI2_HOST", got)
	}
	if got := alloc.SPIHosts["display_bus"].Symbol; got != "SPI3_HOST" {
		t.Errorf("second bus host = %s, want SPI3_HOST", got)
	}
	if _, has := alloc.SPIHosts["status"]; has {
		t.Error("an LED got an SPI host")
	}
}

func TestNoSPIHost(t *testing.T) {
	c3 := mustTarget(t, "esp32-c3")
	c3.SPI.Hosts = nil // a SoC whose SPI controllers all serve the flash
	_, err := resources.Allocate(c3, []resources.Claim{spiBus("main_bus", 4, 2, 5)})
	if want := "no SPI hosts available on ESP32-C3"; err == nil || err.Error() != want {
		t.Errorf("error = %v, want %q", err, want)
	}
}

// Appending a bus never moves the host of an earlier one.
func TestSPIHostsAreStable(t *testing.T) {
	esp32 := mustTarget(t, "esp32")
	first, err := resources.Allocate(esp32, []resources.Claim{spiBus("a", 23, 19, 18)})
	if err != nil {
		t.Fatal(err)
	}
	both, err := resources.Allocate(esp32, []resources.Claim{spiBus("a", 23, 19, 18), spiBus("b", 13, 12, 14)})
	if err != nil {
		t.Fatal(err)
	}
	if first.SPIHosts["a"] != both.SPIHosts["a"] {
		t.Errorf("adding a bus moved the first one from %v to %v", first.SPIHosts["a"], both.SPIHosts["a"])
	}
}

// The bus owns all of its GPIOs.
func TestSPIBusGPIOConflicts(t *testing.T) {
	esp32 := mustTarget(t, "esp32")
	for _, claims := range [][]resources.Claim{
		{gpio("status", 23), spiBus("main_bus", 23, 19, 18)},
		{gpio("status", 19), spiBus("main_bus", 23, 19, 18)},
		{gpio("status", 18), spiBus("main_bus", 23, 19, 18)},
	} {
		pin := claims[0].GPIOs[0]
		_, err := resources.Allocate(esp32, claims)
		if want := fmt.Sprintf("GPIO%d is already used by component \"status\"", pin); err == nil || err.Error() != want {
			t.Errorf("LED on GPIO%d: error = %v, want %q", pin, err, want)
		}
	}
}

// A claim with a kind is named by it in conflicts: a CS line on a GPIO of its
// bus, or of another device, says which one it is.
func TestConflictsNameKinds(t *testing.T) {
	esp32 := mustTarget(t, "esp32")
	bus := spiBus("main_bus", 23, 19, 18)
	bus.Kind = "SPI bus"
	device := func(name string, cs int) resources.Claim {
		return resources.Claim{Component: name, Kind: "SPI device", Needs: resources.Needs{GPIOs: []int{cs}}}
	}
	tests := []struct {
		claims  []resources.Claim
		wantErr string
	}{
		{[]resources.Claim{bus, device("display", 23)}, `GPIO23 is already used by SPI bus "main_bus"`},
		{[]resources.Claim{bus, device("display", 18)}, `GPIO18 is already used by SPI bus "main_bus"`},
		{[]resources.Claim{bus, device("display", 5), device("sensor", 5)}, `GPIO5 is already used by SPI device "display"`},
		{[]resources.Claim{gpio("status", 5), device("display", 5)}, `GPIO5 is already used by component "status"`},
	}
	for _, tt := range tests {
		_, err := resources.Allocate(esp32, tt.claims)
		if err == nil || err.Error() != tt.wantErr {
			t.Errorf("error = %v, want %q", err, tt.wantErr)
		}
	}
	// Devices on one bus share its lines: only their CS lines are theirs.
	alloc, err := resources.Allocate(esp32, []resources.Claim{bus, device("display", 5), device("sensor", 17)})
	if err != nil {
		t.Fatal(err)
	}
	if len(alloc.SPIHosts) != 1 || alloc.SPIHosts["main_bus"].Symbol != "SPI2_HOST" {
		t.Errorf("SPI hosts = %v, want SPI2_HOST for the bus only", alloc.SPIHosts)
	}
}
