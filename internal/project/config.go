package project

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"parrot/internal/generator"
)

// ConfigFile is the name of the Parrot manifest in a project's root.
const ConfigFile = "parrot.json"

// Config is the content of parrot.json.
type Config struct {
	// Platform is the ID of the project's platform, e.g. "esp32". Manifests
	// written before Parrot had platforms have none: see PlatformID.
	Platform string `json:"platform,omitempty"`
	// Target is the ID of the MCU the project is compiled for, as the
	// platform names it (e.g. "esp32-c3"), not the SDK's name for it.
	Target string `json:"target"`
	// Board is the ID of the development board, e.g. "esp32-c3-devkitm-1";
	// empty when the project names none. The board's MCU is the target.
	Board string `json:"board,omitempty"`

	Components []ComponentConfig `json:"components,omitempty"`
}

// DefaultPlatform is the platform of a project whose parrot.json names none:
// every project was an ESP32 project before Parrot had platforms. New
// projects get it too, unless another one is asked for.
const DefaultPlatform = "esp32"

// PlatformID returns the ID of the project's platform.
func (cfg Config) PlatformID() string {
	if cfg.Platform == "" {
		return DefaultPlatform
	}
	return cfg.Platform
}

// ComponentConfig records a component added with `parrot add`. Settings
// specific to the component type live in Config, which each component package
// encodes and decodes with its own typed struct:
//
//	{"type": "pwm", "name": "fan", "config": {"pin": 18, "frequency": 5000}}
type ComponentConfig struct {
	Type   string          `json:"type"`
	Name   string          `json:"name"`
	Config json.RawMessage `json:"config"`
}

// NewComponentConfig builds a manifest entry, encoding config as its settings.
func NewComponentConfig(typ, name string, config any) (ComponentConfig, error) {
	raw, err := json.Marshal(config)
	if err != nil {
		return ComponentConfig{}, err
	}
	return ComponentConfig{Type: typ, Name: name, Config: raw}, nil
}

// Decode decodes the component's settings into v, rejecting unknown fields.
func (c ComponentConfig) Decode(v any) error {
	dec := json.NewDecoder(bytes.NewReader(c.Config))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return fmt.Errorf("invalid config for component %q in %s: %w", c.Name, ConfigFile, err)
	}
	return nil
}

// UnmarshalJSON also accepts the format used before settings moved into
// "config", where every component had a top-level pin:
//
//	{"type": "led", "name": "status", "pin": 4}
//
// Such entries are read as {"config": {"pin": 4}} and saved in the new format.
func (c *ComponentConfig) UnmarshalJSON(data []byte) error {
	type current ComponentConfig // same fields, without this method
	var v struct {
		current
		Pin *int `json:"pin"`
	}
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	*c = ComponentConfig(v.current)
	if c.Config == nil && v.Pin != nil {
		c.Config = json.RawMessage(fmt.Sprintf(`{"pin":%d}`, *v.Pin))
	}
	return nil
}

// Component returns the component called name, if cfg declares one. Components
// refer to each other by name, e.g. an I2C device to its bus.
func (cfg Config) Component(name string) (ComponentConfig, bool) {
	for _, c := range cfg.Components {
		if c.Name == name {
			return c, true
		}
	}
	return ComponentConfig{}, false
}

// AddComponent appends c to the config in memory. It fails if a component
// with the same name already exists. Hardware conflicts are checked by
// internal/resources.
func (cfg *Config) AddComponent(c ComponentConfig) error {
	if _, exists := cfg.Component(c.Name); exists {
		return fmt.Errorf("component %q already exists", c.Name)
	}
	cfg.Components = append(cfg.Components, c)
	return nil
}

// ErrNotProject is returned when there is no parrot.json to load.
var ErrNotProject = errors.New("current directory is not a Parrot project")

// LoadConfig reads parrot.json from the current directory.
func LoadConfig() (Config, error) {
	return LoadConfigFrom(".")
}

// LoadConfigFrom reads parrot.json from dir. It returns ErrNotProject if dir
// has none.
func LoadConfigFrom(dir string) (Config, error) {
	var cfg Config
	data, err := os.ReadFile(filepath.Join(dir, ConfigFile))
	if errors.Is(err, fs.ErrNotExist) {
		return cfg, ErrNotProject
	}
	if err != nil {
		return cfg, err
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("invalid %s: %w", ConfigFile, err)
	}
	return cfg, nil
}

// SaveConfig overwrites parrot.json in the current directory.
func SaveConfig(cfg Config) error {
	data, err := encodeConfig(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(ConfigFile, data, 0o644)
}

// writeConfig creates parrot.json in dir and returns its path.
// It fails if the file already exists.
func writeConfig(dir string, cfg Config) (string, error) {
	data, err := encodeConfig(cfg)
	if err != nil {
		return "", err
	}
	path := filepath.Join(dir, ConfigFile)
	return path, generator.WriteFile(path, data)
}

func encodeConfig(cfg Config) ([]byte, error) {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}
