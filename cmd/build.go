package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
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
	p, err := openProject(cmd)
	if err != nil {
		return err
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Parrot build\n\n%s\nBuilding project...\n\n", hardwareLines(p.Hardware()))
	if err := p.Build(cmd.Context()); err != nil {
		fmt.Fprintln(cmd.ErrOrStderr(), "\n✗ Build failed.")
		return err
	}
	fmt.Fprintln(cmd.OutOrStdout(), "\n✓ Build completed successfully.")
	return nil
}
