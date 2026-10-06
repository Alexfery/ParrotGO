package spi

import (
	"strings"

	"parrot/internal/components"
	"parrot/internal/targets"
)

// templateDir is the folder of the bus templates in templates/components.
const templateDir = "spi"

// templateData is what the bus templates see. The host comes from the
// allocator: the templates only print it.
type templateData struct {
	Name    string // C symbol prefix, e.g. "display_bus"
	Macro   string // C macro prefix, e.g. "DISPLAY_BUS"
	Target  string // e.g. "ESP32"
	MOSI    int
	HasMISO bool
	MISO    int // set if HasMISO
	SCLK    int
	Host    string // spi_host_device_t, e.g. "SPI2_HOST"
}

// Create generates the component under projectDir/components/<name>, on the
// SPI host allocated to it, and returns the paths it created.
func Create(projectDir string, b Bus, target targets.Target, host targets.SPIHost) ([]string, error) {
	data := templateData{
		Name:   b.Name,
		Macro:  strings.ToUpper(b.Name),
		Target: target.DisplayName,
		MOSI:   b.MOSI,
		SCLK:   b.SCLK,
		Host:   host.Symbol,
	}
	if b.MISO != nil {
		data.HasMISO, data.MISO = true, *b.MISO
	}
	return components.Generate(projectDir, templateDir, b.Name, data)
}
