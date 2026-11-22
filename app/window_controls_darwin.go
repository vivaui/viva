//go:build darwin && !ios

package app

// controlsDrawnByViva is false on macOS: the system draws the traffic lights.
const controlsDrawnByViva = false

// platformTitlebarInsets is how much of a WindowCustomTitlebar's strip the
// window buttons cover while they show: leading at the start of the strip,
// trailing at its end, in logical pixels. On macOS the traffic lights are the system's, at the leading end.
func platformTitlebarInsets() (leading, trailing float32) { return 70, 0 }
