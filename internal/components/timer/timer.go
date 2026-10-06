// Package timer is the periodic hardware timer component, built on ESP-IDF's
// GPTimer driver. The user gives a period; the generated timer counts up at
// 1 MHz, raises an alarm at the end of every period and reloads its count in
// hardware, and the alarm runs a callback the user fills in.
//
// A timer owns no GPIO. It takes one of the target's general purpose timers,
// which ESP-IDF picks when the timer is created, so Parrot only counts them
// (see internal/resources).
package timer

import (
	"parrot/internal/components"
	"parrot/internal/resources"
	"parrot/internal/targets"
)

// Type is the component type used in the CLI and in parrot.json.
const Type = "timer"

// ModePeriodic is the only mode Parrot generates: an alarm at the end of
// every period, after which the hardware reloads the count to 0.
const ModePeriodic = "periodic"

// ResolutionHz is the frequency every timer counts at: 1 MHz, so one tick is
// one microsecond and the alarm count is the period in microseconds.
const ResolutionHz = 1_000_000

// Timer is a periodic hardware timer.
type Timer struct {
	Name string // canonical component name, also the C symbol prefix
	Config
}

// Config is the timer entry in parrot.json. The period is stored in
// microseconds, the timer's tick, so it is exactly the alarm count of the
// generated code: no unit or rounding to interpret.
type Config struct {
	Mode     string `json:"mode"`      // ModePeriodic
	PeriodUS uint64 `json:"period_us"` // period in microseconds
}

// AlarmCount is the count the alarm fires at: the period in ticks of
// ResolutionHz, which is the period in microseconds.
func (c Config) AlarmCount() uint64 {
	return c.PeriodUS
}

// Needs reports the resources a timer uses: one general purpose timer, and
// no GPIO.
func (c Config) Needs(targets.Target) (resources.Needs, error) {
	if err := c.check(); err != nil {
		return resources.Needs{}, err
	}
	return resources.Needs{GPTimer: true}, nil
}

// New builds a periodic timer from a user-supplied name, normalizing it, and
// a period in microseconds, as ParsePeriod returns it.
func New(name string, periodUS uint64) (Timer, error) {
	normalized, err := components.NormalizeName(name)
	if err != nil {
		return Timer{}, err
	}
	c := Config{Mode: ModePeriodic, PeriodUS: periodUS}
	if err := c.check(); err != nil {
		return Timer{}, err
	}
	return Timer{Name: normalized, Config: c}, nil
}
