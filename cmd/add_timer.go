package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"parrot/internal/components/timer"
	"parrot/internal/resources"
	"parrot/internal/targets"
)

var addTimerCmd = &cobra.Command{
	Use:   "timer <name> --period <duration>",
	Short: "Add a periodic hardware timer driven by the GPTimer peripheral",
	Long: `Add a periodic hardware timer driven by the GPTimer peripheral.

The timer counts up at 1 MHz (one tick per microsecond) and raises an alarm at
the end of every period, reloading its count in hardware. The alarm runs a
callback in ISR context, where you add your own ISR-safe handling. The timer
uses no GPIO: it takes one of the target's general purpose timers, which
ESP-IDF picks when it is created.

The period is a duration with its unit: us, ms or s. It must be positive and a
whole number of microseconds.`,
	Example: `  parrot add timer heartbeat --period 1s
  parrot add timer sensor_tick --period 50ms
  parrot add timer fast_tick --period 500us`,
	Args: cobra.ExactArgs(1),
	RunE: runAddTimer,
}

func init() {
	addTimerCmd.Flags().String("period", "", "time between two alarms, e.g. 500us, 50ms or 1s")
	addTimerCmd.MarkFlagRequired("period")
	addCmd.AddCommand(addTimerCmd)
}

func runAddTimer(cmd *cobra.Command, args []string) error {
	periodText, err := cmd.Flags().GetString("period")
	if err != nil {
		return err
	}
	periodUS, err := timer.ParsePeriod(periodText)
	if err != nil {
		return err
	}
	t, err := timer.New(args[0], periodUS)
	if err != nil {
		return err
	}
	return addComponent(cmd, componentSpec{
		label:  "Timer",
		kind:   timer.Type,
		name:   t.Name,
		config: t.Config,
		summary: func(targets.Target, resources.Allocation) []string {
			return []string{
				fmt.Sprintf("Period: %s (%d us)", timer.FormatPeriod(t.PeriodUS), t.PeriodUS),
				fmt.Sprintf("GPTimer: %s, count up at 1 MHz, alarm at %d, auto-reload to 0", t.Mode, t.AlarmCount()),
			}
		},
		validate: func(target targets.Target) error { return timer.Validate(target, t.Config) },
		create: func(targets.Target, resources.Allocation) ([]string, error) {
			return timer.Create(".", t)
		},
	})
}
