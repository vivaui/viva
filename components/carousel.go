package components

import (
	"github.com/vivaui/viva/base"
	"github.com/vivaui/viva/gfx"
)

// carouselControlOffset is the 16px between the viewport and a button (-left-12
// puts a size-8 button 48px out).
const carouselControlOffset float32 = 16

// The arrows of the previous and next buttons, Lucide's arrow-left and arrow-right
// rotated a quarter turn for a vertical carousel.
var (
	carouselArrowLeft  = gfx.Icon{ViewBox: 24, Stroke: 2, Cap: gfx.CapRound, Join: gfx.JoinRound, Commands: []gfx.PathCommand{gfx.MoveTo(19, 12), gfx.LineTo(5, 12), gfx.MoveTo(12, 19), gfx.LineTo(5, 12), gfx.LineTo(12, 5)}}
	carouselArrowRight = gfx.Icon{ViewBox: 24, Stroke: 2, Cap: gfx.CapRound, Join: gfx.JoinRound, Commands: []gfx.PathCommand{gfx.MoveTo(5, 12), gfx.LineTo(19, 12), gfx.MoveTo(12, 5), gfx.LineTo(19, 12), gfx.LineTo(12, 19)}}
	carouselArrowUp    = gfx.Icon{ViewBox: 24, Stroke: 2, Cap: gfx.CapRound, Join: gfx.JoinRound, Commands: []gfx.PathCommand{gfx.MoveTo(12, 19), gfx.LineTo(12, 5), gfx.MoveTo(5, 12), gfx.LineTo(12, 5), gfx.LineTo(19, 12)}}
	carouselArrowDown  = gfx.Icon{ViewBox: 24, Stroke: 2, Cap: gfx.CapRound, Join: gfx.JoinRound, Commands: []gfx.PathCommand{gfx.MoveTo(12, 5), gfx.LineTo(12, 19), gfx.MoveTo(19, 12), gfx.LineTo(12, 19), gfx.LineTo(5, 12)}}
)

// CarouselOrientation selects the axis of caller-owned carousel content.
type CarouselOrientation uint8

const (
	CarouselHorizontal CarouselOrientation = iota
	CarouselVertical
)

// CarouselGeometry exposes the full externally controlled carousel layout.
type CarouselGeometry struct {
	Bounds         gfx.Rect
	Viewport       gfx.Rect
	Content        gfx.Rect
	PreviousButton gfx.Rect
	NextButton     gfx.Rect
	Offset         float32
	MaxOffset      float32
	Index          int
	ItemCount      int
	CanPrevious    bool
	CanNext        bool
}

// CarouselItemGeometry maps one caller-owned slide from content coordinates.
type CarouselItemGeometry struct {
	Index         int
	ContentOffset float32
	Extent        float32
	Bounds        gfx.Rect
	Visible       bool
	Selected      bool
}

// Carousel is a clipped viewport with caller-owned offset, selected index,
// item count, and content extent. Keyboard routing, dragging, pointer capture,
// inertia, autoplay, observers, timers, and virtualization are runtime or
// application responsibilities and are intentionally absent from this recipe.
type Carousel struct {
	canvas              *gfx.Canvas
	theme               gfx.Theme
	bounds              gfx.Rect
	itemCount           int
	index               int
	contentExtent       float32
	offset              float32
	orientation         CarouselOrientation
	direction           Direction
	previousID          gfx.ID
	previousLabel       string
	nextID              gfx.ID
	nextLabel           string
	label               string
	disabled            bool
	contentClipIsActive bool
}

// BeginCarousel declares carousel state without retaining item data.
func BeginCarousel(canvas *gfx.Canvas, theme gfx.Theme, bounds gfx.Rect, itemCount, index int) Carousel {
	return Carousel{canvas: canvas, theme: theme, bounds: bounds, itemCount: itemCount, index: index}
}

// Content sets the caller-owned total axis extent and current scroll offset.
func (carousel Carousel) Content(extent, offset float32) Carousel {
	carousel.contentExtent = extent
	carousel.offset = offset
	return carousel
}

// Vertical changes the content axis and control placement to vertical.
func (carousel Carousel) Vertical() Carousel {
	carousel.orientation = CarouselVertical
	return carousel
}

// Direction sets the reading direction of a horizontal carousel. The zero
// value is LTR. Under RTL the content reads right to left: the first slide
// sits at the viewport's right edge and the offset reveals slides on its left,
// the previous control stands on the right of the viewport and the next one on
// the left, and a vertical carousel ignores it, since the block axis has no
// reading direction.
func (carousel Carousel) Direction(direction Direction) Carousel {
	carousel.direction = direction
	return carousel
}

// Controls assigns stable previous and next action IDs and accessible labels.
func (carousel Carousel) Controls(previousID gfx.ID, previousLabel string, nextID gfx.ID, nextLabel string) Carousel {
	carousel.previousID = previousID
	carousel.previousLabel = previousLabel
	carousel.nextID = nextID
	carousel.nextLabel = nextLabel
	return carousel
}

// Label names the carousel region for assistive technology.
func (carousel Carousel) Label(label string) Carousel {
	carousel.label = label
	return carousel
}

// CarouselStepForKey is the carousel region's keyboard movement: Left/Up go to
// the previous slide, Right/Down to the next, and Home/End to the ends. Left
// and Right follow the reading direction, as the framework's other horizontal
// roving keys do (TabsKeyDown, CalendarFocusForKey): an RTL carousel reads
// Left as the next slide and Right as the previous one. The caller owns the
// index and does not move past either end.
func CarouselStepForKey(key gfx.Key, index, count int, direction Direction) (int, bool) {
	if count <= 0 || index < 0 || index >= count {
		return index, false
	}
	switch key {
	case gfx.KeyLeft:
		step := -1
		if direction.IsRTL() {
			step = 1
		}
		if next := index + step; next >= 0 && next < count {
			return next, true
		}
		return index, true
	case gfx.KeyRight:
		step := 1
		if direction.IsRTL() {
			step = -1
		}
		if next := index + step; next >= 0 && next < count {
			return next, true
		}
		return index, true
	case gfx.KeyUp:
		if index > 0 {
			return index - 1, true
		}
		return index, true
	case gfx.KeyDown:
		if index < count-1 {
			return index + 1, true
		}
		return index, true
	case gfx.KeyHome:
		return 0, true
	case gfx.KeyEnd:
		return count - 1, true
	default:
		return index, false
	}
}

// Disabled keeps both control bounds and semantics while making them inert.
func (carousel Carousel) Disabled() Carousel {
	carousel.disabled = true
	return carousel
}

// Geometry returns clamped content and stable external control bounds.
func (carousel Carousel) Geometry() CarouselGeometry {
	viewport := carouselNormalizeRect(carousel.bounds)
	viewportExtent := viewport.Width
	if carousel.orientation == CarouselVertical {
		viewportExtent = viewport.Height
	}
	contentExtent := carouselFiniteNonNegative(carousel.contentExtent)
	if contentExtent < viewportExtent {
		contentExtent = viewportExtent
	}
	maximum := carouselMax(contentExtent-viewportExtent, 0)
	offset := carouselClamp(carousel.offset, 0, maximum)
	itemCount := carousel.itemCount
	if itemCount < 0 {
		itemCount = 0
	}
	index := carousel.index
	if itemCount == 0 {
		index = 0
	} else {
		if index < 0 {
			index = 0
		}
		if index >= itemCount {
			index = itemCount - 1
		}
	}
	controlSize := carousel.theme.Metrics.ControlHeight
	if controlSize < 20 {
		controlSize = 20
	}
	geometry := CarouselGeometry{
		Bounds: viewport, Viewport: viewport,
		Offset: offset, MaxOffset: maximum, Index: index, ItemCount: itemCount,
		CanPrevious: !carousel.disabled && itemCount > 0 && index > 0,
		CanNext:     !carousel.disabled && itemCount > 0 && index+1 < itemCount,
	}
	if carousel.orientation == CarouselVertical {
		geometry.Content = gfx.Rect{X: viewport.X, Y: viewport.Y - offset, Width: viewport.Width, Height: contentExtent}
		geometry.PreviousButton = gfx.Rect{
			X:     viewport.X + (viewport.Width-controlSize)/2,
			Y:     viewport.Y - carouselControlOffset - controlSize,
			Width: controlSize, Height: controlSize,
		}
		geometry.NextButton = gfx.Rect{
			X: geometry.PreviousButton.X, Y: viewport.Y + viewport.Height + carouselControlOffset,
			Width: controlSize, Height: controlSize,
		}
		return geometry
	}
	// LTR content grows to the right and the offset moves it left; RTL content
	// grows to the left from the viewport's right edge and the offset moves it
	// right, so the logical [offset, offset+viewportExtent] window of the
	// content shows through the viewport either way.
	contentX := viewport.X - offset
	if base.WritingDirection(carousel.direction).IsRTL() {
		contentX = viewport.X + offset + viewport.Width - contentExtent
	}
	geometry.Content = gfx.Rect{X: contentX, Y: viewport.Y, Width: contentExtent, Height: viewport.Height}
	// The previous control stands at the content's inline start and the next one
	// at its inline end: left and right in LTR, right and left in RTL.
	previousX, nextX := viewport.X-carouselControlOffset-controlSize, viewport.X+viewport.Width+carouselControlOffset
	if base.WritingDirection(carousel.direction).IsRTL() {
		previousX, nextX = nextX, previousX
	}
	geometry.PreviousButton = gfx.Rect{
		X: previousX, Y: viewport.Y + (viewport.Height-controlSize)/2,
		Width: controlSize, Height: controlSize,
	}
	geometry.NextButton = gfx.Rect{
		X: nextX, Y: geometry.PreviousButton.Y,
		Width: controlSize, Height: controlSize,
	}
	return geometry
}

// ItemGeometry translates one caller-supplied slide offset and extent along
// the carousel axis, placing it at the reading direction's edge of the content:
// its left edge in LTR and its right edge in RTL. It does not retain, measure,
// or virtualize the item.
func (carousel Carousel) ItemGeometry(index int, contentOffset, extent float32) CarouselItemGeometry {
	geometry := carousel.Geometry()
	contentOffset = carouselFiniteNonNegative(contentOffset)
	extent = carouselFiniteNonNegative(extent)
	bounds := gfx.Rect{
		X: geometry.Content.X + contentOffset, Y: geometry.Content.Y,
		Width: extent, Height: geometry.Viewport.Height,
	}
	if base.WritingDirection(carousel.direction).IsRTL() && carousel.orientation != CarouselVertical {
		bounds.X = geometry.Content.X + geometry.Content.Width - contentOffset - extent
	}
	if carousel.orientation == CarouselVertical {
		bounds = gfx.Rect{
			X: geometry.Content.X, Y: geometry.Content.Y + contentOffset,
			Width: geometry.Viewport.Width, Height: extent,
		}
	}
	return CarouselItemGeometry{
		Index: index, ContentOffset: contentOffset, Extent: extent,
		Bounds: bounds, Visible: carouselIntersects(bounds, geometry.Viewport),
		Selected: index == geometry.Index,
	}
}

// BeginContent pushes the viewport clip for caller-painted slides.
func (carousel *Carousel) BeginContent() CarouselGeometry {
	geometry := carousel.Geometry()
	if carousel.canvas != nil && !carousel.contentClipIsActive && geometry.Viewport.Width > 0 && geometry.Viewport.Height > 0 {
		if carousel.label != "" {
			carousel.canvas.Container(geometry.Viewport, gfx.RoleGroup, carousel.label)
		}
		carousel.canvas.Clip(geometry.Viewport)
		carousel.contentClipIsActive = true
	}
	return geometry
}

// EndContent closes the clip and paints both stable navigation controls.
func (carousel *Carousel) EndContent() CarouselGeometry {
	geometry := carousel.Geometry()
	if carousel.canvas == nil {
		return geometry
	}
	if carousel.contentClipIsActive {
		carousel.canvas.Unclip()
		carousel.contentClipIsActive = false
	}
	carousel.paintControl(geometry.PreviousButton, carousel.previousID, carousel.previousLabel, true, !geometry.CanPrevious)
	carousel.paintControl(geometry.NextButton, carousel.nextID, carousel.nextLabel, false, !geometry.CanNext)
	return geometry
}

func (carousel Carousel) paintControl(bounds gfx.Rect, id gfx.ID, label string, previous, disabled bool) {
	if id.IsZero() || bounds.Width <= 0 || bounds.Height <= 0 {
		return
	}
	// An outline icon button, round: bg-background with a border, bg-muted on hover.
	background := carousel.theme.Background
	foreground := carousel.theme.Foreground
	border := carousel.theme.Border
	radius := bounds.Width / 2
	if !disabled && carousel.canvas.Hovered() == id {
		background = carousel.theme.Muted
	}
	if !disabled && carousel.canvas.FocusVisible() == id {
		ring := carousel.theme.Ring
		ring.Alpha *= 0.50
		carousel.canvas.Rounded(carouselExpand(bounds, 3), radius+3, ring)
	}
	if disabled {
		background.Alpha *= 0.50
		foreground.Alpha *= 0.50
		border.Alpha *= 0.50
	}
	carousel.canvas.RoundedBorder(bounds, radius, 1, border, background)
	// The arrow points along the reading direction: back toward the previous
	// slide's edge and forward toward the next one's, mirrored under RTL.
	rtl := base.WritingDirection(carousel.direction).IsRTL() && carousel.orientation != CarouselVertical
	icon := &carouselArrowRight
	switch {
	case carousel.orientation == CarouselVertical && previous:
		icon = &carouselArrowUp
	case carousel.orientation == CarouselVertical:
		icon = &carouselArrowDown
	case previous:
		if rtl {
			icon = &carouselArrowRight
		} else {
			icon = &carouselArrowLeft
		}
	default:
		if rtl {
			icon = &carouselArrowLeft
		} else {
			icon = &carouselArrowRight
		}
	}
	paintChevronSized(carousel.canvas, icon, bounds, 16, foreground)
	carousel.canvas.HitWithOptions(id, bounds, label, gfx.HitOptions{
		Role: gfx.RoleButton, Disabled: disabled,
	})
}

func carouselNormalizeRect(bounds gfx.Rect) gfx.Rect {
	bounds.X = carouselFinite(bounds.X)
	bounds.Y = carouselFinite(bounds.Y)
	bounds.Width = carouselFiniteNonNegative(bounds.Width)
	bounds.Height = carouselFiniteNonNegative(bounds.Height)
	return bounds
}

func carouselFinite(value float32) float32 {
	if value != value || value > 3.4028235e+38 || value < -3.4028235e+38 {
		return 0
	}
	return value
}

func carouselFiniteNonNegative(value float32) float32 {
	value = carouselFinite(value)
	if value < 0 {
		return 0
	}
	return value
}

func carouselClamp(value, minimum, maximum float32) float32 {
	value = carouselFinite(value)
	if value < minimum {
		return minimum
	}
	if value > maximum {
		return maximum
	}
	return value
}

func carouselMax(left, right float32) float32 {
	if left > right {
		return left
	}
	return right
}

func carouselIntersects(bounds, viewport gfx.Rect) bool {
	return bounds.Width > 0 && bounds.Height > 0 &&
		bounds.X < viewport.X+viewport.Width && bounds.X+bounds.Width > viewport.X &&
		bounds.Y < viewport.Y+viewport.Height && bounds.Y+bounds.Height > viewport.Y
}

func carouselExpand(bounds gfx.Rect, amount float32) gfx.Rect {
	return gfx.Rect{
		X: bounds.X - amount, Y: bounds.Y - amount,
		Width: bounds.Width + amount*2, Height: bounds.Height + amount*2,
	}
}
