package app

// OnReopen handles an application reopen request, such as opening an already
// running macOS app from Finder or the Dock. It runs on the application thread
// even when windows are visible. The handler chooses which window to show;
// nil restores the default behavior of restoring hidden or minimized windows.
// Register it during startup. Platforms without reopen events ignore it.
func (application *Application) OnReopen(handler func()) {
	if application == nil || application.host == nil {
		return
	}
	if host, ok := application.host.(interface{ SetReopenHandler(func()) }); ok {
		host.SetReopenHandler(handler)
	}
}
