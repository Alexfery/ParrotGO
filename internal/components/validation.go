package components

import (
	"fmt"

	"parrot/internal/targets"
)

// GPIO returns the pin a component wants to use, rejecting GPIOs that do not
// exist on the target or are wired to its SPI flash. Each component then
// checks the capabilities it needs on the returned pin.
func GPIO(target targets.Target, number int) (targets.Pin, error) {
	p, err := target.GPIO(number)
	if err != nil {
		return p, err
	}
	if p.Reserved {
		return p, fmt.Errorf("GPIO%d is reserved for the SPI flash on %s", number, target.DisplayName)
	}
	return p, nil
}
