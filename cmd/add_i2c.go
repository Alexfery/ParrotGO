package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"parrot/internal/components/i2c"
	"parrot/internal/resources"
	"parrot/internal/targets"
)

var addI2CCmd = &cobra.Command{
	Use:   "i2c <name> --sda <gpio> --scl <gpio>",
	Short: "Add an I2C master bus, to be shared by I2C devices",
	Args:  cobra.ExactArgs(1),
	RunE:  runAddI2C,
}

func init() {
	addI2CCmd.Flags().Int("sda", 0, "GPIO number of the SDA (data) line")
	addI2CCmd.Flags().Int("scl", 0, "GPIO number of the SCL (clock) line")
	addI2CCmd.MarkFlagRequired("sda")
	addI2CCmd.MarkFlagRequired("scl")
	addCmd.AddCommand(addI2CCmd)
}

func runAddI2C(cmd *cobra.Command, args []string) error {
	sda, err := cmd.Flags().GetInt("sda")
	if err != nil {
		return err
	}
	scl, err := cmd.Flags().GetInt("scl")
	if err != nil {
		return err
	}
	b, err := i2c.New(args[0], sda, scl)
	if err != nil {
		return err
	}
	return addComponent(cmd, componentSpec{
		label:  "I2C bus",
		kind:   i2c.Type,
		name:   b.Name,
		config: b.Config,
		summary: func(t targets.Target, alloc resources.Allocation) []string {
			return []string{
				fmt.Sprintf("SDA: GPIO%d", b.SDA),
				fmt.Sprintf("SCL: GPIO%d", b.SCL),
				fmt.Sprintf("I2C controllers: %d of %d in use (ESP-IDF picks one when the bus is created)",
					alloc.I2CControllers, t.I2C.HPControllers),
			}
		},
		validate: func(t targets.Target) error { return i2c.Validate(t, b.Config) },
		create: func(t targets.Target, _ resources.Allocation) ([]string, error) {
			return i2c.Create(".", b, t)
		},
	})
}
