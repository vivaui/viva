package components

import (
	"math"
	"sync"

	"github.com/vivaui/viva/gfx"
)

// AvatarEditorIDs are caller-owned, stable identities. A zero ID omits that
// control. Canvas enables dragging, wheel zoom and keyboard editing; Reset
// adds a reset button when there is room.
type AvatarEditorIDs struct {
	Canvas, Zoom, ZoomOut, ZoomIn, Reset gfx.ID
}

// AvatarEditorLabels localizes the accessible names and empty states. Empty
// fields use English defaults.
type AvatarEditorLabels struct {
	Hint, Canvas, Zoom, ZoomOut, ZoomIn, Reset string
	Loading, Unavailable                       string
}

// AvatarEditorState is caller-owned and survives paints. Zoom 1 fits the
// largest crop of the requested aspect ratio inside the source. CenterX and
// CenterY are normalized source coordinates. Use the zero value for a new
// image or to reset to the largest centered crop allowed by the options.
type AvatarEditorState struct {
	Zoom, CenterX, CenterY float32
	gesture                avatarEditorGesture
	dragX, dragY           float32
}

type avatarEditorGesture uint8

const (
	avatarEditorGestureNone avatarEditorGesture = iota
	avatarEditorGestureCanvas
	avatarEditorGestureZoom
)

// AvatarEditorOptions is a snapshot for one paint. The bitmap remains owned
// by the caller; its dimensions define the coordinate space for Crop. See
// gfx.Bitmap for pixel format and cache identity.
type AvatarEditorOptions struct {
	IDs    AvatarEditorIDs
	Labels AvatarEditorLabels
	Bitmap gfx.Bitmap
	State  AvatarEditorState

	// AspectRatio is width / height, defaulting to 1.
	AspectRatio float32
	// MinZoom defaults to 1 and cannot go below it. MaxZoom defaults to 8
	// (or MinZoom, if larger). Set both equal for a fixed zoom.
	MinZoom, MaxZoom float32
	// MaxStageSize caps the preview's longest edge, defaulting to 320
	// logical pixels. The preview also fits inside the supplied bounds.
	MaxStageSize float32
	// Circular masks a square preview; Crop is still a rectangle. Backdrop
	// must match the containing surface's solid, opaque color and defaults
	// to theme.Background.
	Circular bool
	Backdrop gfx.Color
	// Loading or Error disables editing, even if Bitmap is still present.
	// Error takes precedence when both are set.
	Loading bool
	Error   string
}

// AvatarEditorLayout describes the current frame and routes its input. Keep
// it until the next paint. When Ready, Crop is the selected rectangle in
// bitmap pixels and Stage is the preview in logical window coordinates.
type AvatarEditorLayout struct {
	Stage, Crop gfx.Rect
	Ready       bool
	State       AvatarEditorState

	ids        AvatarEditorIDs
	zoomBounds gfx.Rect
	geometry   avatarCropGeometry
}

// AvatarEditor paints a crop surface. Loading an image, choosing a container
// and applying or exporting the crop are the caller's responsibility.
type AvatarEditor struct {
	canvas  *gfx.Canvas
	theme   gfx.Theme
	bounds  gfx.Rect
	options AvatarEditorOptions
}

// BeginAvatarEditor prepares a crop surface for the caller-provided bounds.
func BeginAvatarEditor(canvas *gfx.Canvas, theme gfx.Theme, bounds gfx.Rect, options AvatarEditorOptions) AvatarEditor {
	return AvatarEditor{canvas: canvas, theme: theme, bounds: bounds, options: options}
}

// Paint draws the preview and the controls that fit. Save the returned State
// so zoom and reset button clicks persist, and save the layout to route input.
func (editor AvatarEditor) Paint() AvatarEditorLayout {
	c, theme, bounds, o := editor.canvas, editor.theme, editor.bounds, editor.options
	out := AvatarEditorLayout{State: o.State, ids: o.IDs}
	if c == nil || !finiteAvatarValue(bounds.Width) || !finiteAvatarValue(bounds.Height) ||
		bounds.Width <= 0 || bounds.Height <= 0 {
		return out
	}
	labels := normalizedAvatarEditorLabels(o.Labels)
	aspect, minZoom, maxZoom := avatarEditorLimits(o)
	stageMax := o.MaxStageSize
	if !finiteAvatarValue(stageMax) || stageMax <= 0 {
		stageMax = 320
	}

	c.Clip(bounds)
	defer c.Unclip()
	row := avatarEditorControls(c, theme, bounds, o.IDs, labels.Reset)
	availableHeight := bounds.Height
	if row.height > 0 {
		availableHeight -= row.height + avatarEditorRowGap
	}
	width, height := min(bounds.Width, stageMax), min(availableHeight, stageMax)
	if aspect >= 1 {
		width = min(width, height*aspect)
		height = width / aspect
	} else {
		height = min(height, width/aspect)
		width = height * aspect
	}
	stage := gfx.Rect{X: bounds.X + (bounds.Width-width)/2, Y: bounds.Y, Width: width, Height: height}
	out.Stage = stage
	if width <= 0 || height <= 0 {
		return out
	}
	controls := row.at(bounds.X+(bounds.Width-row.width)/2, stage.Y+stage.Height+avatarEditorRowGap)
	out.zoomBounds = controls[1]

	ready := !o.Loading && o.Error == "" && avatarBitmapReady(o.Bitmap)
	if ready {
		out.geometry = newAvatarCropGeometry(o.Bitmap, aspect, minZoom, maxZoom)
	}
	state := o.State
	if ready {
		state = out.geometry.normalize(state)
		// Apply button clicks before drawing so Crop and the preview agree.
		switch {
		case controls[0].Width > 0 && c.Clicked(o.IDs.ZoomOut):
			state = out.geometry.zoom(state, state.Zoom/1.2, .5, .5)
		case controls[2].Width > 0 && c.Clicked(o.IDs.ZoomIn):
			state = out.geometry.zoom(state, state.Zoom*1.2, .5, .5)
		case controls[3].Width > 0 && c.Clicked(o.IDs.Reset):
			state = out.geometry.normalize(AvatarEditorState{})
		}
		out.Crop = out.geometry.crop(state)
		ready = out.Crop.X+out.Crop.Width > out.Crop.X && out.Crop.Y+out.Crop.Height > out.Crop.Y
		if !ready {
			out.Crop = gfx.Rect{}
		}
	} else {
		state.gesture = avatarEditorGestureNone
	}
	out.State, out.Ready = state, ready

	avatarEditorPaintStage(c, theme, stage, out, o, labels, o.Circular && aspect == 1)
	if controls[0].Width > 0 {
		avatarEditorZoomButton(c, theme, controls[0], o.IDs.ZoomOut, avatarEditorZoomOutIcon, labels.ZoomOut, !ready || state.Zoom <= minZoom)
	}
	if controls[1].Width > 0 {
		avatarEditorPaintZoom(c, theme, o.IDs.Zoom, controls[1], state.Zoom, minZoom, maxZoom, ready, labels.Zoom)
	}
	if controls[2].Width > 0 {
		avatarEditorZoomButton(c, theme, controls[2], o.IDs.ZoomIn, avatarEditorZoomInIcon, labels.ZoomIn, !ready || state.Zoom >= maxZoom)
	}
	if controls[3].Width > 0 {
		reset := BeginButton(c, theme, o.IDs.Reset, controls[3]).Small()
		if !ready {
			reset = reset.Disabled()
		}
		reset.Outline(labels.Reset)
	}
	return out
}

const avatarEditorRowGap = 12

type avatarEditorControlRow struct {
	widths             [4]float32
	width, gap, height float32
}

func avatarEditorControls(c *gfx.Canvas, theme gfx.Theme, bounds gfx.Rect, ids AvatarEditorIDs, resetLabel string) avatarEditorControlRow {
	height := max(float32(32), theme.Metrics.ControlHeight)
	if bounds.Height <= height+avatarEditorRowGap {
		return avatarEditorControlRow{}
	}
	row := avatarEditorControlRow{height: height, gap: max(float32(10), theme.Metrics.GapItem)}
	if !ids.ZoomOut.IsZero() {
		row.widths[0] = height
	}
	if !ids.Zoom.IsZero() {
		row.widths[1] = 24
	}
	if !ids.ZoomIn.IsZero() {
		row.widths[2] = height
	}
	if !ids.Reset.IsZero() {
		row.widths[3] = max(float32(56), c.TextWidthWeight(resetLabel, theme.Metrics.FontSizeS, gfx.WeightMedium)+24)
	}
	row.width = avatarEditorRowWidth(row.widths, row.gap)
	if row.width > bounds.Width {
		row.widths[3] = 0 // Give the zoom controls priority over reset.
		row.width = avatarEditorRowWidth(row.widths, row.gap)
	}
	if row.width > bounds.Width && row.widths[1] > 0 {
		row.widths[0], row.widths[2] = 0, 0
		row.width = row.widths[1]
	}
	if row.width <= 0 || row.width > bounds.Width {
		return avatarEditorControlRow{}
	}
	if row.widths[1] > 0 {
		extra := max(0, bounds.Width-24-row.width)
		row.widths[1] += extra
		row.width += extra
	}
	return row
}

func avatarEditorRowWidth(widths [4]float32, gap float32) float32 {
	var width float32
	for _, w := range widths {
		if w > 0 {
			if width > 0 {
				width += gap
			}
			width += w
		}
	}
	return width
}

func (row avatarEditorControlRow) at(x, y float32) (controls [4]gfx.Rect) {
	for i, width := range row.widths {
		if width > 0 {
			controls[i] = gfx.Rect{X: x, Y: y, Width: width, Height: row.height}
			x += width + row.gap
		}
	}
	return controls
}

func avatarEditorPaintStage(c *gfx.Canvas, theme gfx.Theme, stage gfx.Rect, out AvatarEditorLayout, o AvatarEditorOptions, labels AvatarEditorLabels, circular bool) {
	c.Clip(stage)
	if circular {
		c.DrawPath(avatarEditorCircleFillPath(), stage, gfx.PathStyle{Fill: theme.Muted})
	} else {
		c.Fill(stage, theme.Muted)
	}
	if out.Ready {
		// Sample the source directly in one draw, including when magnified.
		c.DrawBitmapRegion(o.Bitmap, out.Crop, stage)
		if circular {
			backdrop := o.Backdrop
			if backdrop.Alpha == 0 {
				backdrop = theme.Background
			}
			c.DrawPath(avatarEditorCircleMaskPath(), stage, gfx.PathStyle{Fill: backdrop, FillRule: gfx.PathFillEvenOdd})
		} else {
			avatarEditorPaintGuides(c, theme, stage)
		}
	} else {
		message, color := labels.Unavailable, theme.Danger
		if o.Error != "" {
			message = o.Error
		} else if o.Loading {
			message, color = labels.Loading, theme.MutedForeground
		}
		c.LabelCenter(message, stage, gfx.TextStyle{Size: theme.Metrics.FontSizeM, Color: color})
	}
	id := o.IDs.Canvas
	border := theme.Border
	if !id.IsZero() && (c.Pressed() == id || c.FocusVisible() == id || out.State.gesture == avatarEditorGestureCanvas) {
		border = theme.Ring
	}
	if circular {
		// A stroke is centred on its path, so the path sits half a width inside
		// the stage to keep the whole ring visible, as Avatar does.
		half := avatarEditorBorderWidth / 2
		ring := gfx.Rect{X: stage.X + half, Y: stage.Y + half, Width: stage.Width - 2*half, Height: stage.Height - 2*half}
		c.DrawPath(avatarEditorCircleFillPath(), ring, gfx.PathStyle{Stroke: border, StrokeWidth: avatarEditorBorderWidth})
	} else {
		c.RoundedBorder(stage, 0, avatarEditorBorderWidth, border, gfx.Color{})
	}
	c.Unclip()
	if !id.IsZero() {
		cursor := gfx.PointerCursorHand
		if out.State.gesture == avatarEditorGestureCanvas {
			cursor = gfx.PointerCursorGrabbing
		}
		c.HitWithOptions(id, stage, labels.Canvas, gfx.HitOptions{
			Role: gfx.RoleGroup, Cursor: cursor, Hint: labels.Hint, Disabled: !out.Ready,
		})
	}
}

func avatarEditorZoomButton(c *gfx.Canvas, theme gfx.Theme, bounds gfx.Rect, id gfx.ID, icon gfx.Icon, label string, disabled bool) {
	button := BeginButton(c, theme, id, bounds).IconSmall().WithIcon(icon)
	if disabled {
		button = button.Disabled()
	}
	button.Ghost(label)
}

// Pointer handles primary-button drags of the preview or zoom slider.
func (layout AvatarEditorLayout) Pointer(event gfx.PointerEvent, state AvatarEditorState) (AvatarEditorState, gfx.EventResult) {
	if !layout.Ready || event.Button != gfx.PointerButtonPrimary && event.Button != gfx.PointerButtonNone {
		return state, gfx.EventResult{}
	}
	if event.Kind == gfx.PointerExit && !event.Captured && state.gesture != avatarEditorGestureNone {
		state.gesture = avatarEditorGestureNone
		return state, gfx.EventResult{Repaint: true}
	}
	canvas := !layout.ids.Canvas.IsZero() && event.Target == layout.ids.Canvas
	slider := layout.zoomEnabled() && event.Target == layout.ids.Zoom
	if !canvas && !slider {
		return state, gfx.EventResult{}
	}
	switch event.Kind {
	case gfx.PointerDown:
		state = layout.geometry.normalize(state)
		if slider {
			state.gesture = avatarEditorGestureZoom
			state = layout.zoomAtX(state, event.X)
		} else {
			state.gesture = avatarEditorGestureCanvas
			state.dragX, state.dragY = event.X, event.Y
		}
	case gfx.PointerMove:
		if !event.Captured {
			return state, gfx.EventResult{}
		}
		switch {
		case canvas && state.gesture == avatarEditorGestureCanvas:
			state = layout.pan(state, state.dragX-event.X, state.dragY-event.Y)
			state.dragX, state.dragY = event.X, event.Y
		case slider && state.gesture == avatarEditorGestureZoom:
			state = layout.zoomAtX(state, event.X)
		default:
			return state, gfx.EventResult{}
		}
	case gfx.PointerUp:
		if !(canvas && state.gesture == avatarEditorGestureCanvas || slider && state.gesture == avatarEditorGestureZoom) {
			return state, gfx.EventResult{}
		}
		state.gesture = avatarEditorGestureNone
	default:
		return state, gfx.EventResult{}
	}
	return state, gfx.EventResult{Handled: true, Repaint: true}
}

// Scroll zooms around the pointer. At a zoom limit, a scroll that would go
// farther is left for the containing page to handle.
func (layout AvatarEditorLayout) Scroll(event gfx.ScrollEvent, state AvatarEditorState) (AvatarEditorState, gfx.EventResult) {
	if !layout.Ready || layout.ids.Canvas.IsZero() || event.Target != layout.ids.Canvas || event.DeltaY == 0 {
		return state, gfx.EventResult{}
	}
	view := layout.geometry.normalize(state)
	delta := min(float32(8), max(float32(-8), event.StepY(1)))
	target := view.Zoom * float32(math.Pow(1.15, -float64(delta)))
	stage := layout.Stage
	focusX := min(1, max(0, (event.X-stage.X)/stage.Width))
	focusY := min(1, max(0, (event.Y-stage.Y)/stage.Height))
	next := layout.geometry.zoom(view, target, focusX, focusY)
	if next.Zoom == view.Zoom {
		return state, gfx.EventResult{}
	}
	return next, gfx.EventResult{Handled: true, Repaint: true}
}

// Key handles unmodified editing keys (and Shift for larger moves). Canvas
// arrows pan, +/- and Page Up/Down zoom, and Home resets the view. The zoom
// slider follows standard slider keys. Enter and app shortcuts pass through.
func (layout AvatarEditorLayout) Key(event gfx.KeyEvent, state AvatarEditorState) (AvatarEditorState, gfx.EventResult) {
	if !layout.Ready || event.Release || event.Modifiers&(gfx.ModifierControl|gfx.ModifierMeta|gfx.ModifierAlt) != 0 ||
		event.Target.IsZero() {
		return state, gfx.EventResult{}
	}
	view := layout.geometry.normalize(state)
	result := gfx.EventResult{Handled: true, Repaint: true}
	if event.Target == layout.ids.Zoom && layout.zoomEnabled() {
		g := layout.geometry
		value, handled := SliderKeyDownLarge(view.Zoom, g.minZoom, g.maxZoom, avatarEditorZoomStep, 1, LTR, event.Key, event.Modifiers)
		if !handled {
			return state, gfx.EventResult{}
		}
		// Home and End select the exact endpoints, including custom bounds that
		// aren't an integer number of steps apart.
		if event.Key == gfx.KeyEnd {
			value = g.maxZoom
		}
		return g.zoom(view, value, .5, .5), result
	}
	if event.Target != layout.ids.Canvas {
		return state, gfx.EventResult{}
	}
	step := float32(8)
	if event.Modifiers&gfx.ModifierShift != 0 {
		step = 24
	}
	switch event.Key {
	case gfx.KeyLeft:
		view = layout.pan(view, -step, 0)
	case gfx.KeyRight:
		view = layout.pan(view, step, 0)
	case gfx.KeyUp:
		view = layout.pan(view, 0, -step)
	case gfx.KeyDown:
		view = layout.pan(view, 0, step)
	case gfx.KeyPageUp:
		view = layout.geometry.zoom(view, view.Zoom*1.15, .5, .5)
	case gfx.KeyPageDown:
		view = layout.geometry.zoom(view, view.Zoom/1.15, .5, .5)
	case gfx.KeyHome:
		view = layout.geometry.normalize(AvatarEditorState{})
	default:
		switch event.Text {
		case "+", "=":
			view = layout.geometry.zoom(view, view.Zoom*1.15, .5, .5)
		case "-":
			view = layout.geometry.zoom(view, view.Zoom/1.15, .5, .5)
		default:
			return state, gfx.EventResult{}
		}
	}
	return view, result
}

func normalizedAvatarEditorLabels(labels AvatarEditorLabels) AvatarEditorLabels {
	if labels.Hint == "" {
		labels.Hint = "Drag to reposition. Scroll, use the slider, or use the zoom buttons."
	}
	if labels.Canvas == "" {
		labels.Canvas = "Image crop"
	}
	if labels.Zoom == "" {
		labels.Zoom = "Zoom"
	}
	if labels.ZoomOut == "" {
		labels.ZoomOut = "Zoom out"
	}
	if labels.ZoomIn == "" {
		labels.ZoomIn = "Zoom in"
	}
	if labels.Reset == "" {
		labels.Reset = "Reset"
	}
	if labels.Loading == "" {
		labels.Loading = "Loading image…"
	}
	if labels.Unavailable == "" {
		labels.Unavailable = "This image could not be displayed."
	}
	return labels
}

func finiteAvatarValue(value float32) bool {
	return value >= -math.MaxFloat32 && value <= math.MaxFloat32
}

func avatarEditorLimits(o AvatarEditorOptions) (aspect, minZoom, maxZoom float32) {
	aspect = o.AspectRatio
	if !finiteAvatarValue(aspect) || aspect <= 0 {
		aspect = 1
	}
	minZoom = o.MinZoom
	if !finiteAvatarValue(minZoom) || minZoom < 1 {
		minZoom = 1
	}
	maxZoom = o.MaxZoom
	if !finiteAvatarValue(maxZoom) || maxZoom < minZoom {
		maxZoom = max(8, minZoom)
	}
	return aspect, minZoom, maxZoom
}

func avatarBitmapReady(bitmap gfx.Bitmap) bool {
	if bitmap.Width <= 0 || bitmap.Height <= 0 || bitmap.Width > len(bitmap.Pix)/4 || bitmap.Stride < bitmap.Width*4 {
		return false
	}
	return (len(bitmap.Pix)-bitmap.Width*4)/bitmap.Stride >= bitmap.Height-1
}

type avatarCropGeometry struct {
	image, base      gfx.Size
	minZoom, maxZoom float32
}

func newAvatarCropGeometry(bitmap gfx.Bitmap, aspect, minZoom, maxZoom float32) avatarCropGeometry {
	image := gfx.Size{Width: float32(bitmap.Width), Height: float32(bitmap.Height)}
	base := image
	if image.Width/image.Height > aspect {
		base.Width = image.Height * aspect
	} else {
		base.Height = image.Width / aspect
	}
	return avatarCropGeometry{image: image, base: base, minZoom: minZoom, maxZoom: maxZoom}
}

func (g avatarCropGeometry) normalize(state AvatarEditorState) AvatarEditorState {
	if !finiteAvatarValue(state.Zoom) || state.Zoom <= 0 {
		state.Zoom, state.CenterX, state.CenterY = 1, .5, .5
	}
	state.Zoom = min(g.maxZoom, max(g.minZoom, state.Zoom))
	if !finiteAvatarValue(state.CenterX) {
		state.CenterX = .5
	}
	if !finiteAvatarValue(state.CenterY) {
		state.CenterY = .5
	}
	width, height := g.base.Width/state.Zoom, g.base.Height/state.Zoom
	halfWidth, halfHeight := width/g.image.Width/2, height/g.image.Height/2
	state.CenterX = min(1-halfWidth, max(halfWidth, state.CenterX))
	state.CenterY = min(1-halfHeight, max(halfHeight, state.CenterY))
	return state
}

func (g avatarCropGeometry) crop(state AvatarEditorState) gfx.Rect {
	width, height := g.base.Width/state.Zoom, g.base.Height/state.Zoom
	return gfx.Rect{
		X:     min(g.image.Width-width, max(0, state.CenterX*g.image.Width-width/2)),
		Y:     min(g.image.Height-height, max(0, state.CenterY*g.image.Height-height/2)),
		Width: width, Height: height,
	}
}

func (g avatarCropGeometry) zoom(state AvatarEditorState, target, focusX, focusY float32) AvatarEditorState {
	if math.IsNaN(float64(target)) {
		return state
	}
	target = min(g.maxZoom, max(g.minZoom, target))
	if state.Zoom == target {
		return state
	}
	before := g.crop(state)
	state.Zoom = target
	state.CenterX = (before.X + focusX*before.Width + (.5-focusX)*g.base.Width/target) / g.image.Width
	state.CenterY = (before.Y + focusY*before.Height + (.5-focusY)*g.base.Height/target) / g.image.Height
	return g.normalize(state)
}

func (layout AvatarEditorLayout) pan(state AvatarEditorState, dx, dy float32) AvatarEditorState {
	g := layout.geometry
	state = g.normalize(state)
	// Use the latest draft zoom even if the caller has not painted again yet.
	state.CenterX += dx * (g.base.Width / state.Zoom / g.image.Width) / layout.Stage.Width
	state.CenterY += dy * (g.base.Height / state.Zoom / g.image.Height) / layout.Stage.Height
	return g.normalize(state)
}

func (layout AvatarEditorLayout) zoomEnabled() bool {
	return layout.zoomBounds.Width > 0 && layout.geometry.maxZoom > layout.geometry.minZoom
}

const (
	avatarEditorZoomThumb = 14
	avatarEditorZoomStep  = .1
)

func (layout AvatarEditorLayout) zoomAtX(state AvatarEditorState, x float32) AvatarEditorState {
	track := layout.zoomBounds
	fraction := min(1, max(0, (x-track.X-avatarEditorZoomThumb/2)/(track.Width-avatarEditorZoomThumb)))
	g := layout.geometry
	return g.zoom(g.normalize(state), g.minZoom+fraction*(g.maxZoom-g.minZoom), .5, .5)
}

func avatarEditorPaintGuides(c *gfx.Canvas, theme gfx.Theme, stage gfx.Rect) {
	color := theme.Foreground
	color.Alpha = .22
	for _, fraction := range [...]float32{1.0 / 3, 2.0 / 3} {
		c.Fill(gfx.Rect{X: stage.X + stage.Width*fraction, Y: stage.Y, Width: 1, Height: stage.Height}, color)
		c.Fill(gfx.Rect{X: stage.X, Y: stage.Y + stage.Height*fraction, Width: stage.Width, Height: 1}, color)
	}
}

func avatarEditorPaintZoom(c *gfx.Canvas, theme gfx.Theme, id gfx.ID, bounds gfx.Rect, zoom, minZoom, maxZoom float32, ready bool, label string) {
	centerY := bounds.Y + bounds.Height/2
	track := gfx.Rect{X: bounds.X + avatarEditorZoomThumb/2, Y: centerY - 2, Width: bounds.Width - avatarEditorZoomThumb, Height: 4}
	c.Rounded(track, 2, theme.Muted)
	fraction := float32(0)
	if ready && maxZoom > minZoom {
		fraction = min(1, max(0, (zoom-minZoom)/(maxZoom-minZoom)))
		c.Rounded(gfx.Rect{X: track.X, Y: track.Y, Width: track.Width * fraction, Height: track.Height}, 2, theme.Primary)
	}
	if ready {
		thumbX := track.X + track.Width*fraction
		thumb := gfx.Rect{X: thumbX - avatarEditorZoomThumb/2, Y: centerY - avatarEditorZoomThumb/2, Width: avatarEditorZoomThumb, Height: avatarEditorZoomThumb}
		if c.FocusVisible() == id {
			c.Rounded(expand(thumb, 3), avatarEditorZoomThumb/2+3, theme.Ring)
		}
		c.Rounded(thumb, avatarEditorZoomThumb/2, theme.Background)
		c.RoundedBorder(thumb, avatarEditorZoomThumb/2, 1.5, theme.Primary, gfx.Color{})
	} else {
		zoom = minZoom
	}
	c.HitWithOptions(id, bounds, label, gfx.HitOptions{
		Role:     gfx.RoleSlider,
		Range:    gfx.ValueRange{Min: float64(minZoom), Max: float64(maxZoom), Now: float64(zoom), Step: avatarEditorZoomStep, Set: true},
		Disabled: !ready || maxZoom == minZoom, Cursor: gfx.PointerCursorHand,
	})
}

// avatarEditorBorderWidth is the stage border's thickness in logical pixels. It
// is constant across stage sizes for both the rectangular and circular stage.
const avatarEditorBorderWidth float32 = 1.5

var (
	avatarEditorCirclePathsOnce sync.Once
	avatarEditorCircleMask      gfx.Path
	avatarEditorCircleFill      gfx.Path
)

func avatarEditorCircleMaskPath() gfx.Path {
	avatarEditorCirclePathsOnce.Do(initAvatarEditorCirclePaths)
	return avatarEditorCircleMask
}

func avatarEditorCircleFillPath() gfx.Path {
	avatarEditorCirclePathsOnce.Do(initAvatarEditorCirclePaths)
	return avatarEditorCircleFill
}

func initAvatarEditorCirclePaths() {
	avatarEditorCircleMask = gfx.NewPath(100, 100,
		// Keep the square edge away from the circle's four tangent points.
		// Otherwise even-odd tessellation leaves a seam across the bottom of
		// the stage where the circle contour touches the outer contour.
		gfx.MoveTo(-2, -2), gfx.LineTo(102, -2), gfx.LineTo(102, 102), gfx.LineTo(-2, 102), gfx.ClosePath(),
		gfx.MoveTo(50, 0),
		gfx.CubicTo(77.614, 0, 100, 22.386, 100, 50),
		gfx.CubicTo(100, 77.614, 77.614, 100, 50, 100),
		gfx.CubicTo(22.386, 100, 0, 77.614, 0, 50),
		gfx.CubicTo(0, 22.386, 22.386, 0, 50, 0),
		gfx.ClosePath(),
	)
	avatarEditorCircleFill = gfx.NewPath(100, 100,
		gfx.MoveTo(50, 0),
		gfx.CubicTo(77.614, 0, 100, 22.386, 100, 50),
		gfx.CubicTo(100, 77.614, 77.614, 100, 50, 100),
		gfx.CubicTo(22.386, 100, 0, 77.614, 0, 50),
		gfx.CubicTo(0, 22.386, 22.386, 0, 50, 0),
		gfx.ClosePath(),
	)
}

var avatarEditorZoomOutIcon = gfx.Icon{
	ViewBox: 24, Stroke: 1.8, Cap: gfx.CapRound, Join: gfx.JoinRound,
	Commands: []gfx.PathCommand{
		gfx.MoveTo(10.5, 4),
		gfx.CubicTo(14.09, 4, 17, 6.91, 17, 10.5),
		gfx.CubicTo(17, 14.09, 14.09, 17, 10.5, 17),
		gfx.CubicTo(6.91, 17, 4, 14.09, 4, 10.5),
		gfx.CubicTo(4, 6.91, 6.91, 4, 10.5, 4),
		gfx.ClosePath(),
		gfx.MoveTo(15.2, 15.2), gfx.LineTo(20, 20),
		gfx.MoveTo(7.2, 10.5), gfx.LineTo(13.8, 10.5),
	},
}

var avatarEditorZoomInIcon = gfx.Icon{
	ViewBox: 24, Stroke: 1.8, Cap: gfx.CapRound, Join: gfx.JoinRound,
	Commands: []gfx.PathCommand{
		gfx.MoveTo(10.5, 4),
		gfx.CubicTo(14.09, 4, 17, 6.91, 17, 10.5),
		gfx.CubicTo(17, 14.09, 14.09, 17, 10.5, 17),
		gfx.CubicTo(6.91, 17, 4, 14.09, 4, 10.5),
		gfx.CubicTo(4, 6.91, 6.91, 4, 10.5, 4),
		gfx.ClosePath(),
		gfx.MoveTo(15.2, 15.2), gfx.LineTo(20, 20),
		gfx.MoveTo(7.2, 10.5), gfx.LineTo(13.8, 10.5),
		gfx.MoveTo(10.5, 7.2), gfx.LineTo(10.5, 13.8),
	},
}
