package doctor

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"runtime/debug"
	"strings"

	"parrot/internal/espidf"
)

// environment checks the tools Parrot needs. idf.py, IDF_PATH, Python, CMake
// and Ninja are checked independently; the ESP-IDF version and tools checks
// need IDF_PATH and Python, and are skipped without them.
func (d Doctor) environment(ctx context.Context) []CheckResult {
	idf := d.checkIDF()
	idfPath, idfPathResult := d.checkIDFPath(idf.Status == StatusOK)
	python, pythonResult := d.checkPython(ctx)
	return []CheckResult{
		checkParrot(),
		idf,
		idfPathResult,
		pythonResult,
		d.checkIDFVersion(ctx, idfPath, python),
		d.checkTool(ctx, "CMake", "cmake"),
		d.checkTool(ctx, "Ninja", "ninja"),
		d.checkIDFTools(ctx, idfPath, python),
	}
}

// Reasons of the checks skipped for lack of what they need.
const (
	needIDFPath = "Skipped: requires a valid IDF_PATH."
	needPython  = "Skipped: requires a working Python."
)

func checkParrot() CheckResult {
	var version string
	if info, found := debug.ReadBuildInfo(); found && info.Main.Version != "(devel)" && info.Main.Version != "" {
		version = "Version: " + info.Main.Version
	}
	return ok("Parrot CLI", version)
}

func (d Doctor) checkIDF() CheckResult {
	path, err := d.System.FindIDF()
	if err != nil {
		return failed("idf.py", "idf.py was not found on PATH.\nActivate the ESP-IDF environment and run Parrot again.")
	}
	return ok("idf.py", path)
}

// checkIDFPath returns the ESP-IDF directory named by IDF_PATH, or "" if it
// is not set or not an ESP-IDF directory. idf.py can run without IDF_PATH,
// so a missing one is only a warning.
func (d Doctor) checkIDFPath(idfFound bool) (string, CheckResult) {
	const name = "IDF_PATH"
	path := d.System.Getenv(espidf.IDFPathVar)
	if path == "" {
		if idfFound {
			return "", warning(name, "IDF_PATH is not set, but idf.py is available.")
		}
		return "", warning(name, "IDF_PATH is not set.")
	}
	if _, err := os.Stat(espidf.IDFScript(path)); err != nil {
		return "", failed(name, fmt.Sprintf("IDF_PATH is %s, which is not an ESP-IDF directory:\n%s was not found.", path, espidf.IDFScript(path)))
	}
	return path, ok(name, path)
}

// checkPython returns the Python ESP-IDF uses, or "" if none works. In an
// activated ESP-IDF environment that is the environment's own Python, and
// only that one is checked. Otherwise the first Python on PATH that runs.
func (d Doctor) checkPython(ctx context.Context) (string, CheckResult) {
	const name = "Python"
	if env := d.System.Getenv(espidf.PythonEnvVar); env != "" {
		path := espidf.VenvPython(env)
		version, err := d.version(ctx, path)
		if err != nil {
			return "", failed(name, fmt.Sprintf("The ESP-IDF Python environment (%s) does not work:\n%v", env, err))
		}
		return path, ok(name, version+"\n"+path+" (ESP-IDF Python environment)")
	}

	names := pythonNames()
	var lastErr error
	for _, n := range names {
		path, err := d.System.LookPath(n)
		if err != nil {
			continue
		}
		version, err := d.version(ctx, path)
		if err != nil {
			lastErr = err // e.g. the Microsoft Store stub on Windows
			continue
		}
		return path, ok(name, version+"\n"+path)
	}
	if lastErr != nil {
		return "", failed(name, lastErr.Error())
	}
	return "", failed(name, fmt.Sprintf("Python was not found on PATH (looked for %s).", strings.Join(names, " and ")))
}

// pythonNames returns the names Python has on PATH, the usual one first: on
// Linux and macOS "python" may be missing or Python 2.
func pythonNames() []string {
	if runtime.GOOS == "windows" {
		return []string{"python", "python3"}
	}
	return []string{"python3", "python"}
}

// checkIDFVersion reports the version of the ESP-IDF in idfPath.
func (d Doctor) checkIDFVersion(ctx context.Context, idfPath, python string) CheckResult {
	const name = "ESP-IDF"
	switch {
	case idfPath == "":
		return skipped(name, needIDFPath)
	case python == "":
		return skipped(name, needPython)
	}
	out, err := d.capture(ctx, d.commandTimeout(), python, espidf.VersionArgs(idfPath)...)
	if err != nil {
		return failed(name, err.Error())
	}
	for _, line := range nonEmptyLines(out) {
		if strings.HasPrefix(line, "ESP-IDF ") {
			return ok(name, line)
		}
	}
	return warning(name, "idf.py --version printed no ESP-IDF version:\n"+outputSummary(out))
}

// checkTool checks that executable is on PATH and reports its version.
func (d Doctor) checkTool(ctx context.Context, name, executable string) CheckResult {
	path, err := d.System.LookPath(executable)
	if err != nil {
		return failed(name, name+" was not found on PATH.")
	}
	version, err := d.version(ctx, path)
	if err != nil {
		return failed(name, err.Error())
	}
	return ok(name, version+"\n"+path)
}

// checkIDFTools asks ESP-IDF to check its tools, compilers and debuggers of
// every target included, instead of Parrot knowing them.
//
// A failure is only a warning: idf_tools.py check finds tools on PATH or in
// $IDF_TOOLS_PATH/tools only, so it reports tools installed elsewhere as
// missing, e.g. by EIM, Espressif's installation manager, even when build,
// flash and monitor work.
func (d Doctor) checkIDFTools(ctx context.Context, idfPath, python string) CheckResult {
	const name = "ESP-IDF tools"
	switch {
	case idfPath == "":
		return skipped(name, needIDFPath)
	case python == "":
		return skipped(name, needPython)
	}
	if _, err := d.capture(ctx, d.toolsTimeout(), python, espidf.ToolsCheckArgs(idfPath)...); err != nil {
		return warning(name, err.Error()+"\n"+
			"idf_tools.py only finds tools on PATH or in IDF_TOOLS_PATH/tools, so tools installed\n"+
			"elsewhere (e.g. by EIM) can be reported here while build, flash and monitor still work.")
	}
	return ok(name, "idf_tools.py check found every required tool.")
}
