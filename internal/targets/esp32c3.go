package targets

// esp32C3 describes the ESP32-C3 SoC.
// Sources: ESP32-C3 Series Datasheet v2.4 (Tables 2-4, 2-6);
// ESP-IDF components/soc/esp32c3/include/soc/adc_channel.h;
// ESP-IDF soc/esp32c3/include/soc/soc_caps.h (LEDC, SOC_HP_I2C_NUM, SOC_SPI_PERIPH_NUM) and clk_tree_defs.h (LEDC);
// ESP-IDF esp_driver_spi/include/driver/spi_common.h (spi_bus_initialize:
// "SPI0/1 is not supported") and hal/spi_types.h (spi_host_device_t);
// https://docs.espressif.com/projects/esp-idf/en/stable/esp32c3/api-reference/peripherals/spi_master.html
// https://docs.espressif.com/projects/esp-idf/en/stable/esp32c3/api-reference/peripherals/gpio.html
//
// GPIO5 is wired to ADC2, but ESP-IDF no longer supports ADC2 oneshot on
// ESP32-C3 (hardware errata), so it gets no ADC mapping here.
var esp32C3 = Target{
	ID:          "esp32-c3",
	DisplayName: "ESP32-C3",
	IDFTarget:   "esp32c3",
	LEDC: LEDC{
		Timers: 4, Channels: 6, MaxResolution: 14,
		ClockSource: "LEDC_USE_APB_CLK", ClockHz: 80_000_000,
	},
	I2C: I2C{HPControllers: 1},
	SPI: SPI{Hosts: []SPIHost{spi2}}, // SPI2 is the only GP-SPI
	pins: pinMap(
		adcPins(ioPin, 1, 0, span(0, 4)...), // ADC1 CH0-4
		withPins(ioPin, span(5, 11)...),
		withPins(flashPin, span(12, 17)...),
		withPins(ioPin, span(18, 21)...),
	),
}
