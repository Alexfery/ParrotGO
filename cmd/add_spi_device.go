package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"parrot/internal/components/spi"
	"parrot/internal/components/spidevice"
	"parrot/internal/project"
	"parrot/internal/resources"
	"parrot/internal/targets"
)

var addSPIDeviceCmd = &cobra.Command{
	Use:   "spi-device <name> --bus <bus> --cs <gpio> --frequency <hz> --mode <0-3>",
	Short: "Add a generic SPI device on an SPI bus of the project",
	Long: `Add a generic SPI device on an SPI bus of the project.

The device is added to an existing bus (see parrot add spi) and gets
synchronous transmit and full-duplex transfer functions. It owns its CS line
only: the bus owns MOSI, MISO, SCLK and the SPI host, which all its devices
share. Each device has its own clock frequency and SPI mode:

  mode 0: CPOL 0, CPHA 0    mode 2: CPOL 1, CPHA 0
  mode 1: CPOL 0, CPHA 1    mode 3: CPOL 1, CPHA 1`,
	Example: `  parrot add spi-device display --bus main-bus --cs 5 --frequency 10000000 --mode 0
  parrot add spi-device sensor --bus main-bus --cs 17 --frequency 1000000 --mode 3`,
	Args: cobra.ExactArgs(1),
	RunE: runAddSPIDevice,
}

func init() {
	flags := addSPIDeviceCmd.Flags()
	flags.String("bus", "", "name of the SPI bus the device is on")
	flags.Int("cs", 0, "GPIO number of the device's CS (chip select) line")
	flags.Int("frequency", 0, "SCLK frequency for this device, in Hz")
	flags.Int("mode", 0, "SPI mode, 0 to 3: the clock polarity (CPOL) and phase (CPHA)")
	for _, name := range []string{"bus", "cs", "frequency", "mode"} {
		addSPIDeviceCmd.MarkFlagRequired(name)
	}
	addCmd.AddCommand(addSPIDeviceCmd)
}

func runAddSPIDevice(cmd *cobra.Command, args []string) error {
	flags := cmd.Flags()
	busName, err := flags.GetString("bus")
	if err != nil {
		return err
	}
	cs, err := flags.GetInt("cs")
	if err != nil {
		return err
	}
	frequency, err := flags.GetInt("frequency")
	if err != nil {
		return err
	}
	mode, err := flags.GetInt("mode")
	if err != nil {
		return err
	}
	d, err := spidevice.New(args[0], busName, cs, frequency, mode)
	if err != nil {
		return err
	}

	var bus spi.Bus // set by resolve, used by summary and create
	return addComponent(cmd, componentSpec{
		label:  "SPI device",
		kind:   spidevice.Type,
		name:   d.Name,
		config: d.Config,
		resolve: func(cfg project.Config) error {
			bus, err = d.ResolveBus(cfg)
			return err
		},
		summary: func(_ targets.Target, alloc resources.Allocation) []string {
			miso := "no MISO"
			if bus.MISO != nil {
				miso = fmt.Sprintf("MISO GPIO%d", *bus.MISO)
			}
			return []string{
				fmt.Sprintf("Bus: %s (%s: MOSI GPIO%d, %s, SCLK GPIO%d)",
					bus.Name, alloc.SPIHosts[bus.Name].Symbol, bus.MOSI, miso, bus.SCLK),
				fmt.Sprintf("CS: GPIO%d", d.CS),
				fmt.Sprintf("Frequency: %d Hz", d.Frequency),
				fmt.Sprintf("Mode: %d (CPOL %d, CPHA %d)", d.Mode, d.CPOL(), d.CPHA()),
			}
		},
		validate: func(t targets.Target) error { return spidevice.Validate(t, d.Config) },
		create: func(t targets.Target, _ resources.Allocation) ([]string, error) {
			return spidevice.Create(".", d, bus, t)
		},
	})
}
