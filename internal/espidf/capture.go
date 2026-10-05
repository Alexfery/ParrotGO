package espidf

import (
	"context"
	"os/exec"
	"time"
)

// Capture runs name with args, without a shell, and returns everything it
// printed on stdout and stderr. It is for short read-only commands whose
// output Parrot reads, such as `cmake --version`; idf.py actions go through
// Runner, which streams their output instead.
//
// A non-zero exit status is returned as the exec error, with the output. When
// ctx is done the process is killed, and the error is ctx.Err().
func Capture(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	// A killed process may leave children behind (on Windows, launchers such
	// as idf.py.exe start Python) that keep the output pipe open. Stop waiting
	// for them shortly after the kill, or the time limit would not hold.
	cmd.WaitDelay = time.Second
	out, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		return string(out), ctx.Err()
	}
	return string(out), err
}
