package ui

import (
	"math"
	"time"
)

// Easing selects a cubic-bezier timing curve for a transition. The zero value
// is the standard curve used by Tailwind's default `transition`.
type Easing uint8

const (
	// EaseInOut is cubic-bezier(0.4, 0, 0.2, 1), the default state-change curve.
	EaseInOut Easing = iota
	// EaseOut is cubic-bezier(0, 0, 0.2, 1) for entering elements.
	EaseOut
	// EaseIn is cubic-bezier(0.4, 0, 1, 1) for leaving elements.
	EaseIn
	// EaseLinear is the identity curve.
	EaseLinear
	// EaseCSS is the CSS keyword `ease`, cubic-bezier(0.25, 0.1, 0.25, 1): the
	// curve Tailwind's animate-in and animate-out use for a popup entering or
	// leaving.
	EaseCSS
	// EaseEmphasized is cubic-bezier(0.22, 1, 0.36, 1), a long, soft landing: the
	// slide of a drawer or a toast.
	EaseEmphasized
)

// Motion is one timed state change. A zero Duration is an instant change, so
// hand-built themes that omit motion tokens stay static rather than animated.
type Motion struct {
	Duration time.Duration
	Easing   Easing
}

// DefaultMotion returns the 150ms standard-curve transition shadcn components
// use for color, position, and opacity changes of a control.
func DefaultMotion() Motion {
	return Motion{Duration: 150 * time.Millisecond, Easing: EaseInOut}
}

// bezier holds the two inner control points of a CSS cubic-bezier curve whose
// end points are fixed at (0, 0) and (1, 1).
type bezier struct{ x1, y1, x2, y2 float64 }

func (curve bezier) coordinate(a, b, s float64) float64 {
	inverse := 1 - s
	return 3*inverse*inverse*s*a + 3*inverse*s*s*b + s*s*s
}

func (curve bezier) slope(a, b, s float64) float64 {
	inverse := 1 - s
	return 3*inverse*inverse*a + 6*inverse*s*(b-a) + 3*s*s*(1-b)
}

// solve returns y at the point of the curve whose x equals progress. Newton's
// method converges in a few steps for these shallow curves; bisection covers
// the flat-slope cases deterministically.
func (curve bezier) solve(progress float64) float64 {
	const tolerance = 1e-6
	s := progress
	for range 8 {
		delta := curve.coordinate(curve.x1, curve.x2, s) - progress
		if math.Abs(delta) < tolerance {
			return curve.coordinate(curve.y1, curve.y2, s)
		}
		slope := curve.slope(curve.x1, curve.x2, s)
		if math.Abs(slope) < tolerance {
			break
		}
		s = math.Min(1, math.Max(0, s-delta/slope))
	}
	low, high := 0.0, 1.0
	s = progress
	for range 32 {
		delta := curve.coordinate(curve.x1, curve.x2, s) - progress
		if math.Abs(delta) < tolerance {
			break
		}
		if delta > 0 {
			high = s
		} else {
			low = s
		}
		s = (low + high) / 2
	}
	return curve.coordinate(curve.y1, curve.y2, s)
}

// At maps linear progress in [0, 1] onto the eased curve. Inputs outside the
// range clamp to the exact end points.
func (easing Easing) At(progress float32) float32 {
	if progress <= 0 {
		return 0
	}
	if progress >= 1 {
		return 1
	}
	switch easing {
	case EaseInOut:
		return float32(bezier{0.4, 0, 0.2, 1}.solve(float64(progress)))
	case EaseOut:
		return float32(bezier{0, 0, 0.2, 1}.solve(float64(progress)))
	case EaseIn:
		return float32(bezier{0.4, 0, 1, 1}.solve(float64(progress)))
	case EaseCSS:
		return float32(bezier{0.25, 0.1, 0.25, 1}.solve(float64(progress)))
	case EaseEmphasized:
		return float32(bezier{0.22, 1, 0.36, 1}.solve(float64(progress)))
	}
	return progress
}

// Curve maps linear progress in [0, 1] onto eased progress. It is the timing of
// one segment of a motion a component runs itself: a keyframed animation whose
// segments are eased differently is not one Easing, so it asks Animate for
// linear progress and evaluates its keyframes with curves like CubicBezier.
type Curve func(progress float32) float32

// CubicBezier returns the CSS curve cubic-bezier(x1, y1, x2, y2), whose end
// points are fixed at (0, 0) and (1, 1). x1 and x2 are clamped to [0, 1] so the
// result stays a function of time; y is free, which is how a curve overshoots
// or holds. Progress outside [0, 1] returns the exact end points.
func CubicBezier(x1, y1, x2, y2 float32) Curve {
	curve := bezier{
		x1: float64(min(max(x1, 0), 1)), y1: float64(y1),
		x2: float64(min(max(x2, 0), 1)), y2: float64(y2),
	}
	return func(progress float32) float32 {
		switch {
		case progress <= 0:
			return 0
		case progress >= 1:
			return 1
		}
		return float32(curve.solve(float64(progress)))
	}
}

// Spring returns the step response of a spring, as a curve over the progress of
// one segment: it overshoots 1 and settles back onto it. bounce runs from 0, a
// critically damped settle with no overshoot at all, to 1, a wide wobble that
// takes most of the segment to calm down. The curve reaches exactly 1 at
// progress 1, so a finished motion paints the still glyph.
//
// A spring is a function of the time a motion took rather than of the distance
// it covered, so it belongs on the progress of one segment — the value a
// keyframe segment eases from and to — which is what Curve is for.
func Spring(bounce float32) Curve {
	const angular = 12.0 // radians over the segment: a spring at rest well before 1
	ratio := 1 - 0.65*float64(min(max(bounce, 0), 1))
	decay := ratio * angular
	damped := angular * math.Sqrt(max(1-ratio*ratio, 0))
	return func(progress float32) float32 {
		switch {
		case progress <= 0:
			return 0
		case progress >= 1:
			return 1
		}
		time := float64(progress)
		envelope := math.Exp(-decay * time)
		if damped == 0 {
			return float32(1 - envelope*(1+decay*time))
		}
		return float32(1 - envelope*(math.Cos(damped*time)+decay/damped*math.Sin(damped*time)))
	}
}

// Lerp interpolates from one scalar to another. t is clamped to [0, 1] and the
// end points are returned exactly, so a finished transition paints the same
// pixels as a static state.
func Lerp(from, to, t float32) float32 {
	if t <= 0 {
		return from
	}
	if t >= 1 {
		return to
	}
	return from + (to-from)*t
}

// Mix interpolates from color to other in premultiplied alpha, like CSS color
// transitions, so a fade to a translucent color does not pass through a
// darker or lighter fringe. The end points are returned exactly.
func (color Color) Mix(other Color, t float32) Color {
	if t <= 0 {
		return color
	}
	if t >= 1 {
		return other
	}
	alpha := Lerp(color.Alpha, other.Alpha, t)
	if alpha <= 0 {
		return Color{}
	}
	channel := func(from, to float32) float32 {
		return Lerp(from*color.Alpha, to*other.Alpha, t) / alpha
	}
	return Color{
		Red:   channel(color.Red, other.Red),
		Green: channel(color.Green, other.Green),
		Blue:  channel(color.Blue, other.Blue),
		Alpha: alpha,
	}
}

// Over composites color onto backdrop with ordinary alpha blending, the same
// arithmetic the GPU applies when both are drawn, and returns the result. A
// recipe uses it to flatten a translucent token before handing it to
// Canvas.RoundedBorder, which paints its border color as a solid backing shape:
// left translucent, that color would tint the whole interior.
func (color Color) Over(backdrop Color) Color {
	if color.Alpha >= 1 {
		return color
	}
	if color.Alpha <= 0 {
		return backdrop
	}
	alpha := color.Alpha + backdrop.Alpha*(1-color.Alpha)
	channel := func(top, bottom float32) float32 {
		return (top*color.Alpha + bottom*backdrop.Alpha*(1-color.Alpha)) / alpha
	}
	return Color{
		Red:   channel(color.Red, backdrop.Red),
		Green: channel(color.Green, backdrop.Green),
		Blue:  channel(color.Blue, backdrop.Blue),
		Alpha: alpha,
	}
}

// motionSlot is one animated scalar keyed by hit ID and a component-local
// slot number. It stores only the active segment, never a history.
type motionSlot struct {
	id       ID
	slot     uint8
	seen     uint32
	from     float32
	to       float32
	start    time.Duration
	duration time.Duration
	easing   Easing
}

// onceSlot is one PlayOnce timeline. Unlike motionSlot it never retargets: it
// runs from zero to one once and then holds at rest.
type onceSlot struct {
	id       ID
	slot     uint8
	seen     uint32
	start    time.Duration
	duration time.Duration
	active   bool
	// triggered remembers that trigger was already true, so a held trigger is
	// one gesture and only a fresh press of it starts another.
	triggered bool
}

func (slot *motionSlot) value(now time.Duration) float32 {
	elapsed := now - slot.start
	if slot.duration <= 0 || elapsed >= slot.duration {
		return slot.to
	}
	if elapsed <= 0 {
		return slot.from
	}
	progress := float32(float64(elapsed) / float64(slot.duration))
	return Lerp(slot.from, slot.to, slot.easing.At(progress))
}

func (slot *motionSlot) settled(now time.Duration) bool {
	return slot.duration <= 0 || now-slot.start >= slot.duration || slot.from == slot.to
}

// motionState is the canvas-owned transition table. Slots live in paint order
// so a stable layout resolves each Animate call with one index comparison.
type motionState struct {
	slots     []motionSlot
	onces     []onceSlot
	cursor    int
	onceAt    int
	clock     func() time.Duration
	now       time.Duration
	frame     uint32
	sampled   bool
	active    bool
	animating bool
	// wake is the soonest delayed repaint asked for with RequestFrameIn this
	// frame; zero means none.
	wake   time.Duration
	reduce bool
	// presences are the overlays that are open or still leaving, and open the
	// groups the current paint pass has begun and not yet ended (presence.go).
	presences []presenceEntry
	open      []presenceOpen
	// hitMotions keeps the placement of hits painted inside a moving presence
	// for this frame, so native overlays anchored to them move with the paint.
	hitMotions []hitMotionEntry
}

// frameTime returns the time of the current frame, reading the clock on first
// use only. A frame that paints no animated widget never touches the clock,
// so an application without transitions pays nothing for the capability.
func (state *motionState) frameTime() time.Duration {
	if !state.sampled {
		state.sampled = true
		if now := state.clock(); now > state.now {
			state.now = now
		}
	}
	return state.now
}

const (
	motionShrinkCapacity = 256
	motionShrinkFactor   = 4
)

func (state *motionState) slotFor(id ID, slot uint8, initial float32, now time.Duration) *motionSlot {
	index := state.cursor
	if index >= len(state.slots) || state.slots[index].id != id || state.slots[index].slot != slot {
		index = -1
		for candidate := range state.slots {
			if state.slots[candidate].id == id && state.slots[candidate].slot == slot {
				index = candidate
				break
			}
		}
		if index < 0 {
			// First sighting starts settled at its target, so a widget that
			// appears (or reappears after being hidden) never animates in.
			index = min(state.cursor, len(state.slots))
			state.slots = append(state.slots, motionSlot{})
			copy(state.slots[index+1:], state.slots[index:])
			state.slots[index] = motionSlot{id: id, slot: slot, from: initial, to: initial, start: now}
		}
	}
	state.cursor = index + 1
	return &state.slots[index]
}

// sweep drops slots no widget touched during the finished paint pass, so
// memory follows live widgets rather than history.
func (state *motionState) sweep() {
	kept := state.slots[:0]
	for _, slot := range state.slots {
		if slot.seen == state.frame {
			kept = append(kept, slot)
		}
	}
	clear(state.slots[len(kept):])
	state.slots = kept
	if cap(state.slots) > motionShrinkCapacity && len(state.slots)*motionShrinkFactor < cap(state.slots) {
		state.slots = append(make([]motionSlot, 0, len(state.slots)*2), state.slots...)
	}
	onces := state.onces[:0]
	for _, once := range state.onces {
		if once.seen == state.frame {
			onces = append(onces, once)
		}
	}
	clear(state.onces[len(onces):])
	state.onces = onces
	if cap(state.onces) > motionShrinkCapacity && len(state.onces)*motionShrinkFactor < cap(state.onces) {
		state.onces = append(make([]onceSlot, 0, len(state.onces)*2), state.onces...)
	}
}

// SetClock installs the monotonic time source transitions are measured on.
// Without a clock, or outside BeginFrame and EndFrame, Animate is a
// pass-through and every component paints its final state, which keeps tests,
// static previews, and runtimes without a frame loop deterministic.
func (canvas *Canvas) SetClock(clock func() time.Duration) {
	canvas.motion.clock = clock
}

// BeginFrame opens one animated paint pass. Runtimes call it once before the
// paint callback. The clock is read at most once per frame, and only when a
// widget actually animates, so every transition in the frame shares one time.
func (canvas *Canvas) BeginFrame() {
	state := &canvas.motion
	state.frame++
	state.cursor = 0
	state.onceAt = 0
	state.sampled = false
	state.active = state.clock != nil
	state.animating = false
	state.wake = 0
	state.open = state.open[:0]
}

// EndFrame closes the paint pass opened by BeginFrame and forgets transitions
// whose widgets were not painted.
func (canvas *Canvas) EndFrame() {
	canvas.motion.leave(canvas)
	canvas.motion.active = false
	canvas.motion.sweep()
}

// Animating reports whether the last frame left a transition in flight. The
// runtime must paint another frame; when it is false the canvas is at rest and
// nothing needs to be scheduled.
func (canvas *Canvas) Animating() bool {
	return canvas.motion.animating
}

// HasClock reports whether a frame clock is installed. A widget that would
// otherwise pace itself asks this before doing so: without a clock nothing
// advances, so it paints its resting state and asks for no frame. Tests,
// static previews and runtimes without a frame loop have none.
func (canvas *Canvas) HasClock() bool {
	return canvas.motion.clock != nil
}

// RequestFrame marks the current frame as unfinished for painters that run
// their transitions elsewhere, such as a development reload host mirroring a
// worker canvas. The runtime paints again at display cadence until a frame
// stops asking.
func (canvas *Canvas) RequestFrame() {
	canvas.motion.animating = true
}

// RequestFrameIn asks for one more paint after at least delay, for a display
// that changes slowly: a spinner that steps every 125 ms, a pulse, a clock.
// Painting at display cadence for those would redraw a hundred times for every
// visible change. Like RequestFrame it has to be repeated by every frame that
// still needs the next one; a frame that does not ask leaves the application
// idle. A delay of zero or less is RequestFrame. When several are made in one
// frame the soonest wins.
func (canvas *Canvas) RequestFrameIn(delay time.Duration) {
	if delay <= 0 {
		canvas.motion.animating = true
		return
	}
	if state := &canvas.motion; state.wake == 0 || delay < state.wake {
		state.wake = delay
	}
}

// NextFrame says when the runtime should paint again: after a zero delay,
// paced to the display, while a transition is in flight, after the requested
// delay when only RequestFrameIn asked, and not at all (ok false) when the
// canvas is at rest.
func (canvas *Canvas) NextFrame() (delay time.Duration, ok bool) {
	switch state := &canvas.motion; {
	case state.animating:
		return 0, true
	case state.wake > 0:
		return state.wake, true
	}
	return 0, false
}

// Now returns the monotonic time of the current frame, sampling the clock if
// no widget has yet. Painters that run transitions elsewhere, such as the
// development reload host, forward it to the process that does.
func (canvas *Canvas) Now() time.Duration {
	if canvas.motion.clock == nil {
		return canvas.motion.now
	}
	return canvas.motion.frameTime()
}

// SetReduceMotion makes every transition jump straight to its target, honoring
// the operating system's reduced-motion accessibility preference.
func (canvas *Canvas) SetReduceMotion(reduce bool) {
	canvas.motion.reduce = reduce
}

// ReduceMotion reports whether transitions are suppressed.
func (canvas *Canvas) ReduceMotion() bool {
	return canvas.motion.reduce
}

// Animate returns the displayed value of one animated scalar. Components pass
// the value their state asks for and paint with what comes back: a toggle
// passes 0 or 1 and positions its thumb with the result. A change of target
// starts a transition from the value currently shown, so reversing mid-flight
// is continuous. ID zero (static previews), a missing frame clock, reduced
// motion, and a zero duration all return target unchanged.
//
// Slot distinguishes several animated values owned by one hit ID. State is
// kept per canvas, allocation-free once warm, and dropped as soon as a frame
// does not paint the widget.
func (canvas *Canvas) Animate(id ID, slot uint8, target float32, motion Motion) float32 {
	state := &canvas.motion
	if !state.active || id.IsZero() {
		return target
	}
	now := state.frameTime()
	entry := state.slotFor(id, slot, target, now)
	entry.seen = state.frame
	if state.reduce || motion.Duration <= 0 {
		entry.from, entry.to, entry.duration = target, target, 0
		return target
	}
	if entry.to != target {
		entry.from = entry.value(now)
		entry.to = target
		entry.start = now
		entry.duration = motion.Duration
		entry.easing = motion.Easing
	}
	if !entry.settled(now) {
		state.animating = true
	}
	return entry.value(now)
}

// PlayOnce returns the progress of a one-shot gesture. A false-to-true edge of
// trigger starts a pass at zero; the pass then advances once and ends at one,
// where it is at rest. Holding trigger true plays it once rather than looping,
// and releasing it re-arms the next pass. A static preview, a missing frame
// clock, a reduced-motion session and a zero duration return one without keeping
// state, which lets a caller paint the still icon.
//
// Slot distinguishes gestures owned by one hit ID, as Animate's does.
func (canvas *Canvas) PlayOnce(id ID, slot uint8, trigger bool, duration time.Duration) (progress float32, playing bool) {
	state := &canvas.motion
	if !state.active || id.IsZero() {
		return 1, false
	}
	now := state.frameTime()
	entry := state.onceSlotFor(id, slot, now)
	entry.seen = state.frame
	if state.reduce || duration <= 0 {
		entry.active = false
		entry.triggered = trigger
		return 1, false
	}
	if trigger && !entry.triggered {
		entry.active, entry.start, entry.duration = true, now, duration
	}
	entry.triggered = trigger
	if !entry.active {
		return 1, false
	}
	elapsed := now - entry.start
	if elapsed >= entry.duration {
		entry.active = false
		return 1, false
	}
	if elapsed < 0 {
		elapsed = 0
	}
	state.animating = true
	return float32(float64(elapsed) / float64(entry.duration)), true
}

// onceSlotFor returns the timeline for id and slot, keeping existing timelines
// in paint order so a stable page has one pass over the list.
func (state *motionState) onceSlotFor(id ID, slot uint8, now time.Duration) *onceSlot {
	index := state.onceAt
	if index >= len(state.onces) || state.onces[index].id != id || state.onces[index].slot != slot {
		index = -1
		for candidate := range state.onces {
			if state.onces[candidate].id == id && state.onces[candidate].slot == slot {
				index = candidate
				break
			}
		}
		if index < 0 {
			index = min(state.onceAt, len(state.onces))
			state.onces = append(state.onces, onceSlot{})
			copy(state.onces[index+1:], state.onces[index:])
			state.onces[index] = onceSlot{id: id, slot: slot, start: now}
		}
	}
	state.onceAt = index + 1
	return &state.onces[index]
}
