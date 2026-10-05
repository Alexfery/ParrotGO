// Package led is the LED component: a single GPIO driven as a digital output.
package led

import (
	"strings"

	"parrot/internal/components"
	"parrot/internal/resources"
	"parrot/internal/targets"
)

// Type is the component type used in the CLI and in parrot.json.
const Type = "led"

// LED is an LED connected to a GPIO.
type LED struct {
	Name string // canonical component name, also the C symbol prefix
	Config
}

// Config is the LED entry in parrot.json.
type Config struct {
	Pin int `json:"pin"`
}

// Needs reports the resources an LED uses: its GPIO.
func (c Config) Needs(targets.Target) (resources.Needs, error) {
	return resources.Needs{GPIOs: []int{c.Pin}}, nil
}

// New builds an LED from a user-supplied name, normalizing it.
func New(name string, pin int) (LED, error) {
	normalized, err := components.NormalizeName(name)
	if err != nil {
		return LED{}, err
	}
	return LED{Name: normalized, Config: Config{Pin: pin}}, nil
}

// templateData is what the LED templates see.
type templateData struct {
	Name  string // C symbol prefix, e.g. "status"
	Macro string // C macro prefix, e.g. "STATUS"
	Pin   int
}

// Create generates the component under projectDir/components/<name> and
// returns the paths it created. It fails if the component folder exists.
func Create(projectDir string, l LED) ([]string, error) {
	data := templateData{Name: l.Name, Macro: strings.ToUpper(l.Name), Pin: l.Pin}
	return components.Generate(projectDir, Type, l.Name, data)
}
