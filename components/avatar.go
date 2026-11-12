package components

import (
	"math"
	"sync"

	"github.com/vivaui/viva/gfx"
)

// AvatarSize mirrors the official sm/default/lg size variants.
type AvatarSize uint8

const (
	AvatarSizeDefault AvatarSize = iota
	AvatarSizeSmall
	AvatarSizeLarge
)

// AvatarStatus is a typed convenience for the generic badge slot.
type AvatarStatus uint8

const (
	AvatarStatusNone AvatarStatus = iota
	AvatarStatusOnline
	AvatarStatusBusy
	AvatarStatusOffline
)

// AvatarData is the replaceable data boundary for an avatar. Image is a
// caller-decoded bitmap (Bitmap.ID and Version identify it for the renderer's
// texture cache); it is center-cropped to a circle like AvatarImage's
// object-cover. When Image is empty or invalid, Initials or a geometric
// fallback is painted, like AvatarFallback.
type AvatarData struct {
	Name     string
	Initials string
	Image    gfx.Bitmap
	Tint     gfx.Color
	Status   AvatarStatus
	Size     AvatarSize
}

// avatarRing is the unit circle used to stroke the border over an image. gfx.Path
// has no literal form, so it is built on the first avatar that shows an image
// instead of by a package initializer every importing program would run.
var (
	avatarRing     gfx.Path
	avatarRingOnce sync.Once
)

func avatarRingPath() gfx.Path {
	avatarRingOnce.Do(func() {
		avatarRing = gfx.NewPath(2, 2,
			gfx.MoveTo(2, 1),
			gfx.CubicTo(2, 1.5523, 1.5523, 2, 1, 2),
			gfx.CubicTo(0.4477, 2, 0, 1.5523, 0, 1),
			gfx.CubicTo(0, 0.4477, 0.4477, 0, 1, 0),
			gfx.CubicTo(1.5523, 0, 2, 0.4477, 2, 1),
			gfx.ClosePath(),
		)
	})
	return avatarRing
}

// Avatar paints fallback, border overlay, and optional badge. Explicit bounds
// win when both dimensions are positive; otherwise the size variant is used.
func Avatar(canvas *gfx.Canvas, theme gfx.Theme, bounds gfx.Rect, data AvatarData) {
	size := avatarSize(data.Size)
	if bounds.Width > 0 && bounds.Height > 0 {
		size = bounds.Width
		if bounds.Height < size {
			size = bounds.Height
		}
	}
	if size <= 0 {
		return
	}
	bounds.Width = size
	bounds.Height = size
	fill := data.Tint
	foreground := theme.MutedForeground
	if fill.Alpha == 0 {
		fill = theme.Muted
	} else {
		foreground = theme.PrimaryForeground
	}
	if paintAvatarImage(canvas, bounds, data.Image) {
		// The image strips have stair-stepped edges; the stroked border covers them.
		canvas.DrawPath(avatarRingPath(), gfx.Rect{X: bounds.X + 0.5, Y: bounds.Y + 0.5, Width: size - 1, Height: size - 1},
			gfx.PathStyle{Stroke: theme.Border, StrokeWidth: 1})
	} else if data.Initials != "" {
		canvas.RoundedBorder(bounds, size/2, 1, theme.Border, fill)
		canvas.LabelCenter(data.Initials, bounds, gfx.TextStyle{
			Size: avatarFontSize(theme.Metrics, data.Size), Color: foreground,
		})
	} else {
		canvas.RoundedBorder(bounds, size/2, 1, theme.Border, fill)
		head := size * 0.24
		canvas.Rounded(gfx.Rect{X: bounds.X + (size-head)/2, Y: bounds.Y + size*0.20, Width: head, Height: head}, head/2, foreground)
		shouldersWidth := size * 0.52
		shouldersHeight := size * 0.23
		canvas.Rounded(gfx.Rect{X: bounds.X + (size-shouldersWidth)/2, Y: bounds.Y + size*0.55, Width: shouldersWidth, Height: shouldersHeight}, shouldersHeight/2, foreground)
	}
	if data.Status != AvatarStatusNone {
		statusColor := theme.MutedForeground
		switch data.Status {
		case AvatarStatusOnline:
			statusColor = theme.Success
		case AvatarStatusBusy:
			statusColor = theme.Danger
		}
		dotSize := avatarBadgeSize(data.Size)
		dot := gfx.Rect{X: bounds.X + size - dotSize, Y: bounds.Y + size - dotSize, Width: dotSize, Height: dotSize}
		canvas.Rounded(gfx.Rect{X: dot.X - 2, Y: dot.Y - 2, Width: dot.Width + 4, Height: dot.Height + 4}, dotSize/2+2, theme.Background)
		canvas.Rounded(dot, dotSize/2, statusColor)
	}
}

// paintAvatarImage draws the center-cropped square of bitmap inside the circle
// inscribed in bounds and reports whether anything was drawn. Canvas has no
// rounded image clip, so the circle is composed from one-pixel horizontal strips
// of the bitmap, each as wide as the circle's chord at its middle.
func paintAvatarImage(canvas *gfx.Canvas, bounds gfx.Rect, bitmap gfx.Bitmap) bool {
	if bitmap.Width <= 0 || bitmap.Height <= 0 {
		return false
	}
	side := float32(min(bitmap.Width, bitmap.Height))
	source := gfx.Rect{X: (float32(bitmap.Width) - side) / 2, Y: (float32(bitmap.Height) - side) / 2, Width: side, Height: side}
	size := bounds.Width
	radius := size / 2
	scale := side / size
	before := len(canvas.Images())
	for top := float32(0); top < size; top++ {
		height := min(1, size-top)
		dy := radius - (top + height/2)
		chord := radius*radius - dy*dy
		if chord <= 0 {
			continue
		}
		half := float32(math.Sqrt(float64(chord)))
		left := radius - half
		canvas.DrawBitmapRegion(bitmap,
			gfx.Rect{X: source.X + left*scale, Y: source.Y + top*scale, Width: 2 * half * scale, Height: height * scale},
			gfx.Rect{X: bounds.X + left, Y: bounds.Y + top, Width: 2 * half, Height: height})
	}
	return len(canvas.Images()) > before
}

// AvatarGroup incrementally paints overlapping avatars with deterministic
// spacing. The group count is caller-provided rather than inferred or stored.
type AvatarGroup struct {
	canvas *gfx.Canvas
	theme  gfx.Theme
	x      float32
	y      float32
	size   AvatarSize
}

// BeginAvatarGroup starts an overlapping avatar row at bounds.X/Y.
func BeginAvatarGroup(canvas *gfx.Canvas, theme gfx.Theme, bounds gfx.Rect, size AvatarSize) AvatarGroup {
	return AvatarGroup{canvas: canvas, theme: theme, x: bounds.X, y: bounds.Y, size: size}
}

// Add paints one avatar inside its 2px background ring (ring-2 ring-background)
// and advances by size minus the official 8px overlap.
func (group *AvatarGroup) Add(data AvatarData) {
	data.Size = group.size
	size := avatarSize(group.size)
	bounds := gfx.Rect{X: group.x, Y: group.y, Width: size, Height: size}
	group.paintRing(bounds)
	Avatar(group.canvas, group.theme, bounds, data)
	group.x += size - 8
}

// AddCount paints the official muted group-count fallback.
func (group *AvatarGroup) AddCount(label string) {
	size := avatarSize(group.size)
	bounds := gfx.Rect{X: group.x, Y: group.y, Width: size, Height: size}
	group.paintRing(bounds)
	group.canvas.Rounded(bounds, size/2, group.theme.Muted)
	group.canvas.LabelCenter(label, bounds, gfx.TextStyle{Size: avatarFontSize(group.theme.Metrics, group.size), Color: group.theme.MutedForeground})
	group.x += size - 8
}

func (group *AvatarGroup) paintRing(bounds gfx.Rect) {
	ring := gfx.Rect{X: bounds.X - 2, Y: bounds.Y - 2, Width: bounds.Width + 4, Height: bounds.Height + 4}
	group.canvas.Rounded(ring, ring.Width/2, group.theme.Background)
}

func avatarSize(size AvatarSize) float32 {
	switch size {
	case AvatarSizeSmall:
		return 24
	case AvatarSizeLarge:
		return 40
	default:
		return 32
	}
}

func avatarBadgeSize(size AvatarSize) float32 {
	switch size {
	case AvatarSizeSmall:
		return 8
	case AvatarSizeLarge:
		return 12
	default:
		return 10
	}
}

// avatarFontSize is text-sm for default and large avatars and text-xs for small
// ones, mapped onto the theme's body and caption sizes.
func avatarFontSize(metrics gfx.Metrics, variant AvatarSize) float32 {
	if variant == AvatarSizeSmall {
		return metrics.FontSizeS
	}
	return metrics.FontSizeM
}
