package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"parrot/internal/espidf"
)

var flashCmd = &cobra.Command{
	Use:   "flash",
	Short: "Flash the current Parrot project to an ESP32 device",
	Long: `Flash the current Parrot project to an ESP32 device.

Runs idf.py flash for the target in parrot.json. idf.py rebuilds the project
first when the sources changed, so there is no need to run parrot build before.
Without --port, idf.py looks for the device's serial port itself.`,
	Example: `  parrot flash
  parrot flash --port COM5
  parrot flash -p /dev/ttyUSB0`,
	Args: cobra.NoArgs,
	RunE: runFlash,
}

func init() {
	addPortFlag(flashCmd, "serial port of the device")
	rootCmd.AddCommand(flashCmd)
}

func runFlash(cmd *cobra.Command, args []string) error {
	port, err := cmd.Flags().GetString("port")
	if err != nil {
		return err
	}
	target, runner, err := openIDFProject(cmd)
	if err != nil {
		return err
	}

	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "Parrot flash\n\nTarget: %s\nPort: %s\n\nFlashing device...\n\n", target.DisplayName, portLabel(port))
	flasher := espidf.Flasher{Runner: runner}
	opts := espidf.FlashOptions{ProjectDir: ".", Target: target, Port: port}
	if err := flasher.Flash(cmd.Context(), opts); err != nil {
		fmt.Fprintln(cmd.ErrOrStderr(), "\n✗ Flash failed.")
		return err
	}
	fmt.Fprintln(out, "\n✓ Flash completed successfully.")
	return nil
}
