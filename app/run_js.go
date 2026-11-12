//go:build js && wasm

package app

// Run starts the application in the browser page and blocks until the page
// unloads. The page is the window: RunCanvas supplies the window's frame and
// the native window options in Config are ignored. Use RunCanvas when the
// application already owns a retained canvas.
func Run(config Config) error {
	return RunCanvas(config, nil)
}
