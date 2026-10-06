// Package spidevice is the generic SPI device component: a device on one of
// the project's SPI buses, with its own CS line, clock frequency and SPI
// mode. It moves bytes in full duplex; what they mean (commands, registers,
// dummy bytes) is left to the driver of the actual device, built on top of it.
//
// A device owns the GPIO of its CS line only. MOSI, MISO, SCLK and the SPI
// host belong to its bus, which every device on it shares.
package spidevice

import (
	"parrot/internal/components"
	"parrot/internal/components/spi"
	"parrot/internal/project"
	"parrot/internal/resources"
	"parrot/internal/targets"
)

// Type is the component type used in the CLI and in parrot.json.
const Type = "spi-device"

// Device is an SPI device on a bus of the project.
type Device struct {
	Name string // canonical component name, also the C symbol prefix
	Config
}

// Config is the SPI device entry in parrot.json. The bus is referenced by
// name: its GPIOs and its host are recorded, or derived, from the bus entry
// only.
type Config struct {
	Bus       string `json:"bus"`       // name of an spi-bus component
	CS        int    `json:"cs"`        // GPIO of the CS line
	Frequency int    `json:"frequency"` // SCLK frequency for this device, in Hz
	Mode      int    `json:"mode"`      // SPI mode, 0-3 (see CPOL and CPHA)
}

// CPOL returns the clock polarity of the mode: the level of SCLK when idle.
func (c Config) CPOL() int { return c.Mode >> 1 }

// CPHA returns the clock phase of the mode: 0 samples data on the first SCLK
// edge, 1 on the second.
func (c Config) CPHA() int { return c.Mode & 1 }

// Needs reports the resources a device uses: the GPIO of its CS line. It
// takes no host and none of the bus's GPIOs, which its bus has claimed.
func (c Config) Needs(targets.Target) (resources.Needs, error) {
	if err := c.check(); err != nil {
		return resources.Needs{}, err
	}
	return resources.Needs{GPIOs: []int{c.CS}}, nil
}

// New builds a device from user-supplied values, normalizing its name and the
// name of its bus the same way, so that --bus accepts what `parrot add spi`
// did.
func New(name, bus string, cs, frequency, mode int) (Device, error) {
	normalized, err := components.NormalizeName(name)
	if err != nil {
		return Device{}, err
	}
	busName, err := components.NormalizeName(bus)
	if err != nil {
		return Device{}, err
	}
	c := Config{Bus: busName, CS: cs, Frequency: frequency, Mode: mode}
	if err := c.check(); err != nil {
		return Device{}, err
	}
	return Device{Name: normalized, Config: c}, nil
}

// ResolveBus returns the bus d is on, as declared in cfg. It fails if there
// is no such component, or if it is not an SPI bus.
func (d Device) ResolveBus(cfg project.Config) (spi.Bus, error) {
	return spi.Lookup(cfg, d.Bus)
}
