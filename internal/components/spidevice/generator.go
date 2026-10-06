package spidevice

import (
	"strings"

	"parrot/internal/components"
	"parrot/internal/components/spi"
	"parrot/internal/targets"
)

// templateData is what the device templates see. The bus is only named: its
// GPIOs and host stay in the bus component, which the device code asks for
// the host.
type templateData struct {
	Name      string // C symbol prefix, e.g. "display"
	Macro     string // C macro prefix, e.g. "DISPLAY"
	Target    string // e.g. "ESP32"
	Bus       string // the bus component: its header, C symbol prefix and CMake name
	CS        int
	Frequency int // Hz
	Mode      int
	CPOL      int
	CPHA      int

	// BusHasMISO is false for a write-only bus: the device then cannot
	// receive, and its transfer function says so instead of returning
	// bytes nobody sent.
	BusHasMISO bool
}

// Create generates the component under projectDir/components/<name> and
// returns the paths it created. bus is the device's bus, as resolved from the
// project (see ResolveBus): the generated code adds the device to it and does
// not create it.
func Create(projectDir string, d Device, bus spi.Bus, target targets.Target) ([]string, error) {
	data := templateData{
		Name:       d.Name,
		Macro:      strings.ToUpper(d.Name),
		Target:     target.DisplayName,
		Bus:        bus.Name,
		CS:         d.CS,
		Frequency:  d.Frequency,
		Mode:       d.Mode,
		CPOL:       d.CPOL(),
		CPHA:       d.CPHA(),
		BusHasMISO: bus.MISO != nil,
	}
	return components.Generate(projectDir, Type, d.Name, data)
}
