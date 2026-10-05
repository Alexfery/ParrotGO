package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"parrot/internal/components/i2c"
	"parrot/internal/components/i2cdevice"
	"parrot/internal/project"
	"parrot/internal/resources"
	"parrot/internal/targets"
)

var addI2CDeviceCmd = &cobra.Command{
	Use:   "i2c-device <name> --bus <bus> --address <address> --frequency <hz>",
	Short: "Add a generic I2C device on an I2C bus of the project",
	Long: `Add a generic I2C device on an I2C bus of the project.

The device takes an address on an existing bus (see parrot add i2c) and gets
transmit, receive and transmit-receive functions. It uses no GPIO of its own:
the bus owns SDA and SCL.`,
	Example: `  parrot add i2c-device display --bus sensors --address 0x3C --frequency 400000
  parrot add i2c-device imu --bus sensors --address 104 --frequency 400000`,
	Args: cobra.ExactArgs(1),
	RunE: runAddI2CDevice,
}

func init() {
	flags := addI2CDeviceCmd.Flags()
	flags.String("bus", "", "name of the I2C bus the device is on")
	flags.String("address", "", "7-bit I2C address, without the R/W bit: hexadecimal (0x3C) or decimal (60)")
	flags.Int("frequency", 0, "SCL frequency for this device, in Hz")
	for _, name := range []string{"bus", "address", "frequency"} {
		addI2CDeviceCmd.MarkFlagRequired(name)
	}
	addCmd.AddCommand(addI2CDeviceCmd)
}

func runAddI2CDevice(cmd *cobra.Command, args []string) error {
	busName, err := cmd.Flags().GetString("bus")
	if err != nil {
		return err
	}
	addressText, err := cmd.Flags().GetString("address")
	if err != nil {
		return err
	}
	frequency, err := cmd.Flags().GetInt("frequency")
	if err != nil {
		return err
	}
	address, err := i2cdevice.ParseAddress(addressText)
	if err != nil {
		return err
	}
	d, err := i2cdevice.New(args[0], busName, address, frequency)
	if err != nil {
		return err
	}

	var bus i2c.Bus // set by resolve, shown by summary
	return addComponent(cmd, componentSpec{
		label:  "I2C device",
		kind:   i2cdevice.Type,
		name:   d.Name,
		config: d.Config,
		resolve: func(cfg project.Config) error {
			bus, err = d.ResolveBus(cfg)
			return err
		},
		summary: func(targets.Target, resources.Allocation) []string {
			return []string{
				fmt.Sprintf("Bus: %s (SDA GPIO%d, SCL GPIO%d)", bus.Name, bus.SDA, bus.SCL),
				"Address: " + i2cdevice.FormatAddress(d.Address),
				fmt.Sprintf("Frequency: %d Hz", d.Frequency),
			}
		},
		create: func(targets.Target, resources.Allocation) ([]string, error) {
			return i2cdevice.Create(".", d)
		},
	})
}
