// Package cmd defines the CLI commands. It parses arguments and delegates
// work to the internal packages; it must not contain ESP-IDF logic.
package cmd

import (
	"context"
	"os"
	"os/signal"

	"github.com/spf13/cobra"

	"parrot/internal/term"
)

const about = "Parrot - a developer-friendly CLI for ESP32 and ESP-IDF projects."

var rootCmd = &cobra.Command{
	Use:          "parrot",
	Short:        about,
	Long:         banner(false) + "\n" + about,
	SilenceUsage: true,
}

// Execute runs the root command and exits with a non-zero code on failure.
// Ctrl+C cancels the command's context, which stops child processes such as
// idf.py instead of leaving them running.
func Execute() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	// Help goes to stdout, so the mascot is colored only when stdout is a
	// color terminal; redirected help stays plain text.
	if term.ColorEnabled(os.Stdout) {
		rootCmd.Long = banner(true) + "\n" + about
		// Bare `parrot` greets with the mascot's animation before the help,
		// while `parrot --help` stays instant.
		if len(os.Args) == 1 && !animate(ctx, os.Stdout, intro()) {
			stop()
			os.Exit(1)
		}
	}
	err := rootCmd.ExecuteContext(ctx)
	stop()
	if err != nil {
		os.Exit(1)
	}
}
