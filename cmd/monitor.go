package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"parrot/internal/espidf"
)

var monitorCmd = &cobra.Command{
	Use:   "monitor",
	Short: "Monitor the serial output of the current Parrot project",
	Long: `Monitor the serial output of the current Parrot project.

Opens IDF Monitor (idf.py monitor) on the device's serial console. It neither
builds nor flashes the project: run parrot flash first. Leave the monitor with
Ctrl+]; Ctrl+T opens its menu. Without --port, idf.py picks the serial port
itself.`,
	Example: `  parrot monitor
  parrot monitor --port COM5
  parrot monitor -p /dev/ttyUSB0`,
	Args: cobra.NoArgs,
	RunE: runMonitor,
}

func init() {
	addPortFlag(monitorCmd, "serial port to monitor")
	rootCmd.AddCommand(monitorCmd)
}

// runMonitor prints its header, then leaves the terminal to IDF Monitor until
// the user exits it. A session is not a task that completes, so there is no
// success message; a failure is returned as the command's error.
func runMonitor(cmd *cobra.Command, args []string) error {
	port, err := cmd.Flags().GetString("port")
	if err != nil {
		return err
	}
	target, runner, err := openIDFProject(cmd)
	if err != nil {
		return err
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Parrot monitor\n\nTarget: %s\nPort: %s\n\nStarting serial monitor...\n\n", target.DisplayName, portLabel(port))
	monitor := espidf.Monitor{Runner: runner}
	return monitor.Run(cmd.Context(), espidf.MonitorOptions{ProjectDir: ".", Target: target, Port: port})
}
