// Package targets is the single source of truth for the SoCs Parrot supports
// and the hardware they provide. It describes the chip itself (the ESP-IDF
// target), not a development board: a GPIO listed here may not be routed to a
// header on a given board.
//
// This package only describes hardware. Deciding whether a pin fits a
// component (LED, button, ...) belongs to internal/components.
package targets

import (
	"fmt"
	"slices"
)

// ADCInfo is the ADC input wired to a GPIO.
type ADCInfo struct {
	Unit    int // 1 for ADC1, 2 for ADC2
	Channel int // channel number within the unit
}

// Pin describes the capabilities of one GPIO of a SoC.
type Pin struct {
	Number int

	Input  bool
	Output bool
	Pull   bool     // has internal pull-up and pull-down resistors
	ADC    *ADCInfo // nil if the GPIO has no ADC channel usable from ESP-IDF

	// Reserved marks pins wired to the SoC's SPI flash interface. They exist,
	// but using them for anything else usually breaks the chip.
	Reserved bool
}

// LEDC describes the LED PWM controller of a SoC. LEDC outputs are routed
// through the GPIO matrix, so any output-capable GPIO can carry them.
type LEDC struct {
	Timers        int  // timers per speed mode (SOC_LEDC_TIMER_NUM)
	Channels      int  // channels per speed mode (SOC_LEDC_CHANNEL_NUM)
	HighSpeedMode bool // has a high-speed mode besides the low-speed one (SOC_LEDC_SUPPORT_HS_MODE)
	MaxResolution int  // maximum duty resolution in bits (SOC_LEDC_TIMER_BIT_WIDTH)

	// ClockSource is the ledc_clk_cfg_t of the 80 MHz clock Parrot clocks the
	// timers from, and ClockHz its frequency. On ESP32-C3/S3/C6 all low-speed
	// timers must share one clock source, so Parrot always uses the same one.
	ClockSource string
	ClockHz     int
}

// TimerFits reports whether a timer clocked from ClockSource can run at
// frequency Hz with a duty resolution of resolution bits. It mirrors the
// ESP-IDF driver (ledc.c): divisor = (clock << 8) / (frequency << resolution),
// rounded, must be in [256, 0x3FFFF] (10 integer and 8 fractional bits).
func (l LEDC) TimerFits(frequency, resolution int) bool {
	if frequency <= 0 || resolution < 1 || resolution > l.MaxResolution {
		return false
	}
	ticks := uint64(frequency) << resolution
	divisor := (uint64(l.ClockHz)<<8 + ticks/2) / ticks
	return divisor >= 1<<8 && divisor <= 0x3FFFF
}

// I2C describes the I2C controllers of a SoC that Parrot can use as masters.
type I2C struct {
	// HPControllers is the number of high-power I2C controllers
	// (SOC_HP_I2C_NUM). Their SDA and SCL signals go through the GPIO matrix,
	// so any GPIO that is both input and output can carry them. Low-power I2C
	// controllers (SOC_LP_I2C_NUM), which only reach LP GPIOs, are not
	// described yet: Parrot does not use them.
	HPControllers int
}

// Target describes a supported SoC.
type Target struct {
	ID          string // Parrot ID, used in the CLI and in parrot.json
	DisplayName string // human-readable name
	IDFTarget   string // name expected by ESP-IDF (idf.py set-target)

	LEDC LEDC
	I2C  I2C

	pins map[int]Pin
}

// Pin returns the GPIO with the given number, if the SoC has it.
func (t Target) Pin(number int) (Pin, bool) {
	p, ok := t.pins[number]
	return p.clone(), ok
}

// GPIO is like Pin but returns a user-facing error for a missing GPIO.
func (t Target) GPIO(number int) (Pin, error) {
	p, ok := t.Pin(number)
	if !ok {
		return Pin{}, fmt.Errorf("GPIO%d is not available on %s", number, t.DisplayName)
	}
	return p, nil
}

// Pins returns every GPIO of the SoC, ordered by number.
func (t Target) Pins() []Pin {
	pins := make([]Pin, 0, len(t.pins))
	for _, p := range t.pins {
		pins = append(pins, p.clone())
	}
	slices.SortFunc(pins, func(a, b Pin) int { return a.Number - b.Number })
	return pins
}

// clone copies p so callers cannot modify the registry through p.ADC.
func (p Pin) clone() Pin {
	if p.ADC != nil {
		adc := *p.ADC
		p.ADC = &adc
	}
	return p
}

// Capability presets shared by the target definitions.
var (
	ioPin     = Pin{Input: true, Output: true, Pull: true}
	inputOnly = Pin{Input: true} // no output, no pull resistors
	flashPin  = Pin{Input: true, Output: true, Pull: true, Reserved: true}
)

// withPins returns one copy of preset per GPIO number.
func withPins(preset Pin, numbers ...int) []Pin {
	pins := make([]Pin, len(numbers))
	for i, n := range numbers {
		pins[i] = preset
		pins[i].Number = n
	}
	return pins
}

// adcPins is like withPins, but also wires the GPIOs to consecutive channels
// of an ADC unit: numbers[0] gets firstChannel, numbers[1] the next, etc.
func adcPins(preset Pin, unit, firstChannel int, numbers ...int) []Pin {
	pins := withPins(preset, numbers...)
	for i := range pins {
		pins[i].ADC = &ADCInfo{Unit: unit, Channel: firstChannel + i}
	}
	return pins
}

// span returns the GPIO numbers from first to last, inclusive.
func span(first, last int) []int {
	numbers := make([]int, 0, last-first+1)
	for n := first; n <= last; n++ {
		numbers = append(numbers, n)
	}
	return numbers
}

// pinMap indexes groups of pins by GPIO number.
func pinMap(groups ...[]Pin) map[int]Pin {
	pins := make(map[int]Pin)
	for _, group := range groups {
		for _, p := range group {
			pins[p.Number] = p
		}
	}
	return pins
}
