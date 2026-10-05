package adc

import (
	"fmt"

	"parrot/internal/components"
	"parrot/internal/targets"
)

// Validate checks that pin on target can be read as an analog input: on top of
// the common component checks, the GPIO must be wired to an ADC channel.
// It returns the ADC unit and channel of the pin.
func Validate(target targets.Target, pin int) (targets.ADCInfo, error) {
	p, err := components.GPIO(target, pin)
	if err != nil {
		return targets.ADCInfo{}, err
	}
	if p.ADC == nil {
		return targets.ADCInfo{}, fmt.Errorf("GPIO%d does not support ADC on %s", pin, target.DisplayName)
	}
	return *p.ADC, nil
}
