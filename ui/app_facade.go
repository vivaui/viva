package ui

import (
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/window"
)

// Configure returns base with the page's Tree, Paint, TextInput, OnScroll,
// OnKey, OnShortcut and OnPointer:
//
//   - Tree: base's is adopted when the page was given none; a different one panics.
//   - Paint paints view over the whole window.
//   - TextInput answers for the page's TextFields and asks base's for any other
//     focused field (a terminal of the application's, say).
//   - OnScroll scrolls an open Select, then the page's scroll areas, and gives
//     what they leave to base's.
//   - OnKey, OnShortcut and OnPointer give a DragState its drag, a Shortcut its
//     chord and a Control its own events, then base's the rest.
//
// Configure gives the page an Invalidator when neither PageConfig.Invalidator
// nor base has one, for the page's Effects and, with FollowSystemTheme, the
// appearance watch. OnClosed cancels the page's effects and stops that watch
// before it runs base's OnClosed. Nothing else is touched.
func Configure(page *Page, base app.Config) app.Config {
	if page.GetConfig().View == nil {
		panic("ui: Configure needs PageConfig.View, the function that declares the page")
	}
	page.Init(base.Tree)
	if base.Tree != nil && base.Tree != page.IdentityTree() {
		panic("ui: Configure: the app.Config has another Tree than the page's; give NewPage the window's Tree")
	}
	config := base
	host := Host{
		Tree: config.Tree, Paint: config.Paint, TextInput: config.TextInput,
		OnScroll: config.OnScroll, OnKey: config.OnKey, OnShortcut: config.OnShortcut, OnPointer: config.OnPointer,
	}
	page.Attach(&host)
	config.Tree, config.Paint, config.TextInput = host.Tree, host.Paint, host.TextInput
	config.OnScroll, config.OnKey, config.OnShortcut, config.OnPointer = host.OnScroll, host.OnKey, host.OnShortcut, host.OnPointer
	concrete, _ := page.GetConfig().Invalidator.(*app.Invalidator)
	if concrete == nil {
		concrete = base.Invalidator
	}
	if concrete == nil {
		concrete = new(app.Invalidator)
	}
	config.Invalidator = concrete
	page.SetInvalidator(concrete)
	if page.GetConfig().FollowSystemTheme && page.GetConfig().Theme == nil {
		stop := app.WatchSystemAppearance(func() {
			page.MarkThemeStale()
			concrete.Invalidate()
		})
		closed := config.OnClosed
		config.OnClosed = func() {
			stop()
			if closed != nil {
				closed()
			}
		}
	}
	closed := config.OnClosed
	config.OnClosed = func() {
		page.Close()
		if closed != nil {
			closed()
		}
	}
	return config
}

// systemAppearance is the platform's appearance, which a window page follows.
func systemAppearance() window.Appearance { return app.SystemAppearance() }

// Run opens a window that declares view, and blocks until it closes: an
// application in one call. The config's Tree, Paint, TextInput and OnScroll are
// the page's (see Configure).
func Run(config app.Config, View func(C)) error {
	page := NewPage(PageConfig{View: View, Appearance: systemAppearance})
	var canvas gfx.Canvas
	return app.RunCanvas(Configure(page, config), &canvas)
}

// OpenWindow opens a window of application that paints page, and returns it. It
// is Run for an application with several windows (app.RunApplication), and
// the one call for what NewPage, Configure and a canvas of your own would be:
// the window owns the canvas, and the page is closed, cancelling its Effects,
// when the window closes or fails to open. Call it where Application.OpenWindow
// may be called: the start callback of app.RunApplication or a window's
// callback. config is the window's, and page.View declares what it shows.
//
//	app.RunApplication(func(a *app.Application) error {
//		_, err := ui.OpenWindow(a, app.Config{Title: "Notes", Width: 720, Height: 480},
//			ui.PageConfig{View: m.GetPage()})
//		return err
//	})
//
// A page opened this way reaches its application with C.Application, so a click
// can open another window without the model keeping the application.
func OpenWindow(application *app.Application, config app.Config, page PageConfig) (*app.Window, error) {
	if page.Appearance == nil {
		page.Appearance = systemAppearance
	}
	p := NewPage(page)
	p.SetApplication(application)
	window, err := application.OpenWindow(Configure(p, config), new(gfx.Canvas))
	if err != nil {
		p.Close()
		return nil, err
	}
	return window, nil
}

// Application is the application whose window the page was opened in by
// OpenWindow, so a handler can open more windows. It is nil for a page run by
// Run or RunCanvas, or drawn into a window of someone else's.
func Application(c C) *app.Application {
	application, _ := c.Application().(*app.Application)
	return application
}
