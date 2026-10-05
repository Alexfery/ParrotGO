package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"parrot/internal/components/adc"
	"parrot/internal/resources"
	"parrot/internal/targets"
)

var addADCCmd = &cobra.Command{
	Use:   "adc <name> --pin <gpio>",
	Short: "Add an analog input read through the ADC",
	Args:  cobra.ExactArgs(1),
	RunE:  runAddADC,
}

func init() {
	addPinFlag(addADCCmd, "GPIO number the analog signal is connected to")
	addCmd.AddCommand(addADCCmd)
}

func runAddADC(cmd *cobra.Command, args []string) error {
	pin, err := cmd.Flags().GetInt("pin")
	if err != nil {
		return err
	}
	a, err := adc.New(args[0], pin)
	if err != nil {
		return err
	}
	return addComponent(cmd, componentSpec{
		label:  "ADC",
		kind:   adc.Type,
		name:   a.Name,
		config: a.Config,
		summary: func(t targets.Target, _ resources.Allocation) []string {
			info, _ := adc.Validate(t, a.Pin) // already validated
			return []string{
				fmt.Sprintf("GPIO: %d", a.Pin),
				fmt.Sprintf("ADC: unit %d, channel %d", info.Unit, info.Channel),
			}
		},
		validate: func(t targets.Target) error {
			_, err := adc.Validate(t, a.Pin)
			return err
		},
		create: func(t targets.Target, _ resources.Allocation) ([]string, error) {
			return adc.Create(".", a, t)
		},
	})
}
