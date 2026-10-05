package cmd

import (
	"bytes"
	"strings"
	"testing"

	"parrot/internal/doctor"
)

// isolateESPIDF hides the ESP-IDF environment of the machine running the
// tests, if any, so that only PATH decides what doctor finds.
func isolateESPIDF(t *testing.T) {
	t.Setenv("IDF_PATH", "")
	t.Setenv("IDF_PYTHON_ENV_PATH", "")
}

// Outside a project, with the tools on PATH but no IDF_PATH: only a warning,
// so the command succeeds.
func TestDoctorWarningsDoNotFail(t *testing.T) {
	isolateESPIDF(t)
	installFakeTools(t, "idf.py", "python", "python3", "cmake", "ninja")
	t.Chdir(t.TempDir())

	out, err := runOutput(t, "doctor")
	if err != nil {
		t.Fatalf("doctor error = %v, want success with warnings\n%s", err, out)
	}
	for _, want := range []string{
		"Parrot Doctor\n\nEnvironment\n-----------\n\n✓ Parrot CLI\n",
		"✓ idf.py\n",
		"! IDF_PATH\n  IDF_PATH is not set, but idf.py is available.\n",
		"- ESP-IDF tools\n  Skipped: requires a valid IDF_PATH.\n",
		"Project\n-------\n\n- Parrot project\n  No Parrot project detected",
		"No problems detected (1 warning).\n",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output does not contain %q:\n%s", want, out)
		}
	}
}

func TestDoctorInProject(t *testing.T) {
	isolateESPIDF(t)
	newProject(t, "esp32-c3")
	installFakeTools(t, "idf.py", "python", "python3", "cmake", "ninja")

	out, err := runOutput(t, "doctor")
	if err != nil {
		t.Fatalf("doctor error = %v\n%s", err, out)
	}
	want := "Project\n-------\n\n✓ parrot.json\n\n✓ CMakeLists.txt\n\n✓ Target\n  ESP32-C3 (ESP-IDF target esp32c3)\n"
	if !strings.Contains(out, want) {
		t.Errorf("output does not contain %q:\n%s", want, out)
	}
}

// Nothing on PATH: every problem is reported in one run, and the command fails.
func TestDoctorProblemsFail(t *testing.T) {
	isolateESPIDF(t)
	t.Setenv("PATH", t.TempDir())
	newProject(t, "esp32")

	out, err := runOutput(t, "doctor")
	// idf.py, Python, CMake and Ninja.
	if err == nil || err.Error() != "4 problems detected" {
		t.Errorf("doctor error = %v, want 4 problems detected", err)
	}
	for _, want := range []string{"✗ idf.py\n", "✗ Python\n", "✗ CMake\n", "✗ Ninja\n", "✓ Target\n"} {
		if !strings.Contains(out, want) {
			t.Errorf("output does not contain %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "No problems detected") {
		t.Errorf("output reports no problems:\n%s", out)
	}
}

func TestReportOutcome(t *testing.T) {
	report := func(statuses ...doctor.Status) doctor.Report {
		var results []doctor.CheckResult
		for _, s := range statuses {
			results = append(results, doctor.CheckResult{Name: "check", Status: s})
		}
		return doctor.Report{Sections: []doctor.Section{{Name: "Environment", Results: results}}}
	}
	tests := []struct {
		report  doctor.Report
		wantOut string
		wantErr string
	}{
		{report(doctor.StatusOK, doctor.StatusSkipped), "No problems detected.\n", ""},
		{report(doctor.StatusOK, doctor.StatusWarning), "No problems detected (1 warning).\n", ""},
		{report(doctor.StatusWarning, doctor.StatusWarning, doctor.StatusSkipped), "No problems detected (2 warnings).\n", ""},
		{report(doctor.StatusError, doctor.StatusWarning), "", "1 problem detected"},
		{report(doctor.StatusError, doctor.StatusError, doctor.StatusOK), "", "2 problems detected"},
	}
	for _, tt := range tests {
		var out bytes.Buffer
		err := reportOutcome(&out, tt.report)
		gotErr := ""
		if err != nil {
			gotErr = err.Error()
		}
		if gotErr != tt.wantErr || out.String() != tt.wantOut {
			t.Errorf("reportOutcome(%+v) = %q, error %q; want %q, error %q", tt.report, out.String(), gotErr, tt.wantOut, tt.wantErr)
		}
	}
}
