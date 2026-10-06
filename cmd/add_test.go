package cmd

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// run executes the CLI with args, like the parrot binary would.
func run(t *testing.T, args ...string) error {
	t.Helper()
	_, err := runOutput(t, args...)
	return err
}

// runOutput is like run, but also returns what the CLI printed.
func runOutput(t *testing.T, args ...string) (string, error) {
	t.Helper()
	return runWithInput(t, "", args...)
}

// runWithInput is like runOutput, with input standing for what the user types
// in the terminal. Tests never read the real terminal.
func runWithInput(t *testing.T, input string, args ...string) (string, error) {
	t.Helper()
	resetFlags(rootCmd)
	var out bytes.Buffer
	rootCmd.SetIn(strings.NewReader(input))
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)
	rootCmd.SetArgs(args)
	err := rootCmd.Execute()
	return out.String(), err
}

// resetFlags sets every flag of cmd and its subcommands back to its default.
// The tests share rootCmd, and cobra keeps the flags of the previous Execute,
// whereas each parrot process starts from the defaults.
func resetFlags(cmd *cobra.Command) {
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		f.Value.Set(f.DefValue)
		f.Changed = false
	})
	for _, sub := range cmd.Commands() {
		resetFlags(sub)
	}
}

func mustRun(t *testing.T, args ...string) {
	t.Helper()
	if err := run(t, args...); err != nil {
		t.Fatalf("parrot %v: %v", args, err)
	}
}

func TestAddLEDAndButton(t *testing.T) {
	t.Chdir(t.TempDir())
	mustRun(t, "new", "demo", "--target", "esp32")
	t.Chdir("demo")

	mustRun(t, "add", "led", "status", "--pin", "4")
	mustRun(t, "add", "button", "user", "--pin", "18")

	for _, path := range []string{
		"components/status/status.c",
		"components/status/include/status.h",
		"components/user/user.c",
		"components/user/include/user.h",
		"components/user/CMakeLists.txt",
	} {
		if _, err := os.Stat(path); err != nil {
			t.Errorf("missing %s: %v", path, err)
		}
	}

	data, err := os.ReadFile("parrot.json")
	if err != nil {
		t.Fatal(err)
	}
	want := `{
  "platform": "esp32",
  "target": "esp32",
  "components": [
    {
      "type": "led",
      "name": "status",
      "config": {
        "pin": 4
      }
    },
    {
      "type": "button",
      "name": "user",
      "config": {
        "pin": 18
      }
    }
  ]
}
`
	if string(data) != want {
		t.Errorf("parrot.json =\n%s\nwant\n%s", data, want)
	}

	// Rejected commands must leave parrot.json and components/ untouched.
	rejected := []struct {
		args    []string
		wantErr string
	}{
		{[]string{"add", "button", "conflict", "--pin", "4"}, `GPIO4 is already used by component "status"`},
		{[]string{"add", "button", "user", "--pin", "5"}, `component "user" already exists`},
		{[]string{"add", "button", "far", "--pin", "50"}, "GPIO50 is not available on ESP32"},
		{[]string{"add", "led", "power", "--pin", "34"}, "GPIO34 cannot be used as an output on ESP32"},
	}
	for _, tt := range rejected {
		err := run(t, tt.args...)
		if err == nil || err.Error() != tt.wantErr {
			t.Errorf("parrot %v: error = %v, want %q", tt.args, err, tt.wantErr)
		}
	}
	after, _ := os.ReadFile("parrot.json")
	if string(after) != want {
		t.Errorf("parrot.json changed after rejected commands:\n%s", after)
	}
	for _, dir := range []string{"components/conflict", "components/far", "components/power"} {
		if _, err := os.Stat(dir); err == nil {
			t.Errorf("%s was created by a rejected command", dir)
		}
	}
}

func TestAddOutsideProject(t *testing.T) {
	t.Chdir(t.TempDir())
	err := run(t, "add", "button", "user", "--pin", "18")
	if err == nil || err.Error() != "current directory is not a Parrot project" {
		t.Errorf("error = %v", err)
	}
}

func TestAddADC(t *testing.T) {
	t.Chdir(t.TempDir())
	mustRun(t, "new", "adc-demo", "--target", "esp32")
	t.Chdir("adc-demo")

	mustRun(t, "add", "adc", "light", "--pin", "34")
	mustRun(t, "add", "adc", "pot", "--pin", "35")
	mustRun(t, "add", "led", "status", "--pin", "4")

	for _, path := range []string{
		"components/light/light.c",
		"components/light/include/light.h",
		"components/pot/pot.c",
		"components/parrot_adc/parrot_adc.c",
		"components/parrot_adc/include/parrot_adc.h",
	} {
		if _, err := os.Stat(path); err != nil {
			t.Errorf("missing %s: %v", path, err)
		}
	}

	data, err := os.ReadFile("parrot.json")
	if err != nil {
		t.Fatal(err)
	}
	want := `{
  "platform": "esp32",
  "target": "esp32",
  "components": [
    {
      "type": "adc",
      "name": "light",
      "config": {
        "pin": 34
      }
    },
    {
      "type": "adc",
      "name": "pot",
      "config": {
        "pin": 35
      }
    },
    {
      "type": "led",
      "name": "status",
      "config": {
        "pin": 4
      }
    }
  ]
}
`
	if string(data) != want {
		t.Errorf("parrot.json =\n%s\nwant\n%s", data, want)
	}

	rejected := []struct {
		args    []string
		wantErr string
	}{
		{[]string{"add", "adc", "invalid", "--pin", "18"}, "GPIO18 does not support ADC on ESP32"},
		{[]string{"add", "adc", "again", "--pin", "34"}, `GPIO34 is already used by component "light"`},
		{[]string{"add", "adc", "light", "--pin", "32"}, `component "light" already exists`},
		{[]string{"add", "adc", "far", "--pin", "50"}, "GPIO50 is not available on ESP32"},
		{[]string{"add", "button", "user", "--pin", "34"}, `GPIO34 is already used by component "light"`},
	}
	for _, tt := range rejected {
		err := run(t, tt.args...)
		if err == nil || err.Error() != tt.wantErr {
			t.Errorf("parrot %v: error = %v, want %q", tt.args, err, tt.wantErr)
		}
	}
	after, _ := os.ReadFile("parrot.json")
	if string(after) != want {
		t.Errorf("parrot.json changed after rejected commands:\n%s", after)
	}
}

func TestAddPWM(t *testing.T) {
	t.Chdir(t.TempDir())
	mustRun(t, "new", "pwm-demo", "--target", "esp32")
	t.Chdir("pwm-demo")

	mustRun(t, "add", "pwm", "brightness", "--pin", "4", "--frequency", "5000")
	mustRun(t, "add", "pwm", "fan", "--pin", "18", "--frequency", "5000")
	mustRun(t, "add", "pwm", "buzzer", "--pin", "19", "--frequency", "2000")
	mustRun(t, "add", "led", "status", "--pin", "2")

	// brightness and fan share timer 0; buzzer needs timer 1. Channels follow the order.
	wantDefines := map[string][]string{
		"brightness": {"LEDC_TIMER_0", "LEDC_CHANNEL_0"},
		"fan":        {"LEDC_TIMER_0", "LEDC_CHANNEL_1"},
		"buzzer":     {"LEDC_TIMER_1", "LEDC_CHANNEL_2"},
	}
	for name, defines := range wantDefines {
		data, err := os.ReadFile("components/" + name + "/" + name + ".c")
		if err != nil {
			t.Fatal(err)
		}
		for _, d := range defines {
			if !strings.Contains(string(data), d) {
				t.Errorf("%s.c does not use %s", name, d)
			}
		}
	}

	data, err := os.ReadFile("parrot.json")
	if err != nil {
		t.Fatal(err)
	}
	want := `{
  "platform": "esp32",
  "target": "esp32",
  "components": [
    {
      "type": "pwm",
      "name": "brightness",
      "config": {
        "pin": 4,
        "frequency": 5000
      }
    },
    {
      "type": "pwm",
      "name": "fan",
      "config": {
        "pin": 18,
        "frequency": 5000
      }
    },
    {
      "type": "pwm",
      "name": "buzzer",
      "config": {
        "pin": 19,
        "frequency": 2000
      }
    },
    {
      "type": "led",
      "name": "status",
      "config": {
        "pin": 2
      }
    }
  ]
}
`
	if string(data) != want {
		t.Errorf("parrot.json =\n%s\nwant\n%s", data, want)
	}

	rejected := []struct {
		args    []string
		wantErr string
	}{
		{[]string{"add", "pwm", "motor", "--pin", "2", "--frequency", "5000"}, `GPIO2 is already used by component "status"`},
		{[]string{"add", "pwm", "zero", "--pin", "21", "--frequency", "0"}, "frequency must be positive, got 0 Hz"},
		{[]string{"add", "pwm", "input", "--pin", "34", "--frequency", "5000"}, "GPIO34 cannot be used as an output on ESP32"},
		{[]string{"add", "pwm", "far", "--pin", "50", "--frequency", "5000"}, "GPIO50 is not available on ESP32"},
		{[]string{"add", "pwm", "fast", "--pin", "21", "--frequency", "50000000"}, "50000000 Hz cannot be generated by LEDC on ESP32"},
		{[]string{"add", "pwm", "fan", "--pin", "21", "--frequency", "5000"}, `component "fan" already exists`},
	}
	for _, tt := range rejected {
		err := run(t, tt.args...)
		if err == nil || err.Error() != tt.wantErr {
			t.Errorf("parrot %v: error = %v, want %q", tt.args, err, tt.wantErr)
		}
	}
	after, _ := os.ReadFile("parrot.json")
	if string(after) != want {
		t.Errorf("parrot.json changed after rejected commands:\n%s", after)
	}
}

func TestAddPWMChannelsExhausted(t *testing.T) {
	t.Chdir(t.TempDir())
	mustRun(t, "new", "c3", "--target", "esp32-c3")
	t.Chdir("c3")
	for i, pin := range []string{"0", "1", "2", "3", "4", "5"} {
		mustRun(t, "add", "pwm", "out"+pin, "--pin", pin, "--frequency", fmt.Sprint(1000*(i%4+1)))
	}
	err := run(t, "add", "pwm", "extra", "--pin", "6", "--frequency", "1000")
	if err == nil || err.Error() != "no LEDC channels available on ESP32-C3" {
		t.Errorf("7th PWM on ESP32-C3: error = %v", err)
	}
	if _, err := os.Stat("components/extra"); err == nil {
		t.Error("rejected PWM generated files")
	}
}

func TestAddPWMTimersExhausted(t *testing.T) {
	t.Chdir(t.TempDir())
	mustRun(t, "new", "s3", "--target", "esp32-s3")
	t.Chdir("s3")
	for i, freq := range []string{"1000", "2000", "3000", "4000"} {
		mustRun(t, "add", "pwm", fmt.Sprint("out", i), "--pin", fmt.Sprint(i+1), "--frequency", freq)
	}
	mustRun(t, "add", "pwm", "shared", "--pin", "5", "--frequency", "2000")
	err := run(t, "add", "pwm", "extra", "--pin", "6", "--frequency", "5000")
	if err == nil || !strings.HasPrefix(err.Error(), "no compatible LEDC timer available on ESP32-S3") {
		t.Errorf("5th frequency on ESP32-S3: error = %v", err)
	}
}

// A project created before settings moved into "config" keeps working and is
// rewritten in the new format on the next change.
func TestAddMigratesOldManifest(t *testing.T) {
	t.Chdir(t.TempDir())
	mustRun(t, "new", "old", "--target", "esp32")
	t.Chdir("old")
	old := `{"target": "esp32", "components": [{"type": "led", "name": "status", "pin": 4}]}`
	if err := os.WriteFile("parrot.json", []byte(old), 0o644); err != nil {
		t.Fatal(err)
	}
	err := run(t, "add", "pwm", "fan", "--pin", "4", "--frequency", "5000")
	if err == nil || err.Error() != `GPIO4 is already used by component "status"` {
		t.Errorf("conflict with migrated LED: error = %v", err)
	}
	mustRun(t, "add", "pwm", "fan", "--pin", "5", "--frequency", "5000")
	data, _ := os.ReadFile("parrot.json")
	if !strings.Contains(string(data), `"config": {
        "pin": 4
      }`) {
		t.Errorf("LED was not migrated:\n%s", data)
	}
}
