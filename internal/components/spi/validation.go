package spi

import (
	"fmt"

	"parrot/internal/components"
	"parrot/internal/targets"
)

// Validate checks that the bus's GPIOs can carry its signals on target: on
// top of the common component checks, MOSI and SCLK, which the master
// drives, need output GPIOs, and MISO, which it reads, an input GPIO.
//
// These are the SPI Master driver's own checks (spicommon_bus_initialize_io).
// Nothing else is required: the signals go through the GPIO matrix, which
// reaches every GPIO, and SPI lines are push-pull, so no pull resistor is
// needed. Whether the target has an SPI host left is the allocator's
// decision (internal/resources).
func Validate(target targets.Target, c Config) error {
	if err := c.checkDistinct(); err != nil {
		return err
	}
	for _, l := range c.lines() {
		p, err := components.GPIO(target, l.pin)
		if err != nil {
			return err
		}
		if l.output && !p.Output {
			return fmt.Errorf("GPIO%d cannot be used for SPI %s on %s: the master drives %s, so it needs an output GPIO",
				p.Number, l.name, target.DisplayName, l.name)
		}
		if !l.output && !p.Input {
			return fmt.Errorf("GPIO%d cannot be used for SPI %s on %s: it needs an input GPIO", p.Number, l.name, target.DisplayName)
		}
	}
	return nil
}
