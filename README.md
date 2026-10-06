# Parrot

```
   \
   (o>
\_//)
 \_/_)
  _|_
```

Parrot is a developer-friendly CLI for ESP32 and ESP-IDF projects, written in Go.
It aims to bring a modern framework-CLI experience (similar to Nest CLI) to
embedded development. Its mascot is a parrot.

Presentation page: <https://parrotgodocs-exgywo07p-alexferys-projects.vercel.app/>

> Status: `parrot new`, `parrot add led|button|adc|pwm|i2c|i2c-device|spi|spi-device`, `parrot add sensor bme280`, `parrot build`, `parrot flash`, `parrot monitor`, `parrot doctor` and `parrot inspect` are implemented.

## Usage

```bash
go run . --help
go run . new my-project
go run . new my-project --target esp32-c3
go run . new my-project --board esp32-c3-devkitm-1   # the board sets the target
go run . add led status --pin 4     # run inside a Parrot project
go run . add button user --pin 18
go run . add adc light --pin 34
go run . add pwm fan --pin 18 --frequency 5000
go run . add i2c sensors --sda 21 --scl 22   # I2C master bus
go run . add spi main-bus --mosi 23 --miso 19 --sclk 18   # SPI master bus
go run . add spi display-bus --mosi 23 --sclk 18          # write-only: no MISO
go run . add spi-device display --bus main-bus --cs 5 --frequency 10000000 --mode 0
go run . add i2c-device display --bus sensors --address 0x3C --frequency 400000
go run . add i2c-device environment-device --bus sensors --address 0x76 --frequency 400000
go run . add sensor bme280 environment --device environment-device
go run . build                     # needs an activated ESP-IDF environment
go run . flash                     # port detected by idf.py
go run . flash --port COM5         # or -p /dev/ttyUSB0
go run . monitor                   # Ctrl+] to leave
go run . monitor --port COM5
go run . doctor                    # checks the environment, and the project if any
go run . inspect                   # explains the project's hardware structure
```

In a terminal, the help shows the mascot in color, and `parrot` without
arguments first plays a short animation of it (`parrot --help` shows the help
right away). When the output is redirected or `NO_COLOR` is set, the help is
plain text, without the animation.

Build the `parrot` executable:

```bash
go build -o parrot .
```

## Layout

| Path                          | Responsibility                                                    |
|-------------------------------|-------------------------------------------------------------------|
| `main.go`                     | Entry point; starts the root command.                             |
| `cmd/`                        | CLI commands: argument parsing, delegating to `internal/`.        |
| `internal/core/`              | Platform interface and registry; opens a project with its platform. |
| `internal/hardware/`          | Vendor, family, MCU and board models.                             |
| `internal/platforms/esp32/`   | The ESP32 platform: its boards, ESP-IDF project creation and lifecycle. |
| `internal/generator/`         | Template rendering engine for projects and components.            |
| `internal/project/`           | Project directory and `parrot.json`, whatever the platform.       |
| `internal/targets/`           | ESP32 chips and their hardware (GPIOs, ADC, LEDC, I2C, SPI).      |
| `internal/components/`        | Hardware component definitions (LED, button, ADC, PWM, I2C, ...). |
| `internal/components/sensor/` | Sensor drivers, one package per sensor (`bme280/`).               |
| `internal/espidf/`            | Integration with ESP-IDF tooling (`idf.py` build/flash/monitor).  |
| `internal/doctor/`            | Read-only checks of the environment and the project.              |
| `internal/inspect/`           | Turns `parrot.json` into component trees, and renders them.       |
| `templates/`                  | Templates for generated files, embedded into the binary.          |

The commands never call ESP-IDF themselves: `build`, `flash`, `monitor` and
`new` go through the platform named in `parrot.json`, and ESP32 is the first
platform. See [docs/architecture/platforms.md](docs/architecture/platforms.md)
for the layers, the vendor/family/MCU/board model and how to add a platform.

Generated projects are meant to stay compatible with standard ESP-IDF tooling
(ESP-IDF v5.3 or newer: the components depend on `esp_driver_gpio`,
`esp_driver_ledc`, `esp_adc`, `esp_driver_i2c` and `esp_driver_spi`).

Component names are normalized (lowercase, `-` becomes `_`) and must not be
`main`: ESP-IDF adds `components/` after the project's `main` folder, and a
component added later replaces one with the same name, so `components/main`
would silently replace `app_main`.

## I2C buses

`parrot add i2c <name> --sda <gpio> --scl <gpio>` adds an I2C master bus: a
shared resource that the I2C devices of a future `parrot add i2c-device` will
attach to.

- **The bus owns its two GPIOs and one I2C controller.** Devices will only
  reference the bus and have an address, so they never claim GPIOs again.
- **SDA and SCL** must be different GPIOs that are both input and output, with
  pull resistors: the driver sets each line up as an open-drain input/output
  routed through the GPIO matrix.
- **Controllers:** only high-power (HP) I2C controllers are used: 2 on ESP32 and
  ESP32-S3, 1 on ESP32-C3 and ESP32-C6 (whose LP I2C controller is not used).
  Parrot rejects a bus when they are all taken. It does not pick one: the
  generated code passes `i2c_port = -1` and ESP-IDF takes a free controller
  when the bus is created. The controllers are interchangeable, so nothing is
  stored in `parrot.json`.
- **No frequency on the bus:** with ESP-IDF's I2C master driver, the SCL speed
  is set per device (`i2c_device_config_t.scl_speed_hz`).
- **Pull-ups:** the generated bus enables the internal pull-ups. They are weak:
  fine for short wires and simulation, but real buses usually need external
  pull-up resistors from SDA and SCL to 3.3 V.

The component exposes `<name>_init()`, `<name>_deinit()` (both return
`esp_err_t`) and `<name>_get_handle()`, which returns the
`i2c_master_bus_handle_t` devices pass to `i2c_master_bus_add_device()`. Its
`CMakeLists.txt` uses `REQUIRES esp_driver_i2c` (public), because its header
exposes that handle type.

### I2C devices

`parrot add i2c-device <name> --bus <bus> --address <address> --frequency <hz>`
adds a generic device to an existing bus:

```
sensors (SDA 21, SCL 22)
├── display @ 0x3C, 400 kHz
└── imu     @ 0x68, 400 kHz
```

- **The device owns an address, not GPIOs.** It references its bus by name
  (`"bus": "sensors"` in `parrot.json`); the bus entry is the only place SDA and
  SCL are recorded. `--bus` must name an existing `i2c-bus` component.
- **Addresses** are 7-bit, without the R/W bit, given as `0x3C` or `60` and
  stored as a number (`"address": 60`). Two devices conflict only if they have
  the same address *on the same bus*. The addresses the I2C specification
  reserves (0x00-0x07, 0x78-0x7F) are rejected; for an 8-bit address from a
  datasheet (e.g. 0x78), the error suggests the 7-bit one (0x3C).
- **`--frequency`** is the SCL frequency of this device
  (`i2c_device_config_t.scl_speed_hz`); devices on one bus can differ.

The component exposes `<name>_init()`, `<name>_deinit()`, `<name>_get_handle()`
(an `i2c_master_dev_handle_t`) and three generic transactions, each with a
timeout in milliseconds (`-1` waits forever): `<name>_transmit()`,
`<name>_receive()` and `<name>_transmit_receive()` (write, repeated START, read,
for register reads). They move bytes; what the bytes mean is up to the driver
of the actual device, built on top of them.

Lifecycle is explicit: `sensors_init()` creates the bus, `display_init()` adds
the device (and fails with `ESP_ERR_INVALID_STATE` if the bus does not exist
yet), `display_deinit()` removes the device only, and `sensors_deinit()` refuses
while devices are attached. In CMake the device has `REQUIRES esp_driver_i2c`
(its header exposes the handle type) and `PRIV_REQUIRES <bus>` (only its `.c`
includes the bus header).

## SPI buses

`parrot add spi <name> --mosi <gpio> [--miso <gpio>] --sclk <gpio>` adds an SPI
master bus: the shared transport of the SPI devices `parrot add spi-device`
attaches to it.

```
main_bus
└── SPI Bus
    ├── Host: SPI2_HOST
    ├── MOSI: GPIO23
    ├── MISO: GPIO19
    └── SCLK: GPIO18
```

- **The bus owns its GPIOs and one SPI host.** CS, clock frequency, SPI mode
  and queue size belong to each device (`spi_device_interface_config_t`), so
  the bus has no `--cs`, `--frequency` or `--mode`.
- **Hosts** come from the target registry: only the general purpose SPI
  controllers that `spi_bus_initialize` accepts (never SPI0 or SPI1, which
  serve the flash). ESP32 and ESP32-S3 have `SPI2_HOST` and `SPI3_HOST`;
  ESP32-C3 and ESP32-C6 only `SPI2_HOST`.
- **Allocation:** unlike I2C, ESP-IDF needs the host when the bus is created,
  so Parrot picks it: buses take the target's hosts in order, in `parrot.json`
  order, and a bus too many fails with `no SPI hosts available on <target>`.
  The host is not stored: it is derived again on every change, and
  `parrot inspect` shows the same one.
- **GPIOs:** MOSI and SCLK, which the master drives, need output GPIOs; MISO,
  which it reads, an input GPIO (an input-only GPIO such as the ESP32's
  GPIO34-39 is fine). The signals go through the GPIO matrix, so any such GPIO
  works; pins must be distinct, and the flash pins are rejected.
- **No MISO:** without `--miso` the bus only sends (e.g. to a display):
  `parrot.json` has no `"miso"` and the generated code sets
  `.miso_io_num = -1`. Unused quad and octal lines are set to `-1` too.
- **DMA:** `SPI_DMA_CH_AUTO`, accepted on every supported target: the driver
  picks the DMA channel. With a 32 Mbit PSRAM at 80 MHz, the ESP32 uses one
  GP-SPI host for the PSRAM clock (SPI3 by default), which is why SPI2 is
  allocated first.

The component exposes `<name>_init()` (`spi_bus_initialize`; returns
`ESP_ERR_INVALID_STATE` if the bus is already initialized), `<name>_deinit()`
(`spi_bus_free`, which fails while devices are attached: remove them first)
and `<name>_get_host()`, the `spi_host_device_t` devices pass to
`spi_bus_add_device()`. Errors are returned, never `ESP_ERROR_CHECK`ed. Its
`CMakeLists.txt` uses `REQUIRES esp_driver_spi` (public), because its header
exposes `spi_host_device_t`. There are no transactions on the bus: they
belong to the devices.

### SPI devices

`parrot add spi-device <name> --bus <bus> --cs <gpio> --frequency <hz> --mode <0-3>`
adds a generic device to an existing SPI bus:

```
main_bus
└── SPI Bus
    ├── Host: SPI2_HOST
    ├── MOSI: GPIO23
    ├── MISO: GPIO19
    ├── SCLK: GPIO18
    └── Devices
        ├── display (SPI Device)
        │   ├── CS: GPIO5
        │   ├── Frequency: 10000000 Hz
        │   └── Mode: 0 (CPOL 0, CPHA 0)
        └── sensor (SPI Device)
            ├── CS: GPIO17
            ├── Frequency: 1000000 Hz
            └── Mode: 3 (CPOL 1, CPHA 1)
```

```bash
parrot add spi main-bus --mosi 23 --miso 19 --sclk 18
parrot add spi-device display --bus main-bus --cs 5 --frequency 10000000 --mode 0
parrot add spi-device sensor --bus main-bus --cs 17 --frequency 1000000 --mode 3
```

- **The device owns its CS GPIO only.** MOSI, MISO, SCLK and the host belong
  to the bus and are shared by all its devices, so they are not copied into
  the device's entry: `{"bus": "main_bus", "cs": 5, "frequency": 10000000,
  "mode": 0}`. `--bus` must name an existing `spi-bus` component (looked up in
  `parrot.json`, never in `components/`).
- **CS** must exist on the target, be an output GPIO (the master drives it,
  and `spi_bus_add_device` rejects anything else) and not be a flash pin. A
  GPIO already used by another component is rejected by the resource
  allocator, which names the owner: `GPIO23 is already used by SPI bus
  "main_bus"` for one of the bus's lines, `GPIO5 is already used by SPI device
  "display"` for another device's CS.
- **Frequency and mode belong to the device**, as ESP-IDF sets them per
  device (`spi_device_interface_config_t`): devices on one bus can differ.
  The frequency must be positive and fit in the C `int` of `clock_speed_hz`;
  ESP-IDF rejects one above the SPI clock source, and the device's driver
  knows what the chip accepts. The mode is 0-3: mode 0 is (CPOL 0, CPHA 0),
  1 is (0, 1), 2 is (1, 0), 3 is (1, 1). CPOL is the idle level of SCLK, CPHA
  whether data is sampled on the first or the second edge.
- **Queue size 1:** Parrot's transactions are synchronous
  (`spi_device_transmit`), so no more than one is ever queued. There is no
  `--queue-size`.
- **Not generated:** queued or asynchronous transactions, callbacks, bus
  locking (`spi_device_acquire_bus`) and command/address/dummy phases. They
  depend on the device's protocol and belong to its driver.

The component exposes:

```c
esp_err_t display_init(void);    // spi_bus_add_device(main_bus_get_host(), ...)
esp_err_t display_deinit(void);  // spi_bus_remove_device: the bus stays
spi_device_handle_t display_get_handle(void);
esp_err_t display_transmit(const void *data, size_t length);
esp_err_t display_transfer(const void *tx_data, void *rx_data, size_t length);
```

Lengths are in **bytes**; ESP-IDF counts bits (`spi_transaction_t.length`), so
the component converts them, and returns `ESP_ERR_INVALID_SIZE` if `length * 8`
would overflow. `display_transfer()` is full duplex: it sends `tx_data` on MOSI
while it receives into `rx_data` from MISO. With `tx_data` NULL nothing is sent
(receive only); with `rx_data` NULL nothing is kept (that is
`display_transmit()`); both NULL is `ESP_ERR_INVALID_ARG`. A length of 0 does
nothing and returns `ESP_OK`. On a bus without MISO, `rx_data` gives
`ESP_ERR_NOT_SUPPORTED`. With DMA, ESP-IDF may write `rx_data` in 4-byte units,
so give it room for `length` rounded up to a multiple of 4.

Lifecycle stays explicit and bottom up. `display_init()` never initializes the
bus: when the bus is not initialized, `spi_bus_add_device` returns
`ESP_ERR_INVALID_STATE`, which is passed on.

```c
ESP_ERROR_CHECK(main_bus_init());
ESP_ERROR_CHECK(display_init());
// transactions...
ESP_ERROR_CHECK(display_deinit());
ESP_ERROR_CHECK(main_bus_deinit()); // fails while devices are attached
```

In CMake the device has `REQUIRES esp_driver_spi` (its header exposes
`spi_device_handle_t`) and `PRIV_REQUIRES <bus>` (only its `.c` includes the
bus header). The bus cannot be called `main`: that is the name of the ESP-IDF
project's own component.

## Sensors

`parrot add sensor <type> <name> --device <i2c-device>` adds a sensor driver
built on an existing I2C device. Nothing below the sensor is created or
changed: add the bus and the device first.

```
environment          sensor-bme280   registers, calibration, compensation
└── environment_device   i2c-device  address 0x76, 400 kHz
    └── sensors          i2c-bus     SDA GPIO21, SCL GPIO22, one controller
```

### BME280

```bash
parrot add i2c sensors --sda 21 --scl 22
parrot add i2c-device environment-device --bus sensors --address 0x76 --frequency 400000
parrot add sensor bme280 environment --device environment-device
```

- **`--device` names an `i2c-device` component**, stored as
  `{"device": "environment_device"}`. The sensor records no address,
  frequency, bus or GPIO: those stay in the device and bus entries, and are
  not passed to its templates either.
- **Address:** a BME280 answers on 0x76 (SDO to GND) or 0x77 (SDO to VDDIO),
  so the device must use one of them. This rule is the sensor's; a generic
  device accepts any valid address.
- **Resources:** the sensor claims no GPIO, controller or address. It claims
  its device: a device has one driver, which owns the chip's state, so a
  second sensor on the same device is rejected.
- **CMake:** `PRIV_REQUIRES <device>` only. The sensor's header needs nothing
  but `esp_err.h`; the device brings the bus and `esp_driver_i2c`.

The generated component exposes:

```c
typedef struct {
    float temperature_c;    // °C
    float pressure_pa;      // Pa
    float humidity_percent; // %RH
} environment_reading_t;

esp_err_t environment_init(void);
esp_err_t environment_read(environment_reading_t *reading);
```

Lifecycle stays explicit, bottom up: `sensors_init()`,
`environment_device_init()`, then `environment_init()`, which never
initializes the layers below it. It reads the chip ID (0x60), soft-resets the
sensor, waits for its NVM copy, reads the calibration coefficients and turns
the IIR filter off. `environment_read()` then runs one forced-mode
measurement (oversampling x1 for temperature, pressure and humidity, as the
data sheet suggests for weather monitoring), sleeps for the maximum
measurement time, polls the status register with a bounded number of
attempts, reads the eight data registers in one burst and applies the data
sheet's integer compensation formulas.

Errors are returned, never `ESP_ERROR_CHECK`ed inside the driver:
`ESP_ERR_INVALID_ARG` for a `NULL` reading, `ESP_ERR_INVALID_STATE` before a
successful init, `ESP_ERR_NOT_FOUND` when the device is not a BME280,
`ESP_ERR_TIMEOUT` when the sensor does not finish, and otherwise the error of
the device transaction that failed.

The driver follows the Bosch BME280 data sheet (BST-BME280-DS001, rev. 1.24).
Its decoding and compensation are tested in C on the host
(`internal/components/sensor/bme280/testdata/driver_test.c`) against Bosch's
worked example and the data sheet's double precision formulas, with a
simulated sensor behind the device functions. `go test` runs these tests when
a C compiler is available (`$CC`, or `cc`, `gcc` or `clang` on `PATH`), and
skips them otherwise.

## Building

`parrot build` runs ESP-IDF for the target in `parrot.json`:

```bash
parrot build   # runs: idf.py -DIDF_TARGET=<target> build
```

- Activate the ESP-IDF environment first (`export.sh`, `export.ps1`, or the EIM
  activation script); Parrot looks for `idf.py` on `PATH` (`idf.py.exe`, the
  launcher ESP-IDF installs, on Windows) and never installs tools.
- The target is passed on the command line instead of running
  `idf.py set-target`, so `sdkconfig` and `build/` are kept. If they were made
  for another target, idf.py stops and tells you to run `idf.py set-target` or
  `idf.py fullclean`.
- idf.py's output is shown live; a failed build makes `parrot build` fail too.

Generated projects ignore `build/`, `sdkconfig` and `sdkconfig.old`, like
Espressif's esp-idf-template. Keep configuration you want versioned in
`sdkconfig.defaults` (`idf.py save-defconfig` writes it).

## Flashing

`parrot flash` writes the project to the device, for the target in
`parrot.json`:

```bash
parrot flash                 # runs: idf.py -DIDF_TARGET=<target> flash
parrot flash --port COM5     # runs: idf.py -DIDF_TARGET=<target> -p COM5 flash
```

- idf.py rebuilds the project first when it is out of date, so there is no
  need to run `parrot build` before.
- Without `--port` (`-p`), idf.py looks for the device's serial port itself.
  The port is passed to idf.py as given; Parrot does not check that it exists.
- The port is not saved in `parrot.json`: it depends on the machine, while
  `parrot.json` describes the project.
- Errors from idf.py and esptool (e.g. a port that cannot be opened) are shown
  as they are, and make `parrot flash` fail.

## Monitoring

`parrot monitor` opens IDF Monitor on the device's serial console:

```bash
parrot monitor               # runs: idf.py -DIDF_TARGET=<target> monitor
parrot monitor --port COM5   # runs: idf.py -DIDF_TARGET=<target> -p COM5 monitor
```

- The terminal belongs to IDF Monitor: Parrot passes it stdin, stdout and
  stderr unchanged and reads no keys itself. Leave with `Ctrl+]`; `Ctrl+T`
  opens IDF Monitor's menu. `Ctrl+C` is IDF Monitor's too (it sends it to the
  device), so Parrot does not stop the monitor on `Ctrl+C`.
- It neither builds nor flashes: run `parrot flash` first. On a project that
  was never built, idf.py configures it (for the target in `parrot.json`) and
  IDF Monitor warns that it cannot decode addresses without the ELF file.
- `--port` works as for `parrot flash`, and is not saved either.
- A non-zero exit status of idf.py makes `parrot monitor` fail. IDF Monitor
  itself exits with 0 when it cannot open the port, after listing the
  available ones.

## Doctor

`parrot doctor` explains what is missing before build, flash or monitor fail.
It works in any directory, and also checks the project when run in one. It
only inspects: it never installs, activates or changes anything.

| Check          | How                                                           | Needs            |
|----------------|---------------------------------------------------------------|------------------|
| idf.py         | on `PATH` (`idf.py.exe` on Windows)                           |                  |
| IDF_PATH       | set, and contains `tools/idf.py`; unset is only a warning     |                  |
| Python         | ESP-IDF's own (`IDF_PYTHON_ENV_PATH`), else `python`/`python3` |                  |
| ESP-IDF        | `python $IDF_PATH/tools/idf.py --version`                     | IDF_PATH, Python |
| CMake, Ninja   | on `PATH`, `--version`                                        |                  |
| ESP-IDF tools  | `python $IDF_PATH/tools/idf_tools.py check`                   | IDF_PATH, Python |
| parrot.json    | readable and valid                                            | a project        |
| CMakeLists.txt | present                                                       | a project        |
| Target         | in Parrot's target registry                                   | parrot.json      |

- `✓` passed, `!` warning, `✗` problem, `-` skipped. A check is skipped when
  what it needs failed; that failure is reported once, by its own check.
- The exit status is non-zero only if there is a problem; warnings and skipped
  checks do not count, so `parrot doctor` can be used in scripts and CI.
- The ESP-IDF version comes from the idf.py script, not from `idf.py.exe`:
  on Windows that launcher answers `--version` with its own version.
- A failed `idf_tools.py check` is a warning: it only finds tools on `PATH` or
  in `$IDF_TOOLS_PATH/tools`, so it reports tools that EIM installed elsewhere
  as missing even when everything works.
- Every command has a time limit: 20 s for version commands, 2 min for
  `idf_tools.py check` (about 10 s, but up to a minute on a cold start).

## Inspect

`parrot inspect` explains the hardware structure of the current project. It
does not pretty-print `parrot.json`: it turns the flat list of components into
trees, and adds what Parrot derives instead of storing.

```
sensors
└── I2C Bus
    ├── SDA: GPIO21
    ├── SCL: GPIO22
    └── Devices
        └── environment_device (I2C Device)
            ├── Address: 0x76
            ├── Frequency: 400000 Hz
            └── Sensor
                └── environment (BME280)
```

- **Hierarchy:** an I2C or SPI device is listed under its bus and a sensor
  under its device, whatever their order in `parrot.json`, and nowhere else.
  Other components are listed at
  the top level, in manifest order, so the output is always the same.
- **Derived information:** the ADC unit and channel of a GPIO come from the
  target registry; the LEDC timer and channel of a PWM output and the host of
  an SPI bus come from the same allocator code generation uses, so inspect
  shows what the generated code contains. The I2C controller of a bus is not
  shown: ESP-IDF picks it.
- **Integrity:** a missing dependency, a dependency of the wrong type, a
  duplicate name, an invalid config, a resource conflict or an unsupported
  target is shown as `ERROR:` on the component and makes the command exit with
  a non-zero status. An unknown component type (e.g. from a newer Parrot) is a
  `WARNING:`. When a component cannot be allocated, the LEDC channels and SPI
  hosts of the components after it are not shown, as they depend on it.
- **Read only:** nothing is written, generated or fixed (an old manifest is not
  migrated), and ESP-IDF is not needed.

The analysis (`inspect.Resolve`) returns a model and prints nothing; a
renderer (`inspect.TextRenderer`) presents it as plain text, without colors, so
it reads the same in a terminal, a file or a CI log.

## parrot.json

`parrot.json` records the user's intent: the platform, the target (the MCU),
optionally the board, and the components with their settings. Hardware
details that Parrot can derive (ADC unit/channel, LEDC timer/channel, SPI
host) are not stored; they are recomputed from the target and the component
order. The I2C controller of a bus is not stored either: ESP-IDF picks it at
run time.

```json
{
  "platform": "esp32",
  "target": "esp32",
  "components": [
    { "type": "led", "name": "status", "config": { "pin": 4 } },
    { "type": "pwm", "name": "fan", "config": { "pin": 18, "frequency": 5000 } }
  ]
}
```

With `parrot new --board <board>`, the board is recorded as well
(`"board": "esp32-c3-devkitm-1"`) and its MCU becomes the target; a target
that does not match the board is rejected. Manifests without `"platform"`
(written before Parrot had platforms) are ESP32 projects, and are not
rewritten to add it.

Older manifests with a top-level `"pin"` per component are still read and are
rewritten in this format on the next `parrot add`.
