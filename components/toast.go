package components

import (
	"time"

	"github.com/vivaui/viva/gfx"
)

// ToastType selects the official semantic icon treatment.
type ToastType uint8

const (
	ToastDefault ToastType = iota
	ToastSuccess
	ToastInfo
	ToastWarning
	ToastError
	ToastLoading
)

// ToastSnapshot is the complete caller-owned state painted for one frame.
// Timing, expiry, swipe interpretation, and stack membership live outside the
// component.
type ToastSnapshot struct {
	Title       string
	Description string
	Type        ToastType
	Index       uint16
	Expanded    bool
	Limited     bool
	OffsetY     float32
	SwipeX      float32
	SwipeY      float32
}

// ToastQueueTimeout and ToastQueueLimit are Base UI's default toast lifetime
// (5 seconds) and visible limit.
const (
	ToastQueueTimeout = 5 * time.Second
	ToastQueueLimit   = 3
)

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

// ToastPosition is the corner or edge of the viewport a toast stack grows from.
// Toasts stack away from it: up from a bottom position, down from a top one.
type ToastPosition uint8

const (
	ToastBottomRight ToastPosition = iota
	ToastBottomLeft
	ToastBottomCenter
	ToastTopRight
	ToastTopLeft
	ToastTopCenter
)

// ToastOptions contains stable action and overlay identities.
type ToastOptions struct {
	Layer       uint16
	Scope       gfx.ID
	DismissID   gfx.ID
	ActionID    gfx.ID
	ActionLabel string
	Disabled    bool
	// Position is where the toast sits; the zero value is the bottom right.
	Position ToastPosition
}

// ToastLayout exposes deterministic geometry for pointer routing and tests.
type ToastLayout struct {
	Bounds gfx.Rect
	Icon   gfx.Rect
	Body   gfx.Rect
	Action gfx.Rect
	Close  gfx.Rect
}

// Toast geometry, from Base UI's toast: a `max-w-sm` box 16px in from the
// viewport, `p-4` around content whose parts are `gap-3` apart, a title and a
// description `gap-1` apart, and a stack that peeks 12px per toast behind the
// front one (`--peek`) or opens with 12px between toasts (`--gap`).
const (
	toastMaxWidth  = float32(384)
	toastInset     = float32(16)
	toastPadding   = float32(16)
	toastItemGap   = float32(12)
	toastTextGap   = float32(4)
	toastPeek      = float32(12)
	toastStackGap  = float32(12)
	toastIconSize  = float32(16)
	toastCloseSize = float32(32)
)

// toastGeometry measures a toast without painting it: the width, the wrapped
// description, the actions' room and the height those make.
type toastGeometry struct {
	width, height float32
	text          float32 // height of the title and description lines
	bodyWidth     float32
	actionWidth   float32
	hasIcon       bool
}

func toastMeasure(canvas *gfx.Canvas, theme gfx.Theme, viewport gfx.Rect, snapshot ToastSnapshot, options ToastOptions, scratch *overlayLines) (toastGeometry, []string) {
	var geometry toastGeometry
	geometry.width = min(toastMaxWidth, viewport.Width-2*toastInset)
	if geometry.width <= 0 {
		return geometry, nil
	}
	metrics := theme.Metrics
	geometry.hasIcon = snapshot.Type != ToastDefault
	geometry.bodyWidth = geometry.width - 2*toastPadding - toastCloseSize - toastItemGap
	if geometry.hasIcon {
		geometry.bodyWidth -= toastIconSize + toastItemGap
	}
	if !options.ActionID.IsZero() && options.ActionLabel != "" {
		geometry.actionWidth = BeginButton(canvas, theme, gfx.ID{}, gfx.Rect{}).Small().SizeToFit(options.ActionLabel).BoundsWidth()
		geometry.bodyWidth -= geometry.actionWidth + toastItemGap
	}
	geometry.bodyWidth = max(geometry.bodyWidth, 0)
	lines := overlayWrap(canvas, scratch, snapshot.Description, metrics.FontSizeM, gfx.WeightRegular, geometry.bodyWidth)
	geometry.text = overlayLineHeight(metrics)
	if len(lines) > 0 {
		geometry.text += toastTextGap + float32(len(lines))*overlayLineHeight(metrics)
	}
	geometry.height = max(geometry.text, toastCloseSize) + 2*toastPadding
	return geometry, lines
}

// ToastHeight is how tall PaintToast makes the toast, for a caller that stacks
// toasts itself.
func ToastHeight(canvas *gfx.Canvas, theme gfx.Theme, viewport gfx.Rect, snapshot ToastSnapshot, options ToastOptions) float32 {
	var scratch overlayLines
	geometry, _ := toastMeasure(canvas, theme, viewport, snapshot, options, &scratch)
	return geometry.height
}

// PaintToast paints one toast snapshot at the viewport position the options
// name, bottom right by default. Its height follows its text: the description
// wraps. It schedules no work and retains no lifetime state, except that a
// loading toast asks for a frame every 100ms to turn its spinner, until it is
// no longer painted.
func PaintToast(canvas *gfx.Canvas, theme gfx.Theme, viewport gfx.Rect, snapshot ToastSnapshot, options ToastOptions) ToastLayout {
	if canvas == nil || viewport.Width <= 0 || viewport.Height <= 0 || snapshot.Limited {
		return ToastLayout{}
	}
	var scratch overlayLines
	geometry, lines := toastMeasure(canvas, theme, viewport, snapshot, options, &scratch)
	if geometry.width <= 0 {
		return ToastLayout{}
	}
	metrics := theme.Metrics
	width, height := geometry.width, geometry.height
	var x, y float32
	switch options.Position {
	case ToastBottomLeft, ToastTopLeft:
		x = viewport.X + toastInset
	case ToastBottomCenter, ToastTopCenter:
		x = viewport.X + (viewport.Width-width)/2
	default:
		x = viewport.X + viewport.Width - width - toastInset
	}
	stack := float32(snapshot.Index) * toastPeek
	if snapshot.Expanded {
		stack = float32(snapshot.Index) * (height + toastStackGap)
	}
	top := options.Position >= ToastTopRight
	if top {
		y = viewport.Y + toastInset + stack
	} else {
		y = viewport.Y + viewport.Height - height - toastInset - stack
	}
	x += snapshot.SwipeX
	y += snapshot.OffsetY + snapshot.SwipeY
	bounds := gfx.Rect{X: x, Y: y, Width: width, Height: height}
	layout := ToastLayout{Bounds: bounds}
	canvas.PushLayer(options.Layer)
	if !options.DismissID.IsZero() {
		// A toast may still be shown while a dialog or menu is open. Claim
		// Escape at the outer depth so the overlay the user is working in keeps
		// it; a focused toast is not modelled here.
		canvas.SetEscapeTarget(options.DismissID)
	}
	if !options.Scope.IsZero() {
		canvas.PushFocusScope(options.Scope)
	}
	canvas.BeginPresence(overlayKey(options.DismissID, options.Scope), overlaySurfacePart, toastPresence(theme, options.Position, height), centerOf(bounds))
	// sonner's own shadow, 0 4px 12px rgb(0 0 0 / 10%): the pinned shadcn snapshot has none.
	canvas.Shadow(bounds, metrics.RadiusSurface, gfx.ShadowStyle{OffsetY: 4, Blur: 12}, gfx.Color{Alpha: 0.10})
	canvas.RoundedBorder(bounds, metrics.RadiusSurface, 1, theme.Border, theme.Popover)

	// The icon, the text, the action and the close button are centered on the row
	// the padding leaves.
	row := gfx.Rect{X: bounds.X + toastPadding, Y: bounds.Y + toastPadding, Width: bounds.Width - 2*toastPadding, Height: bounds.Height - 2*toastPadding}
	textX := row.X
	if geometry.hasIcon {
		layout.Icon = gfx.Rect{X: row.X, Y: row.Y + (row.Height-toastIconSize)/2, Width: toastIconSize, Height: toastIconSize}
		paintToastIcon(canvas, theme, layout.Icon, snapshot.Type)
		textX += toastIconSize + toastItemGap
	}
	layout.Close = gfx.Rect{X: row.X + row.Width - toastCloseSize, Y: row.Y + (row.Height-toastCloseSize)/2, Width: toastCloseSize, Height: toastCloseSize}
	right := layout.Close.X - toastItemGap
	if geometry.actionWidth > 0 {
		button := BeginButton(canvas, theme, options.ActionID, gfx.Rect{X: right - geometry.actionWidth, Y: row.Y, Width: geometry.actionWidth}).Small()
		layout.Action = gfx.Rect{X: right - geometry.actionWidth, Y: row.Y + (row.Height-button.BoundsHeight())/2, Width: geometry.actionWidth, Height: button.BoundsHeight()}
		button.bounds = layout.Action
		if options.Disabled {
			button = button.Disabled()
		}
		button.paint(options.ActionLabel, ButtonOutline)
		right = layout.Action.X - toastItemGap
	}
	closeColor := theme.MutedForeground
	if options.Disabled {
		closeColor.Alpha *= 0.50
	}
	if !options.DismissID.IsZero() && !options.Disabled && canvas.Hovered() == options.DismissID {
		canvas.Rounded(layout.Close, metrics.RadiusControl, theme.Accent)
		closeColor = theme.AccentForeground
	}
	paintCross(canvas, layout.Close, 16, closeColor)
	if !options.DismissID.IsZero() {
		canvas.HitWithOptions(options.DismissID, layout.Close, "Close toast", gfx.HitOptions{Role: gfx.RoleButton, Disabled: options.Disabled})
	}

	layout.Body = gfx.Rect{X: textX, Y: row.Y + (row.Height-geometry.text)/2, Width: max(0, right-textX), Height: geometry.text}
	lineHeight := overlayLineHeight(metrics)
	canvas.Label(snapshot.Title, gfx.Rect{X: layout.Body.X, Y: layout.Body.Y, Width: layout.Body.Width, Height: lineHeight}, gfx.TextStyle{Size: metrics.FontSizeM, Weight: gfx.WeightMedium, Color: theme.PopoverForeground})
	overlayParagraph(canvas, lines, gfx.Rect{X: layout.Body.X, Y: layout.Body.Y + lineHeight + toastTextGap, Width: layout.Body.Width}, lineHeight, gfx.TextStyle{
		Size: metrics.FontSizeM, Weight: gfx.WeightRegular, Color: theme.MutedForeground,
	}, false)
	canvas.EndPresence()
	if !options.Scope.IsZero() {
		canvas.PopFocusScope()
	}
	canvas.PopLayer()
	return layout
}

// toastPresence is how a toast comes and goes: it slides from and to beyond its
// edge of the window, by 150% of its height, over 500ms (Base UI's
// `translateY(150%)` with `cubic-bezier(0.22, 1, 0.36, 1)`), without fading.
func toastPresence(theme gfx.Theme, position ToastPosition, height float32) gfx.Presence {
	if !moving(theme) {
		return gfx.Presence{}
	}
	slide := gfx.Point{Y: height * 1.5}
	if position >= ToastTopRight {
		slide.Y = -slide.Y
	}
	return gfx.Presence{Motion: gfx.Motion{Duration: toastDuration, Easing: gfx.EaseEmphasized}, SlideIn: slide, SlideOut: slide}
}

// toastSpinnerStep is how long a loading toast's spinner holds each of its eight spokes.
const toastSpinnerStep = 100 * time.Millisecond

// paintToastIcon draws the icon of a toast type in the 16px box, as Base UI's
// ToastIcon does: Lucide's circle-check, info, triangle-alert and octagon-x, in
// the text color (the error's in the destructive one), and a spinner for a
// loading toast, which turns with the frame clock and asks for the next step.
func paintToastIcon(canvas *gfx.Canvas, theme gfx.Theme, bounds gfx.Rect, kind ToastType) {
	switch kind {
	case ToastLoading:
		phase := SpinnerPhase(0)
		if !canvas.ReduceMotion() {
			phase = SpinnerPhase(canvas.Now() / toastSpinnerStep % 8)
			canvas.RequestFrameIn(toastSpinnerStep)
		}
		PaintSpinner(canvas, theme, bounds, phase)
	case ToastSuccess:
		canvas.DrawIcon(circleCheckIcon, bounds, theme.PopoverForeground)
	case ToastInfo:
		canvas.DrawIcon(infoIcon, bounds, theme.PopoverForeground)
	case ToastWarning:
		canvas.DrawIcon(triangleAlertIcon, bounds, theme.PopoverForeground)
	case ToastError:
		canvas.DrawIcon(octagonXIcon, bounds, theme.Danger)
	}
}

// Lucide's status icons, on the 24 unit grid with a 2 unit stroke. The circle is
// four cubic arcs; the triangle's rounded corners are quadratic curves. Each is
// a literal so the linker keeps it as data and no package initializer builds it.
var (
	circleCheckIcon = gfx.Icon{ViewBox: 24, Stroke: 2, Cap: gfx.CapRound, Join: gfx.JoinRound, Commands: []gfx.PathCommand{
		gfx.MoveTo(22, 12),
		gfx.CubicTo(22, 17.523, 17.523, 22, 12, 22), gfx.CubicTo(6.477, 22, 2, 17.523, 2, 12),
		gfx.CubicTo(2, 6.477, 6.477, 2, 12, 2), gfx.CubicTo(17.523, 2, 22, 6.477, 22, 12),
		gfx.ClosePath(),
		gfx.MoveTo(9, 12), gfx.LineTo(11, 14), gfx.LineTo(15, 10),
	}}
	infoIcon = gfx.Icon{ViewBox: 24, Stroke: 2, Cap: gfx.CapRound, Join: gfx.JoinRound, Commands: []gfx.PathCommand{
		gfx.MoveTo(22, 12),
		gfx.CubicTo(22, 17.523, 17.523, 22, 12, 22), gfx.CubicTo(6.477, 22, 2, 17.523, 2, 12),
		gfx.CubicTo(2, 6.477, 6.477, 2, 12, 2), gfx.CubicTo(17.523, 2, 22, 6.477, 22, 12),
		gfx.ClosePath(),
		gfx.MoveTo(12, 16), gfx.LineTo(12, 12), gfx.MoveTo(12, 8), gfx.LineTo(12.01, 8),
	}}
	octagonXIcon = gfx.Icon{ViewBox: 24, Stroke: 2, Cap: gfx.CapRound, Join: gfx.JoinRound, Commands: []gfx.PathCommand{
		gfx.MoveTo(7.86, 2), gfx.LineTo(16.14, 2), gfx.LineTo(22, 7.86), gfx.LineTo(22, 16.14), gfx.LineTo(16.14, 22),
		gfx.LineTo(7.86, 22), gfx.LineTo(2, 16.14), gfx.LineTo(2, 7.86), gfx.ClosePath(),
		gfx.MoveTo(15, 9), gfx.LineTo(9, 15), gfx.MoveTo(9, 9), gfx.LineTo(15, 15),
	}}
	triangleAlertIcon = gfx.Icon{ViewBox: 24, Stroke: 2, Cap: gfx.CapRound, Join: gfx.JoinRound, Commands: []gfx.PathCommand{
		gfx.MoveTo(21.73, 18), gfx.LineTo(13.73, 4), gfx.QuadTo(12, 1.97, 10.25, 4), gfx.LineTo(2.27, 18),
		gfx.QuadTo(0.56, 21, 4, 21), gfx.LineTo(20, 21), gfx.QuadTo(23.44, 21, 21.73, 18), gfx.ClosePath(),
		gfx.MoveTo(12, 9), gfx.LineTo(12, 13), gfx.MoveTo(12, 17), gfx.LineTo(12.01, 17),
	}}
)

// toastDuration is the 500ms of Base UI's toast transition.
const toastDuration = 500 * time.Millisecond
