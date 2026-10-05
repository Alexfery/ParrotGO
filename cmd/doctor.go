package cmd

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"parrot/internal/doctor"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check that this machine and the current project are ready for Parrot",
	Long: `Check that this machine and the current project are ready for Parrot.

Checks the ESP-IDF environment (idf.py, IDF_PATH, Python, CMake, Ninja and
ESP-IDF's tools) and, inside a Parrot project, parrot.json, CMakeLists.txt and
the target. It only inspects: nothing is installed or changed.

Exits with a non-zero status when a problem (✗) is found. Warnings (!) and
skipped checks (-) do not count.`,
	Args: cobra.NoArgs,
	RunE: runDoctor,
}

func init() {
	rootCmd.AddCommand(doctorCmd)
}

func runDoctor(cmd *cobra.Command, args []string) error {
	out := cmd.OutOrStdout()
	// Printed before the checks run: some take several seconds.
	fmt.Fprint(out, "Parrot Doctor\n\n")
	d := doctor.Doctor{System: doctor.Host{}, ProjectDir: "."}
	report := d.Run(cmd.Context())
	printReport(out, report)
	return reportOutcome(out, report)
}

// statusSymbols mark each result, so the report reads without colors.
var statusSymbols = map[doctor.Status]string{
	doctor.StatusOK:      "✓",
	doctor.StatusWarning: "!",
	doctor.StatusError:   "✗",
	doctor.StatusSkipped: "-",
}

func printReport(w io.Writer, report doctor.Report) {
	for _, section := range report.Sections {
		fmt.Fprintf(w, "%s\n%s\n\n", section.Name, strings.Repeat("-", len(section.Name)))
		for _, result := range section.Results {
			fmt.Fprintf(w, "%s %s\n", statusSymbols[result.Status], result.Name)
			if result.Message != "" {
				for _, line := range strings.Split(result.Message, "\n") {
					fmt.Fprintf(w, "  %s\n", line)
				}
			}
			fmt.Fprintln(w)
		}
	}
}

// reportOutcome prints the summary of a report without problems, or returns
// the summary as the command's error, which makes parrot exit with a
// non-zero status.
func reportOutcome(w io.Writer, report doctor.Report) error {
	if problems := report.Count(doctor.StatusError); problems > 0 {
		return fmt.Errorf("%s detected", count(problems, "problem"))
	}
	if warnings := report.Count(doctor.StatusWarning); warnings > 0 {
		fmt.Fprintf(w, "No problems detected (%s).\n", count(warnings, "warning"))
		return nil
	}
	fmt.Fprintln(w, "No problems detected.")
	return nil
}

// count returns "1 problem", "2 problems", ...
func count(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	return fmt.Sprintf("%d %ss", n, noun)
}
