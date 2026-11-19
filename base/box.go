package base

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/style"
)

// Box is a handle to one element of the page being declared. It is two words
// and every method returns it, so a declaration reads as a chain; the element it
// names lives in the frame. A Box is valid until the next Begin on its frame:
// Rect and Content stay readable after End, but keeping a Box across paints is a
// mistake, caught where it is used. Keep a gfx.ID or the *ui.Page's name instead.
type Box struct {
	f   *Frame
	i   int32
	gen uint32
}

// Valid reports whether the Box names an element of its frame's current paint.
// The zero value is not valid.
func (b Box) Valid() bool {
	return b.f != nil && b.gen == b.f.gen && b.i >= 0 && int(b.i) < len(b.f.Nodes)
}

func (b Box) N() *Node {
	if b.f == nil || b.gen != b.f.gen {
		panic("ui: a Box is used after its paint; a Box is valid until the next Begin: keep a gfx.ID or the *ui.Page's name across paints")
	}
	return &b.f.Nodes[b.i]
}

// Children declares what is inside the element. fn runs at once, and is not kept.
func (b Box) Children(fn func()) Box {
	b.N()
	saved := b.f.Parent
	b.f.Parent = b.i
	b.N().Flags |= flagChildren
	fn()
	b.f.Parent = saved
	return b
}

// Rect is where the element ended up: valid once the page has been laid out
// (inside a Draw, after End; or from a Begin/End pair).
func (b Box) Rect() Rect { return b.N().Rect }

// Content is the natural height of a Scroll's children, valid after End.
func (b Box) Content() float32 { return b.N().Content }

// Clicked reports whether this paint is delivering a click on the element, which
// needs an ID: a Button, Checkbox or Switch has one, any other element takes one
// with Hit.
func (b Box) Clicked() bool {
	n := b.N()
	if n.Ext != 0 && b.f.Exts[n.Ext].bind != 0 {
		b.f.settleFn(b.f, b.i) // a bound value is written before the caller reads it
	}
	return b.f.Surface != nil && n.Flags&flagVeto == 0 && b.f.Surface.Clicked(n.ID)
}

// Hit makes the element's whole rectangle a pointer target named id, with the
// accessibility label given, so a row or a card can be clicked or hovered.
func (b Box) Hit(ID gfx.ID, Label string) Box {
	n := b.N()
	n.ID, n.Label = ID, Label
	n.Flags = n.Flags&^flagLabelFor | flagHasHit
	return b
}

// MovesWindow makes the element's rectangle a place a custom-titlebar window is
// moved from: pressing and dragging it moves the window, and a double click zooms
// it (see gfx.Canvas.MoveRegion). Controls declared inside the element take their
// own presses. Declare it on the row that is the window's top bar.
func (b Box) MovesWindow() Box {
	b.N()
	b.f.Extra(b.i).moves = true
	return b
}

// HitWith is Hit with the role, cursor and other options of gfx.HitOptions.
func (b Box) HitWith(ID gfx.ID, Label string, options gfx.HitOptions) Box {
	b.N()
	b.f.Extra(b.i).hit = options
	return b.Hit(ID, Label)
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
func (b Box) LabelFor(ID gfx.ID) Box {
	n := b.N()
	n.ID = ID
	n.Flags |= flagHasHit | flagLabelFor
	return b
}

// Width, Height and Size fix a size in pixels. FillWidth, FillHeight and Fill take
// all the room the container has across, or share what is left along it.
func (b Box) Width(v float32) Box  { b.N().Size[AxisX] = Dimension{dimPx, v}; return b }
func (b Box) Height(v float32) Box { b.N().Size[AxisY] = Dimension{dimPx, v}; return b }
func (b Box) Size(w, h float32) Box {
	return b.Width(w).Height(h)
}

// WidthPercent and HeightPercent size an element as a percentage of the room its
// container has (its content box). Basis is the size an element starts from before
// free room is shared, instead of its content's: with Grow it is how proportions are set.
func (b Box) WidthPercent(p float32) Box  { b.N().Size[AxisX] = Dimension{dimPercent, p}; return b }
func (b Box) HeightPercent(p float32) Box { b.N().Size[AxisY] = Dimension{dimPercent, p}; return b }
func (b Box) Basis(v float32) Box {
	n := b.N()
	n.Basis = v
	n.Flags |= flagBasis
	return b
}
func (b Box) FillWidth() Box  { b.N().Size[AxisX] = Dimension{Kind: dimFill}; return b }
func (b Box) FillHeight() Box { b.N().Size[AxisY] = Dimension{Kind: dimFill}; return b }
func (b Box) Fill() Box       { return b.FillWidth().FillHeight() }

// MinWidth, MaxWidth, MinHeight and MaxHeight bound the size the layout gives.
func (b Box) MinWidth(v float32) Box  { b.N().MinS[AxisX] = v; return b }
func (b Box) MaxWidth(v float32) Box  { b.N().MaxS[AxisX] = v; return b }
func (b Box) MinHeight(v float32) Box { b.N().MinS[AxisY] = v; return b }
func (b Box) MaxHeight(v float32) Box { b.N().MaxS[AxisY] = v; return b }

// MaxWidthPercent and MaxHeightPercent cap an element at a share of the room its
// container has (its content box), whatever its content wants: a chat bubble that is at
// most 70% of its row is `MaxWidthPercent(70)`, and a paragraph inside it wraps to what is
// left. They combine with MaxWidth by taking the smaller, and apply to the children of a
// Row, Column or Stack.
func (b Box) MaxWidthPercent(p float32) Box {
	b.N()
	b.f.Extra(b.i).maxPct[AxisX] = p
	return b
}
func (b Box) MaxHeightPercent(p float32) Box {
	b.N()
	b.f.Extra(b.i).maxPct[AxisY] = p
	return b
}

// Grow shares the free room along the container between the elements that grow,
// in proportion to the numbers given. Shrink does the same for a shortage.
func (b Box) Grow(v float32) Box {
	n := b.N()
	n.Grow = v
	n.Flags |= flagGrowSet
	return b
}
func (b Box) Shrink(v float32) Box {
	n := b.N()
	n.Shrink = v
	n.Flags |= flagShrinkSet
	return b
}

// Pad is space inside the element, Margin outside it. The one-argument forms
// give all four sides; the X and Y forms give two; Top, Right, Bottom and Left
// give one; Sides gives them in the order top, right, bottom, left. A later call
// sets the sides it names, so Pad(4).PadTop(8) pads the top by 8 and the other
// sides by 4.
func (b Box) Pad(v float32) Box { return b.PadSides(v, v, v, v) }
func (b Box) PadX(v float32) Box {
	n := b.N()
	n.Pad[1], n.Pad[3], n.Flags = v, v, n.Flags|flagSetPad
	return b
}
func (b Box) PadY(v float32) Box {
	n := b.N()
	n.Pad[0], n.Pad[2], n.Flags = v, v, n.Flags|flagSetPad
	return b
}
func (b Box) PadTop(v float32) Box {
	n := b.N()
	n.Pad[0] = v
	n.Flags |= flagSetPad
	return b
}
func (b Box) PadRight(v float32) Box {
	n := b.N()
	n.Pad[1] = v
	n.Flags |= flagSetPad
	return b
}
func (b Box) PadBottom(v float32) Box {
	n := b.N()
	n.Pad[2] = v
	n.Flags |= flagSetPad
	return b
}
func (b Box) PadLeft(v float32) Box {
	n := b.N()
	n.Pad[3] = v
	n.Flags |= flagSetPad
	return b
}
func (b Box) Margin(v float32) Box  { return b.MarginSides(v, v, v, v) }
func (b Box) MarginX(v float32) Box { n := b.N(); n.Margin[1], n.Margin[3] = v, v; return b }
func (b Box) MarginY(v float32) Box { n := b.N(); n.Margin[0], n.Margin[2] = v, v; return b }
func (b Box) MarginTop(v float32) Box {
	b.N().Margin[0] = v
	return b
}
func (b Box) MarginRight(v float32) Box {
	b.N().Margin[1] = v
	return b
}
func (b Box) MarginBottom(v float32) Box {
	b.N().Margin[2] = v
	return b
}
func (b Box) MarginLeft(v float32) Box {
	b.N().Margin[3] = v
	return b
}
func (b Box) PadSides(top, right, bottom, left float32) Box {
	n := b.N()
	n.Pad, n.Flags = [4]float32{top, right, bottom, left}, n.Flags|flagSetPad
	return b
}
func (b Box) MarginSides(top, right, bottom, left float32) Box {
	b.N().Margin = [4]float32{top, right, bottom, left}
	return b
}

// Gap is the space between the children of a Row, a Column or a Flow, and between
// the lines of a Flow unless LineGap sets them apart.
func (b Box) Gap(v float32) Box { b.N().Gap = v; return b }

// LineGap is the space between the lines of a Flow, when it is not its Gap. It has
// no effect on any other container.
func (b Box) LineGap(v float32) Box {
	if n := b.N(); n.Dir == flow {
		n.GridMin = max(v, 0)
	}
	return b
}

// Items places the children across the container (Start, Center, End or
// Stretch, the default); Self does it for this element alone; Justify places
// them along it when none grows (Start, Center, End or Between).
func (b Box) Items(a Align) Box   { b.N().Items = a; return b }
func (b Box) Self(a Align) Box    { b.N().Self = a; return b }
func (b Box) Justify(a Align) Box { b.N().Justify = a; return b }

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
	n := b.N()
	n.Background, n.Flags = c, n.Flags|flagSetBackground
	return b
}
func (b Box) Radius(v float32) Box {
	n := b.N()
	n.Radius, n.Flags = v, n.Flags|flagHasRadius
	return b
}
func (b Box) Border(width float32, c Color) Box {
	n := b.N()
	n.BorderWidth, n.Flags = width, n.Flags|flagSetBorder
	b.f.Extra(b.i).border = c
	return b
}

// Dashed gives the element's Border or Stroke a repeating dash pattern,
// continuing around rounded corners. It has no effect without a line width;
// calling it again switches back to a solid line.
func (b Box) Dashed() Box {
	b.N()
	extra := b.f.Extra(b.i)
	extra.borderDashed = !extra.borderDashed
	return b
}

// Shadow casts the shadow of that level of the elevation scale (gfx.ShadowXS to
// gfx.ShadowLG) around the element's rounded rectangle, under its fill.
func (b Box) Shadow(level gfx.ShadowLevel) Box {
	b.f.Extra(b.i).shadow = level
	b.N().Flags |= flagSetShadow
	return b
}

// Clip keeps what is drawn inside the element to its rectangle.
func (b Box) Clip() Box { b.N().Flags |= flagClip; return b }

// Text: FontSize, Bold, Weight, Color, Muted and Mono style a Text; TextCenter
// centers it in its rectangle. On a control, FontSize is the size of its text in
// place of the theme's body size.
func (b Box) FontSize(v float32) Box {
	n := b.N()
	n.FontSize = v
	if n.Kind == kindSelect {
		b.f.sizeSelectList(n)
	}
	return b
}
func (b Box) Weight(v float32) Box { b.N().Weight = v; return b }

// LineHeight sets the height of each line of a Text or Paragraph, in pixels: the
// default is the font size plus seven, as in the components.
func (b Box) LineHeight(v float32) Box { b.N().LineH = v; return b }
func (b Box) Bold() Box                { return b.Weight(gfx.WeightSemibold) }
func (b Box) Color(c Color) Box {
	n := b.N()
	n.Color = c
	n.Flags |= flagHasColor
	return b
}
func (b Box) Muted() Box {
	b.N()
	return b.Color(b.f.Palette.MutedForeground)
}
func (b Box) Mono() Box { b.N().Flags |= flagMono; return b }

// IconTransform draws the icon scaled, turned and moved from the box it was laid
// out in, without moving the layout or the hit region: a row keeps its size and
// its target while the glyph moves inside it. Components use it to play the
// short motion a press asks for; see gfx.IconTransform for what it costs to turn
// an icon off the pixel grid.
func (b Box) IconTransform(transform gfx.IconTransform) Box {
	b.N()
	b.f.Extra(b.i).iconTransform = transform
	return b
}

// BindIconHover advances a caller-owned hover gesture for this icon element. The
// element needs a hit region from Hit (or an enclosing interactive row); the
// returned value is the transform the current frame's progress asks for. The
// zero binding, or one without a state, changes nothing.
func (b Box) BindIconHover(hover IconHover) Box {
	b.N()
	return b.IconTransform(hover.Transform(b.f))
}

// MonoFace picks the fixed-pitch family a Mono text is drawn in (gfx.MonoClassic,
// or one installed family named by gfx.MonoFamily); the default is the system's.
func (b Box) MonoFace(Face gfx.MonoFace) Box {
	n := b.N()
	n.Face = Face
	n.Flags |= flagMono
	return b
}

// Ellipsis keeps a Text to one line and ends it with an ellipsis when it does not
// fit the width it gets, instead of clipping it. It is Paragraph(value, 1).
func (b Box) Ellipsis() Box {
	n := b.N()
	n.Flags |= flagWrap
	n.MaxLines = 1
	return b
}
func (b Box) TextCenter() Box { b.N().Flags |= flagTextCenter; return b }

// TextEnd aligns a Text to the end of its direction instead of the start: the right
// edge for left-to-right text, the left edge for Hebrew, Arabic and other right-to-left
// text. A Text starts at its start edge by default, so Hebrew is right-aligned without
// being asked to.
func (b Box) TextEnd() Box { b.N().TextEnd = true; return b }

// TextDirection sets the paragraph direction of a Text or Paragraph instead of taking it
// from its first strong character (UAX #9). A paragraph has one direction: every line of
// it, wrapped, is aligned and ordered by it, including a line that starts with a word of
// the other direction.
func (b Box) TextDirection(direction gfx.TextDirection) Box {
	b.N().TextDirection = direction
	return b
}

// Button variants and sizes.
func (b Box) Secondary() Box   { return b.setVariant(style.VariantSecondary) }
func (b Box) Outline() Box     { return b.setVariant(style.VariantOutline) }
func (b Box) Ghost() Box       { return b.setVariant(style.VariantGhost) }
func (b Box) Link() Box        { return b.setVariant(style.VariantLink) }
func (b Box) Destructive() Box { return b.setVariant(style.VariantDestructive) }
func (b Box) DestructiveOutline() Box {
	return b.setVariant(style.VariantDestructiveOutline)
}
func (b Box) setVariant(v style.Variant) Box { b.N().Variant = v; return b }
func (b Box) XSmall() Box                    { b.N().ButtonSize = style.SizeXSmall; return b }
func (b Box) Small() Box {
	n := b.N()
	n.ButtonSize = style.SizeSmall
	n.Flags |= flagSmall
	return b
}
func (b Box) Large() Box    { b.N().ButtonSize = style.SizeLarge; return b }
func (b Box) IconOnly() Box { b.N().ButtonSize = style.SizeIcon; return b }

// SwitchRecipe gives one Switch its own look, as ButtonRecipe does a Button,
// winning over the page's Styles.Switch. Start from components.SwitchRecipe.
// Other elements ignore it.
//
//	ui.Switch(c, &m.on, "Running").SwitchRecipe(components.SwitchRecipe)
func (b Box) SwitchRecipe(recipe func(gfx.Theme, gfx.State, float32) style.SwitchLook) Box {
	if n := b.N(); n.Kind == kindSwitch {
		b.f.Extra(b.i).switchRecipe = recipe
	}
	return b
}

// ButtonRecipe gives one Button its own look: the function is asked for the colors, radius
// and ring of the button in each state, as PageConfig.Styles.Button is for every button of a
// page, and wins over it. Start from components.ButtonRecipe and change what differs:
//
//	ui.Button(c, "Save").ButtonRecipe(func(t gfx.Theme, v style.Variant, s gfx.State) style.ButtonLook {
//		t.Primary, t.PrimaryForeground = dark, light
//		return components.ButtonRecipe(t, v, s)
//	})
//
// It is for a button whose look is not the theme's; a page that dresses all of them uses Styles,
// and a control with no theme look at all uses ButtonBase. Declare the function once (a
// package-level func or a method value kept in the model), as a closure made in each paint
// allocates. Other elements ignore it.
func (b Box) ButtonRecipe(recipe func(gfx.Theme, style.Variant, gfx.State) style.ButtonLook) Box {
	if n := b.N(); n.Kind == kindButton {
		b.f.Extra(b.i).buttonRecipe = recipe
	}
	return b
}

// Disabled(true) makes a Button, Checkbox, Switch or Tabs inert and dimmed, and an element
// given Hit or HitWith (a Base among them) unavailable to the pointer, the keyboard and
// assistive technology, with the look left to you; Disabled(false)
// makes it enabled again, so a condition is written in the call: `.Disabled(m.count <= 0)`.
func (b Box) Disabled(disabled bool) Box {
	if disabled {
		b.N().Flags |= flagDisabled
	} else {
		b.N().Flags &^= flagDisabled
	}
	return b
}

// Label names a Checkbox or Switch for accessibility.
func (b Box) Label(v string) Box { b.N().Label = v; return b }

// At places a Popup with its top left corner at (x, y), moved if need be to stay
// inside the frame: where a context menu opens, at the pointer.
func (b Box) At(x, y float32) Box {
	n := b.N()
	n.Place, n.Anchor = placeAt, Rect{X: x, Y: y}
	return b
}

// Below places a Popup under anchor, gap pixels away, its left (Start), middle
// (Center) or right (End) edge on the anchor's; if it does not fit under it and fits
// above, it opens above. Above is the same the other way round.
func (b Box) Below(Anchor Rect, Gap float32, align Align) Box {
	n := b.N()
	n.Place, n.Anchor, n.PlaceGap, n.PlaceAlign = placeBelow, Anchor, Gap, align
	return b
}

func (b Box) Above(Anchor Rect, Gap float32, align Align) Box {
	n := b.N()
	n.Place, n.Anchor, n.PlaceGap, n.PlaceAlign = placeAbove, Anchor, Gap, align
	return b
}

// Beside places a Popup to the right of anchor, gap pixels away, its top (Start) or
// bottom (End) edge on the anchor's; it opens on the anchor's left when there is no
// room on the right. It is for a submenu that opens from a row.
func (b Box) Beside(Anchor Rect, Gap float32, align Align) Box {
	n := b.N()
	n.Place, n.Anchor, n.PlaceGap, n.PlaceAlign = placeBeside, Anchor, Gap, align
	return b
}

// Stroke draws a line of the given width around the element without taking any
// room: unlike Border, which is inside the box and eats into the padding, the
// content is laid out as if there were no line.
func (b Box) Stroke(width float32, c Color) Box {
	n := b.N()
	n.BorderWidth = width
	n.Flags |= flagOutline
	b.f.Extra(b.i).border = c
	return b
}

// HitOutset makes the element's pointer region reach beyond its rectangle by the
// given amounts (top, right, bottom, left; negative shrinks it), so a row with an
// inset rounded fill can still be clicked all the way across.
func (b Box) HitOutset(top, right, bottom, left float32) Box {
	b.N()
	b.f.Extra(b.i).hitOut = [4]float32{top, right, bottom, left}
	return b
}

// HitExact makes a styled control's pointer target its laid-out rectangle
// instead of the enlarged desktop target its component paints: a Checkbox in a
// 22x24 slot is clicked across the whole slot, not a 40x32 region around it. The
// painted geometry is unchanged, and it has no effect on controls whose target
// is already their rectangle.
func (b Box) HitExact() Box {
	b.N()
	b.f.Extra(b.i).hitExact = true
	return b
}

// Chosen reports which item this paint is delivering a click on: the option of a
// Segmented control, or the page item of a Pagination. It is false in every other
// paint, like Clicked for the whole control:
//
//	seg := f.Segmented(options, m.theme)
//	if id, ok := seg.Chosen(); ok { m.theme = id }
func (b Box) Chosen() (gfx.ID, bool) {
	n := b.N()
	if b.f.Surface == nil {
		return gfx.ID{}, false
	}
	if n.Flags&flagAlertDialog != 0 {
		spec := &WidgetsOf(b.f).alertDialogs[n.Aux].spec
		for _, id := range [2]gfx.ID{spec.ActionID, spec.CancelID} {
			if !id.IsZero() && b.f.Surface.Clicked(id) {
				return id, true
			}
		}
		return gfx.ID{}, false
	}
	switch n.Kind {
	case kindSegmented:
		for _, option := range WidgetsOf(b.f).segmented[n.Aux].options {
			if b.f.Surface.Clicked(option.ID) {
				return option.ID, true
			}
		}
	case kindPagination:
		for _, item := range b.f.PaginationItems(n) {
			if !item.ID.IsZero() && b.f.Surface.Clicked(item.ID) {
				return item.ID, true
			}
		}
	case kindSplitButton:
		d := &WidgetsOf(b.f).splitButtons[n.Aux]
		for _, id := range [2]gfx.ID{d.primary, d.menu} {
			if !id.IsZero() && b.f.Surface.Clicked(id) {
				return id, true
			}
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
func (b Box) BelowBox(Anchor Box, Gap float32, align Align) Box {
	Anchor.N()
	b.Below(Rect{}, Gap, align)
	b.N().AnchorRef = Anchor.i + 1
	return b
}

func (b Box) AboveBox(Anchor Box, Gap float32, align Align) Box {
	Anchor.N()
	b.Above(Rect{}, Gap, align)
	b.N().AnchorRef = Anchor.i + 1
	return b
}

func (b Box) BesideBox(Anchor Box, Gap float32, align Align) Box {
	Anchor.N()
	b.Beside(Rect{}, Gap, align)
	b.N().AnchorRef = Anchor.i + 1
	return b
}

// ChosenIndex is Chosen for a control made by Choice: the index of the clicked label.
func (b Box) ChosenIndex() (int, bool) {
	if b.N().Kind != kindSegmented {
		return 0, false
	}
	id, ok := b.Chosen()
	if !ok {
		return 0, false
	}
	n := b.N()
	for index, option := range WidgetsOf(b.f).segmented[n.Aux].options {
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
	if b.f.Surface == nil || !b.f.Surface.Clicked(b.N().ID) {
		return gfx.ClickInfo{}, false
	}
	return b.f.Surface.ClickInfo()
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
