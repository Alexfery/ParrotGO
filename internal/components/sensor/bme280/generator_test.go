package bme280

import (
	"reflect"
	"testing"
)

func TestTemplateData(t *testing.T) {
	s := Sensor{Name: "environment", Config: Config{Device: "environment_device"}}
	want := templateData{Name: "environment", Device: "environment_device"}
	if got := newTemplateData(s); got != want {
		t.Errorf("newTemplateData = %+v, want %+v", got, want)
	}
	// The templates get names only: no address, frequency, bus or GPIO, which
	// belong to the components below the sensor.
	typ := reflect.TypeOf(templateData{})
	var fields []string
	for i := range typ.NumField() {
		fields = append(fields, typ.Field(i).Name)
	}
	if !reflect.DeepEqual(fields, []string{"Name", "Device"}) {
		t.Errorf("templateData has fields %v, want only Name and Device", fields)
	}
}
