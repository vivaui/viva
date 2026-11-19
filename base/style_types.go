package base

import (
	"time"

	"github.com/vivaui/viva/gfx"
)

// DialogHeader is the title's metrics: the size the title is painted at, the line
// box it takes above the description, and the space between the two. A zero field
// keeps the overlay dialog's: the body size plus two, one more than that, and the
// theme's item gap.
type DialogHeader struct {
	TitleSize   float32
	TitleHeight float32
	TitleGap    float32
}

// SwipeGesture is caller-owned drag state for a drawer that is swiped toward
// its edge to dismiss. The caller supplies one axis of the pointer: increasing
// toward the closed edge (down for a bottom drawer, up for a top one, right for
// a right drawer, left for a left one). Move clamps the travel to the open
// extent; End decides whether the release dismisses or snaps to one of the
// caller's snap fractions.
type SwipeGesture struct {
	dragging bool
	start    float32
	offset   float32
}

// HoverIntent is caller-owned hover-rest state for a control that opens
// something on hover, such as a hover card, a submenu trigger, a menubar
// trigger while another menu is open, or a navigation menu. Its zero value is
// ready for the first target. Call Rest with the hovered identity each frame:
// it returns the identity once it has rested for delay, and otherwise how long
// the caller should wait before asking again. Moving to another target, or
// passing zero, starts over, so the caller schedules no timer of its own.
type HoverIntent struct {
	Target gfx.ID
	since  time.Time
}

// ScrollAreaGeometry is the complete deterministic layout of one externally
// controlled scroll area. Content is already translated by the clamped offset.
type ScrollAreaGeometry struct {
	Bounds          gfx.Rect
	Viewport        gfx.Rect
	Content         gfx.Rect
	VerticalTrack   gfx.Rect
	VerticalThumb   gfx.Rect
	HorizontalTrack gfx.Rect
	HorizontalThumb gfx.Rect
	Corner          gfx.Rect
	OffsetX         float32
	OffsetY         float32
	MaxOffsetX      float32
	MaxOffsetY      float32
}

// resolve fills the fields the caller left zero with the overlay dialog's.
func (header DialogHeader) Resolve(metrics gfx.Metrics) DialogHeader {
	if header.TitleSize <= 0 {
		header.TitleSize = overlayTitleSize(metrics)
	}
	if header.TitleHeight <= 0 {
		header.TitleHeight = header.TitleSize + 1
	}
	if header.TitleGap <= 0 {
		header.TitleGap = metrics.GapItem
	}
	return header
}

// height is the title and, under it, the description's lines.
func (header DialogHeader) Height(metrics gfx.Metrics, descriptionLines int) float32 {
	height := header.TitleHeight
	if descriptionLines > 0 {
		height += header.TitleGap + float32(descriptionLines)*overlayLineHeight(metrics)
	}
	return height
}

// Begin starts a swipe at pointer.
func (swipe *SwipeGesture) Begin(pointer float32) {
	swipe.dragging, swipe.start, swipe.offset = true, pointer, 0
}

// Move advances the swipe and returns how far it has travelled toward closed,
// clamped to [0, extent].
func (swipe *SwipeGesture) Move(pointer, extent float32) float32 {
	if !swipe.dragging {
		return swipe.offset
	}
	swipe.offset = min(max(pointer-swipe.start, 0), max(extent, 0))
	return swipe.offset
}

// Offset is the current distance travelled toward closed.
func (swipe *SwipeGesture) Offset() float32 { return swipe.offset }

// Dragging reports whether a swipe is in progress.
func (swipe *SwipeGesture) Dragging() bool { return swipe.dragging }

// End finishes the swipe. velocity is the release speed toward closed, in
// pixels per second. It dismisses when the drawer travelled at least half its
// extent or was thrown faster than 600 px/s; otherwise it returns the nearest
// snap fraction (0 is the open edge) times extent. snapPoints are fractions in
// [0,1]; an empty slice snaps to the open or half-open position the drawer is
// already nearest.
func (swipe *SwipeGesture) End(extent, velocity float32, snapPoints []float32) (offset float32, dismissed bool) {
	swipe.dragging = false
	if extent <= 0 {
		return 0, false
	}
	if swipe.offset >= extent/2 || velocity >= 600 {
		swipe.offset = extent
		return extent, true
	}
	best, bestDistance := float32(0), swipe.offset
	for _, point := range snapPoints {
		candidate := min(max(point, 0), 1) * extent
		if distance := absFloat32(candidate - swipe.offset); distance < bestDistance {
			best, bestDistance = candidate, distance
		}
	}
	swipe.offset = best
	return best, false
}

// Rest advances the intent to target at now. It returns target and zero once the
// target has rested for delay; until then it returns zero and the remaining
// wait. A zero target clears the candidate. A non-positive delay makes the
// target ready at once.
func (intent *HoverIntent) Rest(target gfx.ID, now time.Time, delay time.Duration) (gfx.ID, time.Duration) {
	if target.IsZero() {
		*intent = HoverIntent{}
		return gfx.ID{}, 0
	}
	if target != intent.Target || now.Before(intent.since) {
		intent.Target, intent.since = target, now
	}
	if delay <= 0 {
		return target, 0
	}
	if elapsed := now.Sub(intent.since); elapsed >= delay {
		return target, 0
	} else {
		return gfx.ID{}, delay - elapsed
	}
}

// Reset forgets the current candidate, as leaving the control does.
func (intent *HoverIntent) Reset() { *intent = HoverIntent{} }

// VerticalOffsetForThumb returns the content offset that places the top edge
// of the vertical thumb at thumbTop: the inverse of the thumb placement, clamped
// to [0, MaxOffsetY]. A caller turns a pointer drag on the scrollbar into its
// owned offset with it, without repeating the track insets. It is zero when
// the area does not scroll vertically.
func (geometry ScrollAreaGeometry) VerticalOffsetForThumb(thumbTop float32) float32 {
	travel := scrollAreaMax(geometry.VerticalTrack.Height-scrollAreaBarInset*2, 0) - geometry.VerticalThumb.Height
	if travel <= 0 || geometry.MaxOffsetY <= 0 {
		return 0
	}
	ratio := (thumbTop - (geometry.VerticalTrack.Y + scrollAreaBarInset)) / travel
	return scrollAreaClamp(ratio*geometry.MaxOffsetY, 0, geometry.MaxOffsetY)
}

// HorizontalOffsetForThumb returns the content offset that places the left edge
// of the horizontal thumb at thumbLeft. It is zero when the area does not scroll
// horizontally.
func (geometry ScrollAreaGeometry) HorizontalOffsetForThumb(thumbLeft float32) float32 {
	travel := scrollAreaMax(geometry.HorizontalTrack.Width-scrollAreaBarInset*2, 0) - geometry.HorizontalThumb.Width
	if travel <= 0 || geometry.MaxOffsetX <= 0 {
		return 0
	}
	ratio := (thumbLeft - (geometry.HorizontalTrack.X + scrollAreaBarInset)) / travel
	return scrollAreaClamp(ratio*geometry.MaxOffsetX, 0, geometry.MaxOffsetX)
}

func scrollAreaMax(left, right float32) float32 {
	if left > right {
		return left
	}
	return right
}

func scrollAreaFinite(value float32) float32 {
	if value != value || value > 3.4028235e+38 || value < -3.4028235e+38 {
		return 0
	}
	return value
}

func overlayLineHeight(metrics gfx.Metrics) float32 { return metrics.FontSizeM + 7 }

func overlayTitleSize(metrics gfx.Metrics) float32 { return metrics.FontSizeM + 2 }

func scrollAreaClamp(value, minimum, maximum float32) float32 {
	value = scrollAreaFinite(value)
	if value < minimum {
		return minimum
	}
	if value > maximum {
		return maximum
	}
	return value
}

func absFloat32(value float32) float32 {
	if value < 0 {
		return -value
	}
	return value
}

const scrollAreaBarInset float32 = 1
