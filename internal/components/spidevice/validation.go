package spidevice

import (
	"errors"
	"fmt"
	"math"

	"parrot/internal/components"
	"parrot/internal/targets"
)

// errMode is the error of a mode outside 0-3, the four (CPOL, CPHA) pairs.
var errMode = errors.New("SPI mode must be between 0 and 3")

// check validates what a device entry can get wrong on its own, before its
// bus is looked up. The frequency has no upper limit of Parrot's: ESP-IDF
// rejects one above the SPI clock source when the device is added
// (spi_bus_add_device), and the device's driver knows what the chip accepts.
func (c Config) check() error {
	if c.Frequency <= 0 {
		return fmt.Errorf("frequency must be positive, got %d Hz", c.Frequency)
	}
	// ESP-IDF stores it in an int (spi_device_interface_config_t.clock_speed_hz).
	if c.Frequency > math.MaxInt32 {
		return fmt.Errorf("frequency %d Hz is too high", c.Frequency)
	}
	if c.Mode < 0 || c.Mode > 3 {
		return errMode
	}
	return nil
}

// Validate checks that the CS GPIO can carry CS on target: on top of the
// common component checks (the GPIO exists and is not a flash pin), it must
// be an output, as the master drives CS. This is the SPI Master driver's own
// check (spi_bus_add_device: GPIO_IS_VALID_OUTPUT_GPIO). Whether another
// component, the bus included, already uses the GPIO is the allocator's
// decision (internal/resources).
func Validate(target targets.Target, c Config) error {
	if err := c.check(); err != nil {
		return err
	}
	p, err := components.GPIO(target, c.CS)
	if err != nil {
		return err
	}
	if !p.Output {
		return fmt.Errorf("GPIO%d cannot be used for SPI CS on %s: the master drives CS, so it needs an output GPIO",
			p.Number, target.DisplayName)
	}
	return nil
}
