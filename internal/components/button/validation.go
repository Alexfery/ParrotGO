package button

import (
	"fmt"

	"parrot/internal/components"
	"parrot/internal/targets"
)

// Validate checks that pin on target can read a button: on top of the common
// component checks, the GPIO must support input and have an internal pull-up,
// because the generated code relies on it instead of an external resistor.
func Validate(target targets.Target, pin int) error {
	p, err := components.GPIO(target, pin)
	if err != nil {
		return err
	}
	return checkPin(p, target.DisplayName)
}

func checkPin(p targets.Pin, targetName string) error {
	if !p.Input {
		return fmt.Errorf("GPIO%d cannot be used as an input on %s", p.Number, targetName)
	}
	if !p.Pull {
		return fmt.Errorf("GPIO%d has no internal pull-up on %s, which the button needs", p.Number, targetName)
	}
	return nil
}
