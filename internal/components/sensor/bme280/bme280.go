// Package bme280 is the driver component of the Bosch BME280 temperature,
// pressure and humidity sensor, over I2C.
//
// The driver is built on a generic I2C device of the project (see
// internal/components/i2cdevice), named in its config: the device owns the
// address and the SCL frequency, and its bus owns SDA, SCL and the controller.
// The driver records none of them. Its generated code reads and writes the
// sensor's registers through the device's functions, and knows nothing of the
// GPIOs or of how the bus was created.
package bme280

import (
	"parrot/internal/components"
	"parrot/internal/components/i2cdevice"
	"parrot/internal/project"
	"parrot/internal/resources"
	"parrot/internal/targets"
)

// Type is the component type in parrot.json. The CLI spells it
// `parrot add sensor bme280`; the "sensor-" prefix keeps the sensor types
// apart from the transports they are built on.
const Type = "sensor-bme280"

// Sensor is a BME280 on an I2C device of the project.
type Sensor struct {
	Name string // canonical component name, also the C symbol prefix
	Config
}

// Config is the BME280 entry in parrot.json: only the device it is built on.
// Address, frequency, bus and GPIOs are recorded by the device and its bus.
type Config struct {
	Device string `json:"device"` // name of an i2c-device component
}

// Needs reports the resources a BME280 uses: the device it drives. It takes
// no GPIO, controller or address; the device and its bus hold those.
func (c Config) Needs(targets.Target) (resources.Needs, error) {
	return resources.Needs{Drives: c.Device}, nil
}

// New builds a sensor from user-supplied values, normalizing its name and the
// name of its device the same way, so that --device accepts what
// `parrot add i2c-device` did.
func New(name, device string) (Sensor, error) {
	normalized, err := components.NormalizeName(name)
	if err != nil {
		return Sensor{}, err
	}
	deviceName, err := components.NormalizeName(device)
	if err != nil {
		return Sensor{}, err
	}
	return Sensor{Name: normalized, Config: Config{Device: deviceName}}, nil
}

// ResolveDevice returns the I2C device s is built on, as declared in cfg. It
// fails if there is no such component, if it is not an I2C device, or if its
// address is not one a BME280 can have.
func (s Sensor) ResolveDevice(cfg project.Config) (i2cdevice.Device, error) {
	d, err := i2cdevice.Lookup(cfg, s.Device)
	if err != nil {
		return d, err
	}
	return d, checkAddress(d)
}
