// Package spi is the SPI master bus component: the MOSI, MISO and SCLK lines
// and one of the target's SPI hosts, which SPI devices will share.
//
// The bus owns its GPIOs and its host. The devices that will attach to it own
// neither: each one will have its own CS line, clock frequency and SPI mode,
// which ESP-IDF sets per device (spi_device_interface_config_t), so the bus
// has none of them.
package spi

import (
	"fmt"

	"parrot/internal/components"
	"parrot/internal/resources"
	"parrot/internal/targets"
)

// Type is the component type in parrot.json. The CLI calls it `spi`; the
// manifest says "spi-bus" so that the SPI devices can have a type of their own.
const Type = "spi-bus"

// Bus is an SPI master bus.
type Bus struct {
	Name string // canonical component name, also the C symbol prefix
	Config
}

// Config is the SPI bus entry in parrot.json. The host is not stored: Parrot
// derives it from the manifest order (see internal/resources).
type Config struct {
	MOSI int `json:"mosi"`

	// MISO is nil for a bus that only sends, e.g. to a display: the bus is
	// then created without a MISO line, and parrot.json has no "miso".
	MISO *int `json:"miso,omitempty"`

	SCLK int `json:"sclk"`
}

// line is one signal of the bus and its GPIO.
type line struct {
	name   string
	pin    int
	output bool // driven by the master; otherwise read by it
}

// lines returns the signals the bus uses, MISO only if it has one.
func (c Config) lines() []line {
	lines := []line{{"MOSI", c.MOSI, true}}
	if c.MISO != nil {
		lines = append(lines, line{"MISO", *c.MISO, false})
	}
	return append(lines, line{"SCLK", c.SCLK, true})
}

// checkDistinct rejects two signals on the same GPIO.
func (c Config) checkDistinct() error {
	lines := c.lines()
	for i, a := range lines {
		for _, b := range lines[i+1:] {
			if a.pin == b.pin {
				return fmt.Errorf("%s and %s cannot use the same GPIO", a.name, b.name)
			}
		}
	}
	return nil
}

// Needs reports the resources a bus uses: the GPIOs of its signals and an SPI host.
func (c Config) Needs(targets.Target) (resources.Needs, error) {
	if err := c.checkDistinct(); err != nil {
		return resources.Needs{}, err
	}
	lines := c.lines()
	gpios := make([]int, len(lines))
	for i, l := range lines {
		gpios[i] = l.pin
	}
	return resources.Needs{GPIOs: gpios, SPIHost: true}, nil
}

// New builds a bus from user-supplied values, normalizing the name; miso is
// nil for a bus without MISO. It rejects signals on the same GPIO before any
// project check, which would otherwise report the bus as conflicting with
// itself.
func New(name string, mosi int, miso *int, sclk int) (Bus, error) {
	normalized, err := components.NormalizeName(name)
	if err != nil {
		return Bus{}, err
	}
	c := Config{MOSI: mosi, MISO: miso, SCLK: sclk}
	if err := c.checkDistinct(); err != nil {
		return Bus{}, err
	}
	return Bus{Name: normalized, Config: c}, nil
}
