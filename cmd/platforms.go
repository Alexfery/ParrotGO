package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"parrot/internal/core"
	"parrot/internal/platforms/esp32"
)

// platforms are the platforms a project can use. The commands reach an SDK
// only through them; supporting another platform means registering it here.
// Tests replace it with fakes.
var platforms = core.NewRegistry(esp32.Platform{})

// openProject runs the checks shared by the commands that hand the current
// project to its platform's SDK: it loads parrot.json and has the platform
// it names open the project. The SDK uses the command's stdin, stdout and
// stderr: the terminal, outside tests.
func openProject(cmd *cobra.Command) (core.Project, error) {
	return platforms.OpenProject(".", core.IO{Stdin: cmd.InOrStdin(), Stdout: cmd.OutOrStdout(), Stderr: cmd.ErrOrStderr()})
}

// hardwareLines is how the commands' headers show what a project runs on:
// its MCU, then its board if it has one.
func hardwareLines(hw core.Hardware) string {
	lines := fmt.Sprintf("Target: %s\n", hw.MCU.Name)
	if hw.Board != nil {
		lines += fmt.Sprintf("Board: %s\n", hw.Board.Name)
	}
	return lines
}

// addPortFlag registers the optional --port (-p) flag of the commands that
// talk to a device. The port belongs to the machine, not to the project, so
// it is a flag and is not saved in parrot.json.
func addPortFlag(cmd *cobra.Command, usage string) {
	cmd.Flags().StringP("port", "p", "", usage+", e.g. COM5 or /dev/ttyUSB0 (default: detected by idf.py)")
}

// portLabel is how the header of those commands shows port.
func portLabel(port string) string {
	if port == "" {
		return "auto"
	}
	return port
}
