// Package i2cdevice is the generic I2C device component: a device at a 7-bit
// address on one of the project's I2C buses, with its own SCL frequency. It
// moves bytes (transmit, receive, or both with a repeated START); what they
// mean is left to the driver of the actual device, built on top of it.
//
// A device owns no GPIO and no controller: it references its bus, which owns
// them, and takes its address on that bus.
package i2cdevice

import (
	"fmt"

	"parrot/internal/components"
	"parrot/internal/components/i2c"
	"parrot/internal/project"
	"parrot/internal/resources"
	"parrot/internal/targets"
)

// Type is the component type used in the CLI and in parrot.json.
const Type = "i2c-device"

// Device is an I2C device on a bus of the project.
type Device struct {
	Name string // canonical component name, also the C symbol prefix
	Config
}

// Config is the I2C device entry in parrot.json. The bus is referenced by
// name: its GPIOs are recorded in the bus entry only.
type Config struct {
	Bus       string `json:"bus"`       // name of an i2c-bus component
	Address   uint16 `json:"address"`   // 7-bit address, without the R/W bit
	Frequency int    `json:"frequency"` // SCL frequency in Hz
}

// Needs reports the resources a device uses: its address on its bus, and no
// GPIO or controller.
func (c Config) Needs(targets.Target) (resources.Needs, error) {
	if err := c.check(); err != nil {
		return resources.Needs{}, err
	}
	return resources.Needs{I2CAddress: &resources.I2CAddress{Bus: c.Bus, Address: c.Address}}, nil
}

// New builds a device from user-supplied values, normalizing its name and the
// name of its bus the same way, so that --bus accepts what `parrot add i2c` did.
func New(name, bus string, address uint16, frequency int) (Device, error) {
	normalized, err := components.NormalizeName(name)
	if err != nil {
		return Device{}, err
	}
	busName, err := components.NormalizeName(bus)
	if err != nil {
		return Device{}, err
	}
	c := Config{Bus: busName, Address: address, Frequency: frequency}
	if err := c.check(); err != nil {
		return Device{}, err
	}
	return Device{Name: normalized, Config: c}, nil
}

// ResolveBus returns the bus d is on, as declared in cfg. It fails if there
// is no such component, or if it is not an I2C bus.
func (d Device) ResolveBus(cfg project.Config) (i2c.Bus, error) {
	return i2c.Lookup(cfg, d.Bus)
}

// Lookup returns the I2C device called name in cfg, for a driver built on it.
// It fails if cfg has no component of that name, or if that component is not
// an I2C device.
func Lookup(cfg project.Config, name string) (Device, error) {
	c, found := cfg.Component(name)
	if !found {
		return Device{}, fmt.Errorf("I2C device %q does not exist", name)
	}
	if c.Type != Type {
		return Device{}, fmt.Errorf("component %q is not an I2C device", name)
	}
	var config Config
	if err := c.Decode(&config); err != nil {
		return Device{}, err
	}
	return Device{Name: c.Name, Config: config}, nil
}

// templateData is what the device templates see.
type templateData struct {
	Name      string // C symbol prefix, e.g. "display"
	Bus       string // the bus component: its header, C symbol prefix and CMake name
	Address   string // e.g. "0x3C"
	Frequency int    // Hz
}

// Create generates the component under projectDir/components/<name> and
// returns the paths it created. The bus must already exist (see ResolveBus):
// the generated code uses its handle and does not create it.
func Create(projectDir string, d Device) ([]string, error) {
	data := templateData{
		Name:      d.Name,
		Bus:       d.Bus,
		Address:   FormatAddress(d.Address),
		Frequency: d.Frequency,
	}
	return components.Generate(projectDir, Type, d.Name, data)
}
