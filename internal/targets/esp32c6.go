package targets

// esp32C6 describes the ESP32-C6 SoC.
// Sources: ESP32-C6 Series Datasheet v1.5 (Tables 2-4, 2-5, 2-9);
// ESP-IDF components/soc/esp32c6/include/soc/adc_channel.h;
// ESP-IDF soc/esp32c6/include/soc/soc_caps.h (LEDC, SOC_HP_I2C_NUM, SOC_SPI_PERIPH_NUM) and clk_tree_defs.h (LEDC);
// ESP-IDF esp_driver_spi/include/driver/spi_common.h (spi_bus_initialize:
// "SPI0/1 is not supported") and hal/spi_types.h (spi_host_device_t);
// https://docs.espressif.com/projects/esp-idf/en/stable/esp32c6/api-reference/peripherals/spi_master.html
// https://docs.espressif.com/projects/esp-idf/en/stable/esp32c6/api-reference/peripherals/gpio.html
//
// No single package exposes all 31 GPIOs: QFN40 lacks GPIO14, and QFN32
// (in-package flash) lacks GPIO10-11 and GPIO24-30. The SoC has them all,
// so they are listed here. GPIO27 is VDD_SPI (flash power) by default.
var esp32C6 = Target{
	ID:          "esp32-c6",
	DisplayName: "ESP32-C6",
	IDFTarget:   "esp32c6",
	LEDC: LEDC{
		Timers: 4, Channels: 6, MaxResolution: 20,
		ClockSource: "LEDC_USE_PLL_DIV_CLK", ClockHz: 80_000_000, // PLL_F80M
	},
	I2C: I2C{HPControllers: 1},       // plus one LP I2C, not used
	SPI: SPI{Hosts: []SPIHost{spi2}}, // SPI2 is the only GP-SPI
	pins: pinMap(
		adcPins(ioPin, 1, 0, span(0, 6)...), // ADC1 CH0-6
		withPins(ioPin, span(7, 23)...),
		withPins(flashPin, span(24, 30)...),
	),
}
