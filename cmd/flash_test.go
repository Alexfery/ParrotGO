package cmd

import (
	"os"
	"strings"
	"testing"
)

func TestFlash(t *testing.T) {
	// The cases alternate between an explicit port and none, so a port left
	// over from the previous run would show up as an unexpected -p.
	tests := []struct {
		target   string
		flags    []string
		wantArgs string
		wantOut  string
	}{
		{"esp32-c3", []string{"--port", "COM5"}, "-DIDF_TARGET=esp32c3 -p COM5 flash", "Target: ESP32-C3\nPort: COM5\n"},
		{"esp32", nil, "-DIDF_TARGET=esp32 flash", "Target: ESP32\nPort: auto\n"},
		{"esp32-s3", []string{"-p", "/dev/ttyUSB0"}, "-DIDF_TARGET=esp32s3 -p /dev/ttyUSB0 flash", "Target: ESP32-S3\nPort: /dev/ttyUSB0\n"},
		{"esp32-c3", nil, "-DIDF_TARGET=esp32c3 flash", "Target: ESP32-C3\nPort: auto\n"},
	}
	for _, tt := range tests {
		dir := newProject(t, tt.target)
		record := installFakeIDF(t)
		out, err := runWithInput(t, "keys typed by the user", append([]string{"flash"}, tt.flags...)...)
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
		if got.stdin != "" {
			t.Errorf("%s %q: flash read %q from the terminal, want no input", tt.target, tt.flags, got.stdin)
		}
		for _, want := range []string{tt.wantOut, "Flashing device...", "fake idf.py output", "✓ Flash completed successfully."} {
			if !strings.Contains(out, want) {
				t.Errorf("%s %q: output %q does not contain %q", tt.target, tt.flags, out, want)
			}
		}
	}
}

func TestFlashFailure(t *testing.T) {
	newProject(t, "esp32")
	installFakeIDF(t)
	t.Setenv("PARROT_FAKE_IDF_EXIT", "2")
	out, err := runOutput(t, "flash", "--port", "COM5")
	if err == nil || err.Error() != "idf.py -DIDF_TARGET=esp32 -p COM5 flash: exit status 2" {
		t.Errorf("flash error = %v, want the idf.py exit status", err)
	}
	// idf.py's own output stays visible: Parrot does not hide or rewrite it.
	for _, want := range []string{"fake idf.py output", "✗ Flash failed."} {
		if !strings.Contains(out, want) {
			t.Errorf("output %q does not contain %q", out, want)
		}
	}
	if strings.Contains(out, "✓") {
		t.Errorf("output %q reports success", out)
	}
}

func TestFlashDoesNotSavePort(t *testing.T) {
	newProject(t, "esp32")
	installFakeIDF(t)
	before, err := os.ReadFile("parrot.json")
	if err != nil {
		t.Fatal(err)
	}
	mustRun(t, "flash", "--port", "COM5")
	after, err := os.ReadFile("parrot.json")
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Errorf("parrot.json changed to:\n%s", after)
	}
}
