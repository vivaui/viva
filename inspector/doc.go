// Package inspector paints a development overlay that names what the pointer is
// over: the region's identity, role, accessible name, bounds and state, the
// outline of every region the frame declares, or neither.
//
// It is for the person building an application, to see in a real window what a
// headless Tree lists: which hit a click reaches, which region a label belongs
// to, where a control that "does nothing" actually lies. The application owns an
// Inspector, cycles it from a key of its choosing, and calls Paint last in its
// paint function:
//
//	func paint(canvas *ui.Canvas, size ui.Size) {
//		drawPage(canvas, size)
//		inspect.Paint(canvas, ui.Rect{Width: size.Width, Height: size.Height})
//	}
//
// The overlay declares no hit of its own, so it never takes a press or the
// hover from what lies beneath, and it asks for no frame: it is repainted when
// the application is, and the window repaints when the hovered region changes.
// While the Inspector is off, Paint does nothing and allocates nothing.
//
// The package is not linked into an application that does not import it, and
// a release build leaves the import out with a build tag.
package inspector
