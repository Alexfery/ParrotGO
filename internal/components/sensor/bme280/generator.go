package bme280

import "parrot/internal/components"

// templateDir is the folder of the BME280 templates in templates/components.
const templateDir = "sensor/bme280"

// templateData is what the BME280 templates see. It has no address,
// frequency or GPIO: the generated driver reaches the sensor through the
// device's functions, so these stay in the device and bus components.
type templateData struct {
	Name   string // the sensor: C symbol prefix, header and CMake name, e.g. "environment"
	Device string // the I2C device: C symbol prefix, header and CMake name, e.g. "environment_device"
}

func newTemplateData(s Sensor) templateData {
	return templateData{Name: s.Name, Device: s.Device}
}

// Create generates the component under projectDir/components/<name> and
// returns the paths it created. The device must already exist (see
// ResolveDevice): the generated code uses it and never initializes it.
func Create(projectDir string, s Sensor) ([]string, error) {
	return components.Generate(projectDir, templateDir, s.Name, newTemplateData(s))
}
