package i2c

import (
	"fmt"

	"parrot/internal/components"
	"parrot/internal/targets"
)

// Validate checks that the bus can be created on target: on top of the common
// component checks, the target needs an HP I2C controller and both lines need
// GPIOs that can carry I2C.
//
// ESP-IDF's I2C master driver routes SDA and SCL through the GPIO matrix and
// sets each one up as an open-drain input/output, with the internal pull-up
// enabled by the generated code. Each GPIO must therefore be an input, an
// output and have pull resistors. Open drain needs nothing more: it is a mode
// of every GPIO's output driver. The driver itself only checks that the GPIO
// exists, so an input-only GPIO would be accepted there and silently never
// drive the line.
func Validate(target targets.Target, c Config) error {
	if target.I2C.HPControllers == 0 {
		return fmt.Errorf("%s has no I2C controller", target.DisplayName)
	}
	if c.SDA == c.SCL {
		return errSamePin
	}
	for _, line := range []struct {
		name string
		pin  int
	}{{"SDA", c.SDA}, {"SCL", c.SCL}} {
		p, err := components.GPIO(target, line.pin)
		if err != nil {
			return err
		}
		if !p.Input || !p.Output {
			return fmt.Errorf("GPIO%d cannot be used for I2C %s on %s: I2C lines need a GPIO that is both input and output",
				p.Number, line.name, target.DisplayName)
		}
		if !p.Pull {
			return fmt.Errorf("GPIO%d cannot be used for I2C %s on %s: it has no internal pull-up", p.Number, line.name, target.DisplayName)
		}
	}
	return nil
}
