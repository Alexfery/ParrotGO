// Package cmd defines the CLI commands. It parses arguments and delegates
// work to the internal packages; it must not contain ESP-IDF logic.
package cmd

import (
	"context"
	"os"
	"os/signal"

	"github.com/spf13/cobra"
)

const banner = `   \
   (o>
\_//)
 \_/_)
  _|_
`

var rootCmd = &cobra.Command{
	Use:          "parrot",
	Short:        "Parrot - a developer-friendly CLI for ESP32 and ESP-IDF projects.",
	Long:         banner + "\nParrot - a developer-friendly CLI for ESP32 and ESP-IDF projects.",
	SilenceUsage: true,
}

// Execute runs the root command and exits with a non-zero code on failure.
// Ctrl+C cancels the command's context, which stops child processes such as
// idf.py instead of leaving them running.
func Execute() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	err := rootCmd.ExecuteContext(ctx)
	stop()
	if err != nil {
		os.Exit(1)
	}
}
