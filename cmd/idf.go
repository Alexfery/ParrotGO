package cmd

import (
	"github.com/spf13/cobra"

	"parrot/internal/espidf"
	"parrot/internal/project"
	"parrot/internal/targets"
)

// openIDFProject runs the checks shared by the commands that hand the current
// project to idf.py: it loads parrot.json, resolves its target, checks that
// the directory is an ESP-IDF project and finds idf.py. The returned runner
// uses the command's stdin, stdout and stderr: the terminal, outside tests.
func openIDFProject(cmd *cobra.Command) (targets.Target, *espidf.Runner, error) {
	cfg, err := project.LoadConfig()
	if err != nil {
		return targets.Target{}, nil, err
	}
	target, err := targets.Get(cfg.Target)
	if err != nil {
		return targets.Target{}, nil, err
	}
	if err := espidf.CheckProject("."); err != nil {
		return targets.Target{}, nil, err
	}
	runner, err := espidf.NewRunner(cmd.InOrStdin(), cmd.OutOrStdout(), cmd.ErrOrStderr())
	if err != nil {
		return targets.Target{}, nil, err
	}
	return target, runner, nil
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
