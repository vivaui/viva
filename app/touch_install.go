//go:build !darwin

package app

// Windows, Linux, the browser and phones report fingers: their windows read them
// (touch.go). macOS has no touch screen, and a touchpad is a precise wheel there.
func init() { installTouch() }
