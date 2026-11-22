//go:build !windows && !linux && !darwin

package app

// controlsDrawnByViva is false where there is no native window to give controls.
const controlsDrawnByViva = false

// platformTitlebarInsets is how much of a WindowCustomTitlebar's strip the
// window buttons cover while they show: leading at the start of the strip,
// trailing at its end, in logical pixels. There is none here.
func platformTitlebarInsets() (leading, trailing float32) { return 0, 0 }
