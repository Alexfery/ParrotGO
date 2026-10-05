package bme280

import (
	"fmt"
	"slices"

	"parrot/internal/components/i2cdevice"
)

// Addresses are the 7-bit I2C addresses of a BME280: 111011x, where x is the
// level of its SDO pin, 0x76 with SDO on GND and 0x77 with SDO on VDDIO (BME280
// data sheet, 6.2 "I²C Interface").
var Addresses = []uint16{0x76, 0x77}

// checkAddress accepts an I2C device whose address a BME280 can have. Any
// valid I2C address is fine for a generic device; this rule is the sensor's.
func checkAddress(d i2cdevice.Device) error {
	if slices.Contains(Addresses, d.Address) {
		return nil
	}
	return fmt.Errorf("I2C device %q uses address %s; BME280 expects %s or %s",
		d.Name, i2cdevice.FormatAddress(d.Address),
		i2cdevice.FormatAddress(Addresses[0]), i2cdevice.FormatAddress(Addresses[1]))
}
