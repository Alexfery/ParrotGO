package term_test

import (
	"os"
	"path/filepath"
	"testing"

	"parrot/internal/term"
)

func TestColorEnabledIsOffForAFile(t *testing.T) {
	f, err := os.Create(filepath.Join(t.TempDir(), "out.txt"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if term.ColorEnabled(f) {
		t.Error("ColorEnabled(file) = true, want false: a redirected output must stay plain")
	}
}

func TestColorEnabledRespectsNoColor(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	if term.ColorEnabled(os.Stdout) {
		t.Error("ColorEnabled with NO_COLOR set = true, want false")
	}
}
