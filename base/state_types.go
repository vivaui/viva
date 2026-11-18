package base

import (
	"time"

	"github.com/vivaui/viva/gfx"
)

// ColorPickerValue is caller-owned HSV editing state. Keeping HSV rather than
// rebuilding it from RGB on every paint preserves the hue at gray and black.
type ColorPickerValue struct {
	Hue, Saturation, Brightness, Alpha float32
}

// ColorPickerIDs remain stable across opening, resizing and value changes.
type ColorPickerIDs struct {
	Trigger, Dismiss, Saturation, Hue, Alpha, Hex, Clear, Cancel, Apply gfx.ID
}

type ColorPickerLabels struct {
	Color, Saturation, Hue, Alpha, Hex, Clear, Cancel, Apply string
}

// ColorPickerLayout is also the input geometry. Pointer and Key update a copy
// of caller-owned HSV state, using logical pixels at every display scale.
type ColorPickerLayout struct {
	Content, Saturation, Hue, Alpha, Hex gfx.Rect
	IDs                                  ColorPickerIDs
	Disabled, ShowAlpha                  bool
}

func (v ColorPickerValue) Normalized() ColorPickerValue {
	if !(v.Hue >= 0) {
		v.Hue = 0
	}
	v.Hue = min(v.Hue, 360)
	v.Saturation, v.Brightness, v.Alpha = pickerUnit(v.Saturation), pickerUnit(v.Brightness), pickerUnit(v.Alpha)
	return v
}

// Color converts the selected HSV value to straight RGBA without allocating.
func (v ColorPickerValue) Color() gfx.Color {
	v = v.Normalized()
	h := v.Hue / 60
	sector := int(h)
	f := h - float32(sector)
	p, q, t := v.Brightness*(1-v.Saturation), v.Brightness*(1-v.Saturation*f), v.Brightness*(1-v.Saturation*(1-f))
	r, g, b := v.Brightness, t, p
	switch sector % 6 {
	case 1:
		r, g, b = q, v.Brightness, p
	case 2:
		r, g, b = p, v.Brightness, t
	case 3:
		r, g, b = p, q, v.Brightness
	case 4:
		r, g, b = t, p, v.Brightness
	case 5:
		r, g, b = v.Brightness, p, q
	}
	return gfx.Color{Red: r, Green: g, Blue: b, Alpha: v.Alpha}
}

// WithColor loads an RGB edit, retaining hue for achromatic colors and the
// previous saturation at black so a subsequent brightness change is useful.
func (v ColorPickerValue) WithColor(c gfx.Color) ColorPickerValue {
	v = v.Normalized()
	r, g, b := pickerUnit(c.Red), pickerUnit(c.Green), pickerUnit(c.Blue)
	hi, lo := max(r, g, b), min(r, g, b)
	d := hi - lo
	v.Brightness, v.Alpha = hi, pickerUnit(c.Alpha)
	if hi > 0 {
		v.Saturation = d / hi
	}
	if d > 0 {
		switch hi {
		case r:
			v.Hue = 60 * (g - b) / d
		case g:
			v.Hue = 60 * (2 + (b-r)/d)
		default:
			v.Hue = 60 * (4 + (r-g)/d)
		}
		if v.Hue < 0 {
			v.Hue += 360
		}
	}
	return v
}

// AppendHex appends canonical #RRGGBB (or #RRGGBBAA) to caller-owned storage.
// Format only when a value changes; painting consumes the caller's input text.
func (v ColorPickerValue) AppendHex(dst []byte, alpha bool) []byte {
	c := v.Color()
	dst = append(dst, '#')
	channels := [4]float32{c.Red, c.Green, c.Blue, c.Alpha}
	n := 3
	if alpha {
		n = 4
	}
	const digits = "0123456789ABCDEF"
	for _, channel := range channels[:n] {
		b := byte(channel*255 + .5)
		dst = append(dst, digits[b>>4], digits[b&15])
	}
	return dst
}

// Pointer handles primary press and captured dragging, including outside the
// popup. Other buttons and hover do not change the draft or steal text editing.
func (l ColorPickerLayout) Pointer(event gfx.PointerEvent, v ColorPickerValue) (ColorPickerValue, gfx.EventResult) {
	if l.Disabled || l.Content.Width <= 0 || event.Kind != gfx.PointerDown && !(event.Kind == gfx.PointerMove && event.Captured) {
		return v, gfx.EventResult{}
	}
	if event.Kind == gfx.PointerDown && event.Button != gfx.PointerButtonPrimary && event.Button != gfx.PointerButtonNone {
		return v, gfx.EventResult{}
	}
	old := v
	switch event.Target {
	case l.IDs.Saturation:
		if l.Saturation.Width <= 0 || l.Saturation.Height <= 0 {
			return v, gfx.EventResult{}
		}
		v.Saturation = pickerUnit((event.X - l.Saturation.X) / l.Saturation.Width)
		v.Brightness = 1 - pickerUnit((event.Y-l.Saturation.Y)/l.Saturation.Height)
	case l.IDs.Hue:
		if l.Hue.Width <= 0 {
			return v, gfx.EventResult{}
		}
		v.Hue = 360 * pickerUnit((event.X-l.Hue.X)/l.Hue.Width)
	case l.IDs.Alpha:
		if !l.ShowAlpha || l.Alpha.Width <= 0 {
			return v, gfx.EventResult{}
		}
		v.Alpha = pickerUnit((event.X - l.Alpha.X) / l.Alpha.Width)
	default:
		return v, gfx.EventResult{}
	}
	return v, gfx.EventResult{Handled: true, Repaint: v != old, Focus: event.Target}
}

// Key provides two-axis arrows on the color plane and scalar slider keys on
// hue/alpha. Shift uses larger steps. Text-field and action keys stay native.
func (l ColorPickerLayout) Key(event gfx.KeyEvent, v ColorPickerValue) (ColorPickerValue, gfx.EventResult) {
	if l.Disabled || l.Content.Width <= 0 || event.Release || event.Modifiers & ^gfx.ModifierShift != 0 {
		return v, gfx.EventResult{}
	}
	old := v
	step := float32(.01)
	if event.Modifiers&gfx.ModifierShift != 0 {
		step = .1
	}
	switch event.Target {
	case l.IDs.Saturation:
		switch event.Key {
		case gfx.KeyLeft:
			v.Saturation -= step
		case gfx.KeyRight:
			v.Saturation += step
		case gfx.KeyUp:
			v.Brightness += step
		case gfx.KeyDown:
			v.Brightness -= step
		case gfx.KeyHome:
			v.Saturation = 0
		case gfx.KeyEnd:
			v.Saturation = 1
		default:
			return v, gfx.EventResult{}
		}
	case l.IDs.Hue, l.IDs.Alpha:
		value, maximum, unit, large := v.Hue, float32(360), float32(1), float32(10)
		if event.Target == l.IDs.Alpha {
			if !l.ShowAlpha {
				return v, gfx.EventResult{}
			}
			value, maximum, unit, large = v.Alpha, 1, .01, .1
		}
		next, ok := SliderKeyDownLarge(value, 0, maximum, unit, large, WritingLTR, event.Key, event.Modifiers)
		if !ok {
			return v, gfx.EventResult{}
		}
		if event.Target == l.IDs.Hue {
			v.Hue = next
		} else {
			v.Alpha = next
		}
	default:
		return v, gfx.EventResult{}
	}
	v = v.Normalized()
	return v, gfx.EventResult{Handled: true, Repaint: v != old, Focus: event.Target, FocusVisible: true}
}

// IconHover is caller-owned hover state for an icon-only control whose glyph
// plays one short gesture. Call Progress once each time the control paints, with
// the control's stable ID. The pointer must rest for dwell before the gesture
// starts, so crossing a toolbar does not set every icon in motion; once started,
// the gesture finishes even if the pointer leaves. A second hover replays it
// only after the first pass has ended.
//
// The state owns no timer. Progress asks the canvas for the wake-up that ends the
// dwell, and PlayOnce asks for frames while the gesture runs. The result is the
// gesture's linear progress in [0, 1]; the caller maps it onto a transform.
type IconHoverState struct {
	target gfx.ID
	since  time.Duration
	// fired marks the slots that already played for target, so a pointer that
	// stays put does not replay them. One bit per slot: slot is a uint8, so
	// four words cover every value and nothing allocates. A slot is cleared
	// when the pointer reaches or leaves target, which re-arms every gesture
	// on it.
	fired [4]uint64
}

// played reports whether slot already played its gesture for the current target.
func (hover *IconHoverState) played(slot uint8) bool {
	return hover.fired[slot/64]>>(slot%64)&1 != 0
}

// markPlayed records that slot played its gesture for the current target.
func (hover *IconHoverState) markPlayed(slot uint8) {
	hover.fired[slot/64] |= 1 << (slot % 64)
}

// reArm clears every slot's played mark, so the next hover replays them.
func (hover *IconHoverState) reArm() {
	hover.fired = [4]uint64{}
}

// Progress advances one control's hover gesture. id must be the ID under which
// the control declares its hover region; slot separates several gestures owned
// by that ID, and each slot's dwell and one-shot are tracked on their own. dwell
// is how long the pointer must rest before the gesture starts, and duration is
// the gesture itself. A zero ID paints still, which makes the value safe to use
// from a static preview.
func (hover *IconHoverState) Progress(canvas *gfx.Canvas, id gfx.ID, slot uint8, dwell, duration time.Duration) (progress float32, playing bool) {
	if hover == nil || canvas == nil || id.IsZero() {
		return 1, false
	}
	// PlayOnce owns a running gesture: it finishes the pass even when the
	// pointer leaves, and it holds the trigger until the pointer is gone.
	if progress, playing = canvas.PlayOnce(id, slot, false, duration); playing {
		return progress, true
	}
	now := canvas.Now()
	hovered := canvas.Hovered() == id
	if hovered && hover.target != id {
		hover.target = id
		hover.since = now
		hover.reArm()
	}
	if !hovered && hover.target != (gfx.ID{}) {
		hover.target = gfx.ID{}
		hover.reArm()
	}
	if !hovered {
		return 1, false
	}
	if remaining := dwell - (now - hover.since); remaining > 0 {
		canvas.RequestFrameIn(remaining)
		return 1, false
	}
	if hover.played(slot) || canvas.ReduceMotion() {
		return 1, false
	}
	hover.markPlayed(slot)
	return canvas.PlayOnce(id, slot, true, duration)
}

// SonnerToast is one caller-owned toast snapshot plus stable interaction IDs.
// The caller owns insertion, expiry, swipe state, and removal.
type SonnerToast struct {
	Snapshot    ToastSnapshot
	Scope       gfx.ID
	DismissID   gfx.ID
	ActionID    gfx.ID
	ActionLabel string
	Disabled    bool
}

// ToastSnapshot is the complete caller-owned state painted for one frame.
// Timing, expiry, swipe interpretation, and stack membership live outside the
// component.
type ToastSnapshot struct {
	Title       string
	Description string
	Type        ToastKind
	Index       uint16
	Expanded    bool
	Limited     bool
	OffsetY     float32
	SwipeX      float32
	SwipeY      float32
}

// ToastQueueEntry is one live toast as the queue sees it.
type ToastQueueEntry struct {
	ID        gfx.ID
	Loading   bool
	Paused    bool
	Remaining time.Duration // zero for a loading toast or after the deadline
}

type toastQueueEntry struct {
	id        gfx.ID
	deadline  time.Time
	remaining time.Duration
	loading   bool
	paused    bool
}

// ToastQueue is caller-owned toast lifetime policy: Base UI's five-second
// timeout, pause on hover/focus/window blur, loading toasts that do not expire,
// and a visible limit. It never starts a timer; the caller asks Expire when
// NextDeadline comes due and pauses or resumes as the pointer and focus move.
// Painting remains PaintSonner's job.
type ToastQueue struct {
	// Timeout is the lifetime of a new non-loading toast; zero means
	// ToastQueueTimeout.
	Timeout time.Duration
	// Limit is the maximum number of queued toasts; zero means ToastQueueLimit.
	Limit   int
	entries []toastQueueEntry
}

func (queue *ToastQueue) timeout() time.Duration {
	if queue.Timeout > 0 {
		return queue.Timeout
	}
	return ToastQueueTimeout
}

func (queue *ToastQueue) limit() int {
	if queue.Limit > 0 {
		return queue.Limit
	}
	return ToastQueueLimit
}

// Add enqueues a toast. When the limit is full it drops the oldest and returns
// its ID so the caller can paint its exit. Add reports false for a zero ID or
// one that is already queued.
func (queue *ToastQueue) Add(id gfx.ID, now time.Time, loading bool) (evicted gfx.ID, ok bool) {
	if id.IsZero() {
		return gfx.ID{}, false
	}
	for index := range queue.entries {
		if queue.entries[index].id == id {
			return gfx.ID{}, false
		}
	}
	if len(queue.entries) >= queue.limit() {
		evicted = queue.entries[0].id
		queue.entries = queue.entries[1:]
	}
	entry := toastQueueEntry{id: id, loading: loading}
	if !loading {
		entry.deadline = now.Add(queue.timeout())
	}
	queue.entries = append(queue.entries, entry)
	return evicted, true
}

// Remove drops a toast, as its close button or a swipe does.
func (queue *ToastQueue) Remove(id gfx.ID) bool {
	for index := range queue.entries {
		if queue.entries[index].id == id {
			queue.entries = append(queue.entries[:index], queue.entries[index+1:]...)
			return true
		}
	}
	return false
}

// SetLoading changes whether a toast expires. A toast that stops loading gets
// a fresh timeout; one that starts loading never expires.
func (queue *ToastQueue) SetLoading(id gfx.ID, loading bool, now time.Time) bool {
	for index := range queue.entries {
		entry := &queue.entries[index]
		if entry.id != id {
			continue
		}
		entry.loading = loading
		entry.paused = false
		entry.remaining = 0
		if loading {
			entry.deadline = time.Time{}
		} else {
			entry.deadline = now.Add(queue.timeout())
		}
		return true
	}
	return false
}

// Pause stops a toast's clock, remembering how much time was left, as hovering
// or focusing its viewport does.
func (queue *ToastQueue) Pause(id gfx.ID, now time.Time) bool {
	for index := range queue.entries {
		entry := &queue.entries[index]
		if entry.id != id || entry.paused || entry.loading || entry.deadline.IsZero() {
			continue
		}
		entry.remaining = max(entry.deadline.Sub(now), 0)
		entry.deadline = time.Time{}
		entry.paused = true
		return true
	}
	return false
}

// Resume restarts a paused toast's remaining time.
func (queue *ToastQueue) Resume(id gfx.ID, now time.Time) bool {
	for index := range queue.entries {
		entry := &queue.entries[index]
		if entry.id != id || !entry.paused {
			continue
		}
		entry.deadline = now.Add(entry.remaining)
		entry.remaining = 0
		entry.paused = false
		return true
	}
	return false
}

// PauseAll stops every toast, as losing window focus does.
func (queue *ToastQueue) PauseAll(now time.Time) {
	for index := range queue.entries {
		queue.Pause(queue.entries[index].id, now)
	}
}

// ResumeAll restarts every paused toast.
func (queue *ToastQueue) ResumeAll(now time.Time) {
	for index := range queue.entries {
		queue.Resume(queue.entries[index].id, now)
	}
}

// Expire removes and returns the first toast whose time is up. Loading and
// paused toasts never expire. Call it after NextDeadline comes due; it reports
// false when nothing is due.
func (queue *ToastQueue) Expire(now time.Time) (gfx.ID, bool) {
	for index := range queue.entries {
		entry := &queue.entries[index]
		if entry.paused || entry.loading || entry.deadline.IsZero() || now.Before(entry.deadline) {
			continue
		}
		id := entry.id
		queue.entries = append(queue.entries[:index], queue.entries[index+1:]...)
		return id, true
	}
	return gfx.ID{}, false
}

// NextDeadline is the earliest time an unpaused, non-loading toast expires, for
// scheduling one frame. It reports false when no toast has a deadline.
func (queue *ToastQueue) NextDeadline() (time.Time, bool) {
	var next time.Time
	for index := range queue.entries {
		entry := queue.entries[index]
		if entry.paused || entry.loading || entry.deadline.IsZero() {
			continue
		}
		if next.IsZero() || entry.deadline.Before(next) {
			next = entry.deadline
		}
	}
	return next, !next.IsZero()
}

// Entries copies the live toasts into dst in queue order (oldest first) and
// returns the result, for building ToastSnapshots without allocating when dst
// has room.
func (queue *ToastQueue) Entries(now time.Time, dst []ToastQueueEntry) []ToastQueueEntry {
	dst = dst[:0]
	for index := range queue.entries {
		entry := queue.entries[index]
		remaining := entry.remaining
		if !entry.paused && !entry.loading && !entry.deadline.IsZero() {
			remaining = max(entry.deadline.Sub(now), 0)
		}
		dst = append(dst, ToastQueueEntry{ID: entry.id, Loading: entry.loading, Paused: entry.paused, Remaining: remaining})
	}
	return dst
}

// Len is how many toasts are queued.
func (queue *ToastQueue) Len() int { return len(queue.entries) }

// Helper copies the moved types use; components keeps its own for its painters.

func pickerUnit(v float32) float32 {
	if v != v || v < 0 {
		return 0
	}
	v = min(v, 1)
	return v
}

// SliderKeyDownLarge applies Base UI's slider keyboard pattern with a
// configurable large step: the arrows move by step (Right and Up up, Left and
// Down down; Left and Right swap in RTL), Shift with an arrow and PageUp and
// PageDown by largeStep, and Home and End to the minimum and maximum.
func SliderKeyDownLarge(value, minimum, maximum, step, largeStep float32, direction WritingDirection, key gfx.Key, modifiers gfx.Modifiers) (float32, bool) {
	unit := step
	if unit <= 0 {
		unit = 1
	}
	if largeStep <= 0 {
		largeStep = 10
	}
	jump := unit
	if modifiers&gfx.ModifierShift != 0 {
		jump = largeStep
	}
	var next float32
	switch key {
	case gfx.KeyRight, gfx.KeyLeft:
		forward := key == gfx.KeyRight
		if direction.IsRTL() {
			forward = !forward
		}
		next = value - jump
		if forward {
			next = value + jump
		}
	case gfx.KeyUp:
		next = value + jump
	case gfx.KeyDown:
		next = value - jump
	case gfx.KeyPageUp:
		next = value + largeStep
	case gfx.KeyPageDown:
		next = value - largeStep
	case gfx.KeyHome:
		next = minimum
	case gfx.KeyEnd:
		next = maximum
	default:
		return value, false
	}
	return snapSliderValue(next, minimum, maximum, step), true
}

func snapSliderValue(value, minimum, maximum, step float32) float32 {
	if maximum <= minimum {
		return minimum
	}
	if value < minimum {
		value = minimum
	}
	if value > maximum {
		value = maximum
	}
	if step > 0 {
		steps := (value - minimum) / step
		index := int(steps + 0.5)
		value = minimum + float32(index)*step
		if value > maximum {
			value = maximum
		}
	}
	return value
}

// ToastQueueTimeout and ToastQueueLimit are Base UI's default toast lifetime
// (5 seconds) and visible limit.
const (
	ToastQueueTimeout = 5 * time.Second
	ToastQueueLimit   = 3
)
