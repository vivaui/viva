package gfx

// A page often hosts views that were written to own a whole canvas: a session
// page painted in every pane of a workspace, a component demo that paints from
// (0, 0), a plugin pane, a preview. Embed places such a view inside a page
// without the view knowing where it sits or what the page has already named.
//
// The view paints into a canvas of its own, prepared with BeginEmbedded so the
// hover, press and focus the page tracks reach it as the view's own identities.
// Embed then appends what it painted to the page, translated to its position,
// with every identity mounted at a scope of the page's identity tree
// (Tree.Mount): the same control in two panes is two identities, each inside its
// pane. Tree.Local undoes the mounting to route an event to the view.
//
//	view.BeginEmbedded(page, mount)
//	demo.Paint(view, size)
//	view.EndFrame()
//	page.Embed(view, gfx.Point{X: 40, Y: 120}, mount)
//
// Embedding copies primitives and allocates nothing once the page canvas has
// grown to its working size.

// BeginEmbedded prepares a canvas that is reused every frame to paint a view
// that parent will embed at mount. It discards the previous frame, hands the
// view the hover, press, focus and click of parent as its own identities, shares
// parent's identity tree, text measurer, display scale, clock and reduced-motion setting, and
// opens an animated paint pass (end it with EndFrame before Embed).
func (canvas *Canvas) BeginEmbedded(parent *Canvas, mount Scope) {
	canvas.Reset()
	canvas.tree = parent.Tree()
	canvas.iconCache = parent.icons()
	tree := canvas.tree
	canvas.hovered, _ = tree.Local(mount, parent.hovered)
	canvas.pressed, _ = tree.Local(mount, parent.pressed)
	canvas.focused, _ = tree.Local(mount, parent.focused)
	canvas.focusVisible = parent.focusVisible && !canvas.focused.IsZero()
	canvas.clicked, _ = tree.Local(mount, parent.clicked)
	canvas.clickInfo = parent.clickInfo
	canvas.eventPass = parent.eventPass
	if parent.hasPointer {
		canvas.pointer, canvas.hasPointer = parent.pointer, true
		canvas.pointer.Target, _ = tree.Local(mount, parent.pointer.Target)
	}
	canvas.measurer = parent.measurer
	canvas.engine = parent.engine
	canvas.pixelScale = parent.pixelScale
	canvas.motion.clock = parent.motion.clock
	canvas.motion.reduce = parent.motion.reduce
	canvas.BeginFrame()
}

// Embed appends everything child painted: translated by origin, with hit IDs
// and focus scopes mounted at mount, on top of canvas's current layer and
// inside canvas's current clip. Hit regions are clipped like any other
// declaration, so a view scrolled partly out of sight keeps only the part that
// can be reached.
//
// A view's modal and focus state does not carry over: its focus trap, Escape
// target, initial focus, and the keyboard focus it asks for or gives up stay on
// the child, where the host can read them (TrappedFocusScope, EscapeTarget,
// InitialFocus, FocusRequest, FocusRelease) and adopt them with AdoptEmbedded,
// mounted with Tree.MountID, if the view should hold the keyboard for the whole
// host. Adopting by default would let one open dialog among many previews lock the
// rest of the page. A child still animating asks canvas for another frame.
func (canvas *Canvas) Embed(child *Canvas, origin Point, mount Scope) {
	if child == nil || child == canvas {
		return
	}
	canvas.listensForClicks = canvas.listensForClicks || child.listensForClicks
	canvas.listensForPointer = canvas.listensForPointer || child.listensForPointer
	canvas.consumed = canvas.consumed || child.consumed
	layer := canvas.currentLayer()
	outer, hasOuter := canvas.clipSnapshot()
	rectsBefore := uint32(len(canvas.rectangles))
	trianglesBefore := uint32(len(canvas.triangles))
	runsBefore := int32(len(canvas.runs))

	for _, rectangle := range child.rectangles {
		rectangle.Bounds = translateRect(rectangle.Bounds, origin)
		rectangle.Layer += layer
		rectangle.ClipOf, rectangle.HasClip = embedClip(rectangle.ClipOf, rectangle.HasClip, origin, outer, hasOuter)
		canvas.rectangles = append(canvas.rectangles, rectangle)
	}
	for _, triangle := range child.triangles {
		for index := range triangle.Points {
			triangle.Points[index].X += origin.X
			triangle.Points[index].Y += origin.Y
		}
		triangle.Layer += layer
		triangle.RectsBefore += rectsBefore
		triangle.ClipOf, triangle.HasClip = embedClip(triangle.ClipOf, triangle.HasClip, origin, outer, hasOuter)
		canvas.triangles = append(canvas.triangles, triangle)
	}
	for _, shadow := range child.shadows {
		shadow.Bounds = translateRect(shadow.Bounds, origin)
		shadow.Layer += layer
		shadow.RectsBefore += rectsBefore
		shadow.TrianglesBefore += trianglesBefore
		shadow.ClipOf, shadow.HasClip = embedClip(shadow.ClipOf, shadow.HasClip, origin, outer, hasOuter)
		canvas.shadows = append(canvas.shadows, shadow)
	}
	// An image laid out in device pixels stays on them when it moves: the page
	// puts the view at a logical position that need not be one.
	deviceOrigin := origin
	if canvas.pixelScale > 0 {
		deviceOrigin = Point{X: canvas.roundToDevice(origin.X), Y: canvas.roundToDevice(origin.Y)}
	}
	for _, image := range child.images {
		if image.DevicePixels {
			image.Bounds = translateRect(image.Bounds, deviceOrigin)
		} else {
			image.Bounds = translateRect(image.Bounds, origin)
		}
		image.Layer += layer
		image.ClipOf, image.HasClip = embedClip(image.ClipOf, image.HasClip, origin, outer, hasOuter)
		canvas.images = append(canvas.images, image)
	}
	for _, view := range child.nativeViews {
		view.Bounds = translateRect(view.Bounds, origin)
		view.Layer += layer
		view.ClipOf, view.HasClip = embedClip(view.ClipOf, view.HasClip, origin, outer, hasOuter)
		canvas.nativeViews = append(canvas.nativeViews, view)
	}
	for _, text := range child.texts {
		text.Bounds = translateRect(text.Bounds, origin)
		text.Layer += layer
		if text.RunCount > 0 {
			text.RunFirst += runsBefore
		}
		text.ClipOf, text.HasClip = embedClip(text.ClipOf, text.HasClip, origin, outer, hasOuter)
		canvas.texts = append(canvas.texts, text)
	}
	canvas.runs = append(canvas.runs, child.runs...)

	scope := canvas.currentFocusScope()
	tree := canvas.Tree()
	for _, hit := range child.hits {
		hit.Bounds = translateRect(hit.Bounds, origin)
		hit.Layer += layer
		if hasOuter {
			hit.Bounds = intersectCanvasRect(hit.Bounds, outer)
		}
		if hit.Bounds.Width <= 0 || hit.Bounds.Height <= 0 {
			continue
		}
		hit.ID = tree.MountID(mount, hit.ID)
		if hit.Scope.IsZero() {
			hit.Scope = scope
		} else {
			hit.Scope = tree.MountID(mount, hit.Scope)
		}
		if hit.TextBounds != (Rect{}) {
			hit.TextBounds = translateRect(hit.TextBounds, origin)
		}
		canvas.hits = append(canvas.hits, hit)
	}

	if child.motion.animating {
		canvas.motion.animating = true
	}
	if wake := child.motion.wake; wake > 0 && (canvas.motion.wake == 0 || wake < canvas.motion.wake) {
		canvas.motion.wake = wake
	}
}

// AdoptEmbedded makes the modal and focus state of an embedded view the host's:
// its focus trap, Escape target, initial focus, and the focus it asks for or gives
// up, mounted at mount. Call it after Embed for a view that should hold the
// keyboard for the whole host, such as the active pane of a split; a view that must
// not take focus leaves its state on its own canvas. A value the child did not set
// changes nothing on the host, so adopting a quiet view is a no-op.
func (canvas *Canvas) AdoptEmbedded(child *Canvas, mount Scope) {
	if child == nil || child == canvas {
		return
	}
	tree := canvas.Tree()
	if scope := child.TrappedFocusScope(); !scope.IsZero() {
		canvas.TrapFocus(tree.MountID(mount, scope))
	}
	if focus := child.InitialFocus(); !focus.IsZero() {
		canvas.SetInitialFocus(tree.MountID(mount, focus))
	}
	if target := child.EscapeTarget(); !target.IsZero() {
		canvas.SetEscapeTarget(tree.MountID(mount, target))
	}
	if request := child.FocusRequest(); !request.IsZero() {
		canvas.RequestFocus(tree.MountID(mount, request))
	}
	if release := child.FocusRelease(); !release.IsZero() {
		canvas.ReleaseFocus(tree.MountID(mount, release))
	}
}

func translateRect(bounds Rect, origin Point) Rect {
	bounds.X += origin.X
	bounds.Y += origin.Y
	return bounds
}

// embedClip combines a child primitive's own clip (in child space) with the
// clip active on the embedding canvas. An empty intersection is a clip that
// shows nothing, as everywhere else.
func embedClip(clip Rect, hasClip bool, origin Point, outer Rect, hasOuter bool) (Rect, bool) {
	if !hasClip {
		return outer, hasOuter
	}
	clip = translateRect(clip, origin)
	if hasOuter {
		clip = intersectCanvasRect(clip, outer)
	}
	return clip, true
}
