#pragma once

// Host stand-in for ESP-IDF's FreeRTOS.h: the tick type and rate only.

#include <stdint.h>

typedef uint32_t TickType_t;

// ESP-IDF's default CONFIG_FREERTOS_HZ: one tick every 10 ms.
#define configTICK_RATE_HZ 100
