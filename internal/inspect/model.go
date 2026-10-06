// Package inspect explains the hardware structure of a Parrot project. It
// works in two steps that never mix:
//
//   - Resolve analyses a parrot.json: it indexes the components, resolves the
//     components each one is built on, derives what Parrot computes instead
//     of storing (ADC channels, LEDC timers and channels, SPI hosts) and
//     checks the manifest's integrity. It returns an Inspection and prints
//     nothing.
//   - A renderer presents an Inspection. TextRenderer draws it as trees; other
//     formats can be added without touching the analysis.
//
// Inspecting is read only: nothing is written, generated or fixed, and ESP-IDF
// is not needed. Problems in the manifest are part of the result, as Issues.
package inspect

// Inspection is the analysed structure of a project.
type Inspection struct {
	Target TargetInfo

	// Components are the components that are not built on another one, in
	// parrot.json order. The others are nested under the component they are
	// built on (see Component.Children). A component whose dependency cannot
	// be resolved is listed here, with an Issue saying why.
	Components []Component
}

// TargetInfo is the target of the project, as the target registry describes it.
type TargetInfo struct {
	ID          string // Parrot ID, as in parrot.json, e.g. "esp32-c3"
	DisplayName string // e.g. "ESP32-C3"; empty if the target is not supported
	IDFTarget   string // ESP-IDF name, e.g. "esp32c3"; empty if the target is not supported
	Issues      []Issue
}

// Component is one component of parrot.json and the components built on it.
type Component struct {
	Name  string // as in parrot.json
	Type  string // as in parrot.json, e.g. "sensor-bme280"
	Label string // human name of the type, e.g. "BME280"; empty if Parrot does not know the type

	Properties []Property // what the component has, in display order

	// Dependencies are the components this one is built on ("depends on"),
	// e.g. the bus of an I2C device. They differ from the hardware resources
	// the component owns, which are Properties.
	Dependencies []Dependency

	// Children are the components built on this one, grouped by their role,
	// e.g. the devices on a bus.
	Children []Group

	Issues []Issue
}

// Property is a fact about a component, e.g. "Frequency: 5000 Hz" or "GPIO4".
type Property struct {
	Name   string // e.g. "Frequency"; empty when Value says what it is, e.g. "GPIO4"
	Value  string
	Source Source

	// Owned marks a hardware resource the component takes for itself, which
	// no other component can use: a GPIO, an I2C address on a bus, a LEDC
	// channel. A LEDC timer is not owned: PWM outputs with the same frequency
	// share it.
	Owned bool
}

// Source tells where a property comes from.
type Source int

const (
	// FromManifest is a value written in parrot.json, e.g. a GPIO.
	FromManifest Source = iota
	// FromTarget is derived from the target's hardware description and never
	// stored, e.g. the ADC unit and channel wired to a GPIO.
	FromTarget
	// FromAllocation is assigned by the resource allocator and never stored,
	// e.g. a LEDC timer and channel, or an SPI host. Code generation uses the same allocator,
	// so both reach the same values.
	FromAllocation
)

// Dependency is a component another one is built on.
type Dependency struct {
	Role string // what it is to the dependent component, e.g. "bus"
	Name string // the component named in parrot.json
	Type string // the type it must have, e.g. "i2c-bus"

	// Resolved reports whether a component with that name and type exists.
	// When it is false, the dependent component has an Issue saying why.
	Resolved bool
}

// Group lists the components built on another one that have the same role,
// e.g. "Devices" on a bus.
type Group struct {
	Name       string
	Components []Component // in parrot.json order
}

// Severity is how serious an Issue is.
type Severity int

const (
	// SeverityError makes the project invalid: Parrot cannot generate it as
	// it is.
	SeverityError Severity = iota
	// SeverityWarning is worth knowing, but does not make the project invalid,
	// e.g. a component type that a newer Parrot added.
	SeverityWarning
)

// Issue is a problem found in parrot.json.
type Issue struct {
	Severity Severity
	Message  string
}

// Count returns how many issues of the given severity the inspection has,
// on the target and on every component, nested or not.
func (in Inspection) Count(severity Severity) int {
	n := countIssues(in.Target.Issues, severity)
	for _, c := range in.Components {
		n += c.count(severity)
	}
	return n
}

func (c Component) count(severity Severity) int {
	n := countIssues(c.Issues, severity)
	for _, g := range c.Children {
		for _, child := range g.Components {
			n += child.count(severity)
		}
	}
	return n
}

func countIssues(issues []Issue, severity Severity) int {
	n := 0
	for _, issue := range issues {
		if issue.Severity == severity {
			n++
		}
	}
	return n
}
