// Host tests of the generated BME280 driver, run by TestDriverC.
//
// TestDriverC generates the driver "environment" on the I2C device
// "environment_device" and compiles this file with stand-ins for the ESP-IDF
// headers (stubs/) and the real generated environment_device.h. This file
// includes environment.c itself, to test its static decoding and compensation
// functions directly, and implements the device's transactions with a
// simulated BME280: a register file, nothing more.
//
// Expected values come from Bosch documents:
//   - the worked example of the BMP280 data sheet (BST-BMP280-DS001, 3.12,
//     "Calculation of pressure and temperature for BMP280"), whose temperature
//     and pressure formulas are those of the BME280 (BME280 data sheet, 5.2:
//     pressure and temperature are identical to the BMP280);
//   - the double precision formulas of the BME280 data sheet (8.1), which the
//     integer formulas are checked against where Bosch gives no example, as
//     for humidity.

#include <stdbool.h>
#include <stdint.h>
#include <stdio.h>
#include <string.h>

#include "environment.c"

static int failures;

#define CHECK(cond, ...)                                       \
    do {                                                       \
        if (!(cond)) {                                         \
            failures++;                                        \
            printf("%s:%d: %s: ", __FILE__, __LINE__, #cond); \
            printf(__VA_ARGS__);                               \
            printf("\n");                                      \
        }                                                      \
    } while (0)

static double absd(double x)
{
    return x < 0 ? -x : x;
}

// --- Reference: double precision formulas of the data sheet (8.1) --------

static double ref_temperature(const bme280_calibration_t *c, int32_t adc_T, int32_t *t_fine)
{
    double var1 = (((double)adc_T) / 16384.0 - ((double)c->dig_T1) / 1024.0) * ((double)c->dig_T2);
    double var2 = ((((double)adc_T) / 131072.0 - ((double)c->dig_T1) / 8192.0) *
                   (((double)adc_T) / 131072.0 - ((double)c->dig_T1) / 8192.0)) *
                  ((double)c->dig_T3);
    *t_fine = (int32_t)(var1 + var2);
    return (var1 + var2) / 5120.0;
}

static double ref_pressure(const bme280_calibration_t *c, int32_t adc_P, int32_t t_fine)
{
    double var1 = ((double)t_fine / 2.0) - 64000.0;
    double var2 = var1 * var1 * ((double)c->dig_P6) / 32768.0;
    var2 = var2 + var1 * ((double)c->dig_P5) * 2.0;
    var2 = (var2 / 4.0) + (((double)c->dig_P4) * 65536.0);
    var1 = (((double)c->dig_P3) * var1 * var1 / 524288.0 + ((double)c->dig_P2) * var1) / 524288.0;
    var1 = (1.0 + var1 / 32768.0) * ((double)c->dig_P1);
    if (var1 == 0.0) {
        return 0;
    }
    double p = 1048576.0 - (double)adc_P;
    p = (p - (var2 / 4096.0)) * 6250.0 / var1;
    var1 = ((double)c->dig_P9) * p * p / 2147483648.0;
    var2 = p * ((double)c->dig_P8) / 32768.0;
    p = p + (var1 + var2 + ((double)c->dig_P7)) / 16.0;
    return p;
}

static double ref_humidity(const bme280_calibration_t *c, int32_t adc_H, int32_t t_fine)
{
    double var_H = (((double)t_fine) - 76800.0);
    var_H = (adc_H - (((double)c->dig_H4) * 64.0 + ((double)c->dig_H5) / 16384.0 * var_H)) *
            (((double)c->dig_H2) / 65536.0 *
             (1.0 + ((double)c->dig_H6) / 67108864.0 * var_H * (1.0 + ((double)c->dig_H3) / 67108864.0 * var_H)));
    var_H = var_H * (1.0 - ((double)c->dig_H1) * var_H / 524288.0);
    if (var_H > 100.0) {
        var_H = 100.0;
    } else if (var_H < 0.0) {
        var_H = 0.0;
    }
    return var_H;
}

// --- Test data -------------------------------------------------------------

// The BMP280 example's coefficients and raw values, plus humidity
// coefficients, which that example does not have: plausible values, used as
// inputs only. Humidity results are checked against the data sheet's double
// formula, not against numbers of ours.
static const bme280_calibration_t example = {
    .dig_T1 = 27504, .dig_T2 = 26435, .dig_T3 = -1000,
    .dig_P1 = 36477, .dig_P2 = -10685, .dig_P3 = 3024, .dig_P4 = 2855, .dig_P5 = 140,
    .dig_P6 = -7, .dig_P7 = 15500, .dig_P8 = -14600, .dig_P9 = 6000,
    .dig_H1 = 75, .dig_H2 = 362, .dig_H3 = 0, .dig_H4 = 313, .dig_H5 = 50, .dig_H6 = 30,
};
#define EXAMPLE_ADC_T 519888 // UT [20 bit]
#define EXAMPLE_ADC_P 415148 // UP [20 bit]
#define EXAMPLE_ADC_H 30000

// The same coefficients as stored in the sensor (Table 16), LSB first.
static const uint8_t example_calib_tp[CALIB_TP_LENGTH] = {
    0x70, 0x6B, // 0x88 dig_T1 = 27504 = 0x6B70
    0x43, 0x67, // 0x8A dig_T2 = 26435 = 0x6743
    0x18, 0xFC, // 0x8C dig_T3 = -1000 = 0xFC18
    0x7D, 0x8E, // 0x8E dig_P1 = 36477 = 0x8E7D
    0x43, 0xD6, // 0x90 dig_P2 = -10685 = 0xD643
    0xD0, 0x0B, // 0x92 dig_P3 = 3024 = 0x0BD0
    0x27, 0x0B, // 0x94 dig_P4 = 2855 = 0x0B27
    0x8C, 0x00, // 0x96 dig_P5 = 140 = 0x008C
    0xF9, 0xFF, // 0x98 dig_P6 = -7 = 0xFFF9
    0x8C, 0x3C, // 0x9A dig_P7 = 15500 = 0x3C8C
    0xF8, 0xC6, // 0x9C dig_P8 = -14600 = 0xC6F8
    0x70, 0x17, // 0x9E dig_P9 = 6000 = 0x1770
    0xA5,       // 0xA0 no coefficient: must be skipped
    0x4B,       // 0xA1 dig_H1 = 75
};
static const uint8_t example_calib_h[CALIB_H_LENGTH] = {
    0x6A, 0x01, // 0xE1 dig_H2 = 362 = 0x016A
    0x00,       // 0xE3 dig_H3 = 0
    0x13,       // 0xE4 dig_H4[11:4]: dig_H4 = 313 = 0x139
    0x29,       // 0xE5 dig_H5[3:0] = 0x2, dig_H4[3:0] = 0x9
    0x03,       // 0xE6 dig_H5[11:4]: dig_H5 = 50 = 0x032
    0x1E,       // 0xE7 dig_H6 = 30
};
// 0xF7..0xFE for the example's raw values.
static const uint8_t example_data[DATA_LENGTH] = {
    0x65, 0x5A, 0xC0, // adc_P = 415148 = 0x655AC
    0x7E, 0xED, 0x00, // adc_T = 519888 = 0x7EED0
    0x75, 0x30,       // adc_H = 30000 = 0x7530
};

// --- Decoding --------------------------------------------------------------

static void test_asr(void)
{
    // Rounds towards minus infinity, like an arithmetic shift.
    CHECK(asr(7, 3) == 0, "asr(7, 3) = %lld", (long long)asr(7, 3));
    CHECK(asr(8, 3) == 1, "asr(8, 3) = %lld", (long long)asr(8, 3));
    CHECK(asr(-1, 3) == -1, "asr(-1, 3) = %lld", (long long)asr(-1, 3));
    CHECK(asr(-8, 3) == -1, "asr(-8, 3) = %lld", (long long)asr(-8, 3));
    CHECK(asr(-9, 3) == -2, "asr(-9, 3) = %lld", (long long)asr(-9, 3));
    CHECK(asr(-6076000, 14) == -371, "asr(-6076000, 14) = %lld", (long long)asr(-6076000, 14));
}

// dig_H4 = 0xE4[7:0] then 0xE5[3:0]; dig_H5 = 0xE6[7:0] then 0xE5[7:4]
// (Table 16); both 12-bit two's complement.
static void test_decode_h4_h5(void)
{
    static const struct {
        uint8_t e4, e5, e6;
        int16_t h4, h5;
        const char *why;
    } cases[] = {
        {0x13, 0x29, 0x03, 313, 50, "0x139 and 0x032"},
        {0x12, 0x5A, 0x34, 0x12A, 0x345, "low nibble of 0xE5 to dig_H4, high nibble to dig_H5"},
        {0x7F, 0xFF, 0x7F, 2047, 2047, "largest: 0x7FF"},
        {0x80, 0x00, 0x80, -2048, -2048, "smallest: 0x800"},
        {0xFF, 0xFF, 0xFF, -1, -1, "0xFFF"},
        {0xF0, 0x0C, 0x00, -244, 0, "0xF0C = 3852 - 4096"},
        {0x00, 0xC0, 0xF0, 0, -244, "0xF0C = 3852 - 4096"},
        {0x00, 0x0F, 0x00, 15, 0, "nibble 0xF is not a sign"},
        {0x00, 0xF0, 0x00, 0, 15, "nibble 0xF is not a sign"},
    };
    for (size_t i = 0; i < sizeof(cases) / sizeof(cases[0]); i++) {
        int16_t h4 = decode_h4(cases[i].e4, cases[i].e5);
        int16_t h5 = decode_h5(cases[i].e5, cases[i].e6);
        CHECK(h4 == cases[i].h4, "E4=0x%02X E5=0x%02X: dig_H4 = %d, want %d (%s)",
              cases[i].e4, cases[i].e5, h4, cases[i].h4, cases[i].why);
        CHECK(h5 == cases[i].h5, "E5=0x%02X E6=0x%02X: dig_H5 = %d, want %d (%s)",
              cases[i].e5, cases[i].e6, h5, cases[i].h5, cases[i].why);
    }
}

static bool same_calibration(const bme280_calibration_t *a, const bme280_calibration_t *b)
{
    return a->dig_T1 == b->dig_T1 && a->dig_T2 == b->dig_T2 && a->dig_T3 == b->dig_T3 &&
           a->dig_P1 == b->dig_P1 && a->dig_P2 == b->dig_P2 && a->dig_P3 == b->dig_P3 &&
           a->dig_P4 == b->dig_P4 && a->dig_P5 == b->dig_P5 && a->dig_P6 == b->dig_P6 &&
           a->dig_P7 == b->dig_P7 && a->dig_P8 == b->dig_P8 && a->dig_P9 == b->dig_P9 &&
           a->dig_H1 == b->dig_H1 && a->dig_H2 == b->dig_H2 && a->dig_H3 == b->dig_H3 &&
           a->dig_H4 == b->dig_H4 && a->dig_H5 == b->dig_H5 && a->dig_H6 == b->dig_H6;
}

static void print_calibration(const char *label, const bme280_calibration_t *c)
{
    printf("  %s: T %u %d %d, P %u %d %d %d %d %d %d %d %d, H %u %d %u %d %d %d\n", label,
           c->dig_T1, c->dig_T2, c->dig_T3, c->dig_P1, c->dig_P2, c->dig_P3, c->dig_P4, c->dig_P5,
           c->dig_P6, c->dig_P7, c->dig_P8, c->dig_P9, c->dig_H1, c->dig_H2, c->dig_H3, c->dig_H4,
           c->dig_H5, c->dig_H6);
}

static void test_parse_calibration(void)
{
    bme280_calibration_t got;
    parse_calibration(example_calib_tp, example_calib_h, &got);
    CHECK(same_calibration(&got, &example), "calibration differs");
    if (!same_calibration(&got, &example)) {
        print_calibration("got ", &got);
        print_calibration("want", &example);
    }

    // Unsigned and signed words with the top bit set; dig_H6 negative.
    uint8_t tp[CALIB_TP_LENGTH] = {0};
    uint8_t h[CALIB_H_LENGTH] = {0};
    tp[0] = 0xFF, tp[1] = 0xFF; // dig_T1: unsigned short
    tp[2] = 0xFF, tp[3] = 0xFF; // dig_T2: signed short
    tp[6] = 0x00, tp[7] = 0x80; // dig_P1: unsigned short
    tp[22] = 0x00, tp[23] = 0x80; // dig_P9: signed short
    tp[25] = 0xFF; // dig_H1: unsigned char
    h[2] = 0xFF;   // dig_H3: unsigned char
    h[6] = 0x80;   // dig_H6: signed char
    parse_calibration(tp, h, &got);
    CHECK(got.dig_T1 == 65535, "dig_T1 = %u", got.dig_T1);
    CHECK(got.dig_T2 == -1, "dig_T2 = %d", got.dig_T2);
    CHECK(got.dig_P1 == 32768, "dig_P1 = %u", got.dig_P1);
    CHECK(got.dig_P9 == -32768, "dig_P9 = %d", got.dig_P9);
    CHECK(got.dig_H1 == 255, "dig_H1 = %u", got.dig_H1);
    CHECK(got.dig_H3 == 255, "dig_H3 = %u", got.dig_H3);
    CHECK(got.dig_H6 == -128, "dig_H6 = %d", got.dig_H6);
}

static void test_parse_raw(void)
{
    bme280_raw_t raw;
    parse_raw(example_data, &raw);
    CHECK(raw.adc_P == EXAMPLE_ADC_P, "adc_P = %ld", (long)raw.adc_P);
    CHECK(raw.adc_T == EXAMPLE_ADC_T, "adc_T = %ld", (long)raw.adc_T);
    CHECK(raw.adc_H == EXAMPLE_ADC_H, "adc_H = %ld", (long)raw.adc_H);

    // Only bits 7:4 of the xlsb registers are data.
    const uint8_t data[DATA_LENGTH] = {0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF};
    parse_raw(data, &raw);
    CHECK(raw.adc_P == 0xFFFFF, "adc_P = 0x%lX", (long)raw.adc_P);
    CHECK(raw.adc_T == 0xFFFFF, "adc_T = 0x%lX", (long)raw.adc_T);
    CHECK(raw.adc_H == 0xFFFF, "adc_H = 0x%lX", (long)raw.adc_H);
}

// --- Compensation ----------------------------------------------------------

// The BMP280 example: t_fine = 128422, T = 25.08 °C and, as integer result,
// 2508; p = 100653.27 Pa, and 25767236 / 256 Pa as 64-bit integer result. The
// example notes that integer results "may deviate slightly" from its values
// through rounding, and computes its pressure from t_fine before the
// conversion to an integer: the 64-bit formula is checked within 0.05 Pa,
// far below the 0.18 Pa resolution of the pressure output (1.3, Table 3).
static void test_compensation_example(void)
{
    int32_t t_fine;
    int32_t t = compensate_temperature(&example, EXAMPLE_ADC_T, &t_fine);
    CHECK(t_fine == 128422, "t_fine = %ld, want 128422", (long)t_fine);
    CHECK(t == 2508, "T = %ld, want 2508", (long)t);

    uint32_t p = compensate_pressure(&example, EXAMPLE_ADC_P, t_fine);
    CHECK(absd(p / 256.0 - 100653.27) <= 0.05, "p = %lu / 256 = %.4f Pa, want 100653.27", (unsigned long)p, p / 256.0);
    CHECK(absd(p / 256.0 - 25767236 / 256.0) <= 0.05, "p = %lu, want 25767236", (unsigned long)p);

    // The double formulas, the reference of the other tests, give the
    // example's values too.
    int32_t ref_t_fine;
    double ref_t = ref_temperature(&example, EXAMPLE_ADC_T, &ref_t_fine);
    CHECK(ref_t_fine == 128422 && absd(ref_t - 25.08) <= 0.005, "reference T = %f, t_fine = %ld", ref_t, (long)ref_t_fine);
    double ref_p = ref_pressure(&example, EXAMPLE_ADC_P, ref_t_fine);
    CHECK(absd(ref_p - 100653.27) <= 0.05, "reference p = %f", ref_p);
}

// The integer formulas against the double ones, over the sensor's ranges:
// -40 to 85 °C, 300 to 1100 hPa, all raw humidity values. A wrong shift, sign
// or coefficient shows as an error of whole units, not of hundredths.
static void test_compensation_matches_double(void)
{
    double worst_t = 0, worst_p = 0, worst_h = 0;
    int temperatures = 0, pressures = 0;
    for (int32_t adc_T = 0; adc_T < (1 << 20); adc_T += 1021) {
        int32_t t_fine, ref_t_fine;
        int32_t t = compensate_temperature(&example, adc_T, &t_fine);
        double ref_t = ref_temperature(&example, adc_T, &ref_t_fine);
        if (ref_t < -40.0 || ref_t > 85.0) {
            continue;
        }
        temperatures++;
        if (absd(t / 100.0 - ref_t) > worst_t) {
            worst_t = absd(t / 100.0 - ref_t);
        }
        // Pressure and humidity both use the integer t_fine, as the driver does.
        for (int32_t adc_P = 0; adc_P < (1 << 20); adc_P += 4093) {
            double ref_p = ref_pressure(&example, adc_P, t_fine);
            if (ref_p < 30000.0 || ref_p > 110000.0) {
                continue;
            }
            pressures++;
            double p = compensate_pressure(&example, adc_P, t_fine) / 256.0;
            if (absd(p - ref_p) > worst_p) {
                worst_p = absd(p - ref_p);
            }
        }
        for (int32_t adc_H = 0; adc_H < (1 << 16); adc_H += 251) {
            double h = compensate_humidity(&example, adc_H, t_fine) / 1024.0;
            double ref_h = ref_humidity(&example, adc_H, t_fine);
            if (absd(h - ref_h) > worst_h) {
                worst_h = absd(h - ref_h);
            }
        }
    }
    CHECK(temperatures > 300 && pressures > 30000, "only %d temperatures and %d pressures in range", temperatures, pressures);
    CHECK(worst_t <= 0.01, "temperature off by %f °C", worst_t);
    CHECK(worst_p <= 0.05, "pressure off by %f Pa", worst_p);
    CHECK(worst_h <= 0.01, "humidity off by %f %%RH", worst_h);
}

// The humidity formula ends by limiting its result to 0..100 %RH
// (419430400 = 100 << 22, in Q22.10 after >> 12).
static void test_humidity_limits(void)
{
    int32_t t_fine;
    compensate_temperature(&example, EXAMPLE_ADC_T, &t_fine);
    uint32_t low = compensate_humidity(&example, 0, t_fine);
    uint32_t high = compensate_humidity(&example, 65535, t_fine);
    CHECK(low == 0, "humidity of raw 0 = %lu", (unsigned long)low);
    CHECK(high == 100 * 1024, "humidity of raw 65535 = %lu, want %d", (unsigned long)high, 100 * 1024);
    CHECK(ref_humidity(&example, 0, t_fine) == 0.0 && ref_humidity(&example, 65535, t_fine) == 100.0,
          "the reference does not reach the limits there");
}

// --- Simulated sensor behind environment_device ---------------------------

#define SIM_RESET_STATUS_READS 2 // im_update reads after a reset
#define SIM_MEASURING_READS    1 // measuring reads after a forced measurement starts

static struct {
    uint8_t regs[256];
    uint8_t measurement[DATA_LENGTH]; // what the next measurement produces
    int im_update_reads;              // status reads still reporting im_update
    int measuring_reads;              // status reads still reporting measuring
    bool stuck;                       // im_update and measuring never clear
    bool added;                       // environment_device_init() was called
    int fail_reg;                     // transactions on this register fail; -1 for none
    esp_err_t fail_with;
    int transactions;
    int receives;     // environment_device_receive() calls: never expected
    int bad_requests; // malformed transactions
    TickType_t ticks; // simulated time
    TickType_t triggered_at;
    long ticks_before_first_poll; // from the last forced trigger; -1 before
    int status_reads;
    struct {
        uint8_t reg, value;
    } writes[64];
    int write_count;
} sim;

static void sim_reset_registers(void)
{
    // Reset states of Table 18.
    sim.regs[REG_CTRL_HUM] = 0x00;
    sim.regs[REG_CTRL_MEAS] = 0x00;
    sim.regs[REG_CONFIG] = 0x00;
    static const uint8_t data_reset[DATA_LENGTH] = {0x80, 0x00, 0x00, 0x80, 0x00, 0x00, 0x80, 0x00};
    memcpy(&sim.regs[REG_DATA], data_reset, DATA_LENGTH);
}

// A powered BME280 with the example's calibration, on an added device.
static void sim_start(void)
{
    memset(&sim, 0, sizeof(sim));
    sim.added = true;
    sim.fail_reg = -1;
    sim.ticks_before_first_poll = -1;
    sim.regs[REG_CHIP_ID] = CHIP_ID;
    memcpy(&sim.regs[REG_CALIB_TP], example_calib_tp, CALIB_TP_LENGTH);
    memcpy(&sim.regs[REG_CALIB_H], example_calib_h, CALIB_H_LENGTH);
    sim_reset_registers();
    memcpy(sim.measurement, example_data, DATA_LENGTH);
    initialized = false;
    memset(&calibration, 0, sizeof(calibration));
}

static void sim_finish_measurement(void)
{
    memcpy(&sim.regs[REG_DATA], sim.measurement, DATA_LENGTH);
    sim.regs[REG_CTRL_MEAS] &= (uint8_t)~0x03; // back to sleep mode
}

static void sim_write(uint8_t reg, uint8_t value)
{
    if (sim.write_count < 64) {
        sim.writes[sim.write_count].reg = reg;
        sim.writes[sim.write_count].value = value;
        sim.write_count++;
    }
    if (reg == REG_RESET) {
        if (value == RESET_COMMAND) {
            sim_reset_registers();
            sim.im_update_reads = SIM_RESET_STATUS_READS;
        }
        return;
    }
    sim.regs[reg] = value;
    uint8_t mode = value & 0x03;
    if (reg == REG_CTRL_MEAS && (mode == 0x01 || mode == 0x02)) {
        sim.triggered_at = sim.ticks;
        sim.ticks_before_first_poll = -1;
        sim.measuring_reads = SIM_MEASURING_READS;
    }
}

static uint8_t sim_read(uint8_t reg)
{
    if (reg != REG_STATUS) {
        return sim.regs[reg];
    }
    sim.status_reads++;
    if (sim.ticks_before_first_poll < 0) {
        sim.ticks_before_first_poll = (long)(sim.ticks - sim.triggered_at);
    }
    uint8_t status = 0;
    if (sim.stuck || sim.im_update_reads > 0) {
        status |= STATUS_IM_UPDATE;
        sim.im_update_reads--;
    }
    if (sim.stuck || sim.measuring_reads > 0) {
        status |= STATUS_MEASURING;
        if (--sim.measuring_reads == 0 && !sim.stuck) {
            sim_finish_measurement();
        }
    }
    return status;
}

static esp_err_t sim_transaction(uint8_t reg)
{
    sim.transactions++;
    if (!sim.added) {
        return ESP_ERR_INVALID_STATE; // as environment_device before its init
    }
    if (sim.fail_reg == reg) {
        return sim.fail_with;
    }
    return ESP_OK;
}

void vTaskDelay(const TickType_t ticks)
{
    sim.ticks += ticks;
}

esp_err_t environment_device_transmit(const uint8_t *data, size_t length, int timeout_ms)
{
    if (data == NULL || length == 0 || length % 2 != 0 || timeout_ms <= 0) {
        sim.bad_requests++;
        return ESP_ERR_INVALID_ARG;
    }
    esp_err_t err = sim_transaction(data[0]);
    if (err != ESP_OK) {
        return err;
    }
    // Pairs of register address and value (6.2.1).
    for (size_t i = 0; i < length; i += 2) {
        sim_write(data[i], data[i + 1]);
    }
    return ESP_OK;
}

esp_err_t environment_device_receive(uint8_t *data, size_t length, int timeout_ms)
{
    (void)data, (void)length, (void)timeout_ms;
    sim.receives++;
    return ESP_FAIL;
}

esp_err_t environment_device_transmit_receive(const uint8_t *write_data, size_t write_length,
                                              uint8_t *read_data, size_t read_length, int timeout_ms)
{
    if (write_data == NULL || write_length != 1 || read_data == NULL || read_length == 0 || timeout_ms <= 0) {
        sim.bad_requests++;
        return ESP_ERR_INVALID_ARG;
    }
    esp_err_t err = sim_transaction(write_data[0]);
    if (err != ESP_OK) {
        return err;
    }
    // The address increments by itself (6.2.2).
    for (size_t i = 0; i < read_length; i++) {
        read_data[i] = sim_read((uint8_t)(write_data[0] + i));
    }
    return ESP_OK;
}

static int sim_writes_to(uint8_t reg)
{
    int n = 0;
    for (int i = 0; i < sim.write_count; i++) {
        n += sim.writes[i].reg == reg;
    }
    return n;
}

// --- Driver ----------------------------------------------------------------

static void test_read_arguments(void)
{
    sim_start();
    environment_reading_t reading;
    esp_err_t err = environment_read(&reading);
    CHECK(err == ESP_ERR_INVALID_STATE, "read before init = 0x%X", err);
    CHECK(sim.transactions == 0, "read before init made %d transactions", sim.transactions);

    CHECK(environment_init() == ESP_OK, "init failed");
    err = environment_read(NULL);
    CHECK(err == ESP_ERR_INVALID_ARG, "read(NULL) = 0x%X", err);
}

static void test_init_and_read(void)
{
    sim_start();
    esp_err_t err = environment_init();
    CHECK(err == ESP_OK, "init = 0x%X", err);
    CHECK(same_calibration(&calibration, &example), "init stored another calibration");
    // A reset, then the configuration; no measurement yet.
    CHECK(sim.write_count == 2, "init wrote %d registers", sim.write_count);
    CHECK(sim.writes[0].reg == REG_RESET && sim.writes[0].value == RESET_COMMAND, "init does not reset first");
    CHECK(sim.writes[1].reg == REG_CONFIG && sim.writes[1].value == 0x00, "config = 0x%02X", sim.writes[1].value);
    CHECK(sim.im_update_reads == 0, "init did not wait for the NVM copy");

    sim.write_count = 0;
    sim.status_reads = 0;
    environment_reading_t reading = {0};
    err = environment_read(&reading);
    CHECK(err == ESP_OK, "read = 0x%X", err);

    // ctrl_hum, then ctrl_meas: osrs_t x1, osrs_p x1, forced mode.
    CHECK(sim.write_count == 2, "read wrote %d registers", sim.write_count);
    CHECK(sim.writes[0].reg == REG_CTRL_HUM && sim.writes[0].value == 0x01, "first write: 0x%02X = 0x%02X",
          sim.writes[0].reg, sim.writes[0].value);
    CHECK(sim.writes[1].reg == REG_CTRL_MEAS && sim.writes[1].value == 0x25, "second write: 0x%02X = 0x%02X",
          sim.writes[1].reg, sim.writes[1].value);
    // The first status read comes after t_measure,max = 9.3 ms (9.1): with
    // vTaskDelay(n) lasting at least n - 1 ticks of 10 ms, n must be >= 2.
    CHECK((sim.ticks_before_first_poll - 1) * 1000 / configTICK_RATE_HZ >= 9.3,
          "first status read after %ld ticks", sim.ticks_before_first_poll);
    CHECK(sim.status_reads == SIM_MEASURING_READS + 1, "%d status reads", sim.status_reads);

    double want_t, want_p, want_h;
    {
        int32_t t_fine;
        want_t = compensate_temperature(&example, EXAMPLE_ADC_T, &t_fine) / 100.0;
        want_p = compensate_pressure(&example, EXAMPLE_ADC_P, t_fine) / 256.0;
        want_h = ref_humidity(&example, EXAMPLE_ADC_H, t_fine);
    }
    CHECK(absd(reading.temperature_c - 25.08) <= 0.0001, "temperature %f °C, want 25.08", reading.temperature_c);
    CHECK(absd(reading.temperature_c - want_t) <= 0.0001, "temperature %f °C, want %f", reading.temperature_c, want_t);
    CHECK(absd(reading.pressure_pa - 100653.27) <= 0.05, "pressure %f Pa, want 100653.27", reading.pressure_pa);
    CHECK(absd(reading.pressure_pa - want_p) <= 0.01, "pressure %f Pa, want %f", reading.pressure_pa, want_p);
    CHECK(absd(reading.humidity_percent - want_h) <= 0.01, "humidity %f %%, want %f", reading.humidity_percent, want_h);

    // Registers only, through write-then-read transactions.
    CHECK(sim.receives == 0 && sim.bad_requests == 0, "%d receives, %d malformed transactions", sim.receives, sim.bad_requests);

    // A second reading triggers a second measurement.
    sim.write_count = 0;
    sim.measurement[0] = 0x66; // adc_P = 0x665AC: a lower pressure
    err = environment_read(&reading);
    CHECK(err == ESP_OK && sim_writes_to(REG_CTRL_MEAS) == 1, "second read = 0x%X", err);
    CHECK(reading.pressure_pa < 100653.0f, "second read did not use the new measurement: %f Pa", reading.pressure_pa);
}

static void test_init_errors(void)
{
    // Something answers at the address, but not a BME280 (0x58 is a BMP280):
    // nothing is written to it.
    sim_start();
    sim.regs[REG_CHIP_ID] = 0x58;
    esp_err_t err = environment_init();
    CHECK(err == ESP_ERR_NOT_FOUND, "init on chip 0x58 = 0x%X", err);
    CHECK(sim.write_count == 0, "init wrote %d registers to a foreign chip", sim.write_count);
    environment_reading_t reading;
    CHECK(environment_read(&reading) == ESP_ERR_INVALID_STATE, "read after a failed init");

    // environment_device_init() not called: its error comes back as is.
    sim_start();
    sim.added = false;
    err = environment_init();
    CHECK(err == ESP_ERR_INVALID_STATE, "init before the device = 0x%X", err);

    // The NVM copy never ends.
    sim_start();
    sim.stuck = true;
    err = environment_init();
    CHECK(err == ESP_ERR_TIMEOUT, "init with im_update stuck = 0x%X", err);
    CHECK(sim.status_reads == STATUS_POLL_ATTEMPTS, "%d status reads", sim.status_reads);

    // Each transaction's error is returned unchanged, and leaves the driver
    // uninitialized.
    static const int regs[] = {REG_CHIP_ID, REG_RESET, REG_STATUS, REG_CALIB_TP, REG_CALIB_H, REG_CONFIG};
    for (size_t i = 0; i < sizeof(regs) / sizeof(regs[0]); i++) {
        sim_start();
        sim.fail_reg = regs[i];
        sim.fail_with = ESP_ERR_INVALID_RESPONSE; // the device NACKed
        err = environment_init();
        CHECK(err == ESP_ERR_INVALID_RESPONSE, "init with register 0x%02X failing = 0x%X", regs[i], err);
        CHECK(environment_read(&reading) == ESP_ERR_INVALID_STATE, "read after init failed on 0x%02X", regs[i]);
    }

    // A failed second init forgets the first one.
    sim_start();
    CHECK(environment_init() == ESP_OK, "first init");
    sim.regs[REG_CHIP_ID] = 0x58;
    CHECK(environment_init() == ESP_ERR_NOT_FOUND, "second init");
    CHECK(environment_read(&reading) == ESP_ERR_INVALID_STATE, "read after a failed second init");
}

static void test_read_errors(void)
{
    environment_reading_t reading = {.temperature_c = -1, .pressure_pa = -1, .humidity_percent = -1};

    // The measurement never ends: bounded polling, with delays between reads.
    sim_start();
    CHECK(environment_init() == ESP_OK, "init");
    sim.stuck = true;
    sim.status_reads = 0;
    TickType_t before = sim.ticks;
    esp_err_t err = environment_read(&reading);
    CHECK(err == ESP_ERR_TIMEOUT, "read with measuring stuck = 0x%X", err);
    CHECK(sim.status_reads == STATUS_POLL_ATTEMPTS, "%d status reads", sim.status_reads);
    CHECK(sim.ticks - before >= (TickType_t)STATUS_POLL_ATTEMPTS, "only %lu ticks of delay", (unsigned long)(sim.ticks - before));
    CHECK(reading.temperature_c == -1 && reading.pressure_pa == -1 && reading.humidity_percent == -1,
          "a failed read wrote the reading");

    static const int regs[] = {REG_CTRL_HUM, REG_CTRL_MEAS, REG_STATUS, REG_DATA};
    for (size_t i = 0; i < sizeof(regs) / sizeof(regs[0]); i++) {
        sim_start();
        CHECK(environment_init() == ESP_OK, "init");
        sim.fail_reg = regs[i];
        sim.fail_with = ESP_ERR_TIMEOUT; // the bus was busy
        err = environment_read(&reading);
        CHECK(err == ESP_ERR_TIMEOUT, "read with register 0x%02X failing = 0x%X", regs[i], err);
        // The failure is the transaction's, not the driver's state.
        sim.fail_reg = -1;
        CHECK(environment_read(&reading) == ESP_OK, "read after a failed transaction on 0x%02X", regs[i]);
    }

    // dig_P1 = 0: the pressure formula would divide by zero (4.2.3).
    sim_start();
    sim.regs[REG_CALIB_TP + 6] = 0;
    sim.regs[REG_CALIB_TP + 7] = 0;
    CHECK(environment_init() == ESP_OK, "init");
    err = environment_read(&reading);
    CHECK(err == ESP_ERR_INVALID_RESPONSE, "read with dig_P1 = 0: 0x%X", err);
}

int main(void)
{
    test_asr();
    test_decode_h4_h5();
    test_parse_calibration();
    test_parse_raw();
    test_compensation_example();
    test_compensation_matches_double();
    test_humidity_limits();
    test_read_arguments();
    test_init_and_read();
    test_init_errors();
    test_read_errors();
    if (failures > 0) {
        printf("%d failure(s)\n", failures);
        return 1;
    }
    printf("ok\n");
    return 0;
}
