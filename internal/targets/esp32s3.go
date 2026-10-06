package targets

// esp32S3 describes the ESP32-S3 SoC.
// Sources: ESP32-S3 Series Datasheet v2.2 (Tables 2-4, 2-8, 2-14);
// ESP-IDF components/soc/esp32s3/include/soc/adc_channel.h;
// ESP-IDF soc/esp32s3/include/soc/soc_caps.h (LEDC, SOC_HP_I2C_NUM, SOC_SPI_PERIPH_NUM) and clk_tree_defs.h (LEDC);
// ESP-IDF esp_driver_spi/include/driver/spi_common.h (spi_bus_initialize:
// "SPI0/1 is not supported") and hal/spi_types.h (spi_host_device_t);
// ESP-IDF esp_hal_timg/esp32s3/include/hal/timg_ll.h and timer_ll.h (GPTimer);
// https://docs.espressif.com/projects/esp-idf/en/stable/esp32s3/api-reference/peripherals/spi_master.html
// https://docs.espressif.com/projects/esp-idf/en/stable/esp32s3/api-reference/peripherals/gpio.html
//
// GPIO22-25 do not exist. GPIO33-37 are also used by octal flash/PSRAM on
// some chip variants; that depends on the variant, so they are not reserved
// here. On ESP32-S3R8V/R16V, GPIO47-48 work at 1.8 V instead of 3.3 V.
var esp32S3 = Target{
	ID:          "esp32-s3",
	DisplayName: "ESP32-S3",
	IDFTarget:   "esp32s3",
	LEDC: LEDC{
		Timers: 4, Channels: 8, MaxResolution: 14,
		ClockSource: "LEDC_USE_APB_CLK", ClockHz: 80_000_000,
	},
	I2C:     I2C{HPControllers: 2},
	SPI:     SPI{Hosts: []SPIHost{spi2, spi3}},
	GPTimer: GPTimer{Timers: 4, CounterBits: 54}, // two groups of two
	pins: pinMap(
		withPins(ioPin, 0),
		adcPins(ioPin, 1, 0, span(1, 10)...),  // ADC1 CH0-9
		adcPins(ioPin, 2, 0, span(11, 20)...), // ADC2 CH0-9
		withPins(ioPin, 21),
		withPins(flashPin, span(26, 32)...),
		withPins(ioPin, span(33, 48)...),
	),
}
