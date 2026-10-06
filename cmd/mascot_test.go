package cmd

import (
	"bytes"
	"context"
	"regexp"
	"strings"
	"testing"
	"time"
)

const plainMascot = `   \
   (o>
\_//)
 \_/_)
  _|_
`

var ansi = regexp.MustCompile("\x1b\\[[0-9;?]*[a-zA-Z]")

func TestBannerPlainIsTheMascot(t *testing.T) {
	if got := banner(false); got != plainMascot {
		t.Errorf("banner(false) =\n%s\nwant\n%s", got, plainMascot)
	}
}

func TestBannerColorKeepsTheShape(t *testing.T) {
	colored := banner(true)
	if !strings.Contains(colored, "\x1b[") {
		t.Fatal("banner(true) has no ANSI codes")
	}
	if got := ansi.ReplaceAllString(colored, ""); got != plainMascot {
		t.Errorf("banner(true) without colors =\n%s\nwant\n%s", got, plainMascot)
	}
}

func TestHelpIsPlainWhenNotATerminal(t *testing.T) {
	out, err := runOutput(t, "--help")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "\x1b[") {
		t.Errorf("help written to a buffer has ANSI codes:\n%q", out)
	}
	if !strings.Contains(out, plainMascot) {
		t.Errorf("help does not show the mascot:\n%s", out)
	}
}

func TestPoses(t *testing.T) {
	tests := []struct {
		name string
		pose pose
		want string
	}{
		{"head right", pose{shift: 1}, `    \
    (o>
\_//)
 \_/_)
  _|_`},
		{"head left", pose{shift: -1}, `  \
  (o>
\_//)
 \_/_)
  _|_`},
		{"blink", pose{blink: true}, `   \
   (->
\_//)
 \_/_)
  _|_`},
		{"says", pose{says: "Parrot!"}, `   \
   (o>  Parrot!
\_//)
 \_/_)
  _|_`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := strings.Join(tt.pose.render(false), "\n"); got != tt.want {
				t.Errorf("render(false) =\n%s\nwant\n%s", got, tt.want)
			}
			colored := strings.Join(tt.pose.render(true), "\n")
			if got := ansi.ReplaceAllString(colored, ""); got != tt.want {
				t.Errorf("render(true) without colors =\n%s\nwant\n%s", got, tt.want)
			}
		})
	}
}

func TestIntroSaysParrotAndStaysShort(t *testing.T) {
	frames := intro()
	if got := frames[len(frames)-1].pose; got != (pose{says: "Parrot!"}) {
		t.Errorf("last frame = %+v, want the mascot saying Parrot!", got)
	}
	var total time.Duration
	for _, f := range frames {
		total += f.hold
	}
	if total > 2*time.Second {
		t.Errorf("intro lasts %v, want at most 2s: it delays the help", total)
	}
}

func TestAnimateRedrawsInPlaceThenClears(t *testing.T) {
	var out bytes.Buffer
	frames := []frame{{pose{}, 0}, {pose{blink: true}, 0}}
	if !animate(context.Background(), &out, frames) {
		t.Fatal("animate() = false, want true")
	}
	got := out.String()
	if !strings.HasPrefix(got, hideCursor) {
		t.Errorf("output does not hide the cursor first: %q", got)
	}
	// Up to the first line before the second frame, and once more to clear.
	if n := strings.Count(got, "\x1b[5A"); n != 2 {
		t.Errorf("output moves up 5 lines %d times, want 2: %q", n, got)
	}
	if !strings.HasSuffix(got, "\x1b[5A"+clearBelow+showCursor) {
		t.Errorf("output does not clear the mascot and show the cursor at the end: %q", got)
	}
	if !strings.Contains(ansi.ReplaceAllString(got, ""), "(->") {
		t.Errorf("output does not draw the blink frame: %q", got)
	}
}

func TestAnimateStopsOnCtrlC(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var out bytes.Buffer
	frames := []frame{{pose{}, time.Hour}, {pose{blink: true}, time.Hour}}
	if animate(ctx, &out, frames) {
		t.Fatal("animate() = true after Ctrl+C, want false")
	}
	got := out.String()
	if strings.Contains(ansi.ReplaceAllString(got, ""), "(->") {
		t.Errorf("animate() kept playing after Ctrl+C: %q", got)
	}
	if !strings.HasSuffix(got, clearBelow+showCursor) {
		t.Errorf("animate() did not clean up the terminal after Ctrl+C: %q", got)
	}
}
