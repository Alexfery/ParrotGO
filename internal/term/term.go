// Package term tells whether output can use ANSI colors.
package term

import "os"

// ColorEnabled reports whether f is a terminal that shows ANSI colors.
// Colors are off when NO_COLOR is set (https://no-color.org), when TERM is
// "dumb", and when f is redirected to a file or a pipe. On Windows it also
// turns on ANSI processing for the console, which is off by default in the
// classic console.
func ColorEnabled(f *os.File) bool {
	if os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb" {
		return false
	}
	return enableColor(f)
}
