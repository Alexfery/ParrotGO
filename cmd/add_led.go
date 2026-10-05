package cmd

import (
	"github.com/spf13/cobra"

	"parrot/internal/components/led"
	"parrot/internal/resources"
	"parrot/internal/targets"
)

var addLEDCmd = &cobra.Command{
	Use:   "led <name> --pin <gpio>",
	Short: "Add an LED driven by a GPIO",
	Args:  cobra.ExactArgs(1),
	RunE:  runAddLED,
}

func init() {
	addPinFlag(addLEDCmd, "GPIO number the LED is connected to")
	addCmd.AddCommand(addLEDCmd)
}

func runAddLED(cmd *cobra.Command, args []string) error {
	pin, err := cmd.Flags().GetInt("pin")
	if err != nil {
		return err
	}
	l, err := led.New(args[0], pin)
	if err != nil {
		return err
	}
	return addComponent(cmd, componentSpec{
		label:    "LED",
		kind:     led.Type,
		name:     l.Name,
		config:   l.Config,
		summary:  gpioSummary(l.Pin),
		validate: func(t targets.Target) error { return led.Validate(t, l.Pin) },
		create: func(targets.Target, resources.Allocation) ([]string, error) {
			return led.Create(".", l)
		},
	})
}
