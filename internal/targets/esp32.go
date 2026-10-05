package targets

// esp32 describes the original ESP32 SoC.
// Sources: ESP32 Series Datasheet v5.3 (Table 2-1, Section 2.6);
// ESP-IDF components/soc/esp32/include/soc/adc_channel.h;
// ESP-IDF soc/esp32/include/soc/soc_caps.h (LEDC, SOC_HP_I2C_NUM) and clk_tree_defs.h (LEDC);
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
