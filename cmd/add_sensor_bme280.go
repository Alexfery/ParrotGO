package cmd

import (
	"github.com/spf13/cobra"

	"parrot/internal/components/i2cdevice"
	"parrot/internal/components/sensor/bme280"
	"parrot/internal/project"
	"parrot/internal/resources"
	"parrot/internal/targets"
)

var addBME280Cmd = &cobra.Command{
	Use:   "bme280 <name> --device <i2c-device>",
	Short: "Add a Bosch BME280 temperature, pressure and humidity sensor",
	Long: `Add a Bosch BME280 temperature, pressure and humidity sensor.

The sensor is built on an existing I2C device (see parrot add i2c-device),
which sets its address (0x76 or 0x77) and bus. Neither the device nor the bus
is created or changed: add them first.`,
	Example: `  parrot add i2c sensors --sda 21 --scl 22
  parrot add i2c-device environment-device --bus sensors --address 0x76 --frequency 400000
  parrot add sensor bme280 environment --device environment-device`,
	Args: cobra.ExactArgs(1),
	RunE: runAddBME280,
}

func init() {
	addBME280Cmd.Flags().String("device", "", "name of the I2C device the sensor is on")
	addBME280Cmd.MarkFlagRequired("device")
	addSensorCmd.AddCommand(addBME280Cmd)
}

func runAddBME280(cmd *cobra.Command, args []string) error {
	deviceName, err := cmd.Flags().GetString("device")
	if err != nil {
		return err
	}
	s, err := bme280.New(args[0], deviceName)
	if err != nil {
		return err
	}

	var device i2cdevice.Device // set by resolve, shown by summary
	return addComponent(cmd, componentSpec{
		label:  "BME280 sensor",
		kind:   bme280.Type,
		name:   s.Name,
		config: s.Config,
		resolve: func(cfg project.Config) error {
			device, err = s.ResolveDevice(cfg)
			return err
		},
		validated: "BME280 transport",
		summary: func(targets.Target, resources.Allocation) []string {
			return []string{
				"Device: " + device.Name,
				"Bus: " + device.Bus,
				"Address: " + i2cdevice.FormatAddress(device.Address),
			}
		},
		create: func(targets.Target, resources.Allocation) ([]string, error) {
			return bme280.Create(".", s)
		},
	})
}
