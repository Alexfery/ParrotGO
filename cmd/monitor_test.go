package cmd

import (
	"strings"
	"testing"
)

func TestMonitor(t *testing.T) {
	// The cases alternate between an explicit port and none, so a port left
	// over from the previous run would show up as an unexpected -p.
	tests := []struct {
		target   string
		flags    []string
		wantArgs string
		wantOut  string
	}{
		{"esp32", nil, "-DIDF_TARGET=esp32 monitor", "Target: ESP32\nPort: auto\n"},
		{"esp32-c3", []string{"--port", "COM5"}, "-DIDF_TARGET=esp32c3 -p COM5 monitor", "Target: ESP32-C3\nPort: COM5\n"},
		{"esp32-c3", nil, "-DIDF_TARGET=esp32c3 monitor", "Target: ESP32-C3\nPort: auto\n"},
		{"esp32-s3", []string{"-p", "/dev/ttyUSB0"}, "-DIDF_TARGET=esp32s3 -p /dev/ttyUSB0 monitor", "Target: ESP32-S3\nPort: /dev/ttyUSB0\n"},
	}
	for _, tt := range tests {
		dir := newProject(t, tt.target)
		record := installFakeIDF(t)
		// Ctrl+T, then Ctrl+] (exit): the keys must reach IDF Monitor untouched.
		keys := "\x14\x1d"
		out, err := runWithInput(t, keys, append([]string{"monitor"}, tt.flags...)...)
		if err != nil {
			t.Fatalf("%s %q: %v", tt.target, tt.flags, err)
		}

		got := readInvocation(t, record)
		if got.args != tt.wantArgs {
			t.Errorf("%s %q: idf.py %s, want idf.py %s", tt.target, tt.flags, got.args, tt.wantArgs)
		}
		if got.dir != dir {
			t.Errorf("%s %q: idf.py ran in %q, want %q", tt.target, tt.flags, got.dir, dir)
		}
		if got.stdin != keys {
			t.Errorf("%s %q: monitor read %q from the terminal, want %q", tt.target, tt.flags, got.stdin, keys)
		}
		for _, want := range []string{"Parrot monitor\n", tt.wantOut, "Starting serial monitor...", "fake idf.py output"} {
			if !strings.Contains(out, want) {
				t.Errorf("%s %q: output %q does not contain %q", tt.target, tt.flags, out, want)
			}
		}
		// A session is not a task that completes.
		if strings.Contains(out, "✓") {
			t.Errorf("%s %q: output %q reports success", tt.target, tt.flags, out)
		}
	}
}

func TestMonitorFailure(t *testing.T) {
	newProject(t, "esp32")
	installFakeIDF(t)
	t.Setenv("PARROT_FAKE_IDF_EXIT", "2")
	out, err := runOutput(t, "monitor")
	if err == nil || err.Error() != "idf.py -DIDF_TARGET=esp32 monitor: exit status 2" {
		t.Errorf("monitor error = %v, want the idf.py exit status", err)
	}
	if !strings.Contains(out, "fake idf.py output") {
		t.Errorf("output %q does not contain idf.py's output", out)
	}
}
