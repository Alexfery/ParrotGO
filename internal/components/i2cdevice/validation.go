package i2cdevice

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// ParseAddress parses an I2C address as the user writes it: hexadecimal with
// a 0x prefix, as datasheets give it ("0x3C"), or decimal ("60"). A leading
// zero does not mean octal: "060" is 60.
func ParseAddress(s string) (uint16, error) {
	digits, base := s, 10
	if strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X") {
		digits, base = s[2:], 16
	}
	value, err := strconv.ParseUint(digits, base, 16)
	if err != nil {
		return 0, fmt.Errorf("invalid I2C address %q: write it in hexadecimal (0x3C) or decimal (60)", s)
	}
	return uint16(value), nil
}

// FormatAddress writes an address the way datasheets do, e.g. "0x3C".
func FormatAddress(address uint16) string {
	return fmt.Sprintf("0x%02X", address)
}

// Address limits of 7-bit addressing. The I2C specification (NXP UM10204,
// "Reserved addresses") keeps 0x00-0x07 and 0x78-0x7F for special purposes
// (general call, START byte, Hs-mode, 10-bit addressing, device ID), so no
// device uses them.
const (
	maxAddress      = 0x7F
	firstUsable     = 0x08
	lastUsable      = 0x77
	maxEightBitForm = 0xFF
)

// checkAddress accepts the 7-bit addresses a device can have. Datasheets often
// give the 8-bit form instead, which includes the R/W bit (e.g. 0x78 for a
// device at 0x3C); the error then suggests the 7-bit address.
func checkAddress(address uint16) error {
	var problem string
	switch {
	case address > maxAddress:
		problem = fmt.Sprintf("I2C address %s does not fit in 7 bits (10-bit addresses are not supported)", FormatAddress(address))
	case address < firstUsable || address > lastUsable:
		problem = fmt.Sprintf("I2C address %s is reserved by the I2C specification (0x00-0x07 and 0x78-0x7F)", FormatAddress(address))
	default:
		return nil
	}
	if address > lastUsable && address <= maxEightBitForm {
		problem += fmt.Sprintf("; if %s is an 8-bit address that includes the R/W bit, use %s",
			FormatAddress(address), FormatAddress(address>>1))
	}
	return errors.New(problem)
}

// check validates what a device entry can get wrong on its own, before its
// bus is looked up.
func (c Config) check() error {
	if err := checkAddress(c.Address); err != nil {
		return err
	}
	if c.Frequency <= 0 {
		return fmt.Errorf("frequency must be positive, got %d Hz", c.Frequency)
	}
	// ESP-IDF stores it in a uint32_t (i2c_device_config_t.scl_speed_hz).
	if uint64(c.Frequency) > math.MaxUint32 {
		return fmt.Errorf("frequency %d Hz is too high", c.Frequency)
	}
	return nil
}
