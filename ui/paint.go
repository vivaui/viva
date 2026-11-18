package ui

import (
	"github.com/vivaui/viva/gfx"
)

func (f *frame) paint(i int32) {
	n := &f.nodes[i]
	if !n.focusScope.IsZero() {
		// A menu's page controls paint in the menu's focus scope, so a press on
		// them reaches them while the menu traps the pointer.
		f.canvas.PushFocusScope(n.focusScope)
		defer f.canvas.PopFocusScope()
	}
	if n.band != 0 {
		// An application overlay's subtree paints on its band, over the page and
		// under Viva's widgets. Hits declared in it capture the same layer, so the
		// band orders Tab and pointer routing with paint.
		f.canvas.PushLayer(n.band)
		defer f.canvas.PopLayer()
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
	if memo := f.exts[n.ext].memo; memo != 0 && f.memoFn != nil {
		if f.memoFn(f, i, true) {
			return
		}
		defer f.memoFn(f, i, false)
	}
	r := n.rect
	canvas := f.canvas
	extra := &f.exts[n.ext]
	style := f.resolve(n, extra)
	// What the element is drawn with fades with it and with what it is inside; the
	// controls inside it are painted with a theme faded the same way.
	parentOpacity, parentTheme := f.opacity, Theme{}
	f.opacity *= style.opacity
	if style.opacity < 1 {
		parentTheme = f.theme // a theme is large: copied only for what is faded
		f.theme = f.theme.WithOpacity(style.opacity)
	}
	f.paintRing(r, n.radius, &style) // outside the rectangle: before a Clip of the element's own
	if n.Flags&flagClip != 0 {
		canvas.Clip(r)
		defer canvas.Unclip()
	}

	hasHit := n.Flags&flagHasHit != 0 && !n.ID.IsZero()
	background, border := fade(style.background, f.opacity), fade(style.border, f.opacity)
	if style.shadow != gfx.ShadowNone {
		canvas.ElevateFaded(r, n.radius, style.shadow, f.opacity)
	}
	switch {
	case n.borderWidth > 0 && !style.dashed && extra.gradient == 0 && f.opacity >= 1 && background.Alpha >= 1:
		canvas.RoundedBorder(r, n.radius, n.borderWidth, border, background)
	default:
		// RoundedBorder paints the border color as a solid backing under the fill, which a
		// gradient or a faded fill would show through. For those the fill, then the
		// gradient, go down whole, as CSS's background reaches under its border, and the
		// border is only a line over them. An element that is neither keeps RoundedBorder,
		// so what an application painted before looks as it did.
		if background.Alpha > 0 {
			if n.radius > 0 {
				canvas.Rounded(r, n.radius, background)
			} else {
				canvas.Fill(r, background)
			}
		}
		if extra.gradient != 0 {
			f.paintGradient(n, extra)
		}
		if n.borderWidth > 0 {
			if style.dashed {
				canvas.DashedRoundedOutline(n.rect, n.radius, n.borderWidth, border)
			} else {
				canvas.RoundedOutline(r, n.radius, n.borderWidth, border)
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
			text := n.label
			if text == "" && n.Kind == kindText {
				text = n.text
			}
			canvas.HitLabel(n.ID, hit, text, n.Flags&flagDisabled != 0)
		} else {
			options := extra.hit
			// Disabled on the element is the control's own: a Base that was told it is
			// unavailable says so to assistive technology and leaves the Tab order,
			// as the control with a look does.
			options.Disabled = options.Disabled || n.Flags&flagDisabled != 0
			canvas.HitWithOptions(n.ID, hit, n.label, options)
		}
	}

	// A Text and an Icon draw with their color, which is the one the looks give.
	color := n.color
	n.color = fade(style.color, f.opacity)
	if n.hasOwnTheme() {
		saved := f.theme
		f.theme = f.ownTheme(n)
		f.paintControl(n, r)
		f.theme = saved
	} else {
		f.paintControl(n, r)
	}
	n.color = color

	var hooks drawHooks
	if extra.draws != 0 {
		hooks = f.draws[extra.draws-1]
	}
	if hooks.under != nil {
		f.runDraw(n, hooks.under)
	}
	for c := n.First; c >= 0; c = f.nodes[c].Next {
		f.paint(c)
	}
	if n.outOfFlow&absChildren != 0 {
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
		f.canvas.HintSince(hits, n.ID, extra.tooltip)
	}
	f.opacity = parentOpacity
	if style.opacity < 1 {
		f.theme = parentTheme
	}
}

// controlPainter paints a control's look. The engine paints text, icons, images
// and slots itself; the controls are a seam a layer above can replace, which is
// what lets the engine live below the styled controls.
var controlPainter = (*frame).paintControls

// paintControl paints what the element is: its text, icon, or control. The
// background and border of its box are already down.
func (f *frame) paintControl(n *node, r Rect) {
	canvas := f.canvas
	extra := &f.exts[n.ext]
	switch n.Kind {
	case kindText:
		f.paintText(n)
	case kindIcon:
		canvas.DrawIconTransformed(extra.icon, Rect{X: r.X, Y: r.Y, Width: n.pref[axisX], Height: n.pref[axisY]}, n.color, extra.iconTransform)
	case kindImage:
		canvas.DrawBitmap(f.images[n.aux], r)
	case kindSlot:
		if extra.swatch && f.paintSwatchFn != nil {
			f.paintSwatchFn(f, n, r)
		}
		if extra.picker != nil && f.paintPickerFn != nil {
			f.paintPickerFn(f, n)
		}
	default:
		controlPainter(f, n, r)
	}
}

func (f *frame) paintText(n *node) {
	if n.text == "" {
		return
	}
	left, right := n.inset(axisX)
	top, bottom := n.inset(axisY)
	r := Rect{X: n.rect.X + left, Y: n.rect.Y + top, Width: max(n.rect.Width-left-right, 0), Height: max(n.rect.Height-top-bottom, 0)}
	style := gfx.TextStyle{Size: n.FontSize, Weight: n.Weight, Color: n.color, Mono: n.Flags&flagMono != 0, MonoFace: n.Face, Direction: n.TextDirection}
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
			style.Direction = gfx.TextDirectionOf(n.text)
		}
		lh := n.leading()
		for k := int32(0); k < n.LineCount; k++ {
			bounds := Rect{X: r.X, Y: r.Y + float32(k)*lh, Width: r.Width, Height: lh}
			f.canvas.LabelAligned(f.lines[n.LineFirst+k], bounds, style, alignment)
		}
		return
	}
	height := max(n.pref[axisY]-top-bottom, 0)
	bounds := Rect{X: r.X, Y: r.Y + max((r.Height-height)/2, 0), Width: r.Width, Height: height}
	clipped := r.Width < n.natural
	if clipped {
		f.canvas.Clip(r)
	}
	f.canvas.LabelAligned(n.text, bounds, style, alignment)
	if clipped {
		f.canvas.Unclip()
	}
}
