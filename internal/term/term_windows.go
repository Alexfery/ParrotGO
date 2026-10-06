//go:build windows

package term

import (
	"os"
	"syscall"
)

const enableVirtualTerminalProcessing = 0x0004

// kernel32 is a known DLL, so Windows always loads it from System32.
var procSetConsoleMode = syscall.NewLazyDLL("kernel32.dll").NewProc("SetConsoleMode")

// enableColor turns on ANSI processing for the console behind f. It fails
// when f is not a console, or when the console is too old to support it.
func enableColor(f *os.File) bool {
	h := syscall.Handle(f.Fd())
	var mode uint32
	if err := syscall.GetConsoleMode(h, &mode); err != nil {
		return false
	}
	if mode&enableVirtualTerminalProcessing != 0 {
		return true
	}
	ok, _, _ := procSetConsoleMode.Call(uintptr(h), uintptr(mode|enableVirtualTerminalProcessing))
	return ok != 0
}
