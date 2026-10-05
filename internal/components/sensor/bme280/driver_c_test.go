package bme280_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"parrot/internal/components/i2cdevice"
	"parrot/internal/components/sensor/bme280"
)

// TestDriverC compiles the generated driver with testdata/driver_test.c and
// runs it on the host: decoding, compensation and the register traffic
// against a simulated BME280 (see that file). It needs a C compiler: $CC,
// else cc, gcc or clang on PATH; without one it is skipped.
func TestDriverC(t *testing.T) {
	compiler := hostCompiler(t)
	dir := t.TempDir()
	device := i2cdevice.Device{Name: "environment_device", Config: i2cdevice.Config{Bus: "sensors", Address: 0x76, Frequency: 400000}}
	if _, err := i2cdevice.Create(dir, device); err != nil {
		t.Fatal(err)
	}
	sensor := bme280.Sensor{Name: "environment", Config: bme280.Config{Device: "environment_device"}}
	if _, err := bme280.Create(dir, sensor); err != nil {
		t.Fatal(err)
	}

	testdata, err := filepath.Abs("testdata")
	if err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(dir, "driver_test")
	if runtime.GOOS == "windows" {
		exe += ".exe"
	}
	components := filepath.Join(dir, "components")
	args := append(compiler[1:],
		"-std=c99", "-pedantic", "-Wall", "-Wextra", "-Werror",
		"-I", filepath.Join(testdata, "stubs"),
		"-I", filepath.Join(components, "environment_device", "include"),
		"-I", filepath.Join(components, "environment", "include"),
		"-I", filepath.Join(components, "environment"), // environment.c, included by driver_test.c
		"-o", exe,
		filepath.Join(testdata, "driver_test.c"),
	)
	if out, err := exec.Command(compiler[0], args...).CombinedOutput(); err != nil {
		t.Fatalf("%s %s: %v\n%s", compiler[0], strings.Join(args, " "), err, out)
	}
	out, err := exec.Command(exe).CombinedOutput()
	if err != nil {
		t.Fatalf("driver tests failed: %v\n%s", err, out)
	}
	t.Logf("%s", out)
}

// hostCompiler returns the command of the host C compiler, or skips the test.
func hostCompiler(t *testing.T) []string {
	t.Helper()
	if cc := strings.Fields(os.Getenv("CC")); len(cc) > 0 {
		return cc
	}
	for _, name := range []string{"cc", "gcc", "clang"} {
		if path, err := exec.LookPath(name); err == nil {
			return []string{path}
		}
	}
	t.Skip("no C compiler: set CC, or put cc, gcc or clang on PATH, to run the C tests of the generated driver")
	return nil
}
