package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"parrot/internal/components/pwm"
	"parrot/internal/resources"
	"parrot/internal/targets"
)

var addPWMCmd = &cobra.Command{
	Use:   "pwm <name> --pin <gpio> --frequency <hz>",
	Short: "Add a PWM output driven by the LEDC peripheral",
	Args:  cobra.ExactArgs(1),
	RunE:  runAddPWM,
}

func init() {
	addPinFlag(addPWMCmd, "GPIO number the PWM signal is output on")
	addPWMCmd.Flags().Int("frequency", 0, "PWM frequency in Hz")
	addPWMCmd.MarkFlagRequired("frequency")
	addCmd.AddCommand(addPWMCmd)
}

func runAddPWM(cmd *cobra.Command, args []string) error {
	pin, err := cmd.Flags().GetInt("pin")
	if err != nil {
		return err
	}
	frequency, err := cmd.Flags().GetInt("frequency")
	if err != nil {
		return err
	}
	p, err := pwm.New(args[0], pin, frequency)
	if err != nil {
		return err
	}
	return addComponent(cmd, componentSpec{
		label:  "PWM",
		kind:   pwm.Type,
		name:   p.Name,
		config: p.Config,
		summary: func(_ targets.Target, alloc resources.Allocation) []string {
			ledc := alloc.LEDC[p.Name]
			return []string{
				fmt.Sprintf("GPIO: %d", p.Pin),
				fmt.Sprintf("Frequency: %d Hz", p.Frequency),
				fmt.Sprintf("LEDC: timer %d, channel %d, %d-bit duty resolution", ledc.Timer, ledc.Channel, ledc.Resolution),
			}
		},
		validate: func(t targets.Target) error { return pwm.Validate(t, p.Config) },
		create: func(t targets.Target, alloc resources.Allocation) ([]string, error) {
			return pwm.Create(".", p, t, alloc.LEDC[p.Name])
		},
	})
}
