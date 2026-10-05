// Package button is the push-button component, read by polling.
//
// Wiring convention: the button connects the GPIO to GND and the GPIO's
// internal pull-up is enabled, so released = HIGH and pressed = LOW.
package button

import (
	"strings"

	"parrot/internal/components"
	"parrot/internal/resources"
	"parrot/internal/targets"
)

// Type is the component type used in the CLI and in parrot.json.
const Type = "button"

// Button is a push button connected to a GPIO.
type Button struct {
	Name string // canonical component name, also the C symbol prefix
	Config
}

// Config is the Button entry in parrot.json.
type Config struct {
	Pin int `json:"pin"`
}

// Needs reports the resources a button uses: its GPIO.
func (c Config) Needs(targets.Target) (resources.Needs, error) {
	return resources.Needs{GPIOs: []int{c.Pin}}, nil
}

// New builds a Button from a user-supplied name, normalizing it.
func New(name string, pin int) (Button, error) {
	normalized, err := components.NormalizeName(name)
	if err != nil {
		return Button{}, err
	}
	return Button{Name: normalized, Config: Config{Pin: pin}}, nil
}

// templateData is what the button templates see.
type templateData struct {
	Name  string // C symbol prefix, e.g. "user"
	Macro string // C macro prefix, e.g. "USER"
	Pin   int
}

// Create generates the component under projectDir/components/<name> and
// returns the paths it created. It fails if the component folder exists.
func Create(projectDir string, b Button) ([]string, error) {
	data := templateData{Name: b.Name, Macro: strings.ToUpper(b.Name), Pin: b.Pin}
	return components.Generate(projectDir, Type, b.Name, data)
}
