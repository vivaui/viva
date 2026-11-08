//go:build !(js && wasm)

package app

// RequestFullscreen has no process-wide native counterpart: a native
// application asks one Window with Window.SetFullscreen instead.
func RequestFullscreen(bool) error {
	return ErrUnsupported
}
