#pragma once

#include "freertos/FreeRTOS.h"

// Defined by the test, where it advances the simulated time.
void vTaskDelay(const TickType_t ticks);
