// Package avatar is the opt-in page control for avatars and overlapping
// avatar groups of package components. A page declares an avatar with typed
// configuration methods and views it as a ui.Control:
//
//	person := avatar.New()
//	...
//	person.Initials("SK").Name("Skyler").Tint(theme.Primary).Diameter(64).Online()
//	person.View(c, "Skyler").Size(64, 64)
//
//	group := avatar.NewGroup()
//	group.Diameter(40).Overlap(8)
//	group.Add().Initials("AD").Name("Ada").Tint(theme.Primary).Online()
//	group.AddCount("+8")
//	group.View(c, "Collaborators")
//
// The caller owns the image, initials, size, status and group overlap; the
// wrapper owns the control's handlers and draw function. It paints through the
// component with the caller's values. There is no hidden image loading: the
// picture is a bitmap the caller decoded, and an empty image falls back to
// initials or the component's geometric fallback.
//
// It is a separate package so an application that does not import it pays
// nothing for it. Keep an State or Group in the model beside its data: its
// handlers and draw function are made once, so a warm paint allocates nothing.
// Do not copy one after it has been painted.
package avatar

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// Size and Status are the component's typed variants, named here so a page
// that declares an avatar reads through this package alone.
type (
	Size   = components.AvatarSize
	Status = components.AvatarStatus
)

const (
	SizeDefault Size = components.AvatarSizeDefault
	SizeSmall   Size = components.AvatarSizeSmall
	SizeLarge   Size = components.AvatarSizeLarge

	StatusNone    Status = components.AvatarStatusNone
	StatusOnline  Status = components.AvatarStatusOnline
	StatusBusy    Status = components.AvatarStatusBusy
	StatusOffline Status = components.AvatarStatusOffline
)

// State is one avatar's caller-owned values and page control. Configure it
// with the chainable methods, then view it. Reset clears the values so a page
// that reconfigures the avatar every paint does not keep a stale image or
// initials.
type State struct {
	bitmap   gfx.Bitmap
	initials string
	name     string
	tint     gfx.Color
	size     Size
	diameter float32
	status   Status

	handlers ui.Handlers
	drawFn   func(*ui.DrawContext)
	prepared bool
}

// New returns an avatar with the default size and no image, initials or status.
func New() *State { return new(State) }

// Reset clears the image, initials, name, tint, status and explicit diameter,
// returning the avatar to the default size. Call it at the start of a paint that
// reconfigures the avatar, then set the values that apply now.
func (a *State) Reset() *State {
	a.bitmap = gfx.Bitmap{}
	a.initials = ""
	a.name = ""
	a.tint = gfx.Color{}
	a.size = SizeDefault
	a.diameter = 0
	a.status = StatusNone
	return a
}

// Image sets the caller-decoded picture, center-cropped to a circle. It takes
// precedence over initials when it has pixels.
func (a *State) Image(bitmap gfx.Bitmap) *State { a.bitmap = bitmap; return a }

// Initials sets the fallback text painted when Image is empty.
func (a *State) Initials(value string) *State { a.initials = value; return a }

// Name is the accessible name of the avatar; it is not painted.
func (a *State) Name(value string) *State { a.name = value; return a }

// Tint fills an initials or geometric fallback. The zero value uses the theme's
// muted surface, like an untinted State.
func (a *State) Tint(color gfx.Color) *State { a.tint = color; return a }

// Size selects the component's small, default or large variant: 24, 32 or 40
// pixels. It clears any explicit Diameter.
func (a *State) Size(size Size) *State { a.size, a.diameter = size, 0; return a }

// Diameter sets an explicit circle diameter in pixels, overriding Size. The
// small or large text and status-dot variant is chosen from the diameter.
func (a *State) Diameter(pixels float32) *State {
	if pixels < 0 {
		pixels = 0
	}
	a.diameter = pixels
	return a
}

// Status sets the typed status dot: online, busy, offline or none.
func (a *State) Status(status Status) *State { a.status = status; return a }

// Online shows the success status dot.
func (a *State) Online() *State { a.status = StatusOnline; return a }

// Busy shows the danger status dot.
func (a *State) Busy() *State { a.status = StatusBusy; return a }

// Offline shows the muted status dot.
func (a *State) Offline() *State { a.status = StatusOffline; return a }

// View declares the avatar at c. label names it for assistive technology. The
// box is exactly the avatar's diameter square.
//
// Declare the view on its own, not inside a Memo.
func (a *State) View(c ui.C, label string) ui.Box {
	if a == nil {
		panic("avatar: View needs its State")
	}
	a.prepare()
	side := a.dimension()
	return ui.Control(c, label, &a.handlers).Size(side, side).Draw(a.drawFn)
}

func (a *State) prepare() {
	if a.prepared {
		return
	}
	a.prepared = true
	a.handlers.Role = gfx.RoleGroup
	a.drawFn = a.draw
}

func (a *State) draw(d *ui.DrawContext) {
	components.Avatar(d.Canvas, *d.Theme, d.Rect, components.AvatarData{
		Name: a.name, Initials: a.initials, Image: a.bitmap, Tint: a.tint,
		Status: a.status, Size: a.variant(),
	})
}

func (a *State) dimension() float32 {
	if a.diameter > 0 {
		return a.diameter
	}
	return sizePixels(a.size)
}

func (a *State) variant() Size {
	if a.diameter > 0 {
		return variantForPixels(a.diameter)
	}
	return a.size
}

// Member is one avatar of a Group. Configure it with the chainable methods; the
// group paints it in the order it was added.
type Member struct {
	bitmap   gfx.Bitmap
	initials string
	name     string
	tint     gfx.Color
	status   Status
	label    string
	count    bool
}

// Image sets the member's picture.
func (m *Member) Image(bitmap gfx.Bitmap) *Member { m.bitmap = bitmap; return m }

// Initials sets the member's fallback text.
func (m *Member) Initials(value string) *Member { m.initials = value; return m }

// Name is the member's accessible name; it is not painted.
func (m *Member) Name(value string) *Member { m.name = value; return m }

// Tint fills the member's fallback.
func (m *Member) Tint(color gfx.Color) *Member { m.tint = color; return m }

// Status sets the member's status dot.
func (m *Member) Status(status Status) *Member { m.status = status; return m }

// Online shows the member's success status dot.
func (m *Member) Online() *Member { m.status = StatusOnline; return m }

// Busy shows the member's danger status dot.
func (m *Member) Busy() *Member { m.status = StatusBusy; return m }

// Offline shows the member's muted status dot.
func (m *Member) Offline() *Member { m.status = StatusOffline; return m }

// Group is one overlapping row of avatars and its page control. Configure the
// diameter and overlap, Reset it, then Add the members that apply now.
type Group struct {
	size       Size
	diameter   float32
	overlap    float32
	overlapSet bool
	members    []Member

	handlers ui.Handlers
	drawFn   func(*ui.DrawContext)
	prepared bool
}

// NewGroup returns an empty avatar group with the default size and the official
// 8 pixel overlap.
func NewGroup() *Group { return new(Group) }

// Reset removes every member, keeping the size and overlap configuration and
// the member storage so a warm re-declaration allocates nothing.
func (g *Group) Reset() *Group { g.members = g.members[:0]; return g }

// Size selects the component's size variant for every member and clears any
// explicit Diameter.
func (g *Group) Size(size Size) *Group { g.size, g.diameter = size, 0; return g }

// Diameter sets an explicit member diameter in pixels, overriding Size.
func (g *Group) Diameter(pixels float32) *Group {
	if pixels < 0 {
		pixels = 0
	}
	g.diameter = pixels
	return g
}

// Overlap sets how many pixels each member overlaps the one before it. The
// default is the official 8 pixels; 0 spaces the members edge to edge. Values
// are clamped to the member diameter.
func (g *Group) Overlap(pixels float32) *Group {
	if pixels < 0 {
		pixels = 0
	}
	g.overlap, g.overlapSet = pixels, true
	return g
}

// Add appends a member and returns it for configuration.
func (g *Group) Add() *Member {
	if len(g.members) == cap(g.members) {
		g.members = append(g.members, Member{})
	} else {
		g.members = g.members[:len(g.members)+1]
		g.members[len(g.members)-1] = Member{}
	}
	return &g.members[len(g.members)-1]
}

// AddCount appends the muted group-count fallback, such as "+8".
func (g *Group) AddCount(label string) *Group {
	if len(g.members) == cap(g.members) {
		g.members = append(g.members, Member{})
	} else {
		g.members = g.members[:len(g.members)+1]
		g.members[len(g.members)-1] = Member{}
	}
	member := &g.members[len(g.members)-1]
	member.label, member.count = label, true
	return g
}

// View declares the group at c. label names the row for assistive technology.
// The box is as wide as the overlapping members and as tall as one member.
//
// Declare the view on its own, not inside a Memo.
func (g *Group) View(c ui.C, label string) ui.Box {
	if g == nil {
		panic("avatar: View needs its Group")
	}
	g.prepare()
	side := g.dimension()
	width := float32(0)
	if len(g.members) > 0 {
		width = side + float32(len(g.members)-1)*g.step()
	}
	return ui.Control(c, label, &g.handlers).Size(width, side).Draw(g.drawFn)
}

func (g *Group) prepare() {
	if g.prepared {
		return
	}
	g.prepared = true
	g.handlers.Role = gfx.RoleGroup
	g.drawFn = g.draw
}

func (g *Group) draw(d *ui.DrawContext) {
	theme := *d.Theme
	side := g.dimension()
	step := g.step()
	kind := g.variant()
	x := d.Rect.X
	for index := range g.members {
		member := &g.members[index]
		bounds := gfx.Rect{X: x, Y: d.Rect.Y, Width: side, Height: side}
		ring := gfx.Rect{X: bounds.X - 2, Y: bounds.Y - 2, Width: side + 4, Height: side + 4}
		d.Canvas.Rounded(ring, ring.Width/2, theme.Background)
		if member.count {
			d.Canvas.Rounded(bounds, side/2, theme.Muted)
			d.Canvas.LabelCenter(member.label, bounds, gfx.TextStyle{
				Size: fontSize(theme.Metrics, kind), Color: theme.MutedForeground,
			})
		} else {
			components.Avatar(d.Canvas, theme, bounds, components.AvatarData{
				Name: member.name, Initials: member.initials, Image: member.bitmap,
				Tint: member.tint, Status: member.status, Size: kind,
			})
		}
		x += step
	}
}

func (g *Group) dimension() float32 {
	if g.diameter > 0 {
		return g.diameter
	}
	return sizePixels(g.size)
}

func (g *Group) variant() Size {
	if g.diameter > 0 {
		return variantForPixels(g.diameter)
	}
	return g.size
}

// step is the horizontal advance from one member to the next.
func (g *Group) step() float32 {
	side := g.dimension()
	overlap := float32(8)
	if g.overlapSet {
		overlap = g.overlap
	}
	if overlap > side {
		overlap = side
	}
	return side - overlap
}

func sizePixels(size Size) float32 {
	switch size {
	case SizeSmall:
		return 24
	case SizeLarge:
		return 40
	default:
		return 32
	}
}

func variantForPixels(pixels float32) Size {
	switch {
	case pixels <= 24:
		return SizeSmall
	case pixels >= 40:
		return SizeLarge
	default:
		return SizeDefault
	}
}

// fontSize is the component's text-sm for default and large avatars and
// text-xs for small ones, mapped onto the theme's body and caption sizes.
func fontSize(metrics gfx.Metrics, size Size) float32 {
	if size == SizeSmall {
		return metrics.FontSizeS
	}
	return metrics.FontSizeM
}
