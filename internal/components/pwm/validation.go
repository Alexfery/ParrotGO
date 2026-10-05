package pwm

import (
	"fmt"

	"parrot/internal/components"
	"parrot/internal/targets"
)

// Validate checks that c can be generated on target: on top of the common
// component checks, the target needs a LEDC controller, the GPIO must support
// output (LEDC reaches any output GPIO through the GPIO matrix) and a LEDC
// timer must be able to run at the frequency.
func Validate(target targets.Target, c Config) error {
	if target.LEDC.Channels == 0 {
		return fmt.Errorf("%s has no LEDC controller for PWM", target.DisplayName)
	}
	p, err := components.GPIO(target, c.Pin)
	if err != nil {
		return err
	}
	if !p.Output {
		return fmt.Errorf("GPIO%d cannot be used as an output on %s", c.Pin, target.DisplayName)
	}
	_, err = Resolution(target, c.Frequency)
	return err
}
