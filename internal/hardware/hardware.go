// Package hardware names the hardware a Parrot project runs on, at four
// levels that must not be confused:
//
//	Vendor  the company that makes the chip       Espressif
//	Family  a series of related chips             ESP32-C3
//	MCU     the chip a project is compiled for    ESP32-C3
//	Board   a printed circuit board with the MCU  ESP32-C3-DevKitM-1
//
// The types only identify the hardware and link each level to the one above
// it by ID. What a chip can do (GPIOs, peripherals) and how to build for it
// belong to its platform (see internal/core and internal/platforms), which
// also holds the catalog of its vendors, families, MCUs and boards.
package hardware

// Vendor is a company that makes MCUs, e.g. Espressif.
type Vendor struct {
	ID   string // e.g. "espressif"
	Name string // e.g. "Espressif"
}

// Family is a series of MCUs of one vendor that share an architecture and
// peripherals, e.g. ESP32-C3, or STM32F4 for STMicroelectronics.
type Family struct {
	ID       string // e.g. "esp32-c3"
	Name     string // e.g. "ESP32-C3"
	VendorID string // the Vendor that makes it
}

// MCU is the chip a project is compiled for, e.g. ESP32-C3, or STM32F401RE
// in the STM32F4 family. Its ID is the "target" of parrot.json.
type MCU struct {
	ID       string // e.g. "esp32-c3"
	Name     string // e.g. "ESP32-C3"
	FamilyID string // the Family it belongs to
}

// Board is a development board built around an MCU, e.g. ESP32-C3-DevKitM-1.
// Its ID is the "board" of parrot.json.
type Board struct {
	ID    string // e.g. "esp32-c3-devkitm-1"
	Name  string // e.g. "ESP32-C3-DevKitM-1"
	MCUID string // the MCU on the board
}
