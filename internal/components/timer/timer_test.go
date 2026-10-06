package timer_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"parrot/internal/components/timer"
	"parrot/internal/targets"
)

func mustTarget(t *testing.T, id string) targets.Target {
	t.Helper()
	target, err := targets.Get(id)
	if err != nil {
		t.Fatal(err)
	}
	return target
}

func TestParsePeriod(t *testing.T) {
	tests := []struct {
		text string
		want uint64 // microseconds
	}{
		{"1s", 1_000_000},
		{"1000ms", 1_000_000},
		{"50ms", 50_000},
		{"500us", 500},
		{"500µs", 500},
		{"1us", 1},
		{"1.5ms", 1500},
		{"2000ns", 2},
		{"1m", 60_000_000},
	}
	for _, tt := range tests {
		got, err := timer.ParsePeriod(tt.text)
		if err != nil || got != tt.want {
			t.Errorf("ParsePeriod(%q) = %d, %v; want %d", tt.text, got, err, tt.want)
		}
	}
}

// Invalid periods are rejected, never rounded or corrected.
func TestParsePeriodErrors(t *testing.T) {
	tests := []struct {
		text    string
		wantErr string
	}{
		{"", `invalid period "": write a duration with its unit, such as 500us, 50ms or 1s`},
		{"abc", `invalid period "abc": write a duration with its unit, such as 500us, 50ms or 1s`},
		{"1000", `invalid period "1000": write a duration with its unit, such as 500us, 50ms or 1s`},
		{"0ms", "period must be positive, got 0ms"},
		{"0", "period must be positive, got 0"},
		{"-10ms", "period must be positive, got -10ms"},
		{"500ns", "period 500ns is not a whole number of microseconds: the timer counts in 1 us ticks"},
		{"1500ns", "period 1500ns is not a whole number of microseconds: the timer counts in 1 us ticks"},
	}
	for _, tt := range tests {
		got, err := timer.ParsePeriod(tt.text)
		if err == nil || err.Error() != tt.wantErr {
			t.Errorf("ParsePeriod(%q) = %d, %v; want error %q", tt.text, got, err, tt.wantErr)
		}
	}
}

func TestFormatPeriod(t *testing.T) {
	tests := map[uint64]string{
		1_000_000:  "1s",
		60_000_000: "60s",
		50_000:     "50ms",
		1_500_000:  "1500ms",
		500:        "500us",
		1500:       "1500us",
	}
	for us, want := range tests {
		if got := timer.FormatPeriod(us); got != want {
			t.Errorf("FormatPeriod(%d) = %q, want %q", us, got, want)
		}
		// What it writes, --period reads back.
		if back, err := timer.ParsePeriod(want); err != nil || back != us {
			t.Errorf("ParsePeriod(%q) = %d, %v; want %d", want, back, err, us)
		}
	}
}

func TestNew(t *testing.T) {
	tm, err := timer.New("Sensor-Tick", 50_000)
	if err != nil {
		t.Fatal(err)
	}
	want := timer.Timer{Name: "sensor_tick", Config: timer.Config{Mode: "periodic", PeriodUS: 50_000}}
	if tm != want {
		t.Errorf("New = %+v, want %+v", tm, want)
	}
	if tm.AlarmCount() != 50_000 {
		t.Errorf("AlarmCount = %d, want 50000 (one tick per microsecond)", tm.AlarmCount())
	}
	if _, err := timer.New("tick", 0); err == nil || err.Error() != "period must be positive" {
		t.Errorf("New with period 0: error = %v", err)
	}
	if _, err := timer.New("main", 1000); err == nil {
		t.Error("New accepted the name main")
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		target  string
		config  timer.Config
		wantErr string // empty: valid
	}{
		{"esp32", timer.Config{Mode: "periodic", PeriodUS: 1_000_000}, ""},
		{"esp32", timer.Config{Mode: "periodic", PeriodUS: 1<<64 - 1}, ""}, // 64-bit counter
		{"esp32-c3", timer.Config{Mode: "periodic", PeriodUS: 1<<54 - 1}, ""},
		{"esp32-c3", timer.Config{Mode: "periodic", PeriodUS: 1 << 54}, "period of 18014398509481984 us does not fit in the 54-bit counter of the timers of ESP32-C3"},
		{"esp32", timer.Config{Mode: "periodic", PeriodUS: 0}, "period must be positive"},
		{"esp32", timer.Config{Mode: "one-shot", PeriodUS: 1000}, `unsupported timer mode "one-shot": Parrot only generates "periodic" timers`},
		{"esp32", timer.Config{PeriodUS: 1000}, `unsupported timer mode "": Parrot only generates "periodic" timers`},
	}
	for _, tt := range tests {
		err := timer.Validate(mustTarget(t, tt.target), tt.config)
		switch {
		case tt.wantErr == "" && err != nil:
			t.Errorf("Validate(%s, %+v) = %v, want nil", tt.target, tt.config, err)
		case tt.wantErr != "" && (err == nil || err.Error() != tt.wantErr):
			t.Errorf("Validate(%s, %+v) = %v, want %q", tt.target, tt.config, err, tt.wantErr)
		}
	}

	soc := targets.Target{DisplayName: "Test SoC"} // describes no timer
	err := timer.Validate(soc, timer.Config{Mode: "periodic", PeriodUS: 1000})
	if want := "Test SoC has no general purpose timer"; err == nil || err.Error() != want {
		t.Errorf("Validate on a SoC without timers = %v, want %q", err, want)
	}
}

// A timer takes a general purpose timer and no GPIO.
func TestNeeds(t *testing.T) {
	needs, err := timer.Config{Mode: "periodic", PeriodUS: 1000}.Needs(mustTarget(t, "esp32"))
	if err != nil {
		t.Fatal(err)
	}
	if !needs.GPTimer || len(needs.GPIOs) != 0 || needs.LEDC != nil {
		t.Errorf("Needs = %+v, want a general purpose timer only", needs)
	}
	if _, err := (timer.Config{Mode: "periodic"}).Needs(mustTarget(t, "esp32")); err == nil {
		t.Error("Needs accepted a zero period")
	}
}

func TestCreate(t *testing.T) {
	dir := t.TempDir()
	tm := timer.Timer{Name: "heartbeat", Config: timer.Config{Mode: "periodic", PeriodUS: 1_000_000}}
	created, err := timer.Create(dir, tm)
	if err != nil {
		t.Fatal(err)
	}
	if len(created) != 4 {
		t.Fatalf("created %d paths, want 4: %v", len(created), created)
	}

	base := filepath.Join(dir, "components", "heartbeat")
	contains := map[string][]string{
		"heartbeat.c": {
			`#include "driver/gptimer.h"`,
			`#include "esp_attr.h"`,
			`#include "esp_err.h"`,
			`#include "heartbeat.h"`,
			"#define HEARTBEAT_RESOLUTION_HZ 1000000\n",
			"#define HEARTBEAT_ALARM_COUNT   1000000ULL\n",
			"static gptimer_handle_t heartbeat_timer = NULL;",
			"static bool IRAM_ATTR heartbeat_on_alarm(gptimer_handle_t timer, const gptimer_alarm_event_data_t *edata, void *user_ctx)",
			"// Add ISR-safe timer handling here.",
			".clk_src = GPTIMER_CLK_SRC_DEFAULT,",
			".direction = GPTIMER_COUNT_UP,",
			".resolution_hz = HEARTBEAT_RESOLUTION_HZ,",
			"ESP_ERROR_CHECK(gptimer_new_timer(&timer_config, &heartbeat_timer));",
			".alarm_count = HEARTBEAT_ALARM_COUNT,",
			".reload_count = 0,",
			".flags.auto_reload_on_alarm = true,",
			"ESP_ERROR_CHECK(gptimer_set_alarm_action(heartbeat_timer, &alarm_config));",
			".on_alarm = heartbeat_on_alarm,",
			"ESP_ERROR_CHECK(gptimer_register_event_callbacks(heartbeat_timer, &callbacks, NULL));",
			"ESP_ERROR_CHECK(gptimer_enable(heartbeat_timer));",
			"ESP_ERROR_CHECK(gptimer_start(heartbeat_timer));",
			"ESP_ERROR_CHECK(gptimer_stop(heartbeat_timer));",
		},
		"include/heartbeat.h": {
			"#pragma once",
			"raises an alarm every 1s",
			"void heartbeat_init(void);",
			"void heartbeat_start(void);",
			"void heartbeat_stop(void);",
		},
		"CMakeLists.txt": {`SRCS "heartbeat.c"`, "PRIV_REQUIRES esp_driver_gptimer"},
	}
	for file, snippets := range contains {
		data, err := os.ReadFile(filepath.Join(base, file))
		if err != nil {
			t.Fatal(err)
		}
		for _, s := range snippets {
			if !strings.Contains(string(data), s) {
				t.Errorf("%s does not contain %q", file, s)
			}
		}
	}

	// The lifecycle runs in the order the GPTimer driver requires: callbacks
	// are registered before the timer is enabled, and it is enabled in init
	// so that start only starts it.
	data, _ := os.ReadFile(filepath.Join(base, "heartbeat.c"))
	order := []string{"gptimer_new_timer(", "gptimer_set_alarm_action(", "gptimer_register_event_callbacks(", "gptimer_enable(", "gptimer_start("}
	last := -1
	for _, call := range order {
		i := strings.Index(string(data), call)
		if i < last {
			t.Errorf("%s is called out of order in heartbeat.c", call)
		}
		last = i
	}

	// Only the modern driver: no legacy timer API.
	for _, legacy := range []string{"driver/timer.h", "timer_init(", "timer_group_", "TIMER_GROUP_"} {
		if strings.Contains(string(data), legacy) {
			t.Errorf("heartbeat.c uses the legacy timer API: %q", legacy)
		}
	}
}

// An existing component folder is never overwritten.
func TestCreateExisting(t *testing.T) {
	dir := t.TempDir()
	tm := timer.Timer{Name: "heartbeat", Config: timer.Config{Mode: "periodic", PeriodUS: 1_000_000}}
	if _, err := timer.Create(dir, tm); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "components", "heartbeat", "heartbeat.c")
	if err := os.WriteFile(path, []byte("// edited by the user\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	tm.PeriodUS = 50_000
	if _, err := timer.Create(dir, tm); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Errorf("second Create: error = %v, want already exists", err)
	}
	if data, _ := os.ReadFile(path); string(data) != "// edited by the user\n" {
		t.Errorf("heartbeat.c was overwritten:\n%s", data)
	}
}
