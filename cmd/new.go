package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"parrot/internal/project"
	"parrot/internal/targets"
)

var newCmd = &cobra.Command{
	Use:   "new <project-name>",
	Short: "Create a new ESP-IDF project",
	Args:  cobra.ExactArgs(1),
	RunE:  runNew,
}

func init() {
	newCmd.Flags().String("target", targets.DefaultID, "chip model to target")
	rootCmd.AddCommand(newCmd)
}

func runNew(cmd *cobra.Command, args []string) error {
	targetID, err := cmd.Flags().GetString("target")
	if err != nil {
		return err
	}
	target, err := targets.Get(targetID)
	if err != nil {
		return err
	}

	opts := project.CreateOptions{Name: args[0], Target: target}
	created, err := project.Create(opts)
	if err != nil {
		return err
	}

	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "Creating Parrot project: %s\nTarget: %s\n\n", opts.Name, target.DisplayName)
	for _, path := range created {
		fmt.Fprintf(out, "✓ Created %s\n", filepath.ToSlash(path))
	}
	fmt.Fprintf(out, "\nProject created successfully.\n\nNext:\n\n  cd %s\n  idf.py build\n", opts.Name)
	return nil
}
