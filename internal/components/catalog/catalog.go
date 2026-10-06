// Package catalog maps the component types stored in parrot.json to their
// packages, so generic code can ask any declared component which hardware
// resources it uses. Adding a component type means adding a case to needs.
package catalog

import (
	"fmt"

	"parrot/internal/components/adc"
	"parrot/internal/components/button"
	"parrot/internal/components/i2c"
	"parrot/internal/components/i2cdevice"
	"parrot/internal/components/led"
	"parrot/internal/components/pwm"
	"parrot/internal/components/sensor/bme280"
	"parrot/internal/components/spi"
	"parrot/internal/components/spidevice"
	"parrot/internal/project"
	"parrot/internal/resources"
	"parrot/internal/targets"
)

// Claims returns the resources used by every component of cfg on target, in
// manifest order, ready for resources.Allocate.
func Claims(cfg project.Config, target targets.Target) ([]resources.Claim, error) {
	claims := make([]resources.Claim, 0, len(cfg.Components))
	for _, c := range cfg.Components {
		claim, err := Claim(c, target)
		if err != nil {
			return nil, err
		}
		claims = append(claims, claim)
	}
	return claims, nil
}

// Claim returns the resources component c uses on target. It fails if c has
// an unknown type or an invalid config.
func Claim(c project.ComponentConfig, target targets.Target) (resources.Claim, error) {
	n, err := needs(c, target)
	if err != nil {
		return resources.Claim{}, err
	}
	return resources.Claim{Component: c.Name, Kind: conflictKinds[c.Type], Needs: n}, nil
}

// conflictKinds name the types whose GPIOs are easy to mistake for one
// another in a conflict: a CS line next to the lines of its bus. Their
// conflicts say e.g. `GPIO23 is already used by SPI bus "main_bus"`; the other
// types are called "component".
var conflictKinds = map[string]string{
	spi.Type:       "SPI bus",
	spidevice.Type: "SPI device",
}

func needs(c project.ComponentConfig, target targets.Target) (resources.Needs, error) {
	switch c.Type {
	case led.Type:
		return decodeNeeds[led.Config](c, target)
	case button.Type:
		return decodeNeeds[button.Config](c, target)
	case adc.Type:
		return decodeNeeds[adc.Config](c, target)
	case pwm.Type:
		return decodeNeeds[pwm.Config](c, target)
	case i2c.Type:
		return decodeNeeds[i2c.Config](c, target)
	case i2cdevice.Type:
		return decodeNeeds[i2cdevice.Config](c, target)
	case bme280.Type:
		return decodeNeeds[bme280.Config](c, target)
	case spi.Type:
		return decodeNeeds[spi.Config](c, target)
	case spidevice.Type:
		return decodeNeeds[spidevice.Config](c, target)
	}
	return resources.Needs{}, fmt.Errorf("component %q in %s has unknown type %q", c.Name, project.ConfigFile, c.Type)
}

// config is implemented by the Config type of every component package.
type config interface {
	Needs(targets.Target) (resources.Needs, error)
}

func decodeNeeds[T config](c project.ComponentConfig, target targets.Target) (resources.Needs, error) {
	var cfg T
	if err := c.Decode(&cfg); err != nil {
		return resources.Needs{}, err
	}
	return cfg.Needs(target)
}
