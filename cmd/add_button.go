package cmd

import (
	"github.com/spf13/cobra"

	"parrot/internal/components/button"
	"parrot/internal/resources"
	"parrot/internal/targets"
)

var addButtonCmd = &cobra.Command{
	Use:   "button <name> --pin <gpio>",
	Short: "Add a push button (to GND, internal pull-up) read by polling",
	Args:  cobra.ExactArgs(1),
	RunE:  runAddButton,
}

func init() {
	addPinFlag(addButtonCmd, "GPIO number the button is connected to")
	addCmd.AddCommand(addButtonCmd)
}

func runAddButton(cmd *cobra.Command, args []string) error {
	pin, err := cmd.Flags().GetInt("pin")
	if err != nil {
		return err
	}
	b, err := button.New(args[0], pin)
	if err != nil {
		return err
	}
	return addComponent(cmd, componentSpec{
		label:    "Button",
		kind:     button.Type,
		name:     b.Name,
		config:   b.Config,
		summary:  gpioSummary(b.Pin),
		validate: func(t targets.Target) error { return button.Validate(t, b.Pin) },
		create: func(targets.Target, resources.Allocation) ([]string, error) {
			return button.Create(".", b)
		},
	})
}
