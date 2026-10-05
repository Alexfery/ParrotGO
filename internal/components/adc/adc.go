// Package adc is the analog input component: one GPIO read through an ESP-IDF
// ADC oneshot channel. The user only gives the GPIO; the ADC unit and channel
// come from the target.
package adc

import (
	"fmt"
	"os"
	"strings"

	"parrot/internal/components"
	"parrot/internal/resources"
	"parrot/internal/targets"
)

// Type is the component type used in the CLI and in parrot.json.
const Type = "adc"

// unitsComponent is the support component that owns the ADC unit handles.
// ESP-IDF lets each unit be created only once (adc_oneshot_new_unit returns
// ESP_ERR_NOT_FOUND the second time), so every ADC component gets its
// handle from here instead of creating its own.
const unitsComponent = components.ReservedPrefix + "adc"

// ADC is an analog input connected to a GPIO.
type ADC struct {
	Name string // canonical component name, also the C symbol prefix
	Config
}

// Config is the ADC entry in parrot.json.
type Config struct {
	Pin int `json:"pin"`
}

// Needs reports the resources an analog input uses: its GPIO.
func (c Config) Needs(targets.Target) (resources.Needs, error) {
	return resources.Needs{GPIOs: []int{c.Pin}}, nil
}

// New builds an ADC from a user-supplied name, normalizing it.
func New(name string, pin int) (ADC, error) {
	normalized, err := components.NormalizeName(name)
	if err != nil {
		return ADC{}, err
	}
	return ADC{Name: normalized, Config: Config{Pin: pin}}, nil
}

// templateData is what the ADC templates see.
type templateData struct {
	Name    string // C symbol prefix, e.g. "light"
	Macro   string // C macro prefix, e.g. "LIGHT"
	Pin     int
	Target  string // e.g. "ESP32"
	Unit    string // ESP-IDF adc_unit_t, e.g. "ADC_UNIT_1"
	Channel string // ESP-IDF adc_channel_t, e.g. "ADC_CHANNEL_6"
}

// Create generates the component under projectDir/components/<name>, plus the
// shared parrot_adc component if the project does not have it yet, and returns
// the paths it created. It fails if the pin has no ADC on target.
func Create(projectDir string, a ADC, target targets.Target) ([]string, error) {
	info, err := Validate(target, a.Pin)
	if err != nil {
		return nil, err
	}
	created, err := ensureUnitsComponent(projectDir)
	if err != nil {
		return created, err
	}
	data := templateData{
		Name:    a.Name,
		Macro:   strings.ToUpper(a.Name),
		Pin:     a.Pin,
		Target:  target.DisplayName,
		Unit:    unitSymbol(info),
		Channel: channelSymbol(info),
	}
	written, err := components.Generate(projectDir, Type, a.Name, data)
	return append(created, written...), err
}

func ensureUnitsComponent(projectDir string) ([]string, error) {
	if _, err := os.Stat(components.Path(projectDir, unitsComponent)); err == nil {
		return nil, nil
	}
	return components.Generate(projectDir, unitsComponent, unitsComponent, nil)
}

// The ESP-IDF enums are named ADC_UNIT_<n> and ADC_CHANNEL_<n> on every target
// (they come from the target-independent hal/adc_types.h), and the unit and
// channel numbers in internal/targets follow the same numbering.
func unitSymbol(info targets.ADCInfo) string {
	return fmt.Sprintf("ADC_UNIT_%d", info.Unit)
}

func channelSymbol(info targets.ADCInfo) string {
	return fmt.Sprintf("ADC_CHANNEL_%d", info.Channel)
}
