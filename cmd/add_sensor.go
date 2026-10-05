package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// addSensorCmd groups the sensor drivers (e.g. `parrot add sensor bme280`).
// Each one is built on a transport component the project already has.
var addSensorCmd = &cobra.Command{
	Use:   "sensor <type>",
	Short: "Add a sensor driver, built on a component of the project",
	RunE:  runAddSensor,
	// So that `parrot add sensor bmp280 x --device y` reports the unknown
	// sensor rather than the flag, which belongs to the sensor commands.
	FParseErrWhitelist: cobra.FParseErrWhitelist{UnknownFlags: true},
}

func init() {
	addCmd.AddCommand(addSensorCmd)
}

// runAddSensor only runs when no sensor type matched: cobra would otherwise
// print the help for `parrot add sensor bmp280` and succeed.
func runAddSensor(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return cmd.Help()
	}
	var supported []string
	for _, sub := range cmd.Commands() {
		if sub.IsAvailableCommand() {
			supported = append(supported, sub.Name())
		}
	}
	return fmt.Errorf("unknown sensor %q (supported: %s)", args[0], strings.Join(supported, ", "))
}
