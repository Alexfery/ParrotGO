// Package resources checks and assigns the hardware resources the components
// of a project use: GPIOs, which components claim directly; LEDC timers and
// channels and SPI hosts, which Parrot allocates; I2C controllers, which
// Parrot only counts; and I2C addresses, which each device takes on its bus.
//
// Allocation is recomputed from parrot.json on every change instead of being
// stored: components are processed in manifest order, so the same manifest
// always gives the same result and appending a component never moves the
// resources of earlier ones. Generated code hardcodes the LEDC timers and
// channels and the SPI hosts, so a future `parrot remove` must regenerate the
// components whose allocation shifts.
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

	// SPIHost is set by an SPI master bus, which takes one of the target's
	// SPI hosts. Its devices do not set it: they share the bus's host and
	// GPIOs, and each one only claims the GPIO of its CS line.
	SPIHost bool

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

	// Kind names the component's type in conflict messages, e.g. "SPI bus"
	// gives `GPIO23 is already used by SPI bus "main_bus"`. When empty, the
	// component is called a "component".
	Kind string

	Needs
}

// owner is how conflict messages name the component of c.
func (c Claim) owner() string {
	kind := c.Kind
	if kind == "" {
		kind = "component"
	}
	return fmt.Sprintf("%s %q", kind, c.Component)
}

// Allocation is the result of Allocate.
type Allocation struct {
	LEDC map[string]LEDCAssignment // by component name

	// I2CControllers is how many HP I2C controllers the buses use. Which one
	// each bus gets is left to ESP-IDF when the bus is created (i2c_port -1):
	// the controllers are interchangeable, as their signals go through the
	// GPIO matrix, so only their number matters.
	I2CControllers int

	// SPIHosts are the SPI hosts given to the buses, by component name.
	// Unlike an I2C controller, the host must be named when the bus is
	// created (spi_bus_initialize has no "any free host"), so Parrot assigns
	// it: each bus gets the target's next unused host, in the target's order.
	SPIHosts map[string]targets.SPIHost
}

// ClaimError is the error Allocate returns for the first claim it cannot
// satisfy. Its message is the cause's; Component says which claim failed, so
// that callers such as `parrot inspect` can report it on that component.
type ClaimError struct {
	Component string
	Err       error
}

func (e *ClaimError) Error() string { return e.Err.Error() }
func (e *ClaimError) Unwrap() error { return e.Err }

// Allocate checks that no GPIO, I2C address or driven device is claimed twice
// and that the target has enough I2C controllers, and assigns LEDC timers and
// channels, processing claims in order. Its error is a *ClaimError.
func Allocate(target targets.Target, claims []Claim) (Allocation, error) {
	a := allocator{
		target:        target,
		gpioOwners:    make(map[int]string),
		addressOwners: make(map[I2CAddress]string),
		driverOwners:  make(map[string]string),
		ledc:          ledcAllocator{target: target},
		alloc: Allocation{
			LEDC:     make(map[string]LEDCAssignment),
			SPIHosts: make(map[string]targets.SPIHost),
		},
	}
	for _, c := range claims {
		if err := a.take(c); err != nil {
			return Allocation{}, &ClaimError{Component: c.Component, Err: err}
		}
	}
	return a.alloc, nil
}

// allocator is the state of one Allocate call. The owners are named as
// conflict messages show them (see Claim.owner).
type allocator struct {
	target        targets.Target
	gpioOwners    map[int]string
	addressOwners map[I2CAddress]string
	driverOwners  map[string]string // by device
	ledc          ledcAllocator
	spiHosts      int // SPI hosts in use: the first ones of target.SPI.Hosts
	alloc         Allocation
}

// take records the resources of c, or reports the first one it cannot have.
func (a *allocator) take(c Claim) error {
	for _, pin := range c.GPIOs {
		if owner, used := a.gpioOwners[pin]; used {
			return fmt.Errorf("GPIO%d is already used by %s", pin, owner)
		}
		a.gpioOwners[pin] = c.owner()
	}
	if c.LEDC != nil {
		assignment, err := a.ledc.assign(*c.LEDC)
		if err != nil {
			return err
		}
		a.alloc.LEDC[c.Component] = assignment
	}
	if c.I2CController {
		if a.alloc.I2CControllers >= a.target.I2C.HPControllers {
			return fmt.Errorf("no I2C master controllers available on %s", a.target.DisplayName)
		}
		a.alloc.I2CControllers++
	}
	if c.SPIHost {
		if a.spiHosts >= len(a.target.SPI.Hosts) {
			return fmt.Errorf("no SPI hosts available on %s", a.target.DisplayName)
		}
		a.alloc.SPIHosts[c.Component] = a.target.SPI.Hosts[a.spiHosts]
		a.spiHosts++
	}
	if addr := c.I2CAddress; addr != nil {
		if owner, used := a.addressOwners[*addr]; used {
			return fmt.Errorf("I2C address 0x%02X is already used on bus %q by %s", addr.Address, addr.Bus, owner)
		}
		a.addressOwners[*addr] = c.owner()
	}
	if d := c.Drives; d != "" {
		if owner, used := a.driverOwners[d]; used {
			return fmt.Errorf("component %q is already driven by %s", d, owner)
		}
		a.driverOwners[d] = c.owner()
	}
	return nil
}
