package components

import (
	"time"

	"github.com/vivaui/viva/ui"
)

// A text caret is lit for CaretBlinkHalf, dark for as long, and keeps going for
// CaretBlinkSpan after the last thing that happened; then it stays lit, so a
// window left alone asks for no frames. The half period matches the terminal
// cursor's, so an application that blinks both does not read as two rhythms.
const (
	CaretBlinkHalf = 530 * time.Millisecond
	CaretBlinkSpan = 20 * time.Second
)

// CaretBlink is the phase of one text caret, held by the application next to
// the text it belongs to. It decides whether this frame paints the caret and
// asks for the frame that turns it over; the caret itself stays the
// component's, drawn only while Lit reports true.
//
// The zero value is ready and restarting: the first frame paints the caret lit,
// which is what typing looks like. The phase restarts whenever the caret moves
// to another position or the widget loses and regains the keyboard, so the
// caret never goes dark at the moment someone starts typing.
//
// It is not safe for concurrent use, and one CaretBlink serves one caret.
type CaretBlink struct {
	since  time.Duration // when the caret last moved or the keyboard arrived
	anchor int
	seen   bool // since is meaningful: a frame painted this caret focused
}

// Lit reports whether the caret is lit in this frame, and asks for the frame
// that turns it over. anchor is any value that changes when the caret moves,
// such as its byte offset; a different anchor restarts the phase. A canvas
// without a frame clock, a reduced-motion session, and a caret left alone for
// CaretBlinkSpan all stay lit and ask for nothing.
func (blink *CaretBlink) Lit(canvas *ui.Canvas, anchor int) bool {
	if !canvas.HasClock() {
		return true
	}
	now := canvas.Now()
	if !blink.seen || blink.anchor != anchor {
		blink.since, blink.anchor, blink.seen = now, anchor, true
	}
	elapsed := max(now-blink.since, 0)
	if elapsed >= CaretBlinkSpan || canvas.ReduceMotion() {
		return true
	}
	phase := elapsed % (2 * CaretBlinkHalf)
	if phase < CaretBlinkHalf {
		canvas.RequestFrameIn(CaretBlinkHalf - phase)
		return true
	}
	canvas.RequestFrameIn(2*CaretBlinkHalf - phase)
	return false
}

// Rest forgets the phase, so the next frame that paints the caret focused
// starts it lit. An application calls it while the caret is not painted, such
// as when the field loses the keyboard.
func (blink *CaretBlink) Rest() {
	blink.seen = false
}
