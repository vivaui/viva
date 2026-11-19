package base

import (
	"github.com/vivaui/viva/gfx"
)

func (f *Frame) PaintChild(i int32) {
	n := &f.Nodes[i]
	if !n.FocusScope.IsZero() {
		// A menu's page controls paint in the menu's focus scope, so a press on
		// them reaches them while the menu traps the pointer.
		f.Surface.PushFocusScope(n.FocusScope)
		defer f.Surface.PopFocusScope()
	}
	if n.Band != 0 {
		// An application overlay's subtree paints on its band, over the page and
		// under Viva's widgets. Hits declared in it capture the same layer, so the
		// band orders Tab and pointer routing with paint.
		f.Surface.PushLayer(n.Band)
		defer f.Surface.PopLayer()
	}
	if n.Flags&flagDialog != 0 {
		f.paintDialog(n)
		return
	}
	if n.Flags&flagAlertDialog != 0 {
		f.paintAlertDialog(n)
		return
	}
	if n.Flags&flagDrawer != 0 {
		f.paintDrawerFn(f, n)
		return
	}
	if n.Flags&flagPopover != 0 {
		f.paintPopoverFn(f, i, n)
		return
	}
	if memo := f.Exts[n.Ext].memo; memo != 0 && f.memoFn != nil {
		if f.memoFn(f, i, true) {
			return
		}
		defer f.memoFn(f, i, false)
	}
	r := n.Rect
	canvas := f.Surface
	extra := &f.Exts[n.Ext]
	style := f.resolve(n, extra)
	// What the element is drawn with fades with it and with what it is inside; the
	// controls inside it are painted with a theme faded the same way.
	parentOpacity, parentTheme := f.Opacity, Theme{}
	f.Opacity *= style.Opacity
	if style.Opacity < 1 {
		parentTheme = f.Palette // a theme is large: copied only for what is faded
		f.Palette = f.Palette.WithOpacity(style.Opacity)
	}
	f.paintRing(r, n.Radius, &style) // outside the rectangle: before a Clip of the element's own
	if n.Flags&flagClip != 0 {
		canvas.Clip(r)
		defer canvas.Unclip()
	}

	hasHit := n.Flags&flagHasHit != 0 && !n.ID.IsZero()
	background, border := fade(style.Background, f.Opacity), fade(style.border, f.Opacity)
	if style.shadow != gfx.ShadowNone {
		canvas.ElevateFaded(r, n.Radius, style.shadow, f.Opacity)
	}
	switch {
	case n.BorderWidth > 0 && !style.dashed && extra.gradient == 0 && f.Opacity >= 1 && background.Alpha >= 1:
		canvas.RoundedBorder(r, n.Radius, n.BorderWidth, border, background)
	default:
		// RoundedBorder paints the border color as a solid backing under the fill, which a
		// gradient or a faded fill would show through. For those the fill, then the
		// gradient, go down whole, as CSS's background reaches under its border, and the
		// border is only a line over them. An element that is neither keeps RoundedBorder,
		// so what an application painted before looks as it did.
		if background.Alpha > 0 {
			if n.Radius > 0 {
				canvas.Rounded(r, n.Radius, background)
			} else {
				canvas.Fill(r, background)
			}
		}
		if extra.gradient != 0 {
			f.paintGradient(n, extra)
		}
		if n.BorderWidth > 0 {
			if style.dashed {
				canvas.DashedRoundedOutline(n.Rect, n.Radius, n.BorderWidth, border)
			} else {
				canvas.RoundedOutline(r, n.Radius, n.BorderWidth, border)
			}
		}
	}
	if extra.moves {
		canvas.MoveRegion(r)
	}
	if extra.nativeView != 0 {
		canvas.NativeView(extra.nativeView, r)
	}
	var hits gfx.Mark // where the element's own hits start, for its Tooltip
	if extra.tooltip != "" {
		hits = canvas.Mark()
	}
	var group gfx.Mark // where the hits of a FocusGroup start
	if extra.focusGroup != 0 && canvas != nil {
		group = canvas.Mark()
	}
	// A ColorSwatch declares its own hit, as a radio, under whatever id Hit gave it.
	if hasHit && !extra.swatch && (n.Kind == kindContainer || n.Kind == kindText || n.Kind == kindIcon || n.Kind == kindSlot) {
		h := extra.hitOut
		hit := Rect{X: r.X - h[3], Y: r.Y - h[0], Width: r.Width + h[1] + h[3], Height: r.Height + h[0] + h[2]}
		if n.Flags&flagLabelFor != 0 {
			text := n.Label
			if text == "" && n.Kind == kindText {
				text = n.Text
			}
			canvas.HitLabel(n.ID, hit, text, n.Flags&flagDisabled != 0)
		} else {
			options := extra.hit
			// Disabled on the element is the control's own: a Base that was told it is
			// unavailable says so to assistive technology and leaves the Tab order,
			// as the control with a look does.
			options.Disabled = options.Disabled || n.Flags&flagDisabled != 0
			canvas.HitWithOptions(n.ID, hit, n.Label, options)
		}
	}

	// A Text and an Icon draw with their color, which is the one the looks give.
	color := n.Color
	n.Color = fade(style.Color, f.Opacity)
	if n.HasOwnTheme() {
		saved := f.Palette
		f.Palette = f.OwnTheme(n)
		f.paintControl(n, r)
		f.Palette = saved
	} else {
		f.paintControl(n, r)
	}
	n.Color = color

	var hooks drawHooks
	if extra.Draws != 0 {
		hooks = f.Draws[extra.Draws-1]
	}
	if hooks.under != nil {
		f.runDraw(n, hooks.under)
	}
	for c := n.First; c >= 0; c = f.Nodes[c].Next {
		f.PaintChild(c)
	}
	if n.OutOfFlow&absChildren != 0 {
		f.paintAbsolute(i)
	}
	if hooks.over != nil {
		f.runDraw(n, hooks.over)
	}
	if s := extra.scrollState; s != nil && s.bars && canvas != nil {
		f.paintBarsFn(f, n, s) // over the content, inside the area's clip
	}
	if extra.focusGroup != 0 && canvas != nil {
		f.groupFn(f, i, extra.focusGroup, group)
	}
	if extra.tooltip != "" {
		// The element's hit, which a container, a control or its Draw declared.
		f.Surface.HintSince(hits, n.ID, extra.tooltip)
	}
	f.Opacity = parentOpacity
	if style.Opacity < 1 {
		f.Palette = parentTheme
	}
}

// The control painters, by Kind: the engine paints text, icons, images and slots
// itself; a layer above installs the controls' looks, so the engine lives below
// the styled controls and links only the painter a page uses.
const controlKindCount = int(KindPagination) + 1

var controlPainters [controlKindCount]func(f *Frame, n *Node, r Rect)

// SetControlPainter installs the look of one control kind. It is a bridge for
// the styled layer; callers of the page API never need it.
func SetControlPainter(kind Kind, painter func(f *Frame, n *Node, r Rect)) {
	if int(kind) < len(controlPainters) {
		controlPainters[kind] = painter
	}
}

// ControlBaseline is the label baseline of a button.
var ControlBaseline func(f *Frame, n *Node) float32

// paintControl paints what the element is: its text, icon, or control. The
// background and border of its box are already down.
func (f *Frame) paintControl(n *Node, r Rect) {
	canvas := f.Surface
	extra := &f.Exts[n.Ext]
	switch n.Kind {
	case kindText:
		f.paintText(n)
	case kindIcon:
		canvas.DrawIconTransformed(extra.icon, Rect{X: r.X, Y: r.Y, Width: n.Pref[AxisX], Height: n.Pref[AxisY]}, n.Color, extra.iconTransform)
	case kindImage:
		canvas.DrawBitmap(f.Images[n.Aux], r)
	case kindSlot:
		if extra.swatch && f.paintSwatchFn != nil {
			f.paintSwatchFn(f, n, r)
		}
		if extra.picker != nil && f.paintPickerFn != nil {
			f.paintPickerFn(f, n)
		}
	default:
		if int(n.Kind) < len(controlPainters) {
			if painter := controlPainters[n.Kind]; painter != nil {
				painter(f, n, r)
			}
		}
	}
}

func (f *Frame) paintText(n *Node) {
	if n.Text == "" {
		return
	}
	left, right := n.Inset(AxisX)
	top, bottom := n.Inset(AxisY)
	r := Rect{X: n.Rect.X + left, Y: n.Rect.Y + top, Width: max(n.Rect.Width-left-right, 0), Height: max(n.Rect.Height-top-bottom, 0)}
	style := gfx.TextStyle{Size: n.FontSize, Weight: n.Weight, Color: n.Color, Mono: n.Flags&flagMono != 0, MonoFace: n.Face, Direction: n.TextDirection}
	alignment := gfx.AlignStart
	switch {
	case n.Flags&flagTextCenter != 0:
		alignment = gfx.AlignCenter
	case n.TextEnd:
		alignment = gfx.AlignEnd
	}
	if n.Flags&flagWrap != 0 {
		// A paragraph has one direction: each of its lines follows it, not the first strong
		// character of the line.
		if style.Direction == gfx.DirectionAuto {
			style.Direction = gfx.TextDirectionOf(n.Text)
		}
		lh := n.leading()
		for k := int32(0); k < n.LineCount; k++ {
			bounds := Rect{X: r.X, Y: r.Y + float32(k)*lh, Width: r.Width, Height: lh}
			f.Surface.LabelAligned(f.Lines[n.LineFirst+k], bounds, style, alignment)
		}
		return
	}
	height := max(n.Pref[AxisY]-top-bottom, 0)
	bounds := Rect{X: r.X, Y: r.Y + max((r.Height-height)/2, 0), Width: r.Width, Height: height}
	clipped := r.Width < n.Natural
	if clipped {
		f.Surface.Clip(r)
	}
	f.Surface.LabelAligned(n.Text, bounds, style, alignment)
	if clipped {
		f.Surface.Unclip()
	}
}
