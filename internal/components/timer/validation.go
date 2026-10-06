package timer

import (
	"errors"
	"fmt"
	"time"

	"parrot/internal/targets"
)

// ParsePeriod reads a period written as a Go duration with its unit, e.g.
// "500us", "50ms" or "1s" ("µs", "m" and "h" work too), and returns it in
// microseconds, the timer's tick. Nothing is rounded: a period that is not
// positive, or not a whole number of microseconds, is rejected.
//
// Any period it accepts fits in the timers' counters: the longest
// time.Duration, about 292 years, is below 2^54 microseconds.
func ParsePeriod(s string) (uint64, error) {
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, fmt.Errorf("invalid period %q: write a duration with its unit, such as 500us, 50ms or 1s", s)
	}
	if d <= 0 {
		return 0, fmt.Errorf("period must be positive, got %s", s)
	}
	if d%time.Microsecond != 0 {
		return 0, fmt.Errorf("period %s is not a whole number of microseconds: the timer counts in 1 us ticks", s)
	}
	return uint64(d / time.Microsecond), nil
}

// FormatPeriod writes a period in microseconds the way --period takes it,
// in the largest of s, ms and us that divides it exactly, e.g. "1s", "50ms"
// or "1500us".
func FormatPeriod(us uint64) string {
	switch {
	case us%1_000_000 == 0:
		return fmt.Sprintf("%ds", us/1_000_000)
	case us%1_000 == 0:
		return fmt.Sprintf("%dms", us/1_000)
	}
	return fmt.Sprintf("%dus", us)
}

// Validate checks that c can be generated on target: on top of the checks
// of the config itself, the target needs general purpose timers, and the
// alarm count must fit in their counter. Whether one is left is the
// allocator's decision (internal/resources).
//
// The counters are at least 54 bits wide, more than 570 years at 1 MHz, so
// only a hand-edited parrot.json can have a period that does not fit.
func Validate(target targets.Target, c Config) error {
	if err := c.check(); err != nil {
		return err
	}
	g := target.GPTimer
	if g.Timers == 0 {
		return fmt.Errorf("%s has no general purpose timer", target.DisplayName)
	}
	if g.CounterBits < 64 && c.AlarmCount() >= 1<<g.CounterBits {
		return fmt.Errorf("period of %d us does not fit in the %d-bit counter of the timers of %s",
			c.PeriodUS, g.CounterBits, target.DisplayName)
	}
	return nil
}

// check rejects a config Parrot cannot generate on any target. A zero
// period would also be invalid for ESP-IDF: with auto-reload, the alarm
// count must differ from the reload count, 0.
func (c Config) check() error {
	if c.Mode != ModePeriodic {
		return fmt.Errorf("unsupported timer mode %q: Parrot only generates %q timers", c.Mode, ModePeriodic)
	}
	if c.PeriodUS == 0 {
		return errors.New("period must be positive")
	}
	return nil
}
