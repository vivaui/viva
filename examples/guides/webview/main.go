package main

import (
	"log"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/flex"
	"github.com/vivaui/viva/ui"
)

// A page of the system's web engine sits among Viva's own controls. The application opens it when
// the window is ready, reserves its room with a flex Slot, and declares the place with
// Canvas.NativeView. The menu is a popup on a higher layer: it stands over the page and takes the
// pointer where it covers it.
var (
	tree = ui.Tree{}
	home = tree.Feature("browser")

	back        = tree.ID(home, "back")
	forward     = tree.ID(home, "forward")
	reload      = tree.ID(home, "reload")
	menu        = tree.ID(home, "menu")
	menuDismiss = tree.ID(home, "menu-dismiss")
	menuHello   = tree.ID(home, "menu-hello")
)

// The page is served by the application itself, from memory, under its own scheme: no server, no files.
const page = `<!doctype html><meta charset=utf-8>
<body style="font:16px system-ui;margin:24px">
<h2><img src="logo.svg" width=28 height=28 style="vertical-align:middle"> A page in a Viva window</h2>
<p><input placeholder="type here"> <button onclick="viva.postMessage('hello from the page')">Tell Viva</button></p>
<p id=says>Viva has not said anything yet.</p>
<script>viva.onmessage = e => says.textContent = 'Viva says: ' + e.data</script>`

type model struct {
	web      app.WebHost
	view     *app.WebView
	flex     flex.View
	menuOpen bool
	status   string
}

func newConfig(m *model) app.Config {
	theme := ui.LightTheme()
	config := app.Config{Tree: &tree, Title: "Web view", Width: 760, Height: 520, Web: &m.web}

	m.web.OnReady = func() {
		view, err := m.web.Open(app.WebViewConfig{
			URL: "viva-app://page/index.html",
			Scheme: &app.WebScheme{Name: "viva-app", Serve: func(path string) (string, []byte, bool) {
				switch path {
				case "index.html":
					return "text/html; charset=utf-8", []byte(page), true
				case "logo.svg":
					return "image/svg+xml", []byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 28 28"><circle cx="14" cy="14" r="13" fill="#2563eb"/></svg>`), true
				}
				return "", nil, false
			}},
			OnMessage: func(text string) { m.status = "page: " + text }, // a paint follows
		})
		if err != nil {
			m.status = err.Error() // no engine installed, or a platform without pages
			return
		}
		m.view = view
	}

	config.Paint = func(canvas *ui.Canvas, size ui.Size) {
		bounds := ui.Rect{Width: size.Width, Height: size.Height}
		canvas.Fill(bounds, theme.Background)

		var content flex.Box
		m.flex.Draw(canvas, theme, bounds, func(f *flex.Frame) {
			f.PopupLayer(1) // the menu stands over the page, which is on layer 0
			f.Column().Fill().Pad(8).Gap(8).Children(func() {
				f.Row().Gap(8).Items(flex.Center).Children(func() {
					if f.Button(back, "Back").Outline().Clicked() && m.view != nil {
						m.view.Back()
					}
					if f.Button(forward, "Forward").Outline().Clicked() && m.view != nil {
						m.view.Forward()
					}
					if f.Button(reload, "Reload").Outline().Clicked() && m.view != nil {
						m.view.Reload()
					}
					menuButton := f.Button(menu, "Menu").Outline()
					if menuButton.Clicked() {
						m.menuOpen = !m.menuOpen
					}
					f.Text(m.status).Muted()

					if m.menuOpen {
						canvas.SetEscapeTarget(menuDismiss)
						f.Popup().At(0, 0).Fill().Hit(menuDismiss, "Close menu").Children(func() {})
						if canvas.Clicked(menuDismiss) {
							m.menuOpen = false
						}
						f.Popup().BelowBox(menuButton, 4, flex.Start).Width(200).Pad(4).Background(theme.Popover).
							Border(1, theme.Border).Radius(theme.Metrics.RadiusControl).Shadow(ui.ShadowMD).Children(func() {
							if f.Button(menuHello, "Say hello to the page").Ghost().Clicked() && m.view != nil {
								m.view.PostMessage("hello from Viva")
								m.menuOpen = false
							}
						})
					}
				})
				content = f.Slot().Grow(1)
			})
		})
		if m.view != nil {
			canvas.NativeView(m.view.ID(), content.Rect())
		}
	}
	return config
}

func main() {
	var canvas ui.Canvas
	if err := app.RunCanvas(newConfig(&model{}), &canvas); err != nil {
		log.Fatal(err)
	}
}
