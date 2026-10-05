package led

import (
	"fmt"

	"parrot/internal/components"
	"parrot/internal/targets"
)

// Validate checks that pin on target can drive an LED: on top of the common
// component checks, the GPIO must support output.
func Validate(target targets.Target, pin int) error {
	p, err := components.GPIO(target, pin)
	if err != nil {
		return err
	}
	if !p.Output {
		return fmt.Errorf("GPIO%d cannot be used as an output on %s", pin, target.DisplayName)
	}
	return nil
}
