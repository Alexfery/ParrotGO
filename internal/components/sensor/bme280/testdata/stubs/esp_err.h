#pragma once

// Host stand-in for ESP-IDF's esp_err.h (components/esp_common): the same
// type and the codes the BME280 driver uses, with ESP-IDF's values.

typedef int esp_err_t;

#define ESP_OK                   0
#define ESP_FAIL                 -1
#define ESP_ERR_INVALID_ARG      0x102
#define ESP_ERR_INVALID_STATE    0x103
#define ESP_ERR_NOT_FOUND        0x105
#define ESP_ERR_TIMEOUT          0x107
#define ESP_ERR_INVALID_RESPONSE 0x108
