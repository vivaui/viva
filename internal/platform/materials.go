package platform

// WindowMaterials selects an optional native material beneath window content.
type WindowMaterials struct {
	// WindowBackdrop is read on the window thread after each paint. Windows
	// 11 22H2+ supports all materials; Windows 10 1809 through 11 21H2 supports
	// Acrylic through an undocumented accent policy. Other cases use ClearColor.
	// High contrast and disabled transparency suppress all materials; battery
	// saver suppresses Acrylic. Use an opaque ClearColor for a solid fallback.
	// Nil keeps the ordinary renderer. A non-nil callback opts into composition
	// at window creation, even if it initially returns BackdropNone, allowing
	// later material changes without rebuilding the application.
	WindowBackdrop func() BackdropStyle
	// OnBackdropChange reports API activation initially and when it changes.
	// Windows may still substitute a solid color. Other platforms do not call
	// it. Update paint state without blocking; a change schedules a repaint.
	OnBackdropChange func(active bool)
}
