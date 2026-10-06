// Package esp32 is the ESP32 platform: Espressif's ESP32 chips, whose
// projects are ESP-IDF projects, built, flashed and monitored with idf.py.
//
// It implements core.Platform, and is the only place where Parrot's core
// meets ESP-IDF: it joins the chip registry (internal/targets) and the
// ESP-IDF tooling (internal/espidf) behind the platform interface.
package esp32

import (
	"fmt"

	"parrot/internal/core"
	"parrot/internal/hardware"
	"parrot/internal/project"
	"parrot/internal/targets"
)

// ID names the platform in parrot.json and on the command line.
const ID = "esp32"

// Platform is the ESP32 platform. It holds no state: what a project needs,
// such as idf.py, is looked up on the machine when the project is opened.
type Platform struct{}

var _ core.Platform = Platform{}

// ID returns "esp32".
func (Platform) ID() string { return ID }

// Target returns the target of the ESP32 project cfg describes. It is the way
// into the platform for the commands that only exist for ESP32 so far, such
// as parrot add: it fails if cfg is on another platform, or if its target and
// board do not agree.
func Target(cfg project.Config) (targets.Target, error) {
	if id := cfg.PlatformID(); id != ID {
		return targets.Target{}, fmt.Errorf("%s names platform %q, but this command only supports %q", project.ConfigFile, id, ID)
	}
	target, _, err := resolve(cfg.Target, cfg.Board)
	return target, err
}

// resolve returns the target named targetID, and the hardware of a project
// on it and on the board named boardID, if any. A board implies its MCU, so
// targetID may be empty when boardID is not; when both are given, they must
// agree.
func resolve(targetID, boardID string) (targets.Target, core.Hardware, error) {
	var board *hardware.Board
	if boardID != "" {
		b, err := lookupBoard(boardID)
		if err != nil {
			return targets.Target{}, core.Hardware{}, err
		}
		switch targetID {
		case "":
			targetID = b.MCUID
		case b.MCUID:
		default:
			return targets.Target{}, core.Hardware{}, fmt.Errorf("target %q does not match board %q, whose target is %q", targetID, b.ID, b.MCUID)
		}
		board = &b
	}
	target, err := targets.Get(targetID)
	if err != nil {
		return targets.Target{}, core.Hardware{}, err
	}
	return target, core.Hardware{MCU: mcu(target), Board: board}, nil
}
