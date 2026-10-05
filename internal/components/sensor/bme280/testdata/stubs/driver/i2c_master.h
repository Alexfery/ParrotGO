#pragma once

// Host stand-in for ESP-IDF's driver/i2c_master.h, which the generated I2C
// device header includes: only the handle type it exposes.

typedef struct i2c_master_dev_t *i2c_master_dev_handle_t;
