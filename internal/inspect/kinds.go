package inspect

import (
	"fmt"
	"strconv"

	"parrot/internal/components/adc"
	"parrot/internal/components/button"
	"parrot/internal/components/i2c"
	"parrot/internal/components/i2cdevice"
	"parrot/internal/components/led"
	"parrot/internal/components/pwm"
	"parrot/internal/components/sensor/bme280"
	"parrot/internal/components/spi"
	"parrot/internal/components/spidevice"
	"parrot/internal/components/timer"
	"parrot/internal/project"
	"parrot/internal/targets"
)

// kind is what inspect knows about one component type. A type missing from
// kinds is reported as unknown, not rejected: a manifest written by a newer
// Parrot can still be inspected.
type kind struct {
	label string // human name of the type, e.g. "I2C Device"
	group string // the group it is listed in under the component it is built on, e.g. "Devices"

	// describe decodes the component's config, adds its properties and
	// issues, and declares the component it is built on, if any. target is
	// nil when the project's target is not supported: only what parrot.json
	// says can be described then. The checks are the component packages'
	// own, so inspect and `parrot add` apply the same rules.
	describe func(e *entry, target *targets.Target)
}

var kinds = map[string]kind{
	led.Type:       {label: "LED", describe: describeLED},
	button.Type:    {label: "Button", describe: describeButton},
	adc.Type:       {label: "ADC", describe: describeADC},
	pwm.Type:       {label: "PWM", describe: describePWM},
	i2c.Type:       {label: "I2C Bus", describe: describeI2CBus},
	i2cdevice.Type: {label: "I2C Device", group: "Devices", describe: describeI2CDevice},
	spi.Type:       {label: "SPI Bus", describe: describeSPIBus},
	spidevice.Type: {label: "SPI Device", group: "Devices", describe: describeSPIDevice},
	bme280.Type:    {label: "BME280", group: "Sensor", describe: describeBME280},
	timer.Type:     {label: "Timer", describe: describeTimer},
}

func describeLED(e *entry, target *targets.Target) {
	cfg, ok := decode[led.Config](e)
	if !ok {
		return
	}
	e.add(gpio("", cfg.Pin))
	if target != nil {
		e.check(led.Validate(*target, cfg.Pin))
	}
}

// describeButton shows the GPIO only: the button's active level is a
// convention of the generated code, not a setting of parrot.json.
func describeButton(e *entry, target *targets.Target) {
	cfg, ok := decode[button.Config](e)
	if !ok {
		return
	}
	e.add(gpio("", cfg.Pin))
	if target != nil {
		e.check(button.Validate(*target, cfg.Pin))
	}
}

// describeADC derives the ADC unit and channel from the target: parrot.json
// only has the GPIO.
func describeADC(e *entry, target *targets.Target) {
	cfg, ok := decode[adc.Config](e)
	if !ok {
		return
	}
	e.add(gpio("", cfg.Pin))
	if target == nil {
		return
	}
	info, err := adc.Validate(*target, cfg.Pin)
	if err != nil {
		e.fail(err)
		return
	}
	e.add(derived("ADC Unit", strconv.Itoa(info.Unit)), derived("ADC Channel", strconv.Itoa(info.Channel)))
}

// describePWM derives the duty resolution from the target and shows the
// LEDC timer and channel the allocator gave the output.
func describePWM(e *entry, target *targets.Target) {
	cfg, ok := decode[pwm.Config](e)
	if !ok {
		return
	}
	e.add(gpio("", cfg.Pin), setting("Frequency", hz(cfg.Frequency)))
	if target == nil {
		return
	}
	e.check(pwm.Validate(*target, cfg))
	if resolution, err := pwm.Resolution(*target, cfg.Frequency); err == nil {
		e.add(derived("Duty Resolution", fmt.Sprintf("%d bits", resolution)))
	}
	if e.allocation == nil {
		e.unallocated("LEDC timer and channel")
		return
	}
	if ledc, ok := e.allocation.LEDC[e.config.Name]; ok {
		e.add(
			Property{Name: "LEDC Timer", Value: strconv.Itoa(ledc.Timer), Source: FromAllocation},
			Property{Name: "LEDC Channel", Value: strconv.Itoa(ledc.Channel), Source: FromAllocation, Owned: true},
		)
	}
}

// describeI2CBus shows the bus's GPIOs. Its controller is not shown: ESP-IDF
// picks one when the bus is created, so Parrot does not know which.
func describeI2CBus(e *entry, target *targets.Target) {
	cfg, ok := decode[i2c.Config](e)
	if !ok {
		return
	}
	e.add(gpio("SDA", cfg.SDA), gpio("SCL", cfg.SCL))
	if target != nil {
		e.check(i2c.Validate(*target, cfg))
	}
}

// describeSPIBus shows the SPI host the allocator gave the bus, then its
// lines. A bus without MISO shows it as disabled.
func describeSPIBus(e *entry, target *targets.Target) {
	cfg, ok := decode[spi.Config](e)
	if !ok {
		return
	}
	if e.allocation == nil {
		e.unallocated("SPI host")
	} else if host, ok := e.allocation.SPIHosts[e.config.Name]; ok {
		e.add(Property{Name: "Host", Value: host.Symbol, Source: FromAllocation, Owned: true})
	}
	e.add(gpio("MOSI", cfg.MOSI))
	if cfg.MISO != nil {
		e.add(gpio("MISO", *cfg.MISO))
	} else {
		e.add(setting("MISO", "disabled"))
	}
	e.add(gpio("SCLK", cfg.SCLK))
	if target != nil {
		e.check(spi.Validate(*target, cfg))
	}
}

// describeI2CDevice puts the device on its bus. Its address and frequency
// are checked by its claim (see resolver.allocate).
func describeI2CDevice(e *entry, _ *targets.Target) {
	cfg, ok := decode[i2cdevice.Config](e)
	if !ok {
		return
	}
	e.dependsOn(requirement{role: "bus", name: cfg.Bus, typ: i2c.Type, what: "an I2C bus"})
	e.add(
		Property{Name: "Address", Value: i2cdevice.FormatAddress(cfg.Address), Source: FromManifest, Owned: true},
		setting("Frequency", hz(cfg.Frequency)),
	)
}

// describeSPIDevice puts the device on its bus, with its CS line, which it
// owns, and its clock settings. Its frequency and mode are checked by its
// claim (see resolver.allocate), and a CS on a GPIO of its bus by the
// allocator.
func describeSPIDevice(e *entry, target *targets.Target) {
	cfg, ok := decode[spidevice.Config](e)
	if !ok {
		return
	}
	e.dependsOn(requirement{role: "bus", name: cfg.Bus, typ: spi.Type, what: "a SPI bus"})
	e.add(
		gpio("CS", cfg.CS),
		setting("Frequency", hz(cfg.Frequency)),
		setting("Mode", fmt.Sprintf("%d (CPOL %d, CPHA %d)", cfg.Mode, cfg.CPOL(), cfg.CPHA())),
	)
	if target != nil {
		e.check(spidevice.Validate(*target, cfg))
	}
}

// describeBME280 puts the sensor on its device. It shows nothing else: the
// address, frequency and GPIOs belong to the device and the bus above it.
func describeBME280(e *entry, _ *targets.Target) {
	cfg, ok := decode[bme280.Config](e)
	if !ok {
		return
	}
	e.dependsOn(requirement{role: "device", name: cfg.Device, typ: i2cdevice.Type, what: "an I2C device", check: checkBME280Device})
}

// describeTimer shows the timer's mode and period. Its general purpose timer
// is not shown: ESP-IDF picks it when the timer is created, so Parrot does not
// know which.
func describeTimer(e *entry, target *targets.Target) {
	cfg, ok := decode[timer.Config](e)
	if !ok {
		return
	}
	e.add(setting("Mode", cfg.Mode), setting("Period", timer.FormatPeriod(cfg.PeriodUS)))
	if target != nil {
		e.check(timer.Validate(*target, cfg))
	}
}

// checkBME280Device applies the BME280's rule to the device it is built on.
func checkBME280Device(device project.ComponentConfig) error {
	var cfg i2cdevice.Config
	if device.Decode(&cfg) != nil {
		return nil // reported on the device itself
	}
	return bme280.CheckDevice(i2cdevice.Device{Name: device.Name, Config: cfg})
}

// decode reads e's config as T. If it cannot, the error becomes an issue of e.
func decode[T any](e *entry) (T, bool) {
	var cfg T
	if err := e.config.Decode(&cfg); err != nil {
		e.fail(err)
		return cfg, false
	}
	return cfg, true
}

// gpio is a GPIO the component owns, as parrot.json gives it.
func gpio(name string, pin int) Property {
	return Property{Name: name, Value: fmt.Sprintf("GPIO%d", pin), Source: FromManifest, Owned: true}
}

func setting(name, value string) Property {
	return Property{Name: name, Value: value, Source: FromManifest}
}

func derived(name, value string) Property {
	return Property{Name: name, Value: value, Source: FromTarget}
}

func hz(frequency int) string {
	return fmt.Sprintf("%d Hz", frequency)
}
