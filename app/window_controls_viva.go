//go:build windows || linux

package app

// controlsDrawnByViva is true where the system draws no minimize, maximize or
// close buttons for a window without a titlebar, so Viva draws them.
const controlsDrawnByViva = true

// platformTitlebarInsets is how much of a WindowCustomTitlebar's strip the
// window buttons cover while they show: leading at the start of the strip,
// trailing at its end, in logical pixels. Here the buttons are Viva's, at the trailing end.
func platformTitlebarInsets() (leading, trailing float32) { return 0, 3 * WindowControlWidth }
