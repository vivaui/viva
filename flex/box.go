package flex

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/ui"
)

// Box is a handle to one element of the page being declared. It is two words
// and every method returns it, so a declaration reads as a chain; the element it
// names lives in the Frame.
type Box struct {
	f *Frame
	i int32
}

func (b Box) n() *node { return &b.f.nodes[b.i] }

// Children declares what is inside the element. fn runs at once, and is not kept.
func (b Box) Children(fn func()) Box {
	saved := b.f.parent
	b.f.parent = b.i
	b.n().flags |= flagChildren
	fn()
	b.f.parent = saved
	return b
}

// Rect is where the element ended up: valid once the page has been laid out
// (inside a Draw, after End; or from a Begin/End pair).
func (b Box) Rect() ui.Rect { return b.n().rect }

// Content is the natural height of a Scroll's children, valid after End.
func (b Box) Content() float32 { return b.n().content }

// Clicked reports whether this paint is delivering a click on the element, which
// needs an ID: a Button, Checkbox or Switch has one, any other element takes one
// with Hit.
func (b Box) Clicked() bool {
	n := b.n()
	return b.f.canvas != nil && b.f.canvas.Clicked(n.id)
}

// Hit makes the element's whole rectangle a pointer target named id, with the
// accessibility label given, so a row or a card can be clicked or hovered.
func (b Box) Hit(id ui.ID, label string) Box {
	n := b.n()
	n.id, n.label = id, label
	n.flags |= flagHasHit
	return b
}

// MovesWindow makes the element's rectangle a place a custom-titlebar window is
// moved from: pressing and dragging it moves the window, and a double click zooms
// it (see ui.Canvas.MoveRegion). Controls declared inside the element take their
// own presses. Declare it on the row that is the window's top bar.
func (b Box) MovesWindow() Box {
	b.f.extra(b.i).moves = true
	return b
}

// HitWith is Hit with the role, cursor and other options of ui.HitOptions.
func (b Box) HitWith(id ui.ID, label string, options ui.HitOptions) Box {
	b.f.extra(b.i).hit = options
	return b.Hit(id, label)
}

// Width, Height and Size fix a size in pixels. FillWidth, FillHeight and Fill take
// all the room the container has across, or share what is left along it.
func (b Box) Width(v float32) Box  { b.n().size[axisX] = dimension{dimPx, v}; return b }
func (b Box) Height(v float32) Box { b.n().size[axisY] = dimension{dimPx, v}; return b }
func (b Box) Size(w, h float32) Box {
	return b.Width(w).Height(h)
}

// WidthPercent and HeightPercent size an element as a percentage of the room its
// container has (its content box). Basis is the size an element starts from before
// free room is shared, instead of its content's: with Grow it is how proportions are set.
func (b Box) WidthPercent(p float32) Box  { b.n().size[axisX] = dimension{dimPercent, p}; return b }
func (b Box) HeightPercent(p float32) Box { b.n().size[axisY] = dimension{dimPercent, p}; return b }
func (b Box) Basis(v float32) Box {
	n := b.n()
	n.basis = v
	n.flags |= flagBasis
	return b
}
func (b Box) FillWidth() Box  { b.n().size[axisX] = dimension{kind: dimFill}; return b }
func (b Box) FillHeight() Box { b.n().size[axisY] = dimension{kind: dimFill}; return b }
func (b Box) Fill() Box       { return b.FillWidth().FillHeight() }

// MinWidth, MaxWidth, MinHeight and MaxHeight bound the size the layout gives.
func (b Box) MinWidth(v float32) Box  { b.n().minS[axisX] = v; return b }
func (b Box) MaxWidth(v float32) Box  { b.n().maxS[axisX] = v; return b }
func (b Box) MinHeight(v float32) Box { b.n().minS[axisY] = v; return b }
func (b Box) MaxHeight(v float32) Box { b.n().maxS[axisY] = v; return b }

// MaxWidthPercent and MaxHeightPercent cap an element at a share of the room its
// container has (its content box), whatever its content wants: a chat bubble that is at
// most 70% of its row is `MaxWidthPercent(70)`, and a paragraph inside it wraps to what is
// left. They combine with MaxWidth by taking the smaller, and apply to the children of a
// Row, Column or Stack.
func (b Box) MaxWidthPercent(p float32) Box  { b.f.extra(b.i).maxPct[axisX] = p; return b }
func (b Box) MaxHeightPercent(p float32) Box { b.f.extra(b.i).maxPct[axisY] = p; return b }

// Grow shares the free room along the container between the elements that grow,
// in proportion to the numbers given. Shrink does the same for a shortage.
func (b Box) Grow(v float32) Box {
	n := b.n()
	n.grow = v
	n.flags |= flagGrowSet
	return b
}
func (b Box) Shrink(v float32) Box {
	n := b.n()
	n.shrink = v
	n.flags |= flagShrinkSet
	return b
}

// Pad is space inside the element, Margin outside it. The one-argument forms
// give all four sides; the X and Y forms give two; Sides gives them in the order
// top, right, bottom, left.
func (b Box) Pad(v float32) Box     { return b.PadSides(v, v, v, v) }
func (b Box) PadX(v float32) Box    { n := b.n(); n.pad[1], n.pad[3] = v, v; return b }
func (b Box) PadY(v float32) Box    { n := b.n(); n.pad[0], n.pad[2] = v, v; return b }
func (b Box) Margin(v float32) Box  { return b.MarginSides(v, v, v, v) }
func (b Box) MarginX(v float32) Box { n := b.n(); n.margin[1], n.margin[3] = v, v; return b }
func (b Box) MarginY(v float32) Box { n := b.n(); n.margin[0], n.margin[2] = v, v; return b }
func (b Box) PadSides(top, right, bottom, left float32) Box {
	b.n().pad = [4]float32{top, right, bottom, left}
	return b
}
func (b Box) MarginSides(top, right, bottom, left float32) Box {
	b.n().margin = [4]float32{top, right, bottom, left}
	return b
}

// Gap is the space between the children of a Row or a Column.
func (b Box) Gap(v float32) Box { b.n().gap = v; return b }

// Items places the children across the container (Start, Center, End or
// Stretch, the default); Self does it for this element alone; Justify places
// them along it when none grows (Start, Center, End or Between).
func (b Box) Items(a Align) Box   { b.n().items = a; return b }
func (b Box) Self(a Align) Box    { b.n().self = a; return b }
func (b Box) Justify(a Align) Box { b.n().justify = a; return b }

// Center centers the children both ways.
func (b Box) Center() Box { return b.Items(Center).Justify(Center) }

// Background fills the element's rectangle; Radius rounds it; Border draws a
// line of the given width inside it; HoverBackground replaces the fill while the
// pointer is over an element that has an ID. On a control (an Input, a Select, a
// Button), Radius is also the radius of the control's own corners in place of the
// theme's.
func (b Box) Background(c ui.Color) Box      { b.n().background = c; return b }
func (b Box) HoverBackground(c ui.Color) Box { b.f.extra(b.i).hover = c; return b }
func (b Box) Radius(v float32) Box {
	n := b.n()
	n.radius, n.flags = v, n.flags|flagHasRadius
	return b
}
func (b Box) Border(width float32, c ui.Color) Box {
	n := b.n()
	n.borderWidth = width
	b.f.extra(b.i).border = c
	return b
}

// Shadow casts the shadow of that level of the elevation scale (ui.ShadowXS to
// ui.ShadowLG) around the element's rounded rectangle, under its fill.
func (b Box) Shadow(level ui.ShadowLevel) Box { b.f.extra(b.i).shadow = level; return b }

// Clip keeps what is drawn inside the element to its rectangle.
func (b Box) Clip() Box { b.n().flags |= flagClip; return b }

// Text: FontSize, Bold, Weight, Color, Muted and Mono style a Text; TextCenter
// centers it in its rectangle. On a control, FontSize is the size of its text in
// place of the theme's body size.
func (b Box) FontSize(v float32) Box {
	n := b.n()
	n.fontSize = v
	if n.kind == kindSelect {
		b.f.sizeSelectList(n)
	}
	return b
}
func (b Box) Weight(v float32) Box { b.n().weight = v; return b }

// LineHeight sets the height of each line of a Text or Paragraph, in pixels: the
// default is the font size plus seven, as in the components.
func (b Box) LineHeight(v float32) Box { b.n().lineH = v; return b }
func (b Box) Bold() Box                { return b.Weight(ui.WeightSemibold) }
func (b Box) Color(c ui.Color) Box {
	n := b.n()
	n.color = c
	n.flags |= flagHasColor
	return b
}
func (b Box) Muted() Box { return b.Color(b.f.theme.MutedForeground) }
func (b Box) Mono() Box  { b.n().flags |= flagMono; return b }

// IconTransform draws the icon scaled, turned and moved from the box it was laid
// out in, without moving the layout or the hit region: a row keeps its size and
// its target while the glyph moves inside it. Components use it to play the
// short motion a press asks for; see ui.IconTransform for what it costs to turn
// an icon off the pixel grid.
func (b Box) IconTransform(transform ui.IconTransform) Box {
	b.f.extra(b.i).iconTransform = transform
	return b
}

// MonoFace picks the fixed-pitch family a Mono text is drawn in (ui.MonoClassic,
// or one installed family named by ui.MonoFamily); the default is the system's.
func (b Box) MonoFace(face ui.MonoFace) Box { n := b.n(); n.face = face; n.flags |= flagMono; return b }

// Ellipsis keeps a Text to one line and ends it with an ellipsis when it does not
// fit the width it gets, instead of clipping it. It is Paragraph(value, 1).
func (b Box) Ellipsis() Box {
	n := b.n()
	n.flags |= flagWrap
	n.maxLines = 1
	return b
}
func (b Box) TextCenter() Box { b.n().flags |= flagTextCenter; return b }

// TextEnd aligns a Text to the end of its direction instead of the start: the right
// edge for left-to-right text, the left edge for Hebrew, Arabic and other right-to-left
// text. A Text starts at its start edge by default, so Hebrew is right-aligned without
// being asked to.
func (b Box) TextEnd() Box { b.n().textEnd = true; return b }

// TextDirection sets the paragraph direction of a Text or Paragraph instead of taking it
// from its first strong character (UAX #9). A paragraph has one direction: every line of
// it, wrapped, is aligned and ordered by it, including a line that starts with a word of
// the other direction.
func (b Box) TextDirection(direction ui.TextDirection) Box { b.n().textDirection = direction; return b }

// Button variants and sizes.
func (b Box) Secondary() Box   { return b.variant(components.ButtonSecondary) }
func (b Box) Outline() Box     { return b.variant(components.ButtonOutline) }
func (b Box) Ghost() Box       { return b.variant(components.ButtonGhost) }
func (b Box) Link() Box        { return b.variant(components.ButtonLink) }
func (b Box) Destructive() Box { return b.variant(components.ButtonDestructive) }
func (b Box) DestructiveOutline() Box {
	return b.variant(components.ButtonDestructiveOutline)
}
func (b Box) variant(v components.ButtonVariant) Box { b.n().variant = v; return b }
func (b Box) XSmall() Box                            { b.n().buttonSize = components.ButtonSizeXSmall; return b }
func (b Box) Small() Box {
	n := b.n()
	n.buttonSize = components.ButtonSizeSmall
	n.flags |= flagSmall
	return b
}
func (b Box) Large() Box    { b.n().buttonSize = components.ButtonSizeLarge; return b }
func (b Box) IconOnly() Box { b.n().buttonSize = components.ButtonSizeIcon; return b }

// Disabled(true) makes a Button, Checkbox, Switch or Tabs inert and dimmed; Disabled(false)
// leaves it as it is, so a condition is written in the call: `.Disabled(m.count <= 0)`.
func (b Box) Disabled(off bool) Box {
	if off {
		b.n().flags |= flagDisabled
	} else {
		b.n().flags &^= flagDisabled
	}
	return b
}

// Label names a Checkbox or Switch for accessibility.
func (b Box) Label(v string) Box { b.n().label = v; return b }

// At places a Popup with its top left corner at (x, y), moved if need be to stay
// inside the frame: where a context menu opens, at the pointer.
func (b Box) At(x, y float32) Box {
	n := b.n()
	n.place, n.anchor = placeAt, ui.Rect{X: x, Y: y}
	return b
}

// Below places a Popup under anchor, gap pixels away, its left (Start), middle
// (Center) or right (End) edge on the anchor's; if it does not fit under it and fits
// above, it opens above. Above is the same the other way round.
func (b Box) Below(anchor ui.Rect, gap float32, align Align) Box {
	n := b.n()
	n.place, n.anchor, n.placeGap, n.placeAlign = placeBelow, anchor, gap, align
	return b
}

func (b Box) Above(anchor ui.Rect, gap float32, align Align) Box {
	n := b.n()
	n.place, n.anchor, n.placeGap, n.placeAlign = placeAbove, anchor, gap, align
	return b
}

// Beside places a Popup to the right of anchor, gap pixels away, its top (Start) or
// bottom (End) edge on the anchor's; it opens on the anchor's left when there is no
// room on the right. It is for a submenu that opens from a row.
func (b Box) Beside(anchor ui.Rect, gap float32, align Align) Box {
	n := b.n()
	n.place, n.anchor, n.placeGap, n.placeAlign = placeBeside, anchor, gap, align
	return b
}

// Stroke draws a line of the given width around the element without taking any
// room: unlike Border, which is inside the box and eats into the padding, the
// content is laid out as if there were no line.
func (b Box) Stroke(width float32, c ui.Color) Box {
	n := b.n()
	n.borderWidth = width
	n.flags |= flagOutline
	b.f.extra(b.i).border = c
	return b
}

// HitOutset makes the element's pointer region reach beyond its rectangle by the
// given amounts (top, right, bottom, left; negative shrinks it), so a row with an
// inset rounded fill can still be clicked all the way across.
func (b Box) HitOutset(top, right, bottom, left float32) Box {
	b.f.extra(b.i).hitOut = [4]float32{top, right, bottom, left}
	return b
}

// Chosen reports which option of a Segmented control this paint is delivering a click
// on, and false in every other paint, like Clicked for the whole control:
//
//	seg := f.Segmented(options, m.theme)
//	if id, ok := seg.Chosen(); ok { m.theme = id }
func (b Box) Chosen() (ui.ID, bool) {
	n := b.n()
	if n.kind != kindSegmented || b.f.canvas == nil {
		return ui.ID{}, false
	}
	for _, option := range b.f.segmented[n.aux].options {
		if b.f.canvas.Clicked(option.ID) {
			return option.ID, true
		}
	}
	return ui.ID{}, false
}

// BelowBox, AboveBox and BesideBox are Below, Above and Beside for a Popup that hangs
// off another element of the page: the element's rectangle is read when the page is
// laid out, which is the only time it is known (Box.Rect before End is zero).
//
//	filter := f.Button(filterID, "Filter").Outline()
//	f.Popup().BelowBox(filter, 6, flex.End).Children(func() { ... })
func (b Box) BelowBox(anchor Box, gap float32, align Align) Box {
	b.Below(ui.Rect{}, gap, align)
	b.n().anchorRef = anchor.i + 1
	return b
}

func (b Box) AboveBox(anchor Box, gap float32, align Align) Box {
	b.Above(ui.Rect{}, gap, align)
	b.n().anchorRef = anchor.i + 1
	return b
}

func (b Box) BesideBox(anchor Box, gap float32, align Align) Box {
	b.Beside(ui.Rect{}, gap, align)
	b.n().anchorRef = anchor.i + 1
	return b
}

// ChosenIndex is Chosen for a control made by Choice: the index of the clicked label.
func (b Box) ChosenIndex() (int, bool) {
	id, ok := b.Chosen()
	if !ok {
		return 0, false
	}
	n := b.n()
	for index, option := range b.f.segmented[n.aux].options {
		if option.ID == id {
			return index, true
		}
	}
	return 0, false
}
