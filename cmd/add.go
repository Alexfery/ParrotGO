package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"parrot/internal/components/catalog"
	"parrot/internal/project"
	"parrot/internal/resources"
	"parrot/internal/targets"
)

// addCmd groups the component subcommands (e.g. `parrot add led`).
var addCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a hardware component to the current project",
}

func init() {
	rootCmd.AddCommand(addCmd)
}

// addPinFlag registers the required --pin flag on a component subcommand.
func addPinFlag(cmd *cobra.Command, usage string) {
	cmd.Flags().Int("pin", 0, usage)
	cmd.MarkFlagRequired("pin")
}

// componentSpec is what a component subcommand hands to addComponent.
type componentSpec struct {
	label  string // shown to the user, e.g. "LED"
	kind   string // component type in parrot.json, e.g. "led"
	name   string // canonical component name
	config any    // the component's typed Config, stored in parrot.json

	// summary returns the lines describing the component, e.g. "GPIO: 4".
	summary func(targets.Target, resources.Allocation) []string
	// resolve checks the components this one refers to, e.g. the bus of an
	// I2C device; nil if it refers to none.
	resolve func(project.Config) error
	// validated names what resolve checks beyond existence, shown as
	// "✓ Validated <validated>"; empty to show nothing.
	validated string
	// validate checks the component-specific hardware rules; nil if none.
	validate func(targets.Target) error
	create   func(targets.Target, resources.Allocation) ([]string, error) // generates the component files
}

// gpioSummary is the summary of components that only use a GPIO.
func gpioSummary(pin int) func(targets.Target, resources.Allocation) []string {
	return func(targets.Target, resources.Allocation) []string {
		return []string{fmt.Sprintf("GPIO: %d", pin)}
	}
}

// addComponent runs the steps shared by every `parrot add <type>` command.
// parrot.json is saved last, so a failed generation never registers the component.
func addComponent(cmd *cobra.Command, spec componentSpec) error {
	cfg, err := project.LoadConfig()
	if err != nil {
		return err
	}
	target, err := targets.Get(cfg.Target)
	if err != nil {
		return err
	}
	entry, err := project.NewComponentConfig(spec.kind, spec.name, spec.config)
	if err != nil {
		return err
	}
	// Only in memory until the files are generated.
	if err := cfg.AddComponent(entry); err != nil {
		return err
	}
	if spec.resolve != nil {
		if err := spec.resolve(cfg); err != nil {
			return err
		}
	}
	claims, err := catalog.Claims(cfg, target)
	if err != nil {
		return err
	}
	alloc, err := resources.Allocate(target, claims)
	if err != nil {
		return err
	}
	if spec.validate != nil {
		if err := spec.validate(target); err != nil {
			return err
		}
	}
	created, err := spec.create(target, alloc)
	if err != nil {
		return err
	}
	if err := project.SaveConfig(cfg); err != nil {
		return err
	}

	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "Adding %s: %s\nTarget: %s\n", spec.label, spec.name, target.DisplayName)
	for _, line := range spec.summary(target, alloc) {
		fmt.Fprintln(out, line)
	}
	fmt.Fprintln(out)
	if spec.validated != "" {
		fmt.Fprintf(out, "✓ Validated %s\n", spec.validated)
	}
	for _, path := range created {
		fmt.Fprintf(out, "✓ Created %s\n", filepath.ToSlash(path))
	}
	fmt.Fprintf(out, "✓ Updated %s\n\n%s component added successfully.\n", project.ConfigFile, spec.label)
	return nil
}
