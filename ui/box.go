package ui

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
)

// Box is a handle to one element of the page being declared. It is two words
// and every method returns it, so a declaration reads as a chain; the element it
// names lives in the frame. A Box is valid until the next Begin on its frame:
// Rect and Content stay readable after End, but keeping a Box across paints is a
// mistake, caught where it is used. Keep a gfx.ID or the *ui.Page's name instead.
type Box struct {
	f   *frame
	i   int32
	gen uint32
}

// Valid reports whether the Box names an element of its frame's current paint.
// The zero value is not valid.
func (b Box) Valid() bool {
	return b.f != nil && b.gen == b.f.gen && b.i >= 0 && int(b.i) < len(b.f.nodes)
}

func (b Box) n() *node {
	if b.f == nil || b.gen != b.f.gen {
		panic("ui: a Box is used after its paint; a Box is valid until the next Begin: keep a gfx.ID or the *ui.Page's name across paints")
	}
	return &b.f.nodes[b.i]
}

// Children declares what is inside the element. fn runs at once, and is not kept.
func (b Box) Children(fn func()) Box {
	b.n()
	saved := b.f.parent
	b.f.parent = b.i
	b.n().flags |= flagChildren
	fn()
	b.f.parent = saved
	return b
}

// Rect is where the element ended up: valid once the page has been laid out
// (inside a Draw, after End; or from a Begin/End pair).
func (b Box) Rect() Rect { return b.n().rect }

// Content is the natural height of a Scroll's children, valid after End.
func (b Box) Content() float32 { return b.n().content }

// Clicked reports whether this paint is delivering a click on the element, which
// needs an ID: a Button, Checkbox or Switch has one, any other element takes one
// with Hit.
func (b Box) Clicked() bool {
	n := b.n()
	if n.ext != 0 && b.f.exts[n.ext].bind != 0 {
		b.f.settleFn(b.f, b.i) // a bound value is written before the caller reads it
	}
	return b.f.canvas != nil && n.flags&flagVeto == 0 && b.f.canvas.Clicked(n.id)
}

// Hit makes the element's whole rectangle a pointer target named id, with the
// accessibility label given, so a row or a card can be clicked or hovered.
func (b Box) Hit(id gfx.ID, label string) Box {
	n := b.n()
	n.id, n.label = id, label
	n.flags = n.flags&^flagLabelFor | flagHasHit
	return b
}

// MovesWindow makes the element's rectangle a place a custom-titlebar window is
// moved from: pressing and dragging it moves the window, and a double click zooms
// it (see gfx.Canvas.MoveRegion). Controls declared inside the element take their
// own presses. Declare it on the row that is the window's top bar.
func (b Box) MovesWindow() Box {
	b.n()
	b.f.extra(b.i).moves = true
	return b
}

// HitWith is Hit with the role, cursor and other options of gfx.HitOptions.
func (b Box) HitWith(id gfx.ID, label string, options gfx.HitOptions) Box {
	b.n()
	b.f.extra(b.i).hit = options
	return b.Hit(id, label)
}

// LabelFor makes the element's rectangle a label of the control named id, as HTML's
// <label for> is (see gfx.Canvas.HitLabel): a click on it is the control's click, it
// is hovered and pressed as the control is, and it is no tab stop and no node of its
// own for assistive technology. A ContextMenu given the element opens on the control
// too, so what a component declares inside one of its rows, such as a Tree's name, can
// carry the row's menu without taking the row's clicks. The element's Label, or a
// Text's words, names the control in place of its own label; a label without
// words names nothing, and the control keeps its own name. Declare LabelFor
// before HitWith when the element needs the options too: after it, Hit restores
// the element's own control semantics, and LabelFor would leave them unclaimed.
//
//	name := ui.Text(c, file.Name).LabelFor(m.files.RowID(i))
//	ui.ContextMenu(c, name, func(menu *ui.MenuItems) { ... })
func (b Box) LabelFor(id gfx.ID) Box {
	n := b.n()
	n.id = id
	n.flags |= flagHasHit | flagLabelFor
	return b
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
func (b Box) MaxWidthPercent(p float32) Box {
	b.n()
	b.f.extra(b.i).maxPct[axisX] = p
	return b
}
func (b Box) MaxHeightPercent(p float32) Box {
	b.n()
	b.f.extra(b.i).maxPct[axisY] = p
	return b
}

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
// give all four sides; the X and Y forms give two; Top, Right, Bottom and Left
// give one; Sides gives them in the order top, right, bottom, left. A later call
// sets the sides it names, so Pad(4).PadTop(8) pads the top by 8 and the other
// sides by 4.
func (b Box) Pad(v float32) Box { return b.PadSides(v, v, v, v) }
func (b Box) PadX(v float32) Box {
	n := b.n()
	n.pad[1], n.pad[3], n.flags = v, v, n.flags|flagSetPad
	return b
}
func (b Box) PadY(v float32) Box {
	n := b.n()
	n.pad[0], n.pad[2], n.flags = v, v, n.flags|flagSetPad
	return b
}
func (b Box) PadTop(v float32) Box {
	n := b.n()
	n.pad[0] = v
	n.flags |= flagSetPad
	return b
}
func (b Box) PadRight(v float32) Box {
	n := b.n()
	n.pad[1] = v
	n.flags |= flagSetPad
	return b
}
func (b Box) PadBottom(v float32) Box {
	n := b.n()
	n.pad[2] = v
	n.flags |= flagSetPad
	return b
}
func (b Box) PadLeft(v float32) Box {
	n := b.n()
	n.pad[3] = v
	n.flags |= flagSetPad
	return b
}
func (b Box) Margin(v float32) Box  { return b.MarginSides(v, v, v, v) }
func (b Box) MarginX(v float32) Box { n := b.n(); n.margin[1], n.margin[3] = v, v; return b }
func (b Box) MarginY(v float32) Box { n := b.n(); n.margin[0], n.margin[2] = v, v; return b }
func (b Box) MarginTop(v float32) Box {
	b.n().margin[0] = v
	return b
}
func (b Box) MarginRight(v float32) Box {
	b.n().margin[1] = v
	return b
}
func (b Box) MarginBottom(v float32) Box {
	b.n().margin[2] = v
	return b
}
func (b Box) MarginLeft(v float32) Box {
	b.n().margin[3] = v
	return b
}
func (b Box) PadSides(top, right, bottom, left float32) Box {
	n := b.n()
	n.pad, n.flags = [4]float32{top, right, bottom, left}, n.flags|flagSetPad
	return b
}
func (b Box) MarginSides(top, right, bottom, left float32) Box {
	b.n().margin = [4]float32{top, right, bottom, left}
	return b
}

// Gap is the space between the children of a Row, a Column or a Flow, and between
// the lines of a Flow unless LineGap sets them apart.
func (b Box) Gap(v float32) Box { b.n().gap = v; return b }

// LineGap is the space between the lines of a Flow, when it is not its Gap. It has
// no effect on any other container.
func (b Box) LineGap(v float32) Box {
	if n := b.n(); n.dir == flow {
		n.gridMin = max(v, 0)
	}
	return b
}

// Items places the children across the container (Start, Center, End or
// Stretch, the default); Self does it for this element alone; Justify places
// them along it when none grows (Start, Center, End or Between).
func (b Box) Items(a Align) Box   { b.n().items = a; return b }
func (b Box) Self(a Align) Box    { b.n().self = a; return b }
func (b Box) Justify(a Align) Box { b.n().justify = a; return b }

// Center centers the children both ways.
func (b Box) Center() Box { return b.Items(Center).Justify(Center) }

// Background fills the element's rectangle; Radius rounds it; Border draws a
// line of the given width inside it. How the element looks while the pointer is
// over it, pressed or focused is declared with Hover, Pressed and Focus (see
// style.go). On a control (an Input, a Select, a Button), Radius is also the
// radius of the control's own corners in place of the theme's.
//
// Which controls take these calls as their look: Input and Textarea do (what is set replaces
// the recipe's value, the rest stays), and every Base (InputBase, ButtonBase, CheckboxBase...)
// is dressed by them alone. A Button, Checkbox, Switch, Select, Tabs, Segmented, Slider or
// InputGroup with the theme's look paints its own surface over the element's, so to give one
// a look of your own use its Base, or the items it is made of (see bases.go); the recipes in
// package components (ButtonRecipe, CheckboxRecipe...) give the theme's values to start from.
func (b Box) Background(c Color) Box {
	n := b.n()
	n.background, n.flags = c, n.flags|flagSetBackground
	return b
}
func (b Box) Radius(v float32) Box {
	n := b.n()
	n.radius, n.flags = v, n.flags|flagHasRadius
	return b
}
func (b Box) Border(width float32, c Color) Box {
	n := b.n()
	n.borderWidth, n.flags = width, n.flags|flagSetBorder
	b.f.extra(b.i).border = c
	return b
}

// Shadow casts the shadow of that level of the elevation scale (gfx.ShadowXS to
// gfx.ShadowLG) around the element's rounded rectangle, under its fill.
func (b Box) Shadow(level gfx.ShadowLevel) Box {
	b.f.extra(b.i).shadow = level
	b.n().flags |= flagSetShadow
	return b
}

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
func (b Box) Bold() Box                { return b.Weight(gfx.WeightSemibold) }
func (b Box) Color(c Color) Box {
	n := b.n()
	n.color = c
	n.flags |= flagHasColor
	return b
}
func (b Box) Muted() Box {
	b.n()
	return b.Color(b.f.theme.MutedForeground)
}
func (b Box) Mono() Box { b.n().flags |= flagMono; return b }

// IconTransform draws the icon scaled, turned and moved from the box it was laid
// out in, without moving the layout or the hit region: a row keeps its size and
// its target while the glyph moves inside it. Components use it to play the
// short motion a press asks for; see gfx.IconTransform for what it costs to turn
// an icon off the pixel grid.
func (b Box) IconTransform(transform gfx.IconTransform) Box {
	b.n()
	b.f.extra(b.i).iconTransform = transform
	return b
}

// BindIconHover advances a caller-owned hover gesture for this icon element. The
// element needs a hit region from Hit (or an enclosing interactive row); the
// returned value is the transform the current frame's progress asks for. The
// zero binding, or one without a state, changes nothing.
func (b Box) BindIconHover(hover IconHover) Box {
	b.n()
	return b.IconTransform(hover.Transform(b.f))
}

// MonoFace picks the fixed-pitch family a Mono text is drawn in (gfx.MonoClassic,
// or one installed family named by gfx.MonoFamily); the default is the system's.
func (b Box) MonoFace(face gfx.MonoFace) Box {
	n := b.n()
	n.face = face
	n.flags |= flagMono
	return b
}

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
func (b Box) TextDirection(direction gfx.TextDirection) Box {
	b.n().textDirection = direction
	return b
}

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
	n.place, n.anchor = placeAt, Rect{X: x, Y: y}
	return b
}

// Below places a Popup under anchor, gap pixels away, its left (Start), middle
// (Center) or right (End) edge on the anchor's; if it does not fit under it and fits
// above, it opens above. Above is the same the other way round.
func (b Box) Below(anchor Rect, gap float32, align Align) Box {
	n := b.n()
	n.place, n.anchor, n.placeGap, n.placeAlign = placeBelow, anchor, gap, align
	return b
}

func (b Box) Above(anchor Rect, gap float32, align Align) Box {
	n := b.n()
	n.place, n.anchor, n.placeGap, n.placeAlign = placeAbove, anchor, gap, align
	return b
}

// Beside places a Popup to the right of anchor, gap pixels away, its top (Start) or
// bottom (End) edge on the anchor's; it opens on the anchor's left when there is no
// room on the right. It is for a submenu that opens from a row.
func (b Box) Beside(anchor Rect, gap float32, align Align) Box {
	n := b.n()
	n.place, n.anchor, n.placeGap, n.placeAlign = placeBeside, anchor, gap, align
	return b
}

// Stroke draws a line of the given width around the element without taking any
// room: unlike Border, which is inside the box and eats into the padding, the
// content is laid out as if there were no line.
func (b Box) Stroke(width float32, c Color) Box {
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
	b.n()
	b.f.extra(b.i).hitOut = [4]float32{top, right, bottom, left}
	return b
}

// Chosen reports which option of a Segmented control this paint is delivering a click
// on, and false in every other paint, like Clicked for the whole control:
//
//	seg := f.Segmented(options, m.theme)
//	if id, ok := seg.Chosen(); ok { m.theme = id }
func (b Box) Chosen() (gfx.ID, bool) {
	n := b.n()
	if n.kind != kindSegmented || b.f.canvas == nil {
		return gfx.ID{}, false
	}
	for _, option := range b.f.segmented[n.aux].options {
		if b.f.canvas.Clicked(option.ID) {
			return option.ID, true
		}
	}
	return gfx.ID{}, false
}

// BelowBox, AboveBox and BesideBox are Below, Above and Beside for a Popup that hangs
// off another element of the page: the element's rectangle is read when the page is
// laid out, which is the only time it is known (Box.Rect before End is zero).
//
//	filter := f.Button(filterID, "Filter").Outline()
//	f.Popup().BelowBox(filter, 6, ui.End).Children(func() { ... })
func (b Box) BelowBox(anchor Box, gap float32, align Align) Box {
	anchor.n()
	b.Below(Rect{}, gap, align)
	b.n().anchorRef = anchor.i + 1
	return b
}

func (b Box) AboveBox(anchor Box, gap float32, align Align) Box {
	anchor.n()
	b.Above(Rect{}, gap, align)
	b.n().anchorRef = anchor.i + 1
	return b
}

func (b Box) BesideBox(anchor Box, gap float32, align Align) Box {
	anchor.n()
	b.Beside(Rect{}, gap, align)
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

// ClickModifiers returns the modifier keys held on the click this paint delivers
// on the element: Primary (Command on macOS, Control elsewhere) distinguishes a
// multi-selection click, Shift one that extends it. It is zero outside the paint
// that delivers the click, and for Enter or Space on the element, which are not
// pointer clicks.
func (b Box) ClickModifiers() gfx.Modifiers {
	info, _ := b.clickInfo()
	return info.Modifiers
}

// DoubleClicked reports whether the click this paint delivers on the element was
// the second of a double click.
func (b Box) DoubleClicked() bool {
	info, ok := b.clickInfo()
	return ok && info.Clicks >= 2
}

func (b Box) clickInfo() (gfx.ClickInfo, bool) {
	if b.f.canvas == nil || !b.f.canvas.Clicked(b.n().id) {
		return gfx.ClickInfo{}, false
	}
	return b.f.canvas.ClickInfo()
}

// ScrollWith makes the element's content follow the sideways scroll of the rows of
// list (ListState.RowWidth): a header above the list, whose columns line up with the
// rows. The element is taken out of the flow and placed at the top left of its
// container, moved left by the list's offset; give it the rows' width, and put it in
// a container with the header's height that clips. It follows the rows exactly: a
// negative margin would widen the header instead of moving it.
//
//	ui.Row(c).Clip().Height(28).Children(func() {
//		ui.Row(c).Width(m.files.RowWidth).ScrollWith(&m.files).Children(m.columnTitles)
//	})
func (b Box) ScrollWith(list *ListState) Box {
	return b.Absolute().Top(0).Left(-list.ScrollX())
}
