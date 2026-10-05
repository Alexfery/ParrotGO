package resources

import (
	"fmt"
	"slices"

	"parrot/internal/targets"
)

// LEDCTimer is the timer configuration a LEDC channel needs. Channels with
// the same configuration can share a timer.
type LEDCTimer struct {
	Frequency  int // Hz
	Resolution int // duty resolution in bits
}

// LEDCAssignment is the LEDC timer and channel given to a component, in the
// low-speed mode.
//
// Parrot only allocates from the low-speed mode: it is the only mode on
// ESP32-C3/S3/C6 and exists on ESP32 too, so one code path fits every target.
// The ESP32's extra high-speed timers and channels are not used yet.
type LEDCAssignment struct {
	Timer   int
	Channel int
	LEDCTimer
}

// ledcAllocator gives each claim the next free channel, and a timer shared
// with earlier claims that have the same configuration, or else the next free
// one. A smarter strategy only needs to change this type.
type ledcAllocator struct {
	target   targets.Target
	timers   []LEDCTimer // configuration of each timer in use, by timer number
	channels int         // channels in use
}

func (a *ledcAllocator) assign(timer LEDCTimer) (LEDCAssignment, error) {
	if a.channels >= a.target.LEDC.Channels {
		return LEDCAssignment{}, fmt.Errorf("no LEDC channels available on %s", a.target.DisplayName)
	}
	timerNum := slices.Index(a.timers, timer)
	if timerNum < 0 {
		if len(a.timers) >= a.target.LEDC.Timers {
			return LEDCAssignment{}, fmt.Errorf(
				"no compatible LEDC timer available on %s: all %d timers run other frequencies",
				a.target.DisplayName, a.target.LEDC.Timers)
		}
		a.timers = append(a.timers, timer)
		timerNum = len(a.timers) - 1
	}
	channel := a.channels
	a.channels++
	return LEDCAssignment{Timer: timerNum, Channel: channel, LEDCTimer: timer}, nil
}
