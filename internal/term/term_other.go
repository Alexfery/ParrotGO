//go:build !windows

package term

import "os"

// enableColor reports whether f is a terminal. Unix terminals process ANSI
// codes without any setup.
func enableColor(f *os.File) bool {
	fi, err := f.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}
