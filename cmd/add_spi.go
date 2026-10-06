package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"parrot/internal/components/spi"
	"parrot/internal/resources"
	"parrot/internal/targets"
)

var addSPICmd = &cobra.Command{
	Use:   "spi <name> --mosi <gpio> [--miso <gpio>] --sclk <gpio>",
	Short: "Add an SPI master bus, to be shared by SPI devices",
	Long: `Add an SPI master bus, to be shared by SPI devices.

The bus owns its MOSI, MISO and SCLK lines and one of the target's SPI hosts,
which Parrot picks: the first bus gets the first host, the next bus the next
one. Without --miso the bus only sends, e.g. to a display.

CS, clock frequency and SPI mode belong to each device, not to the bus.`,
	Example: `  parrot add spi main-bus --mosi 23 --miso 19 --sclk 18
  parrot add spi display-bus --mosi 23 --sclk 18`,
	Args: cobra.ExactArgs(1),
	RunE: runAddSPI,
}

func init() {
	flags := addSPICmd.Flags()
	flags.Int("mosi", 0, "GPIO number of the MOSI line (data from the master)")
	flags.Int("miso", 0, "GPIO number of the MISO line (data to the master); omit it for a bus that only sends")
	flags.Int("sclk", 0, "GPIO number of the SCLK line (clock)")
	addSPICmd.MarkFlagRequired("mosi")
	addSPICmd.MarkFlagRequired("sclk")
	addCmd.AddCommand(addSPICmd)
}

func runAddSPI(cmd *cobra.Command, args []string) error {
	mosi, err := cmd.Flags().GetInt("mosi")
	if err != nil {
		return err
	}
	sclk, err := cmd.Flags().GetInt("sclk")
	if err != nil {
		return err
	}
	// GPIO0 is a valid MISO, so only an absent flag means "no MISO".
	var miso *int
	if cmd.Flags().Changed("miso") {
		pin, err := cmd.Flags().GetInt("miso")
		if err != nil {
			return err
		}
		miso = &pin
	}
	b, err := spi.New(args[0], mosi, miso, sclk)
	if err != nil {
		return err
	}
	return addComponent(cmd, componentSpec{
		label:  "SPI bus",
		kind:   spi.Type,
		name:   b.Name,
		config: b.Config,
		summary: func(t targets.Target, alloc resources.Allocation) []string {
			misoLine := "MISO: disabled"
			if b.MISO != nil {
				misoLine = fmt.Sprintf("MISO: GPIO%d", *b.MISO)
			}
			return []string{
				fmt.Sprintf("MOSI: GPIO%d", b.MOSI),
				misoLine,
				fmt.Sprintf("SCLK: GPIO%d", b.SCLK),
				fmt.Sprintf("Host: %s (%d of %d SPI hosts in use)",
					alloc.SPIHosts[b.Name].Symbol, len(alloc.SPIHosts), len(t.SPI.Hosts)),
			}
		},
		validate: func(t targets.Target) error { return spi.Validate(t, b.Config) },
		create: func(t targets.Target, alloc resources.Allocation) ([]string, error) {
			return spi.Create(".", b, t, alloc.SPIHosts[b.Name])
		},
	})
}
