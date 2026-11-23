package interaction

import (
	"math"
	"time"
)

// A finger is read as the platforms read it (Android's ViewConfiguration and
// GestureDetector, UIKit's scroll views): its press waits until it is clearly
// not the start of a pan, a pan scrolls what is under it and flings on when the
// finger leaves moving, and a press held still is a secondary press.
const (
	// TouchSlop is how far a finger moves, in logical pixels, before its press
	// becomes a pan.
	TouchSlop = 8
	// TouchTapTimeout is how long a still finger waits before it presses, so a
	// pan that starts at once presses nothing (Android's TAP_TIMEOUT).
	TouchTapTimeout = 100 * time.Millisecond
	// TouchLongPress is how long a still finger presses before it is a secondary
	// press.
	TouchLongPress = 500 * time.Millisecond
	// touchDoubleTap and touchDoubleTapSlop are how soon and how near a tap
	// follows the last one to count as its second.
	touchDoubleTap     = 300 * time.Millisecond
	touchDoubleTapSlop = 24
	// flingMinVelocity is the speed, in logical pixels a second, a lifted finger
	// needs to fling; a fling stops below flingStopVelocity.
	flingMinVelocity  = 50
	flingStopVelocity = 10
	// flingDecay is how much of a fling's speed is left after a millisecond:
	// UIScrollView's normal deceleration rate.
	flingDecay = 0.998
	// velocityWindow is how much of a pan's recent motion its speed is read from.
	velocityWindow = 100 * time.Millisecond
)

// PinchPhase is where two fingers are in a pinch.
type PinchPhase uint8

const (
	PinchNone PinchPhase = iota
	PinchBegin
	PinchChange
	PinchEnd
)

// pinchMinDistance is the least the two fingers of a pinch may start apart, in logical
// pixels: closer, the ratio of their distances is noise.
const pinchMinDistance = 24

type touchPhase uint8

const (
	touchIdle    touchPhase = iota
	touchPending            // down, not yet a press nor a pan
	touchPressed            // delivered as a primary press
	touchPanning            // scrolling what was under it when it went down
	touchLong               // delivered as a secondary press; the rest of it is ignored
)

// TouchStep is what one event or moment of a finger means to the runtime. The
// runtime carries it out in the order of the fields: it ends the primary press
// (Cancel), presses (Press), moves the press (Move), releases it (Release),
// presses and releases the secondary button (Secondary), takes the hover away
// (Lift), and scrolls (ScrollX, ScrollY at ScrollAtX, ScrollAtY).
type TouchStep struct {
	Cancel, Press, Move, Release, Secondary, Lift bool
	// X and Y are where the press, its move, its release or the secondary press
	// is, and Clicks the press's count among quick taps (2 for a double tap).
	X, Y   float32
	Clicks int
	// ScrollX and ScrollY scroll the content under ScrollAtX, ScrollAtY: by the
	// finger's motion, or, when Momentum, by what is left of a fling.
	ScrollX, ScrollY     float32
	ScrollAtX, ScrollAtY float32
	Momentum             bool
	// Pinch is where two fingers are in a pinch: at its start, as it changes and at its
	// end; zero when this step is no part of one. Scale is the distance between the fingers
	// as a multiple of what it was at the start, Delta the same since the last step, and
	// PinchX, PinchY the midpoint.
	Pinch                  PinchPhase
	PinchX, PinchY         float32
	PinchScale, PinchDelta float32
	// ScrollEnds says the finger that was scrolling lifted or was taken: what it held
	// past an end of the content is let go.
	ScrollEnds bool
	// Wake asks for a frame after WakeAfter, when Advance has more to do.
	Wake      bool
	WakeAfter time.Duration
}

// Scrolls reports whether the step scrolls.
func (s TouchStep) Scrolls() bool { return s.ScrollX != 0 || s.ScrollY != 0 }

type touchSample struct {
	x, y float32
	at   time.Duration
}

// Touch follows the one finger that presses or pans; a second finger ends what
// the first was doing. The zero value is ready to use. It keeps no clock: the
// runtime gives it the time of each event and asks it to Advance at the frames
// it asked for.
type Touch struct {
	phase                  touchPhase
	id                     int32
	down                   time.Duration
	startX, startY         float32
	lastX, lastY           float32
	samples                [8]touchSample
	sampled                int
	lastTap                time.Duration
	lastTapX, lastTapY     float32
	clicks                 int
	pressDue, longPressDue bool
	flinging               bool
	flingVX, flingVY       float32 // the finger's velocity, logical pixels a second
	flingAt                time.Duration
	// A pinch is the first finger (id) and a second one (pinchID) held together.
	pinching              bool
	pinchID               int32
	pinchAX, pinchAY      float32 // where the first finger is
	pinchBX, pinchBY      float32 // where the second is
	pinchStart, pinchLast float32 // the distance between them when it began, and at the last step
}

// Active reports whether a finger is down or a fling moves.
func (t *Touch) Active() bool { return t.phase != touchIdle || t.flinging || t.pinching }

// Down is finger id touching at x, y. claimsDrag says what is under it takes a
// moving finger as a drag (a slider, a divider): it presses at once.
func (t *Touch) Down(id int32, x, y float32, now time.Duration, claimsDrag bool) TouchStep {
	if t.pinching {
		return TouchStep{} // a third finger is nothing
	}
	if t.phase != touchIdle && id != t.id {
		// A second finger is not a press: the first one's gesture ends, and the two are a pinch.
		ax, ay := t.lastX, t.lastY
		step := t.Cancel()
		if distance := float32(math.Hypot(float64(x-ax), float64(y-ay))); distance >= pinchMinDistance {
			t.pinching, t.pinchID = true, id
			t.pinchAX, t.pinchAY, t.pinchBX, t.pinchBY = ax, ay, x, y
			t.pinchStart, t.pinchLast = distance, distance
			step.Pinch, step.PinchX, step.PinchY = PinchBegin, (ax+x)/2, (ay+y)/2
			step.PinchScale, step.PinchDelta = 1, 1
		}
		return step
	}
	t.flinging = false
	t.phase, t.id, t.down = touchPending, id, now
	t.startX, t.startY, t.lastX, t.lastY = x, y, x, y
	t.sampled = 0
	t.sample(x, y, now)
	t.pressDue, t.longPressDue = true, true
	if claimsDrag {
		return t.press(now)
	}
	return TouchStep{Wake: true, WakeAfter: TouchTapTimeout}
}

// Move is finger id moving to x, y.
func (t *Touch) Move(id int32, x, y float32, now time.Duration) TouchStep {
	if t.pinching {
		return t.pinchMove(id, x, y)
	}
	if t.phase == touchIdle || id != t.id {
		return TouchStep{}
	}
	t.sample(x, y, now)
	moved := math.Hypot(float64(x-t.startX), float64(y-t.startY)) > TouchSlop
	switch t.phase {
	case touchPending:
		if !moved {
			return TouchStep{}
		}
		t.phase, t.pressDue, t.longPressDue = touchPanning, false, false
		return t.pan(x, y)
	case touchPanning:
		return t.pan(x, y)
	case touchPressed:
		if moved {
			t.longPressDue = false
		}
		t.lastX, t.lastY = x, y
		return TouchStep{Move: true, X: x, Y: y}
	}
	return TouchStep{}
}

// Up is finger id lifting at x, y.
func (t *Touch) Up(id int32, x, y float32, now time.Duration) TouchStep {
	if t.pinching {
		if id != t.id && id != t.pinchID {
			return TouchStep{}
		}
		step := t.pinchMove(id, x, y)
		return t.endPinch(step)
	}
	if t.phase == touchIdle || id != t.id {
		return TouchStep{}
	}
	phase := t.phase
	t.phase, t.pressDue, t.longPressDue = touchIdle, false, false
	switch phase {
	case touchPending: // lifted before it pressed: a tap
		return TouchStep{Press: true, Release: true, Lift: true, X: x, Y: y, Clicks: t.tapCount(x, y, now)}
	case touchPressed:
		return TouchStep{Release: true, Lift: true, X: x, Y: y, Clicks: t.clicks}
	case touchPanning:
		vx, vy := t.velocity(now)
		step := TouchStep{ScrollEnds: true, ScrollAtX: t.startX, ScrollAtY: t.startY}
		if math.Hypot(float64(vx), float64(vy)) >= flingMinVelocity {
			t.flinging, t.flingVX, t.flingVY, t.flingAt = true, vx, vy, now
			step.Wake = true
		}
		return step
	case touchLong:
		return TouchStep{Lift: true, X: x, Y: y}
	}
	return TouchStep{}
}

// Cancel ends what the finger was doing without a click: the platform took it,
// or a second finger came.
func (t *Touch) Cancel() TouchStep {
	if t.pinching {
		return t.endPinch(TouchStep{})
	}
	phase := t.phase
	t.phase, t.pressDue, t.longPressDue, t.flinging = touchIdle, false, false, false
	if phase == touchPanning {
		return TouchStep{ScrollEnds: true, ScrollAtX: t.startX, ScrollAtY: t.startY}
	}
	if phase != touchPressed {
		return TouchStep{}
	}
	return TouchStep{Cancel: true, Lift: true, X: t.lastX, Y: t.lastY}
}

// StopFling ends a fling, as a wheel or a touchpad taking over does.
func (t *Touch) StopFling() { t.flinging = false }

// Advance is what the finger's time brings at now: a still finger's press once
// its tap timeout passed, its secondary press once it was held long enough, and
// the next step of a fling.
func (t *Touch) Advance(now time.Duration) TouchStep {
	switch {
	case t.phase == touchPending && t.pressDue && now-t.down >= TouchTapTimeout:
		return t.press(now)
	case t.phase == touchPressed && t.longPressDue && now-t.down >= TouchLongPress:
		// Held still: the press becomes a secondary press, as a right click.
		t.longPressDue, t.phase = false, touchLong
		return TouchStep{Cancel: true, Secondary: true, X: t.lastX, Y: t.lastY, Clicks: 1}
	case t.flinging:
		return t.fling(now)
	}
	return TouchStep{}
}

// Fling reports a fling that moved nothing, at the end of what it scrolls: it
// stops.
func (t *Touch) FlingBlocked() { t.flinging = false }

// pinchMove follows one of the two fingers of a pinch and says how far apart they are now.
func (t *Touch) pinchMove(id int32, x, y float32) TouchStep {
	switch id {
	case t.id:
		t.pinchAX, t.pinchAY = x, y
	case t.pinchID:
		t.pinchBX, t.pinchBY = x, y
	default:
		return TouchStep{}
	}
	distance := float32(math.Hypot(float64(t.pinchBX-t.pinchAX), float64(t.pinchBY-t.pinchAY)))
	step := TouchStep{
		Pinch: PinchChange, PinchX: (t.pinchAX + t.pinchBX) / 2, PinchY: (t.pinchAY + t.pinchBY) / 2,
		PinchScale: distance / t.pinchStart, PinchDelta: 1,
	}
	if t.pinchLast > 0 {
		step.PinchDelta = distance / t.pinchLast
	}
	t.pinchLast = distance
	return step
}

// endPinch ends a pinch where the fingers last were: the finger that is left is no gesture.
func (t *Touch) endPinch(step TouchStep) TouchStep {
	scale := float32(1)
	if t.pinchStart > 0 {
		scale = t.pinchLast / t.pinchStart
	}
	step.Pinch = PinchEnd
	step.PinchX, step.PinchY = (t.pinchAX+t.pinchBX)/2, (t.pinchAY+t.pinchBY)/2
	step.PinchScale, step.PinchDelta = scale, 1
	t.pinching = false
	t.phase = touchIdle
	return step
}

func (t *Touch) press(now time.Duration) TouchStep {
	t.phase, t.pressDue = touchPressed, false
	step := TouchStep{Press: true, X: t.lastX, Y: t.lastY, Clicks: t.tapCount(t.lastX, t.lastY, now)}
	if t.longPressDue {
		step.Wake, step.WakeAfter = true, max(TouchLongPress-(now-t.down), 0)
	}
	return step
}

// pan scrolls what was under the finger when it went down by how far the finger
// moved: the content follows it.
func (t *Touch) pan(x, y float32) TouchStep {
	step := TouchStep{ScrollX: t.lastX - x, ScrollY: t.lastY - y, ScrollAtX: t.startX, ScrollAtY: t.startY}
	t.lastX, t.lastY = x, y
	return step
}

// fling moves a fling on by the time since its last step, slowing it as
// UIScrollView does, and asks for the next frame while it still moves.
func (t *Touch) fling(now time.Duration) TouchStep {
	elapsed := now - t.flingAt
	if elapsed <= 0 {
		return TouchStep{Wake: true}
	}
	t.flingAt = now
	seconds := float32(elapsed.Seconds())
	step := TouchStep{ScrollX: -t.flingVX * seconds, ScrollY: -t.flingVY * seconds, ScrollAtX: t.startX, ScrollAtY: t.startY, Momentum: true}
	decay := float32(math.Pow(flingDecay, float64(elapsed)/float64(time.Millisecond)))
	t.flingVX, t.flingVY = t.flingVX*decay, t.flingVY*decay
	if math.Hypot(float64(t.flingVX), float64(t.flingVY)) < flingStopVelocity {
		t.flinging = false
	} else {
		step.Wake = true
	}
	return step
}

func (t *Touch) sample(x, y float32, at time.Duration) {
	copy(t.samples[1:], t.samples[:len(t.samples)-1])
	t.samples[0] = touchSample{x: x, y: y, at: at}
	t.sampled = min(t.sampled+1, len(t.samples))
}

// velocity is the finger's speed over its recent motion, in logical pixels a
// second; a finger that stopped before it lifted has none.
func (t *Touch) velocity(now time.Duration) (float32, float32) {
	if t.sampled < 2 || now-t.samples[0].at > velocityWindow/2 {
		return 0, 0
	}
	newest, oldest := t.samples[0], t.samples[0]
	for _, sample := range t.samples[1:t.sampled] {
		if newest.at-sample.at > velocityWindow {
			break
		}
		oldest = sample
	}
	seconds := float32((newest.at - oldest.at).Seconds())
	if seconds <= 0 {
		return 0, 0
	}
	return (newest.x - oldest.x) / seconds, (newest.y - oldest.y) / seconds
}

// tapCount numbers this press among quick taps close together, as a double
// click is counted.
func (t *Touch) tapCount(x, y float32, now time.Duration) int {
	if t.clicks > 0 && now-t.lastTap <= touchDoubleTap &&
		math.Hypot(float64(x-t.lastTapX), float64(y-t.lastTapY)) <= touchDoubleTapSlop {
		t.clicks = t.clicks%3 + 1
	} else {
		t.clicks = 1
	}
	t.lastTap, t.lastTapX, t.lastTapY = now, x, y
	return t.clicks
}
