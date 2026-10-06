package esp32

import (
	"fmt"
	"strings"

	"parrot/internal/hardware"
	"parrot/internal/targets"
)

// espressif makes every chip of the platform.
var espressif = hardware.Vendor{ID: "espressif", Name: "Espressif"}

// Parrot describes ESP32 chips per series, the level ESP-IDF builds for
// (internal/targets is the single source of truth for them): the chips of
// the ESP32-C3 series differ in package, in-package flash and temperature
// range, and ESP-IDF builds for all of them as one target. So each series is
// a family with a single MCU, and both take the target's ID and name.

// family returns the family of target.
func family(target targets.Target) hardware.Family {
	return hardware.Family{ID: target.ID, Name: target.DisplayName, VendorID: espressif.ID}
}

// mcu returns target as an MCU: its "target" in parrot.json.
func mcu(target targets.Target) hardware.MCU {
	return hardware.MCU{ID: target.ID, Name: target.DisplayName, FamilyID: family(target).ID}
}

// boards are the development boards the platform knows. A board only names
// its MCU so far: the GPIOs routed to its headers, its LEDs and its buttons
// are not described, so components are still checked against the chip.
var boards = []hardware.Board{
	{ID: "esp32-devkit-v1", Name: "DOIT ESP32 DevKit V1", MCUID: "esp32"},     // DOIT, ESP-WROOM-32 module
	{ID: "esp32-c3-devkitm-1", Name: "ESP32-C3-DevKitM-1", MCUID: "esp32-c3"}, // Espressif, ESP32-C3-MINI-1 module
	{ID: "esp32-s3-devkitc-1", Name: "ESP32-S3-DevKitC-1", MCUID: "esp32-s3"}, // Espressif, ESP32-S3-WROOM-1 module
	{ID: "esp32-c6-devkitc-1", Name: "ESP32-C6-DevKitC-1", MCUID: "esp32-c6"}, // Espressif, ESP32-C6-WROOM-1 module
}

// lookupBoard returns the board with the given ID, or an error listing the
// boards the platform knows.
func lookupBoard(id string) (hardware.Board, error) {
	for _, b := range boards {
		if b.ID == id {
			return b, nil
		}
	}
	var list strings.Builder
	for _, b := range boards {
		fmt.Fprintf(&list, "\n  %s", b.ID)
	}
	return hardware.Board{}, fmt.Errorf("unsupported board %q\n\nSupported boards:%s", id, list.String())
}
