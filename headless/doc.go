// Package headless operates a Viva application without opening a window.
//
// A Session runs the application's real paint, input routing, focus and
// text-input paths through app.Driver, with an animation clock that only moves
// when told to, and can turn the current frame into an image. On macOS the
// image comes from the window's own Metal pipelines and CoreText
// rasterizers, drawing into a texture instead of a drawable, so a headless
// screenshot is the pixels the window would show. That renderer is linked only
// into builds made with -tags vivahead, so release builds carry none of it;
// without the tag sessions still operate and answer queries and Image reports
// ErrNoRenderer. Windows (Direct3D 11, GDI text) and Linux (Vulkan, Pango text)
// draw their screenshots the same way, with their own renderers,
// in any build that links this package; only the macOS one also draws the text being
// edited, which the platform's native editor paints over the scene. A session
// lays the application out with the platform's text engine, as a window does
// (Options.ReferenceText asks for the platform-independent reference metrics).
//
//	session, err := headless.Open(config, nil, headless.Options{})
//	session.ClickLabel("New Host")
//	session.Type("example.com")
//	session.Keys("cmd+c")
//	text := session.Driver().Clipboard()
//	session.Screenshot("after.png")
//
// Tree lists the controls of the frame with their role, identity, accessible name and state, and
// Expect checks a condition about the frame (a text, a control's state, the focus, a clean
// layout, every control named), failing with what the frame has instead. A control is named in
// commands by its accessible name, by "id:home/save", or by role and name ("button:Save"), so a
// control with no name of its own can still be operated:
//
//	session.Expect("state", "checkbox:", "unchecked")
//	session.ClickLabel("id:home/auto")
//	session.Expect("text", "Connected", "within", "5000")
//
// The clock belongs to the test: Now returns it, Advance moves it, and Frame
// paints one frame without moving it. Transitioning reports whether the last
// frame asked for the next one, so a test can prove that an animation ended:
//
//	session.Advance(200 * time.Millisecond)
//	session.Expect("text", "Saved")
//	if session.Transitioning() {
//		t.Error("the transition is still running")
//	}
//
// Access returns the last frame's accessibility tree in reading order: what
// assistive technology is told about every control, with its role, bounds,
// name, value, states and range. AccessByRole and AccessByName find one node,
// as a screen reader or a test would:
//
//	node, ok := session.AccessByRole(gfx.RoleCheckbox, "Auto update")
//	if !ok || !node.Checked { ... }
//
// A session has no notification center: Notifications returns the ones the
// application sent, in order, instead of showing them.
//
// Record plays the animation clock and writes the frames whose pixels changed,
// so a transition can be reviewed as a sequence rather than a still:
//
//	frames, err := session.Record("run/frames", headless.RecordOptions{
//		Duration: 700 * time.Millisecond,
//	})
//
// The PNG files and their lengths in manifest.json are what a video tool needs;
// frames that repeat are written once, so an application at rest records as one
// still frame. Recording uses the same offscreen renderers as Image.
//
// A running application can be driven the same way without any change to its
// code: link package headless/auto (behind a build tag, so release builds never
// contain it) and start the process with VIVA_HEADLESS=1. RunCanvas then serves
// the line protocol documented on Serve on standard input and output instead
// of opening a window.
package headless
