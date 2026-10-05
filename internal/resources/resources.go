// Package resources checks and assigns the hardware resources the components
// of a project use: GPIOs, which components claim directly; LEDC timers and
// channels, which Parrot allocates; I2C controllers, which Parrot only counts;
// and I2C addresses, which each device takes on its bus.
//
// Allocation is recomputed from parrot.json on every change instead of being
// stored: components are processed in manifest order, so the same manifest
// always gives the same result and appending a component never moves the
// resources of earlier ones. Generated code hardcodes the LEDC timers and
// channels, so a future `parrot remove` must regenerate the components whose
// allocation shifts.
package resources

import (
	"fmt"

	"parrot/internal/targets"
)

// Needs lists the hardware resources one component uses.
type Needs struct {
	GPIOs []int
	LEDC  *LEDCTimer // nil if the component drives no LEDC channel

	// I2CController is set by an I2C master bus, which takes one of the
	// target's HP I2C controllers. Its devices do not set it: they share the
	// bus's controller and GPIOs.
	I2CController bool

	// I2CAddress is set by an I2C device: the address it takes on its bus.
	I2CAddress *I2CAddress

	// Drives is set by a device driver, e.g. a BME280 sensor: the name of the
	// device component it is built on. It takes neither the device's address
	// nor its bus's GPIOs, which stay with them; it takes the device itself,
	// as the driver owns the state of the chip (configuration, calibration),
	// so a device has at most one driver.
	Drives string
}

// I2CAddress is an address on one of the project's I2C buses. Two devices
// conflict only when both the bus and the address match: the same address on
// two buses is two different devices.
type I2CAddress struct {
	Bus     string // name of the i2c-bus component
	Address uint16 // 7-bit, without the R/W bit
}

// Claim is the resource usage of one component.
type Claim struct {
	Component string
	Needs
}

// Allocation is the result of Allocate.
type Allocation struct {
	LEDC map[string]LEDCAssignment // by component name

	// I2CControllers is how many HP I2C controllers the buses use. Which one
	// each bus gets is left to ESP-IDF when the bus is created (i2c_port -1):
	// the controllers are interchangeable, as their signals go through the
	// GPIO matrix, so only their number matters.
	I2CControllers int
}

// Allocate checks that no GPIO, I2C address or driven device is claimed twice
// and that the target has enough I2C controllers, and assigns LEDC timers and
// channels, processing claims in order.
func Allocate(target targets.Target, claims []Claim) (Allocation, error) {
	gpioOwners := make(map[int]string)
	addressOwners := make(map[I2CAddress]string)
	driverOwners := make(map[string]string) // by device
	ledc := ledcAllocator{target: target}
	alloc := Allocation{LEDC: make(map[string]LEDCAssignment)}

	for _, c := range claims {
		for _, pin := range c.GPIOs {
			if owner, used := gpioOwners[pin]; used {
				return Allocation{}, fmt.Errorf("GPIO%d is already used by component %q", pin, owner)
			}
			gpioOwners[pin] = c.Component
		}
		if c.LEDC != nil {
			a, err := ledc.assign(*c.LEDC)
			if err != nil {
				return Allocation{}, err
			}
			alloc.LEDC[c.Component] = a
		}
		if c.I2CController {
			if alloc.I2CControllers >= target.I2C.HPControllers {
				return Allocation{}, fmt.Errorf("no I2C master controllers available on %s", target.DisplayName)
			}
			alloc.I2CControllers++
		}
		if a := c.I2CAddress; a != nil {
			if owner, used := addressOwners[*a]; used {
				return Allocation{}, fmt.Errorf("I2C address 0x%02X is already used on bus %q by component %q", a.Address, a.Bus, owner)
			}
			addressOwners[*a] = c.Component
		}
		if d := c.Drives; d != "" {
			if owner, used := driverOwners[d]; used {
				return Allocation{}, fmt.Errorf("component %q is already driven by component %q", d, owner)
			}
			driverOwners[d] = c.Component
		}
	}
	return alloc, nil
}
