package inspect_test

import (
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"strconv"
	"testing"

	"parrot/internal/components/catalog"
	"parrot/internal/inspect"
	"parrot/internal/project"
	"parrot/internal/resources"
	"parrot/internal/targets"
)

func component(typ, name, config string) project.ComponentConfig {
	return project.ComponentConfig{Type: typ, Name: name, Config: json.RawMessage(config)}
}

func resolve(target string, components ...project.ComponentConfig) inspect.Inspection {
	return inspect.Resolve(project.Config{Target: target, Components: components})
}

// The components of the demo project: a bus with two devices, a BME280 on
// one of them, and components that use GPIOs directly.
var (
	status      = component("led", "status", `{"pin": 4}`)
	motor       = component("pwm", "motor", `{"pin": 5, "frequency": 5000}`)
	sensors     = component("i2c-bus", "sensors", `{"sda": 21, "scl": 22}`)
	envDevice   = component("i2c-device", "environment_device", `{"bus": "sensors", "address": 118, "frequency": 400000}`)
	display     = component("i2c-device", "display", `{"bus": "sensors", "address": 60, "frequency": 100000}`)
	environment = component("sensor-bme280", "environment", `{"device": "environment_device"}`)
)

func owned(name, value string) inspect.Property {
	return inspect.Property{Name: name, Value: value, Source: inspect.FromManifest, Owned: true}
}

func setting(name, value string) inspect.Property {
	return inspect.Property{Name: name, Value: value, Source: inspect.FromManifest}
}

func derived(name, value string) inspect.Property {
	return inspect.Property{Name: name, Value: value, Source: inspect.FromTarget}
}

func errorIssue(message string) inspect.Issue {
	return inspect.Issue{Severity: inspect.SeverityError, Message: message}
}

func warningIssue(message string) inspect.Issue {
	return inspect.Issue{Severity: inspect.SeverityWarning, Message: message}
}

// only returns the single top-level component of in.
func only(t *testing.T, in inspect.Inspection) inspect.Component {
	t.Helper()
	if len(in.Components) != 1 {
		t.Fatalf("got %d top-level components, want 1: %+v", len(in.Components), in.Components)
	}
	return in.Components[0]
}

func checkComponent(t *testing.T, got, want inspect.Component) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("component =\n%+v\nwant\n%+v", got, want)
	}
}

// checkCounts checks how many errors and warnings in has.
func checkCounts(t *testing.T, in inspect.Inspection, errors, warnings int) {
	t.Helper()
	if got := in.Count(inspect.SeverityError); got != errors {
		t.Errorf("errors = %d, want %d", got, errors)
	}
	if got := in.Count(inspect.SeverityWarning); got != warnings {
		t.Errorf("warnings = %d, want %d", got, warnings)
	}
}

func TestResolveEmptyProject(t *testing.T) {
	in := resolve("esp32")
	if len(in.Components) != 0 {
		t.Errorf("components = %+v, want none", in.Components)
	}
	checkCounts(t, in, 0, 0)
}

// The target comes from the registry: inspect has no mapping of its own.
func TestResolveTarget(t *testing.T) {
	for _, target := range targets.All() {
		got := resolve(target.ID).Target
		want := inspect.TargetInfo{ID: target.ID, DisplayName: target.DisplayName, IDFTarget: target.IDFTarget}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("target = %+v, want %+v", got, want)
		}
	}
	c3 := resolve("esp32-c3").Target
	if c3.DisplayName != "ESP32-C3" || c3.IDFTarget != "esp32c3" {
		t.Errorf("esp32-c3 = %+v", c3)
	}
}

// An unsupported target is an error. The components are still listed, with
// what parrot.json says: nothing can be checked or derived without a target.
func TestResolveUnsupportedTarget(t *testing.T) {
	in := resolve("esp99", status, component("adc", "light", `{"pin": 34}`))
	want := inspect.TargetInfo{ID: "esp99", Issues: []inspect.Issue{
		errorIssue(`unsupported target "esp99" (supported: esp32, esp32-c3, esp32-s3, esp32-c6)`),
	}}
	if !reflect.DeepEqual(in.Target, want) {
		t.Errorf("target = %+v, want %+v", in.Target, want)
	}
	if got := in.Components[1].Properties; !reflect.DeepEqual(got, []inspect.Property{owned("", "GPIO34")}) {
		t.Errorf("ADC properties = %+v, want only its GPIO", got)
	}
	checkCounts(t, in, 1, 0)
}

func TestResolveLED(t *testing.T) {
	checkComponent(t, only(t, resolve("esp32", status)), inspect.Component{
		Name: "status", Type: "led", Label: "LED",
		Properties: []inspect.Property{owned("", "GPIO4")},
	})
}

func TestResolveButton(t *testing.T) {
	checkComponent(t, only(t, resolve("esp32", component("button", "user", `{"pin": 18}`))), inspect.Component{
		Name: "user", Type: "button", Label: "Button",
		Properties: []inspect.Property{owned("", "GPIO18")},
	})
}

// The checks are the component packages' own, the ones `parrot add` runs.
func TestResolveComponentValidation(t *testing.T) {
	tests := []struct {
		component project.ComponentConfig
		wantErr   string
	}{
		{component("led", "power", `{"pin": 34}`), "GPIO34 cannot be used as an output on ESP32"},
		{component("button", "user", `{"pin": 34}`), "GPIO34 has no internal pull-up on ESP32, which the button needs"},
		{component("adc", "light", `{"pin": 18}`), "GPIO18 does not support ADC on ESP32"},
		{component("pwm", "fan", `{"pin": 34, "frequency": 5000}`), "GPIO34 cannot be used as an output on ESP32"},
		{component("i2c-bus", "bus", `{"sda": 34, "scl": 23}`),
			"GPIO34 cannot be used for I2C SDA on ESP32: I2C lines need a GPIO that is both input and output"},
		{component("led", "flash", `{"pin": 6}`), "GPIO6 is reserved for the SPI flash on ESP32"},
		// Found by the claim, which validates the device's config.
		{component("i2c-device", "dev", `{"bus": "sensors", "address": 120, "frequency": 400000}`),
			"I2C address 0x78 is reserved by the I2C specification (0x00-0x07 and 0x78-0x7F); if 0x78 is an 8-bit address that includes the R/W bit, use 0x3C"},
	}
	for _, tt := range tests {
		in := resolve("esp32", sensors, tt.component)
		c := in.Components[len(in.Components)-1]
		if tt.component.Type == "i2c-device" {
			c = in.Components[0].Children[0].Components[0]
		}
		if want := []inspect.Issue{errorIssue(tt.wantErr)}; !reflect.DeepEqual(c.Issues, want) {
			t.Errorf("%s issues = %+v, want %+v", tt.component.Name, c.Issues, want)
		}
	}
}

// An invalid config is reported once, although both the claim and the
// component's description decode it.
func TestResolveInvalidConfig(t *testing.T) {
	c := only(t, resolve("esp32", component("led", "status", `{"pin": 4, "frequency": 5}`)))
	want := []inspect.Issue{errorIssue(`invalid config for component "status" in parrot.json: json: unknown field "frequency"`)}
	if !reflect.DeepEqual(c.Issues, want) {
		t.Errorf("issues = %+v, want %+v", c.Issues, want)
	}
}

// parrot.json only has the GPIO; the ADC unit and channel come from the target.
func TestResolveADCDerivesUnitAndChannel(t *testing.T) {
	tests := []struct {
		target        string
		pin           int
		unit, channel string
	}{
		{"esp32", 34, "1", "6"},
		{"esp32", 36, "1", "0"},
		{"esp32", 4, "2", "0"},
		{"esp32-c3", 2, "1", "2"},
	}
	for _, tt := range tests {
		c := only(t, resolve(tt.target, component("adc", "light", fmt.Sprintf(`{"pin": %d}`, tt.pin))))
		want := []inspect.Property{
			owned("", fmt.Sprintf("GPIO%d", tt.pin)),
			derived("ADC Unit", tt.unit),
			derived("ADC Channel", tt.channel),
		}
		if !reflect.DeepEqual(c.Properties, want) || len(c.Issues) != 0 {
			t.Errorf("%s GPIO%d: properties = %+v, issues = %+v; want %+v", tt.target, tt.pin, c.Properties, c.Issues, want)
		}
	}
}

// LEDC timers and channels come from the allocator code generation uses, so
// both reach the same values.
func TestResolvePWMAllocation(t *testing.T) {
	cfg := project.Config{Target: "esp32", Components: []project.ComponentConfig{
		component("pwm", "brightness", `{"pin": 4, "frequency": 5000}`),
		component("led", "status", `{"pin": 2}`),
		component("pwm", "fan", `{"pin": 18, "frequency": 5000}`),
		component("pwm", "buzzer", `{"pin": 19, "frequency": 2000}`),
	}}
	in := inspect.Resolve(cfg)

	// What generation computes: the same claims, the same allocator.
	esp32, _ := targets.Get("esp32")
	claims, err := catalog.Claims(cfg, esp32)
	if err != nil {
		t.Fatal(err)
	}
	alloc, err := resources.Allocate(esp32, claims)
	if err != nil {
		t.Fatal(err)
	}

	// brightness and fan share timer 0; buzzer needs timer 1. Channels follow the order.
	want := []struct {
		index          int // in parrot.json
		gpio, freq     string
		timer, channel int
	}{
		{0, "GPIO4", "5000 Hz", 0, 0},
		{2, "GPIO18", "5000 Hz", 0, 1},
		{3, "GPIO19", "2000 Hz", 1, 2},
	}
	for _, w := range want {
		name := cfg.Components[w.index].Name
		if g := alloc.LEDC[name]; g.Timer != w.timer || g.Channel != w.channel {
			t.Fatalf("generation gives %s timer %d channel %d, want %d and %d", name, g.Timer, g.Channel, w.timer, w.channel)
		}
		checkComponent(t, in.Components[w.index], inspect.Component{
			Name: name, Type: "pwm", Label: "PWM",
			Properties: []inspect.Property{
				owned("", w.gpio),
				setting("Frequency", w.freq),
				derived("Duty Resolution", "13 bits"),
				{Name: "LEDC Timer", Value: strconv.Itoa(w.timer), Source: inspect.FromAllocation},
				{Name: "LEDC Channel", Value: strconv.Itoa(w.channel), Source: inspect.FromAllocation, Owned: true},
			},
		})
	}
	checkCounts(t, in, 0, 0)
}

// The bus owns its two GPIOs. The controller is not shown: ESP-IDF picks it.
func TestResolveI2CBus(t *testing.T) {
	checkComponent(t, only(t, resolve("esp32", sensors)), inspect.Component{
		Name: "sensors", Type: "i2c-bus", Label: "I2C Bus",
		Properties: []inspect.Property{owned("SDA", "GPIO21"), owned("SCL", "GPIO22")},
	})
}

func deviceComponent(name, address, frequency string, children ...inspect.Group) inspect.Component {
	return inspect.Component{
		Name: name, Type: "i2c-device", Label: "I2C Device",
		Properties:   []inspect.Property{owned("Address", address), setting("Frequency", frequency)},
		Dependencies: []inspect.Dependency{{Role: "bus", Name: "sensors", Type: "i2c-bus", Resolved: true}},
		Children:     children,
	}
}

func busComponent(devices ...inspect.Component) inspect.Component {
	bus := inspect.Component{
		Name: "sensors", Type: "i2c-bus", Label: "I2C Bus",
		Properties: []inspect.Property{owned("SDA", "GPIO21"), owned("SCL", "GPIO22")},
	}
	if len(devices) > 0 {
		bus.Children = []inspect.Group{{Name: "Devices", Components: devices}}
	}
	return bus
}

var environmentComponent = inspect.Component{
	Name: "environment", Type: "sensor-bme280", Label: "BME280",
	Dependencies: []inspect.Dependency{{Role: "device", Name: "environment_device", Type: "i2c-device", Resolved: true}},
}

// A device is listed under its bus, not at the top level.
func TestResolveI2CDevice(t *testing.T) {
	checkComponent(t, only(t, resolve("esp32", sensors, envDevice)),
		busComponent(deviceComponent("environment_device", "0x76", "400000 Hz")))
}

// Devices on the same bus are listed under it in manifest order; devices on
// another bus go under that one.
func TestResolveDevicesOnTheSameBus(t *testing.T) {
	in := resolve("esp32",
		sensors,
		component("i2c-bus", "display_bus", `{"sda": 18, "scl": 19}`),
		display,
		component("i2c-device", "panel", `{"bus": "display_bus", "address": 61, "frequency": 400000}`),
		envDevice,
	)
	if len(in.Components) != 2 {
		t.Fatalf("top level = %+v, want the two buses", in.Components)
	}
	checkComponent(t, in.Components[0], busComponent(
		deviceComponent("display", "0x3C", "100000 Hz"),
		deviceComponent("environment_device", "0x76", "400000 Hz"),
	))
	panels := in.Components[1].Children
	if len(panels) != 1 || len(panels[0].Components) != 1 || panels[0].Components[0].Name != "panel" {
		t.Errorf("display_bus children = %+v, want panel", panels)
	}
}

// The full chain BME280 -> I2C device -> I2C bus becomes one tree, whatever
// the order of the manifest.
func TestResolveBME280Chain(t *testing.T) {
	want := busComponent(
		deviceComponent("environment_device", "0x76", "400000 Hz",
			inspect.Group{Name: "Sensor", Components: []inspect.Component{environmentComponent}}),
		deviceComponent("display", "0x3C", "100000 Hz"),
	)
	checkComponent(t, only(t, resolve("esp32", sensors, envDevice, display, environment)), want)
	// Built on components listed after them: still nested, siblings still in manifest order.
	checkComponent(t, only(t, resolve("esp32", environment, envDevice, display, sensors)), want)
}

func TestResolveMissingDependency(t *testing.T) {
	in := resolve("esp32", environment, component("i2c-device", "lost", `{"bus": "nobus", "address": 118, "frequency": 400000}`))
	checkComponent(t, in.Components[0], inspect.Component{
		Name: "environment", Type: "sensor-bme280", Label: "BME280",
		Dependencies: []inspect.Dependency{{Role: "device", Name: "environment_device", Type: "i2c-device"}},
		Issues:       []inspect.Issue{errorIssue(`dependency "environment_device" not found`)},
	})
	if got := in.Components[1].Issues; !reflect.DeepEqual(got, []inspect.Issue{errorIssue(`dependency "nobus" not found`)}) {
		t.Errorf("device issues = %+v", got)
	}
	checkCounts(t, in, 2, 0)
}

func TestResolveWrongDependencyType(t *testing.T) {
	in := resolve("esp32",
		status,
		sensors,
		component("sensor-bme280", "on_led", `{"device": "status"}`),
		component("sensor-bme280", "on_bus", `{"device": "sensors"}`),
		component("i2c-device", "on_led_bus", `{"bus": "status", "address": 118, "frequency": 400000}`),
	)
	wantErrs := map[string]string{
		"on_led":     `"status" is not an I2C device (its type is "led")`,
		"on_bus":     `"sensors" is not an I2C device (its type is "i2c-bus")`,
		"on_led_bus": `"status" is not an I2C bus (its type is "led")`,
	}
	if len(in.Components) != 5 {
		t.Fatalf("top level = %+v, want every component", in.Components)
	}
	for _, c := range in.Components[2:] {
		if want := []inspect.Issue{errorIssue(wantErrs[c.Name])}; !reflect.DeepEqual(c.Issues, want) {
			t.Errorf("%s issues = %+v, want %+v", c.Name, c.Issues, want)
		}
		if c.Dependencies[0].Resolved {
			t.Errorf("%s dependency resolved, want unresolved", c.Name)
		}
	}
	checkCounts(t, in, 3, 0)
}

// The BME280's own rule on its device: the device exists with the right type,
// so the sensor stays under it, with the error.
func TestResolveBME280DeviceAddress(t *testing.T) {
	in := resolve("esp32", sensors, display, component("sensor-bme280", "environment", `{"device": "display"}`))
	sensor := only(t, in).Children[0].Components[0].Children[0].Components[0]
	want := []inspect.Issue{errorIssue(`I2C device "display" uses address 0x3C; BME280 expects 0x76 or 0x77`)}
	if sensor.Name != "environment" || !reflect.DeepEqual(sensor.Issues, want) {
		t.Errorf("sensor = %+v, want environment with %+v", sensor, want)
	}
}

// A second component with the same name makes the project invalid. Names
// refer to the first one, as in code generation.
func TestResolveDuplicateNames(t *testing.T) {
	in := resolve("esp32", sensors, component("led", "sensors", `{"pin": 4}`), envDevice)
	if len(in.Components) != 2 {
		t.Fatalf("top level = %+v, want the bus and the duplicate", in.Components)
	}
	checkComponent(t, in.Components[0], busComponent(deviceComponent("environment_device", "0x76", "400000 Hz")))
	dup := in.Components[1]
	want := []inspect.Issue{errorIssue(`duplicate component name "sensors": an earlier component in parrot.json has it`)}
	if dup.Type != "led" || !reflect.DeepEqual(dup.Issues, want) {
		t.Errorf("duplicate = %+v, want the LED with %+v", dup, want)
	}
	checkCounts(t, in, 1, 0)
}

// An unknown type, e.g. from a newer Parrot, is a warning, not an error. The
// LEDC allocation of the components after it cannot be known.
func TestResolveUnknownType(t *testing.T) {
	in := resolve("esp32",
		component("pwm", "before", `{"pin": 4, "frequency": 5000}`),
		component("future-component", "something", `{"anything": [1, 2]}`),
		component("pwm", "after", `{"pin": 5, "frequency": 5000}`),
	)
	checkComponent(t, in.Components[1], inspect.Component{
		Name: "something", Type: "future-component",
		Issues: []inspect.Issue{warningIssue("unknown component type: this version of Parrot cannot inspect it")},
	})
	if before := in.Components[0].Properties; len(before) != 5 || before[4].Name != "LEDC Channel" {
		t.Errorf("before properties = %+v, want its LEDC channel", before)
	}
	after := in.Components[2]
	wantIssues := []inspect.Issue{warningIssue(`LEDC timer and channel cannot be determined: "something" comes earlier in parrot.json and cannot be allocated`)}
	if len(after.Properties) != 3 || !reflect.DeepEqual(after.Issues, wantIssues) {
		t.Errorf("after = %+v, want no LEDC and %+v", after, wantIssues)
	}
	checkCounts(t, in, 0, 2)
}

// A resource conflict is reported on the component that causes it, and the
// allocation before it is kept.
func TestResolveResourceConflict(t *testing.T) {
	in := resolve("esp32",
		status,
		component("pwm", "fan", `{"pin": 18, "frequency": 5000}`),
		component("button", "user", `{"pin": 4}`),
		component("pwm", "pump", `{"pin": 19, "frequency": 5000}`),
	)
	user := in.Components[2]
	if want := []inspect.Issue{errorIssue(`GPIO4 is already used by component "status"`)}; !reflect.DeepEqual(user.Issues, want) {
		t.Errorf("user issues = %+v, want %+v", user.Issues, want)
	}
	if fan := in.Components[1]; len(fan.Properties) != 5 || len(fan.Issues) != 0 {
		t.Errorf("fan = %+v, want its LEDC channel", fan)
	}
	if pump := in.Components[3]; len(pump.Properties) != 3 || len(pump.Issues) != 1 {
		t.Errorf("pump = %+v, want no LEDC and a warning", pump)
	}
	checkCounts(t, in, 1, 1)
}

func TestResolveInvalidName(t *testing.T) {
	c := only(t, resolve("esp32", component("led", "", `{"pin": 4}`)))
	want := []inspect.Issue{errorIssue(`invalid component name "": start with a letter, then use letters, digits, '-' and '_'`)}
	if !reflect.DeepEqual(c.Issues, want) {
		t.Errorf("issues = %+v, want %+v", c.Issues, want)
	}
}

// The top-level components keep the manifest order, and resolving twice
// gives the same result: nothing depends on map iteration order.
func TestResolveDeterministic(t *testing.T) {
	cfg := project.Config{Target: "esp32", Components: []project.ComponentConfig{
		component("pwm", "zeta", `{"pin": 18, "frequency": 1000}`),
		environment,
		component("adc", "alpha", `{"pin": 34}`),
		display,
		component("future-component", "mid", `{}`),
		sensors,
		envDevice,
		status,
		component("i2c-device", "ghost", `{"bus": "nobus", "address": 80, "frequency": 400000}`),
	}}
	first := inspect.Resolve(cfg)
	var names []string
	for _, c := range first.Components {
		names = append(names, c.Name)
	}
	if want := []string{"zeta", "alpha", "mid", "sensors", "status", "ghost"}; !reflect.DeepEqual(names, want) {
		t.Errorf("top level = %v, want %v", names, want)
	}
	for range 50 {
		if again := inspect.Resolve(cfg); !reflect.DeepEqual(again, first) {
			t.Fatalf("Resolve gave a different result:\n%+v\nthen\n%+v", first, again)
		}
	}
}

func host(symbol string) inspect.Property {
	return inspect.Property{Name: "Host", Value: symbol, Source: inspect.FromAllocation, Owned: true}
}

// The SPI host comes from the allocator code generation uses: the first bus
// gets the target's first host, the next bus the next one.
func TestResolveSPIBus(t *testing.T) {
	cfg := project.Config{Target: "esp32", Components: []project.ComponentConfig{
		component("spi-bus", "main_bus", `{"mosi": 23, "miso": 19, "sclk": 18}`),
		status,
		component("spi-bus", "display_bus", `{"mosi": 13, "sclk": 14}`),
	}}
	in := inspect.Resolve(cfg)

	esp32, _ := targets.Get("esp32")
	claims, err := catalog.Claims(cfg, esp32)
	if err != nil {
		t.Fatal(err)
	}
	alloc, err := resources.Allocate(esp32, claims)
	if err != nil {
		t.Fatal(err)
	}
	if alloc.SPIHosts["main_bus"].Symbol != "SPI2_HOST" || alloc.SPIHosts["display_bus"].Symbol != "SPI3_HOST" {
		t.Fatalf("generation gives hosts %v, want SPI2_HOST then SPI3_HOST", alloc.SPIHosts)
	}

	checkComponent(t, in.Components[0], inspect.Component{
		Name: "main_bus", Type: "spi-bus", Label: "SPI Bus",
		Properties: []inspect.Property{host("SPI2_HOST"), owned("MOSI", "GPIO23"), owned("MISO", "GPIO19"), owned("SCLK", "GPIO18")},
	})
	// No MISO: shown as disabled, not as a made-up GPIO.
	checkComponent(t, in.Components[2], inspect.Component{
		Name: "display_bus", Type: "spi-bus", Label: "SPI Bus",
		Properties: []inspect.Property{host("SPI3_HOST"), owned("MOSI", "GPIO13"), setting("MISO", "disabled"), owned("SCLK", "GPIO14")},
	})
	checkCounts(t, in, 0, 0)
}

func TestResolveSPIBusProblems(t *testing.T) {
	in := resolve("esp32-c3",
		component("spi-bus", "main_bus", `{"mosi": 7, "miso": 2, "sclk": 6}`),
		component("spi-bus", "second", `{"mosi": 4, "sclk": 5}`),
	)
	if want := []inspect.Issue{errorIssue("no SPI hosts available on ESP32-C3")}; !reflect.DeepEqual(in.Components[1].Issues, want) {
		t.Errorf("second bus issues = %+v, want %+v", in.Components[1].Issues, want)
	}

	in = resolve("esp32", component("spi-bus", "main_bus", `{"mosi": 34, "miso": 19, "sclk": 18}`))
	want := []inspect.Issue{errorIssue("GPIO34 cannot be used for SPI MOSI on ESP32: the master drives MOSI, so it needs an output GPIO")}
	if c := only(t, in); !reflect.DeepEqual(c.Issues, want) {
		t.Errorf("issues = %+v, want %+v", c.Issues, want)
	}

	in = resolve("esp32", component("spi-bus", "main_bus", `{"mosi": 23, "miso": 23, "sclk": 18}`))
	if c := only(t, in); len(c.Issues) != 1 || c.Issues[0].Message != "MOSI and MISO cannot use the same GPIO" {
		t.Errorf("issues = %+v, want one about MOSI and MISO", c.Issues)
	}

	// After a component that cannot be allocated, the host is not known.
	in = resolve("esp32", component("future-component", "something", `{}`), component("spi-bus", "main_bus", `{"mosi": 23, "sclk": 18}`))
	bus := in.Components[1]
	if bus.Properties[0].Name == "Host" || len(bus.Issues) != 1 ||
		bus.Issues[0].Message != `SPI host cannot be determined: "something" comes earlier in parrot.json and cannot be allocated` {
		t.Errorf("bus = %+v, want no host and a warning", bus)
	}
}

var mainBus = component("spi-bus", "main_bus", `{"mosi": 23, "miso": 19, "sclk": 18}`)

func spiDevice(name string, cs, frequency, mode int) project.ComponentConfig {
	return component("spi-device", name, fmt.Sprintf(`{"bus": "main_bus", "cs": %d, "frequency": %d, "mode": %d}`, cs, frequency, mode))
}

func spiDeviceComponent(name, cs, frequency, mode string) inspect.Component {
	return inspect.Component{
		Name: name, Type: "spi-device", Label: "SPI Device",
		Properties:   []inspect.Property{owned("CS", cs), setting("Frequency", frequency), setting("Mode", mode)},
		Dependencies: []inspect.Dependency{{Role: "bus", Name: "main_bus", Type: "spi-bus", Resolved: true}},
	}
}

// SPI devices are listed under their bus, in manifest order, and only there,
// each with its own CS, frequency and mode.
func TestResolveSPIDevices(t *testing.T) {
	in := resolve("esp32", spiDevice("display", 5, 10_000_000, 0), status, mainBus, spiDevice("sensor", 17, 1_000_000, 3))
	if len(in.Components) != 2 || in.Components[0].Name != "status" {
		t.Fatalf("top level = %+v, want status and main_bus", in.Components)
	}
	checkComponent(t, in.Components[1], inspect.Component{
		Name: "main_bus", Type: "spi-bus", Label: "SPI Bus",
		Properties: []inspect.Property{host("SPI2_HOST"), owned("MOSI", "GPIO23"), owned("MISO", "GPIO19"), owned("SCLK", "GPIO18")},
		Children: []inspect.Group{{Name: "Devices", Components: []inspect.Component{
			spiDeviceComponent("display", "GPIO5", "10000000 Hz", "0 (CPOL 0, CPHA 0)"),
			spiDeviceComponent("sensor", "GPIO17", "1000000 Hz", "3 (CPOL 1, CPHA 1)"),
		}}},
	})
	checkCounts(t, in, 0, 0)
}

func TestResolveSPIDeviceProblems(t *testing.T) {
	tests := []struct {
		name       string
		components []project.ComponentConfig
		wantIssue  string
	}{
		{"missing bus", []project.ComponentConfig{spiDevice("display", 5, 10_000_000, 0)},
			`dependency "main_bus" not found`},
		{"wrong bus type", []project.ComponentConfig{component("led", "main_bus", `{"pin": 4}`), spiDevice("display", 5, 10_000_000, 0)},
			`"main_bus" is not a SPI bus (its type is "led")`},
		{"CS on a bus line", []project.ComponentConfig{mainBus, spiDevice("display", 23, 10_000_000, 0)},
			`GPIO23 is already used by SPI bus "main_bus"`},
		{"CS of another device", []project.ComponentConfig{mainBus, spiDevice("sensor", 5, 1_000_000, 3), spiDevice("display", 5, 10_000_000, 0)},
			`GPIO5 is already used by SPI device "sensor"`},
		{"input-only CS", []project.ComponentConfig{mainBus, spiDevice("display", 34, 10_000_000, 0)},
			"GPIO34 cannot be used for SPI CS on ESP32: the master drives CS, so it needs an output GPIO"},
		{"invalid mode", []project.ComponentConfig{mainBus, spiDevice("display", 5, 10_000_000, 4)},
			"SPI mode must be between 0 and 3"},
		{"invalid frequency", []project.ComponentConfig{mainBus, spiDevice("display", 5, 0, 0)},
			"frequency must be positive, got 0 Hz"},
	}
	for _, tt := range tests {
		in := resolve("esp32", tt.components...)
		var display *inspect.Component
		var find func(cs []inspect.Component)
		find = func(cs []inspect.Component) {
			for i := range cs {
				if cs[i].Name == "display" {
					display = &cs[i]
				}
				for _, g := range cs[i].Children {
					find(g.Components)
				}
			}
		}
		find(in.Components)
		if display == nil {
			t.Errorf("%s: display not found in %+v", tt.name, in.Components)
			continue
		}
		if !slices.Contains(display.Issues, errorIssue(tt.wantIssue)) {
			t.Errorf("%s: display issues = %+v, want %q", tt.name, display.Issues, tt.wantIssue)
		}
	}
}

// A timer shows its mode and period. Its general purpose timer is not shown:
// ESP-IDF picks it when the timer is created.
func TestResolveTimer(t *testing.T) {
	checkComponent(t, only(t, resolve("esp32", component("timer", "heartbeat", `{"mode": "periodic", "period_us": 1000000}`))), inspect.Component{
		Name: "heartbeat", Type: "timer", Label: "Timer",
		Properties: []inspect.Property{setting("Mode", "periodic"), setting("Period", "1s")},
	})
}

// The timers' problems are the ones `parrot add timer` reports: its own
// checks, and a timer too many for the target.
func TestResolveTimerProblems(t *testing.T) {
	timer := func(name string, periodUS uint64) project.ComponentConfig {
		return component("timer", name, fmt.Sprintf(`{"mode": "periodic", "period_us": %d}`, periodUS))
	}
	tests := []struct {
		name       string
		target     string
		components []project.ComponentConfig
		wantIssue  string // on the last component
	}{
		{"zero period", "esp32", []project.ComponentConfig{timer("tick", 0)},
			"period must be positive"},
		{"unsupported mode", "esp32", []project.ComponentConfig{component("timer", "tick", `{"mode": "one-shot", "period_us": 1000}`)},
			`unsupported timer mode "one-shot": Parrot only generates "periodic" timers`},
		{"period beyond the counter", "esp32-c3", []project.ComponentConfig{timer("tick", 1<<54)},
			"period of 18014398509481984 us does not fit in the 54-bit counter of the timers of ESP32-C3"},
		{"no timer left", "esp32-c3", []project.ComponentConfig{timer("a", 1000), timer("b", 2000), timer("c", 500)},
			"no general purpose timers available on ESP32-C3"},
	}
	for _, tt := range tests {
		in := resolve(tt.target, tt.components...)
		last := in.Components[len(in.Components)-1]
		if want := []inspect.Issue{errorIssue(tt.wantIssue)}; !reflect.DeepEqual(last.Issues, want) {
			t.Errorf("%s: issues = %+v, want %+v", tt.name, last.Issues, want)
		}
		checkCounts(t, in, 1, 0)
	}
}
