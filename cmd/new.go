package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"parrot/internal/core"
	"parrot/internal/project"
)

var newCmd = &cobra.Command{
	Use:   "new <project-name>",
	Short: "Create a new ESP-IDF project",
	Example: `  parrot new my-project
  parrot new my-project --target esp32-c3
  parrot new my-project --board esp32-c3-devkitm-1`,
	Args: cobra.ExactArgs(1),
	RunE: runNew,
}

func init() {
	flags := newCmd.Flags()
	flags.String("platform", project.DefaultPlatform, "embedded platform of the project")
	flags.String("target", "", "chip model to target (default: the board's chip, or the platform's default, e.g. esp32)")
	flags.String("board", "", "development board, e.g. esp32-c3-devkitm-1; its chip is the target")
	rootCmd.AddCommand(newCmd)
}

func runNew(cmd *cobra.Command, args []string) error {
	flags := cmd.Flags()
	platformID, err := flags.GetString("platform")
	if err != nil {
		return err
	}
	target, err := flags.GetString("target")
	if err != nil {
		return err
	}
	board, err := flags.GetString("board")
	if err != nil {
		return err
	}
	platform, err := platforms.Get(platformID)
	if err != nil {
		return err
	}

	opts := core.CreateProjectOptions{Name: args[0], Target: target, Board: board}
	created, err := platform.CreateProject(cmd.Context(), opts)
	if err != nil {
		return err
	}

	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "Creating Parrot project: %s\n%s\n", opts.Name, hardwareLines(created.Hardware))
	for _, path := range created.Paths {
		fmt.Fprintf(out, "✓ Created %s\n", filepath.ToSlash(path))
	}
	fmt.Fprintf(out, "\nProject created successfully.\n\nNext:\n\n  cd %s\n  parrot build\n", opts.Name)
	return nil
}
