package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"parrot/internal/espidf"
)

var buildCmd = &cobra.Command{
	Use:   "build",
	Short: "Build the project with ESP-IDF for the target in parrot.json",
	Args:  cobra.NoArgs,
	RunE:  runBuild,
}

func init() {
	rootCmd.AddCommand(buildCmd)
}

func runBuild(cmd *cobra.Command, args []string) error {
	target, runner, err := openIDFProject(cmd)
	if err != nil {
		return err
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Parrot build\n\nTarget: %s\n\nBuilding project...\n\n", target.DisplayName)
	builder := espidf.Builder{Runner: runner}
	if err := builder.Build(cmd.Context(), ".", target); err != nil {
		fmt.Fprintln(cmd.ErrOrStderr(), "\n✗ Build failed.")
		return err
	}
	fmt.Fprintln(cmd.OutOrStdout(), "\n✓ Build completed successfully.")
	return nil
}
