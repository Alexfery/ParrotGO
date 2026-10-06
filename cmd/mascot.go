package cmd

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"
)

// ANSI 256-color codes. Mid tones read well on dark and light backgrounds.
const (
	red    = "\x1b[38;5;196m"
	yellow = "\x1b[38;5;214m"
	green  = "\x1b[38;5;34m"
	blue   = "\x1b[38;5;33m"
	brown  = "\x1b[38;5;130m"
	bold   = "\x1b[1m"
	reset  = "\x1b[0m"
)

// ANSI controls that let the animation redraw the mascot in place.
const (
	hideCursor = "\x1b[?25l"
	showCursor = "\x1b[?25h"
	clearLine  = "\x1b[2K"
	clearBelow = "\x1b[J"
)

type bannerPart struct{ color, text string }

// pose is one drawing of the mascot: a red head and chest, a yellow beak,
// green and blue wings, on a brown perch. The eye keeps the terminal's own
// color. The zero pose is the mascot standing still.
type pose struct {
	shift int    // head moved left (-1) or right (+1), for the dance
	blink bool   // eye closed
	says  string // shown next to the beak
}

// lines returns the pose, one slice of colored parts per line.
func (p pose) lines() [][]bannerPart {
	indent := strings.Repeat(" ", 3+p.shift)
	eye := "o"
	if p.blink {
		eye = "-"
	}
	head := []bannerPart{{"", indent}, {red, "("}, {"", eye}, {yellow, ">"}}
	if p.says != "" {
		head = append(head, bannerPart{"", "  "}, bannerPart{bold, p.says})
	}
	return [][]bannerPart{
		{{"", indent}, {red, `\`}},
		head,
		{{green, `\_//`}, {red, ")"}},
		{{"", " "}, {blue, `\_/`}, {red, "_)"}},
		{{"", "  "}, {brown, "_|_"}},
	}
}

// render draws the pose line by line, in color or as plain text.
func (p pose) render(color bool) []string {
	var out []string
	for _, line := range p.lines() {
		var b strings.Builder
		for _, part := range line {
			if color && part.color != "" {
				b.WriteString(part.color + part.text + reset)
			} else {
				b.WriteString(part.text)
			}
		}
		out = append(out, b.String())
	}
	return out
}

// banner draws the mascot standing still, in color or as plain text.
func banner(color bool) string {
	return strings.Join(pose{}.render(color), "\n") + "\n"
}

type frame struct {
	pose pose
	hold time.Duration
}

// intro is the animation bare `parrot` plays before its help: the mascot
// bobs its head, blinks, then says its name one letter at a time. It lasts
// about 1.5 seconds.
func intro() []frame {
	const beat = 110 * time.Millisecond
	frames := []frame{
		{pose{shift: 1}, beat},
		{pose{}, beat},
		{pose{shift: -1}, beat},
		{pose{}, beat},
		{pose{shift: 1}, beat},
		{pose{}, beat},
		{pose{blink: true}, beat},
		{pose{}, 150 * time.Millisecond},
	}
	const name = "Parrot!"
	for i := 1; i <= len(name); i++ {
		frames = append(frames, frame{pose{says: name[:i]}, 50 * time.Millisecond})
	}
	frames[len(frames)-1].hold = 400 * time.Millisecond
	return frames
}

// animate plays frames in place on w, which must be a color terminal, then
// clears them, so that what is printed next starts where the first frame did.
// It stops early and returns false when ctx is canceled (Ctrl+C).
func animate(ctx context.Context, w io.Writer, frames []frame) bool {
	io.WriteString(w, hideCursor)
	defer io.WriteString(w, showCursor)
	height := 0
	ok := true
	for _, f := range frames {
		// Each frame is one write, which avoids flicker.
		var b strings.Builder
		if height > 0 {
			fmt.Fprintf(&b, "\x1b[%dA", height) // back to the first line
		}
		lines := f.pose.render(true)
		for _, line := range lines {
			b.WriteString(clearLine + line + "\n")
		}
		height = len(lines)
		io.WriteString(w, b.String())
		if !sleep(ctx, f.hold) {
			ok = false
			break
		}
	}
	if height > 0 {
		fmt.Fprintf(w, "\x1b[%dA%s", height, clearBelow)
	}
	return ok
}

// sleep waits for d, or until ctx is canceled. It reports whether d elapsed.
func sleep(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return true
	case <-ctx.Done():
		return false
	}
}
