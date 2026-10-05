package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

// When PARROT_FAKE_IDF names a file, the test binary acts as idf.py: it records
// its working directory, arguments and standard input there, then exits with
// PARROT_FAKE_IDF_EXIT.
func TestMain(m *testing.M) {
	if record := os.Getenv("PARROT_FAKE_IDF"); record != "" {
		wd, _ := os.Getwd()
		input, _ := io.ReadAll(os.Stdin)
		os.WriteFile(record, []byte(wd+"\n"+strings.Join(os.Args[1:], " ")+"\n"+string(input)), 0o644)
		fmt.Println("[1/1] fake idf.py output")
		code, _ := strconv.Atoi(os.Getenv("PARROT_FAKE_IDF_EXIT"))
		os.Exit(code)
	}
	os.Exit(m.Run())
}

// installFakeIDF puts a copy of the test binary on PATH under idf.py's name
// and returns the file where it records its invocation.
func installFakeIDF(t *testing.T) string {
	t.Helper()
	return installFakeTools(t, "idf.py")
}

// installFakeTools makes PATH hold only copies of the test binary, one under
// each name, and returns the file where they record their last invocation.
func installFakeTools(t *testing.T, names ...string) string {
	t.Helper()
	bin := t.TempDir()
	self, err := os.Executable() // absolute, unlike os.Args[0]
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range names {
		if runtime.GOOS == "windows" {
			name += ".exe" // idf.py.exe is the launcher ESP-IDF puts on PATH
		}
		copyFile(t, self, filepath.Join(bin, name))
	}

	record := filepath.Join(t.TempDir(), "invocation")
	t.Setenv("PATH", bin)
	t.Setenv("PARROT_FAKE_IDF", record)
	return record
}

func copyFile(t *testing.T, from, to string) {
	t.Helper()
	src, err := os.Open(from)
	if err != nil {
		t.Fatal(err)
	}
	defer src.Close()
	dst, err := os.OpenFile(to, os.O_CREATE|os.O_WRONLY, 0o755)
	if err != nil {
		t.Fatal(err)
	}
	defer dst.Close()
	if _, err := io.Copy(dst, src); err != nil {
		t.Fatal(err)
	}
}

// invocation is how the fake idf.py was run.
type invocation struct {
	dir   string // working directory, with symlinks resolved
	args  string // arguments, joined by spaces
	stdin string // everything it read from its standard input
}

// readInvocation returns the invocation the fake idf.py recorded in record.
func readInvocation(t *testing.T, record string) invocation {
	t.Helper()
	data, err := os.ReadFile(record)
	if err != nil {
		t.Fatalf("idf.py was not run: %v", err)
	}
	parts := strings.SplitN(string(data), "\n", 3)
	dir, _ := filepath.EvalSymlinks(parts[0])
	return invocation{dir: dir, args: parts[1], stdin: parts[2]}
}

// newProject creates a project in a temporary directory, makes it the
// current directory and returns its path, with symlinks resolved.
func newProject(t *testing.T, target string) string {
	t.Helper()
	t.Chdir(t.TempDir())
	mustRun(t, "new", "app", "--target", target)
	t.Chdir("app")
	wd, _ := os.Getwd()
	dir, _ := filepath.EvalSymlinks(wd)
	return dir
}

func TestBuild(t *testing.T) {
	tests := map[string]string{
		"esp32":    "-DIDF_TARGET=esp32 build",
		"esp32-c3": "-DIDF_TARGET=esp32c3 build",
		"esp32-s3": "-DIDF_TARGET=esp32s3 build",
	}
	for target, wantArgs := range tests {
		dir := newProject(t, target)
		record := installFakeIDF(t)
		if _, err := runWithInput(t, "keys typed by the user", "build"); err != nil {
			t.Fatalf("%s: %v", target, err)
		}

		got := readInvocation(t, record)
		if got.args != wantArgs {
			t.Errorf("%s: idf.py %s, want idf.py %s", target, got.args, wantArgs)
		}
		if got.dir != dir {
			t.Errorf("%s: idf.py ran in %q, want %q", target, got.dir, dir)
		}
		if got.stdin != "" {
			t.Errorf("%s: build read %q from the terminal, want no input", target, got.stdin)
		}
	}
}

func TestBuildFailure(t *testing.T) {
	newProject(t, "esp32")
	installFakeIDF(t)
	t.Setenv("PARROT_FAKE_IDF_EXIT", "2")
	err := run(t, "build")
	if err == nil || err.Error() != "idf.py -DIDF_TARGET=esp32 build: exit status 2" {
		t.Errorf("build error = %v, want the idf.py exit status", err)
	}
}

// TestIDFCommandErrors checks the errors build, flash and monitor report before
// idf.py runs. In every case idf.py must not be started.
func TestIDFCommandErrors(t *testing.T) {
	notRun := func(t *testing.T, record string) {
		t.Helper()
		if _, err := os.Stat(record); err == nil {
			t.Error("idf.py was run")
		}
	}
	for _, command := range []string{"build", "flash", "monitor"} {
		t.Run(command+"/not a Parrot project", func(t *testing.T) {
			t.Chdir(t.TempDir())
			record := installFakeIDF(t)
			err := run(t, command)
			if err == nil || err.Error() != "current directory is not a Parrot project" {
				t.Errorf("error = %v", err)
			}
			notRun(t, record)
		})
		t.Run(command+"/no CMakeLists.txt", func(t *testing.T) {
			newProject(t, "esp32")
			record := installFakeIDF(t)
			os.Remove("CMakeLists.txt")
			err := run(t, command)
			if err == nil || err.Error() != "invalid ESP-IDF project: CMakeLists.txt not found" {
				t.Errorf("error = %v", err)
			}
			notRun(t, record)
		})
		t.Run(command+"/invalid target", func(t *testing.T) {
			newProject(t, "esp32")
			record := installFakeIDF(t)
			os.WriteFile("parrot.json", []byte(`{"target": "banana"}`), 0o644)
			err := run(t, command)
			if err == nil || !strings.HasPrefix(err.Error(), `unsupported target "banana"`) {
				t.Errorf("error = %v", err)
			}
			notRun(t, record)
		})
		t.Run(command+"/idf.py not found", func(t *testing.T) {
			newProject(t, "esp32")
			t.Setenv("PATH", t.TempDir())
			err := run(t, command)
			if err == nil || !strings.HasPrefix(err.Error(), "idf.py was not found") {
				t.Errorf("error = %v", err)
			}
		})
	}
}
