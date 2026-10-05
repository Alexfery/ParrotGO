package doctor_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"parrot/internal/doctor"
	"parrot/internal/espidf"
)

// fakeSystem is a machine whose PATH, environment and commands are tables.
type fakeSystem struct {
	idf     string                // what FindIDF returns; "" when idf.py is not on PATH
	paths   map[string]string     // executables on PATH
	env     map[string]string     // environment variables
	outputs map[string]fakeOutput // what each command line prints
	hang    string                // a command line that runs until its time limit
}

type fakeOutput struct {
	out string
	err error // e.g. a non-zero exit status
}

func commandLine(name string, args ...string) string {
	return strings.Join(append([]string{name}, args...), " ")
}

func (s *fakeSystem) FindIDF() (string, error) {
	if s.idf == "" {
		return "", espidf.ErrIDFNotFound
	}
	return s.idf, nil
}

func (s *fakeSystem) LookPath(name string) (string, error) {
	if path, found := s.paths[name]; found {
		return path, nil
	}
	return "", exec.ErrNotFound
}

func (s *fakeSystem) Getenv(key string) string {
	return s.env[key]
}

func (s *fakeSystem) Capture(ctx context.Context, name string, args ...string) (string, error) {
	line := commandLine(name, args...)
	if line == s.hang {
		<-ctx.Done()
		return "", ctx.Err()
	}
	output, found := s.outputs[line]
	if !found {
		return "", fmt.Errorf("exec: %q: file does not exist", name)
	}
	return output.out, output.err
}

// Paths of the healthy machine's tools.
const (
	cmakePath = "/tools/cmake"
	ninjaPath = "/tools/ninja"
	venv      = "/tools/python-env"
)

// healthy returns a machine with an activated ESP-IDF environment in which
// every check passes, and its ESP-IDF directory.
func healthy(t *testing.T) (*fakeSystem, string) {
	t.Helper()
	idfPath := t.TempDir()
	if err := os.MkdirAll(filepath.Join(idfPath, "tools"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(espidf.IDFScript(idfPath), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	python := espidf.VenvPython(venv)
	return &fakeSystem{
		idf:   "/tools/idf.py",
		paths: map[string]string{"cmake": cmakePath, "ninja": ninjaPath},
		env:   map[string]string{espidf.IDFPathVar: idfPath, espidf.PythonEnvVar: venv},
		outputs: map[string]fakeOutput{
			commandLine(python, "--version"):                       {out: "Python 3.14.7\n"},
			commandLine(python, espidf.VersionArgs(idfPath)...):    {out: "ESP-IDF v6.1\n"},
			commandLine(python, espidf.ToolsCheckArgs(idfPath)...): {out: "Checking for installed tools...\n"},
			commandLine(cmakePath, "--version"):                    {out: "cmake version 4.0.3\n\nCMake suite maintained and supported by Kitware.\n"},
			commandLine(ninjaPath, "--version"):                    {out: "1.12.1\n"},
		},
	}, idfPath
}

func diagnose(s doctor.System, projectDir string) doctor.Report {
	return doctor.Doctor{System: s, ProjectDir: projectDir}.Run(context.Background())
}

// find returns the result of the check called name.
func find(t *testing.T, report doctor.Report, name string) doctor.CheckResult {
	t.Helper()
	for _, section := range report.Sections {
		for _, result := range section.Results {
			if result.Name == name {
				return result
			}
		}
	}
	t.Fatalf("no %q check in %+v", name, report)
	return doctor.CheckResult{}
}

// expect checks the status of a check and that its message contains want.
func expect(t *testing.T, report doctor.Report, name string, status doctor.Status, want string) {
	t.Helper()
	result := find(t, report, name)
	if result.Status != status {
		t.Errorf("%s: status %d, want %d (message %q)", name, result.Status, status, result.Message)
	}
	if !strings.Contains(result.Message, want) {
		t.Errorf("%s: message %q does not contain %q", name, result.Message, want)
	}
}

func TestAllChecksPass(t *testing.T) {
	s, idfPath := healthy(t)
	report := diagnose(s, t.TempDir())

	expect(t, report, "Parrot CLI", doctor.StatusOK, "")
	expect(t, report, "idf.py", doctor.StatusOK, "/tools/idf.py")
	expect(t, report, "IDF_PATH", doctor.StatusOK, idfPath)
	expect(t, report, "Python", doctor.StatusOK, "Python 3.14.7\n"+espidf.VenvPython(venv)+" (ESP-IDF Python environment)")
	expect(t, report, "ESP-IDF", doctor.StatusOK, "ESP-IDF v6.1")
	expect(t, report, "CMake", doctor.StatusOK, "cmake version 4.0.3\n"+cmakePath)
	expect(t, report, "Ninja", doctor.StatusOK, "1.12.1\n"+ninjaPath)
	expect(t, report, "ESP-IDF tools", doctor.StatusOK, "")
	if n := report.Count(doctor.StatusError) + report.Count(doctor.StatusWarning); n != 0 {
		t.Errorf("%d errors and warnings, want none: %+v", n, report)
	}
}

// A failed check does not stop the independent ones.
func TestIDFMissing(t *testing.T) {
	s, _ := healthy(t)
	s.idf = ""
	report := diagnose(s, t.TempDir())

	expect(t, report, "idf.py", doctor.StatusError, "Activate the ESP-IDF environment")
	for _, name := range []string{"IDF_PATH", "Python", "ESP-IDF", "CMake", "Ninja", "ESP-IDF tools"} {
		expect(t, report, name, doctor.StatusOK, "")
	}
	if n := report.Count(doctor.StatusError); n != 1 {
		t.Errorf("%d errors, want 1", n)
	}
}

func TestPythonMissing(t *testing.T) {
	s, _ := healthy(t)
	delete(s.env, espidf.PythonEnvVar)
	report := diagnose(s, t.TempDir())

	expect(t, report, "Python", doctor.StatusError, "Python was not found on PATH")
	// The checks that need Python are skipped, not counted as more problems.
	expect(t, report, "ESP-IDF", doctor.StatusSkipped, "requires a working Python")
	expect(t, report, "ESP-IDF tools", doctor.StatusSkipped, "requires a working Python")
	expect(t, report, "CMake", doctor.StatusOK, "")
	if n := report.Count(doctor.StatusError); n != 1 {
		t.Errorf("%d errors, want 1", n)
	}
}

// Outside an ESP-IDF environment, the first Python on PATH that runs is used:
// a broken one, like the Microsoft Store stub, is passed over.
func TestPythonFromPath(t *testing.T) {
	s, idfPath := healthy(t)
	delete(s.env, espidf.PythonEnvVar)
	s.paths["python"] = "/stub/python"
	s.paths["python3"] = "/usr/bin/python3"
	s.outputs[commandLine("/stub/python", "--version")] = fakeOutput{err: errors.New("exit status 9009")}
	s.outputs[commandLine("/usr/bin/python3", "--version")] = fakeOutput{out: "Python 3.12.10\n"}
	s.outputs[commandLine("/usr/bin/python3", espidf.VersionArgs(idfPath)...)] = fakeOutput{out: "ESP-IDF v6.1\n"}
	report := diagnose(s, t.TempDir())

	expect(t, report, "Python", doctor.StatusOK, "Python 3.12.10\n/usr/bin/python3")
	expect(t, report, "ESP-IDF", doctor.StatusOK, "ESP-IDF v6.1")
}

// In an ESP-IDF environment, only its own Python matters: another one on PATH
// does not make up for it.
func TestBrokenESPIDFPython(t *testing.T) {
	s, _ := healthy(t)
	delete(s.outputs, commandLine(espidf.VenvPython(venv), "--version"))
	s.paths["python"] = "/usr/bin/python"
	s.outputs[commandLine("/usr/bin/python", "--version")] = fakeOutput{out: "Python 3.12.10\n"}
	report := diagnose(s, t.TempDir())

	expect(t, report, "Python", doctor.StatusError, "The ESP-IDF Python environment ("+venv+") does not work")
	expect(t, report, "ESP-IDF", doctor.StatusSkipped, "requires a working Python")
}

func TestToolMissing(t *testing.T) {
	for _, tool := range []struct{ name, executable string }{{"CMake", "cmake"}, {"Ninja", "ninja"}} {
		s, _ := healthy(t)
		delete(s.paths, tool.executable)
		report := diagnose(s, t.TempDir())

		expect(t, report, tool.name, doctor.StatusError, tool.name+" was not found on PATH.")
		if n := report.Count(doctor.StatusError); n != 1 {
			t.Errorf("%s missing: %d errors, want 1", tool.name, n)
		}
	}
}

func TestVersionCommandFails(t *testing.T) {
	s, _ := healthy(t)
	s.outputs[commandLine(cmakePath, "--version")] = fakeOutput{
		out: "cmake: error while loading shared libraries\n",
		err: errors.New("exit status 127"),
	}
	report := diagnose(s, t.TempDir())

	expect(t, report, "CMake", doctor.StatusError, "cmake --version failed (exit status 127).\ncmake: error while loading shared libraries")
}

func TestIDFPathMissing(t *testing.T) {
	s, _ := healthy(t)
	delete(s.env, espidf.IDFPathVar)
	report := diagnose(s, t.TempDir())

	expect(t, report, "IDF_PATH", doctor.StatusWarning, "IDF_PATH is not set, but idf.py is available.")
	expect(t, report, "ESP-IDF", doctor.StatusSkipped, "requires a valid IDF_PATH")
	expect(t, report, "ESP-IDF tools", doctor.StatusSkipped, "requires a valid IDF_PATH")
	if n := report.Count(doctor.StatusError); n != 0 {
		t.Errorf("%d errors, want none: a missing IDF_PATH is a warning", n)
	}
}

func TestIDFPathNotESPIDF(t *testing.T) {
	s, _ := healthy(t)
	dir := t.TempDir()
	s.env[espidf.IDFPathVar] = dir
	report := diagnose(s, t.TempDir())

	expect(t, report, "IDF_PATH", doctor.StatusError, espidf.IDFScript(dir)+" was not found")
	expect(t, report, "ESP-IDF tools", doctor.StatusSkipped, "requires a valid IDF_PATH")
}

// The version line is found among the other lines idf.py may print.
func TestIDFVersionAmongWarnings(t *testing.T) {
	s, idfPath := healthy(t)
	s.outputs[commandLine(espidf.VenvPython(venv), espidf.VersionArgs(idfPath)...)] = fakeOutput{
		out: "WARNING: Python dependencies were not checked\nESP-IDF v6.1-dirty\n",
	}
	report := diagnose(s, t.TempDir())

	expect(t, report, "ESP-IDF", doctor.StatusOK, "ESP-IDF v6.1-dirty")
}

// A failed tools check is a warning, summarized to ESP-IDF's error lines.
func TestToolsCheckFails(t *testing.T) {
	s, idfPath := healthy(t)
	s.outputs[commandLine(espidf.VenvPython(venv), espidf.ToolsCheckArgs(idfPath)...)] = fakeOutput{
		out: "ERROR: The following required tools were not found: xtensa-esp-elf-gdb esp-rom-elfs\n" +
			"Checking for installed tools...\nChecking tool xtensa-esp-elf-gdb\n    no version found in PATH\n",
		err: errors.New("exit status 1"),
	}
	report := diagnose(s, t.TempDir())

	result := find(t, report, "ESP-IDF tools")
	expect(t, report, "ESP-IDF tools", doctor.StatusWarning, "python")
	if !strings.Contains(result.Message, "ERROR: The following required tools were not found: xtensa-esp-elf-gdb esp-rom-elfs") {
		t.Errorf("message %q does not name the missing tools", result.Message)
	}
	if strings.Contains(result.Message, "Checking tool") {
		t.Errorf("message %q repeats the whole output", result.Message)
	}
	if n := report.Count(doctor.StatusError); n != 0 {
		t.Errorf("%d errors, want none", n)
	}
}

// A command that does not finish is stopped at its time limit, reported, and
// does not delay the other checks.
func TestCommandTimeout(t *testing.T) {
	s, _ := healthy(t)
	s.hang = commandLine(cmakePath, "--version")
	d := doctor.Doctor{System: s, ProjectDir: t.TempDir(), CommandTimeout: 50 * time.Millisecond}

	start := time.Now()
	report := d.Run(context.Background())
	if elapsed := time.Since(start); elapsed > 10*time.Second {
		t.Errorf("Run took %v", elapsed)
	}
	expect(t, report, "CMake", doctor.StatusError, "cmake --version did not finish within 50ms.")
	expect(t, report, "Ninja", doctor.StatusOK, "1.12.1")
}

// writeProject creates the files of a Parrot project in a new directory;
// an empty content means the file is not created.
func writeProject(t *testing.T, parrotJSON, cmakeLists string) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range map[string]string{"parrot.json": parrotJSON, "CMakeLists.txt": cmakeLists} {
		if content == "" {
			continue
		}
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

const cmakeLists = "cmake_minimum_required(VERSION 3.16)\n"

func projectSection(t *testing.T, report doctor.Report) []doctor.CheckResult {
	t.Helper()
	for _, section := range report.Sections {
		if section.Name == "Project" {
			return section.Results
		}
	}
	t.Fatal("no Project section")
	return nil
}

func TestOutsideProject(t *testing.T) {
	s, _ := healthy(t)
	report := diagnose(s, t.TempDir())

	results := projectSection(t, report)
	if len(results) != 1 || results[0].Status != doctor.StatusSkipped || !strings.Contains(results[0].Message, "No Parrot project detected") {
		t.Errorf("Project section = %+v, want only a skipped project", results)
	}
	if n := report.Count(doctor.StatusError); n != 0 {
		t.Errorf("%d errors outside a project, want none", n)
	}
}

func TestValidProject(t *testing.T) {
	s, _ := healthy(t)
	report := diagnose(s, writeProject(t, `{"target": "esp32-c3"}`, cmakeLists))

	expect(t, report, "parrot.json", doctor.StatusOK, "")
	expect(t, report, "CMakeLists.txt", doctor.StatusOK, "")
	expect(t, report, "Target", doctor.StatusOK, "ESP32-C3 (ESP-IDF target esp32c3)")
}

// An unreadable parrot.json does not stop the checks that do not need it.
func TestInvalidConfig(t *testing.T) {
	s, _ := healthy(t)
	report := diagnose(s, writeProject(t, `{"target": `, cmakeLists))

	expect(t, report, "parrot.json", doctor.StatusError, "Invalid Parrot configuration.")
	expect(t, report, "CMakeLists.txt", doctor.StatusOK, "")
	expect(t, report, "Target", doctor.StatusSkipped, "requires a valid parrot.json")
}

func TestUnsupportedTarget(t *testing.T) {
	s, _ := healthy(t)
	report := diagnose(s, writeProject(t, `{"target": "banana"}`, cmakeLists))

	expect(t, report, "Target", doctor.StatusError, `Unsupported target "banana".`)
	expect(t, report, "Target", doctor.StatusError, "esp32-c3")
}

func TestMissingCMakeLists(t *testing.T) {
	s, _ := healthy(t)
	report := diagnose(s, writeProject(t, `{"target": "esp32"}`, ""))

	expect(t, report, "CMakeLists.txt", doctor.StatusError, "CMakeLists.txt not found")
	expect(t, report, "Target", doctor.StatusOK, "ESP32")
}
