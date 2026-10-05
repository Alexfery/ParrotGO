package espidf

import (
	"errors"
	"os/exec"
	"path/filepath"
	"runtime"
)

// Environment variables set by ESP-IDF's activation scripts.
const (
	IDFPathVar   = "IDF_PATH"            // the ESP-IDF directory
	PythonEnvVar = "IDF_PYTHON_ENV_PATH" // the Python virtual environment ESP-IDF runs in
)

// IDFScript returns the idf.py Python script of the ESP-IDF in idfPath.
func IDFScript(idfPath string) string {
	return filepath.Join(idfPath, "tools", "idf.py")
}

// VersionArgs returns the Python arguments that print the version of the
// ESP-IDF in idfPath, e.g. "ESP-IDF v6.1".
//
// They run the idf.py script directly: on Windows, the idf.py.exe launcher on
// PATH answers --version with its own version instead of ESP-IDF's.
func VersionArgs(idfPath string) []string {
	return []string{IDFScript(idfPath), "--version"}
}

// ToolsCheckArgs returns the Python arguments that make the ESP-IDF in
// idfPath check its tools. idf_tools.py check exits with a non-zero status
// when a required tool is neither on PATH nor in $IDF_TOOLS_PATH/tools.
func ToolsCheckArgs(idfPath string) []string {
	return []string{filepath.Join(idfPath, "tools", "idf_tools.py"), "check"}
}

// VenvPython returns the Python executable of the virtual environment in dir,
// such as the one PythonEnvVar names.
func VenvPython(dir string) string {
	if runtime.GOOS == "windows" {
		return filepath.Join(dir, "Scripts", "python.exe")
	}
	return filepath.Join(dir, "bin", "python")
}

// ErrIDFNotFound is returned when idf.py is not on PATH.
var ErrIDFNotFound = errors.New("idf.py was not found\n\n" +
	"Make sure the ESP-IDF environment is activated before running Parrot.")

// FindIDF returns the path of the idf.py executable on PATH.
//
// On Linux and macOS, idf.py is the Python script in $IDF_PATH/tools, started
// through its shebang with the Python of the activated ESP-IDF environment.
// On Windows a .py file cannot be started directly, so ESP-IDF's activation
// scripts put idf.py.exe on PATH: a launcher (the "idf-exe" tool) that runs
// idf.py with the environment's Python and returns its exit code.
func FindIDF() (string, error) {
	name := "idf.py"
	if runtime.GOOS == "windows" {
		name = "idf.py.exe"
	}
	path, err := exec.LookPath(name)
	if err != nil {
		return "", ErrIDFNotFound
	}
	return path, nil
}
