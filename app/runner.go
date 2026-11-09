package app

import "github.com/vivaui/viva/gfx"

// runner, when set, replaces the native window loop of RunCanvas.
var runner func(Config, *gfx.Canvas) error

// UseRunner makes RunCanvas call run instead of opening a window. It is how
// development tools such as package headless drive an unmodified application:
// the application's main function still builds its Config and calls RunCanvas,
// and the tool decides what that means. Nothing calls it unless such a tool is
// linked in, so release builds keep the window loop only.
func UseRunner(run func(Config, *gfx.Canvas) error) {
	if run != nil {
		markNonNativeSystemUsed()
	}
	runner = run
}
