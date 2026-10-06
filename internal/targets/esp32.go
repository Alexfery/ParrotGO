package targets

// esp32 describes the original ESP32 SoC.
// Sources: ESP32 Series Datasheet v5.3 (Table 2-1, Section 2.6);
// ESP-IDF components/soc/esp32/include/soc/adc_channel.h;
// ESP-IDF soc/esp32/include/soc/soc_caps.h (LEDC, SOC_HP_I2C_NUM, SOC_SPI_PERIPH_NUM) and clk_tree_defs.h (LEDC);
// ESP-IDF esp_driver_spi/include/driver/spi_common.h (spi_bus_initialize:
// "SPI0/1 is not supported") and hal/spi_types.h (spi_host_device_t);
// ESP-IDF esp_hal_timg/esp32/include/hal/timg_ll.h and timer_ll.h (GPTimer);
// https://docs.espressif.com/projects/esp-idf/en/stable/esp32/api-reference/peripherals/spi_master.html
// https://docs.espressif.com/projects/esp-idf/en/stable/esp32/api-reference/peripherals/gpio.html
//
// GPIO20, GPIO24 and GPIO28-31 do not exist. GPIO34-39 are input only and
// have no internal pull-up/pull-down resistors.
// GPIO16-17 are used by in-package flash/PSRAM on some chip variants
// (e.g. ESP32-U4WDH, ESP32-D0WDRH2-V3) and by PSRAM on some modules
// (e.g. WROVER); that depends on the variant, so they are not reserved here.
var esp32 = Target{
	ID:          "esp32",
	DisplayName: "ESP32",
	IDFTarget:   "esp32",
	LEDC: LEDC{
		Timers: 4, Channels: 8, HighSpeedMode: true, MaxResolution: 20,
		ClockSource: "LEDC_USE_APB_CLK", ClockHz: 80_000_000,
	},
	I2C: I2C{HPControllers: 2},
	// SPI2 (HSPI) and SPI3 (VSPI). SPI2 comes first: with a 32 Mbit PSRAM at
	// 80 MHz, ESP-IDF takes one of them for the PSRAM clock, by default SPI3
	// (CONFIG_SPIRAM_OCCUPY_SPI_HOST), and spi_bus_initialize then reports it
	// as in use.
	SPI: SPI{Hosts: []SPIHost{spi2, spi3}},
	// Two groups of two. esp_timer uses the groups' separate LAC timer, not these.
	GPTimer: GPTimer{Timers: 4, CounterBits: 64},
	pins: pinMap(
		withPins(ioPin, 1, 3, 5, 16, 17, 18, 19, 21, 22, 23),
		withPins(flashPin, span(6, 11)...),
		// ADC1: CH0-3 = GPIO36-39, CH4-5 = GPIO32-33, CH6-7 = GPIO34-35.
		adcPins(inputOnly, 1, 0, 36, 37, 38, 39),
		adcPins(ioPin, 1, 4, 32, 33),
		adcPins(inputOnly, 1, 6, 34, 35),
		// ADC2: CH0-9 in channel order.
		adcPins(ioPin, 2, 0, 4, 0, 2, 15, 13, 12, 14, 27, 25, 26),
	),
}
