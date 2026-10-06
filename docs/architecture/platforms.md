# Platforms

Parrot started as a CLI for ESP32 and ESP-IDF. Its core is now independent of
both: ESP32 is the first implementation of a general *platform*, and other
ecosystems can be added next to it without changing the commands.

```
CLI                     cmd/                       flags, headers, exit codes
 │
 ▼
Parrot Core             internal/core              reads parrot.json, finds the platform
 │                                                 through the Registry
 ▼
Platform interface      core.Platform              CreateProject, OpenProject
 │                      core.Project               Build, Flash, Monitor
 ▼
Platform implementations
 └── esp32              internal/platforms/esp32   the only one so far
      │
      ▼
     SDK                internal/espidf            idf.py
```

The core never imports `internal/espidf` or `internal/targets`: it does not
know that ESP32 projects are built with `idf.py`. It only calls
`project.Build(ctx)` on a project its platform opened.

## Hardware concepts

| Concept      | What it is                                         | ESP32 example                           | Future example (not implemented) | In Parrot                                 |
|--------------|----------------------------------------------------|-----------------------------------------|----------------------------------|-------------------------------------------|
| **Vendor**   | The company that makes the chip                    | Espressif                               | STMicroelectronics               | `hardware.Vendor`                         |
| **Family**   | A series of related chips                          | ESP32-C3                                | STM32F4                          | `hardware.Family`                         |
| **MCU**      | The chip the project is compiled for               | ESP32-C3 (`esp32-c3`)                   | STM32F401RE                      | `hardware.MCU`; `"target"` in parrot.json |
| **Board**    | A development board built around an MCU            | ESP32-C3-DevKitM-1                      | NUCLEO-F401RE                    | `hardware.Board`; `"board"`               |
| **Platform** | An ecosystem: the hardware it knows, and its SDK   | `esp32`                                 | `stm32`                          | `core.Platform`; `"platform"`             |
| **SDK**      | The toolchain that builds, flashes and monitors    | ESP-IDF, through `idf.py`               | STM32Cube HAL                    | hidden inside the platform                |

ESP32 chips are described per series, the level ESP-IDF builds for: the chips
of the ESP32-C3 series differ in package and in-package flash, and ESP-IDF has
one target for all of them. So on the ESP32 platform each family has a single
MCU, with the same ID and name. On STM32, one family (STM32F4) would have many
MCUs (STM32F401RE, STM32F411RE, ...).

The ESP32 platform knows these boards (`internal/platforms/esp32/boards.go`):

| Board ID             | Board                | MCU (target) |
|----------------------|----------------------|--------------|
| `esp32-devkit-v1`    | DOIT ESP32 DevKit V1 | `esp32`      |
| `esp32-c3-devkitm-1` | ESP32-C3-DevKitM-1   | `esp32-c3`   |
| `esp32-s3-devkitc-1` | ESP32-S3-DevKitC-1   | `esp32-s3`   |
| `esp32-c6-devkitc-1` | ESP32-C6-DevKitC-1   | `esp32-c6`   |

A board only names its MCU for now. Its headers, LEDs and buttons are not
described, so components are still checked against the chip.

## parrot.json

```json
{
  "platform": "esp32",
  "target": "esp32-c3",
  "board": "esp32-c3-devkitm-1"
}
```

- `platform` is optional. A manifest without it, which is what every
  manifest written before platforms looks like, is an ESP32 project
  (`project.DefaultPlatform`). Such manifests are not rewritten.
- `target` is the MCU, as before. With a `board`, it may be left out and is
  then the board's MCU. When both are given, they must match.
- `board` is optional.

`parrot new` writes `platform` and, with `--board`, `board`:

```bash
parrot new app                              # esp32 platform, target esp32
parrot new app --target esp32-c3
parrot new app --board esp32-c3-devkitm-1   # target esp32-c3, from the board
```

## How `parrot build` runs

1. `cmd/build.go` calls `platforms.OpenProject(".", terminal)`.
2. `core.Registry.OpenProject` reads `parrot.json`, takes its platform ID
   (`esp32` when there is none), gets that platform from the registry and
   calls `platform.OpenProject(dir, cfg, terminal)`. An unknown ID fails with
   `unsupported platform "<id>"` and the list of supported ones.
3. `esp32.Platform.OpenProject` resolves the target and the board, checks that
   the directory is an ESP-IDF project (`CMakeLists.txt`) and finds `idf.py`.
   Each check fails before anything runs, in the same order as before.
4. `cmd/build.go` prints its header from `project.Hardware()`, e.g.
   `Target: ESP32-C3`, followed by `Board: ESP32-C3-DevKitM-1` if the project
   has a board.
5. `project.Build(ctx)` runs `idf.py -DIDF_TARGET=esp32c3 build`.

`flash` and `monitor` follow the same steps, with `project.Flash(ctx, port)`
and `project.Monitor(ctx, port)`.

Opening a project is a separate step from building it so that every problem
is reported before the header is printed, and the toolchain is found only
once. `database/sql` follows the same pattern: drivers are registered by
name, `Driver.Open` returns a connection, and the work is done on that
connection.

## What is still ESP32-specific

Peripherals are not abstracted on purpose. These packages describe or drive
ESP32 and ESP-IDF only, and are used by the ESP32 platform or by commands that
exist only for ESP32 today:

- `internal/targets`: the ESP32 chips (GPIOs, ADC, LEDC, I2C, SPI hosts,
  general purpose timers).
- `internal/espidf`: the ESP-IDF tooling (the SDK layer of the ESP32
  platform). `parrot doctor` uses it as well.
- `internal/components`, `internal/resources`, `templates/`: ESP-IDF code
  generation for `parrot add` (including SPI buses and devices, on ESP-IDF's
  SPI Master driver), which `parrot inspect` also uses. `parrot add` enters
  the ESP32 platform through `esp32.Target`, which rejects a project on
  another platform and checks its target and board.
- `internal/doctor`: checks of the ESP-IDF environment.
- Help texts of `build`, `flash` and `monitor` that mention `idf.py`.

## Adding a platform (next step, not implemented)

A future STM32 platform would be a new package that implements the
interface:

```go
package stm32 // internal/platforms/stm32

type Platform struct{}

var _ core.Platform = Platform{}

func (Platform) ID() string { return "stm32" }

// CreateProject resolves the target and the board from the catalog
// (STMicroelectronics -> STM32F4 -> STM32F401RE -> NUCLEO-F401RE), calls
// project.Create, then generates the SDK's files.
func (Platform) CreateProject(ctx context.Context, opts core.CreateProjectOptions) (core.CreatedProject, error)

// OpenProject checks the hardware, the project files and the toolchain, and
// returns a core.Project whose Build, Flash and Monitor drive that toolchain.
func (Platform) OpenProject(dir string, cfg project.Config, stdio core.IO) (core.Project, error)
```

It would be registered in `cmd/platforms.go`:

```go
var platforms = core.NewRegistry(esp32.Platform{}, stm32.Platform{})
```

Neither `internal/core` nor `parrot new`, `build`, `flash` and `monitor`
change. `parrot add`, `inspect` and `doctor` would then need a per-platform
entry point (a component catalog and environment checks per platform). That
is the next refactoring, and it is not needed while ESP32 is the only
platform.
