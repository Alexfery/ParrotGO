// Package i2c is the I2C master bus component: an SDA and an SCL GPIO driven
// by one of the target's HP I2C controllers, which I2C devices share.
//
// The bus owns its two GPIOs and its controller. The devices that will attach
// to it own neither: they reference the bus and have their own address and
// SCL frequency, which is why the bus has no frequency (ESP-IDF sets it per
// device, in i2c_device_config_t.scl_speed_hz).
package i2c

import (
	"errors"
	"fmt"
	"strings"

	"parrot/internal/components"
	"parrot/internal/project"
	"parrot/internal/resources"
	"parrot/internal/targets"
)

// Type is the component type in parrot.json. The CLI calls it `i2c`; the
// manifest says "i2c-bus" so that the I2C devices can have a type of their own.
const Type = "i2c-bus"

// templateDir is the folder of the bus templates in templates/components.
const templateDir = "i2c"

// Bus is an I2C master bus.
type Bus struct {
	Name string // canonical component name, also the C symbol prefix
	Config
}

// Config is the I2C bus entry in parrot.json. The controller is not stored:
// ESP-IDF picks a free one when the bus is created.
type Config struct {
	SDA int `json:"sda"`
	SCL int `json:"scl"`
}

// errSamePin is returned when SDA and SCL are the same GPIO.
var errSamePin = errors.New("SDA and SCL cannot use the same GPIO")

// Needs reports the resources a bus uses: its two GPIOs and an I2C controller.
func (c Config) Needs(targets.Target) (resources.Needs, error) {
	if c.SDA == c.SCL {
		return resources.Needs{}, errSamePin
	}
	return resources.Needs{GPIOs: []int{c.SDA, c.SCL}, I2CController: true}, nil
}

// New builds a bus from user-supplied values, normalizing the name. It
// rejects SDA and SCL on the same GPIO before any project check, which would
// otherwise report the bus as conflicting with itself.
func New(name string, sda, scl int) (Bus, error) {
	normalized, err := components.NormalizeName(name)
	if err != nil {
		return Bus{}, err
	}
	if sda == scl {
		return Bus{}, errSamePin
	}
	return Bus{Name: normalized, Config: Config{SDA: sda, SCL: scl}}, nil
}

// Lookup returns the I2C bus called name in cfg, for a component that refers
// to it. It fails if cfg has no component of that name, or if that component
// is not an I2C bus.
func Lookup(cfg project.Config, name string) (Bus, error) {
	c, found := cfg.Component(name)
	if !found {
		return Bus{}, fmt.Errorf("I2C bus %q does not exist", name)
	}
	if c.Type != Type {
		return Bus{}, fmt.Errorf("component %q is not an I2C bus", name)
	}
	var config Config
	if err := c.Decode(&config); err != nil {
		return Bus{}, err
	}
	return Bus{Name: c.Name, Config: config}, nil
}

// templateData is what the bus templates see.
type templateData struct {
	Name   string // C symbol prefix, e.g. "sensors"
	Macro  string // C macro prefix, e.g. "SENSORS"
	Target string // e.g. "ESP32"
	SDA    int
	SCL    int
}

// Create generates the component under projectDir/components/<name> and
// returns the paths it created.
func Create(projectDir string, b Bus, target targets.Target) ([]string, error) {
	data := templateData{
		Name:   b.Name,
		Macro:  strings.ToUpper(b.Name),
		Target: target.DisplayName,
		SDA:    b.SDA,
		SCL:    b.SCL,
	}
	return components.Generate(projectDir, templateDir, b.Name, data)
}
