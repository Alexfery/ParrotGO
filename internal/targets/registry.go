package targets

import (
	"fmt"
	"slices"
	"strings"
)

// DefaultID is the target used when none is specified.
const DefaultID = "esp32"

// supported lists every target Parrot knows about. Add new targets here.
var supported = []Target{esp32, esp32C3, esp32S3, esp32C6}

// All returns every supported target.
func All() []Target {
	return slices.Clone(supported)
}

// Get returns the target with the given Parrot ID, or an error listing the
// supported targets if there is none.
func Get(id string) (Target, error) {
	for _, t := range supported {
		if t.ID == id {
			return t, nil
		}
	}
	return Target{}, fmt.Errorf("unsupported target %q\n\nSupported targets:\n%s", id, supportedList())
}

func supportedList() string {
	var b strings.Builder
	for _, t := range supported {
		fmt.Fprintf(&b, "  %s\n", t.ID)
	}
	return strings.TrimSuffix(b.String(), "\n")
}
