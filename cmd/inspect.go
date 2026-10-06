package cmd

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"parrot/internal/inspect"
	"parrot/internal/project"
)

var inspectCmd = &cobra.Command{
	Use:   "inspect",
	Short: "Inspect the hardware structure of the current Parrot project",
	Long: `Inspect the hardware structure of the current Parrot project.

Shows the target and the components of parrot.json as trees, each I2C device
under its bus and each sensor under its device, with what Parrot derives
instead of storing: the ADC unit and channel of a GPIO, the LEDC timer and
channel of a PWM output, the host of an SPI bus. It only reads parrot.json:
nothing is generated or changed, and ESP-IDF is not needed.

Exits with a non-zero status when parrot.json has a problem (ERROR).
Warnings (WARNING) do not count.`,
	Args: cobra.NoArgs,
	RunE: runInspect,
}

func init() {
	rootCmd.AddCommand(inspectCmd)
}

func runInspect(cmd *cobra.Command, args []string) error {
	cfg, err := project.LoadConfig()
	if err != nil {
		return err
	}
	inspection := inspect.Resolve(cfg)
	out := cmd.OutOrStdout()
	renderer := inspect.TextRenderer{Writer: out}
	if err := renderer.Render(inspection); err != nil {
		return err
	}
	return inspectionOutcome(out, inspection)
}

// inspectionOutcome prints the summary of an inspection without problems, or
// returns the summary as the command's error, which makes parrot exit with a
// non-zero status, as parrot doctor does.
func inspectionOutcome(w io.Writer, inspection inspect.Inspection) error {
	fmt.Fprintln(w)
	if problems := inspection.Count(inspect.SeverityError); problems > 0 {
		return fmt.Errorf("%s detected in %s", count(problems, "problem"), project.ConfigFile)
	}
	if warnings := inspection.Count(inspect.SeverityWarning); warnings > 0 {
		fmt.Fprintf(w, "No problems detected (%s).\n", count(warnings, "warning"))
		return nil
	}
	fmt.Fprintln(w, "No problems detected.")
	return nil
}
