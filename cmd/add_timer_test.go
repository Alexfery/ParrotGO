package cmd

import (
	"os"
	"strings"
	"testing"
)

func TestAddTimer(t *testing.T) {
	t.Chdir(t.TempDir())
	mustRun(t, "new", "timer-demo", "--target", "esp32")
	t.Chdir("timer-demo")

	out, err := runOutput(t, "add", "timer", "heartbeat", "--period", "1s")
	if err != nil {
		t.Fatal(err)
	}
	wantOut := `Adding Timer: heartbeat
Target: ESP32
Period: 1s (1000000 us)
GPTimer: periodic, count up at 1 MHz, alarm at 1000000, auto-reload to 0

✓ Created components/heartbeat
✓ Created components/heartbeat/heartbeat.c
✓ Created components/heartbeat/include/heartbeat.h
✓ Created components/heartbeat/CMakeLists.txt
✓ Updated parrot.json

Timer component added successfully.
`
	if out != wantOut {
		t.Errorf("output =\n%s\nwant\n%s", out, wantOut)
	}
	mustRun(t, "add", "timer", "sensor-tick", "--period", "50ms")
	mustRun(t, "add", "timer", "fast_tick", "--period", "500us")
	mustRun(t, "add", "led", "status", "--pin", "2")          // a timer takes no GPIO
	mustRun(t, "add", "timer", "slow_tick", "--period", "2s") // the ESP32's 4th and last general purpose timer

	data, err := os.ReadFile("parrot.json")
	if err != nil {
		t.Fatal(err)
	}
	want := `{
  "platform": "esp32",
  "target": "esp32",
  "components": [
    {
      "type": "timer",
      "name": "heartbeat",
      "config": {
        "mode": "periodic",
        "period_us": 1000000
      }
    },
    {
      "type": "timer",
      "name": "sensor_tick",
      "config": {
        "mode": "periodic",
        "period_us": 50000
      }
    },
    {
      "type": "timer",
      "name": "fast_tick",
      "config": {
        "mode": "periodic",
        "period_us": 500
      }
    },
    {
      "type": "led",
      "name": "status",
      "config": {
        "pin": 2
      }
    },
    {
      "type": "timer",
      "name": "slow_tick",
      "config": {
        "mode": "periodic",
        "period_us": 2000000
      }
    }
  ]
}
`
	if string(data) != want {
		t.Errorf("parrot.json =\n%s\nwant\n%s", data, want)
	}
	for name, count := range map[string]string{"heartbeat": "1000000ULL", "sensor_tick": "50000ULL", "fast_tick": "500ULL", "slow_tick": "2000000ULL"} {
		data, err := os.ReadFile("components/" + name + "/" + name + ".c")
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(data), "_ALARM_COUNT   "+count) {
			t.Errorf("%s.c does not have an alarm count of %s", name, count)
		}
	}

	// The user's edits to a generated timer must survive a rejected command.
	const edited = "// edited by the user\n"
	if err := os.WriteFile("components/heartbeat/heartbeat.c", []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}

	// Rejected commands must leave parrot.json and components/ untouched.
	rejected := []struct {
		args    []string
		wantErr string
	}{
		{[]string{"add", "timer", "missing"}, `required flag(s) "period" not set`},
		{[]string{"add", "timer", "malformed", "--period", "abc"}, `invalid period "abc": write a duration with its unit, such as 500us, 50ms or 1s`},
		{[]string{"add", "timer", "unitless", "--period", "1000"}, `invalid period "1000": write a duration with its unit, such as 500us, 50ms or 1s`},
		{[]string{"add", "timer", "zero", "--period", "0ms"}, "period must be positive, got 0ms"},
		{[]string{"add", "timer", "negative", "--period", "-10ms"}, "period must be positive, got -10ms"},
		{[]string{"add", "timer", "fraction", "--period", "1500ns"}, "period 1500ns is not a whole number of microseconds: the timer counts in 1 us ticks"},
		{[]string{"add", "timer", "heartbeat", "--period", "1s"}, `component "heartbeat" already exists`},
		{[]string{"add", "timer", "Sensor-Tick", "--period", "10ms"}, `component "sensor_tick" already exists`},
		{[]string{"add", "led", "heartbeat", "--pin", "4"}, `component "heartbeat" already exists`},
		{[]string{"add", "timer", "main", "--period", "1s"}, `invalid component name "main": ESP-IDF projects already have a "main" component (the main folder, with app_main)`},
		{[]string{"add", "timer", "extra", "--period", "1s"}, "no general purpose timers available on ESP32"},
	}
	for _, tt := range rejected {
		err := run(t, tt.args...)
		if err == nil || err.Error() != tt.wantErr {
			t.Errorf("parrot %v: error = %v, want %q", tt.args, err, tt.wantErr)
		}
	}
	after, _ := os.ReadFile("parrot.json")
	if string(after) != want {
		t.Errorf("parrot.json changed after rejected commands:\n%s\nwant\n%s", after, want)
	}
	for _, dir := range []string{"missing", "malformed", "unitless", "zero", "negative", "fraction", "main", "extra"} {
		if _, err := os.Stat("components/" + dir); err == nil {
			t.Errorf("components/%s was created by a rejected command", dir)
		}
	}
	if data, _ := os.ReadFile("components/heartbeat/heartbeat.c"); string(data) != edited {
		t.Errorf("heartbeat.c was overwritten:\n%s", data)
	}
}

// A component folder left without a parrot.json entry is not overwritten
// either: generation refuses to create it again.
func TestAddTimerExistingFolder(t *testing.T) {
	t.Chdir(t.TempDir())
	mustRun(t, "new", "timer-demo", "--target", "esp32")
	t.Chdir("timer-demo")
	if err := os.MkdirAll("components/heartbeat", 0o755); err != nil {
		t.Fatal(err)
	}
	err := run(t, "add", "timer", "heartbeat", "--period", "1s")
	if err == nil || err.Error() != `directory "components/heartbeat" already exists` {
		t.Errorf("error = %v", err)
	}
	data, _ := os.ReadFile("parrot.json")
	if strings.Contains(string(data), "heartbeat") {
		t.Errorf("parrot.json registers a timer that was not generated:\n%s", data)
	}
}

func TestAddTimerHelp(t *testing.T) {
	out, err := runOutput(t, "add", "timer", "--help")
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{
		"Add a periodic hardware timer driven by the GPTimer peripheral.\n",
		"Usage:\n  parrot add timer <name> --period <duration>",
		"parrot add timer heartbeat --period 1s",
		"--period string",
	} {
		if !strings.Contains(out, s) {
			t.Errorf("help does not contain %q:\n%s", s, out)
		}
	}
}
