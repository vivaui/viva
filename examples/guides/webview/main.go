package main

import (
	"log"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/flex"
)

// A page of the system's web engine sits among Viva's own controls. The application opens it when
// the window is ready and gives it the room of an element with Box.NativeView: the page fills the
// element where the layout puts it. The menu is a popover: it stands over the page and takes the
// pointer where it covers it.

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
	menuOpen bool
	status   string
}

// open opens the page once the window can host one.
func (m *model) open() {
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

func (m *model) page(c flex.C) {
	flex.Column(c).Fill().Pad(8).Gap(8).Children(func() {
		flex.Row(c).Gap(8).Items(flex.Center).Children(func() {
			if flex.Button(c, "Back").Outline().Clicked() && m.view != nil {
				m.view.Back()
			}
			if flex.Button(c, "Forward").Outline().Clicked() && m.view != nil {
				m.view.Forward()
			}
			if flex.Button(c, "Reload").Outline().Clicked() && m.view != nil {
				m.view.Reload()
			}
			menu := flex.Button(c, "Menu").Outline()
			if menu.Clicked() {
				m.menuOpen = !m.menuOpen
			}
			flex.Text(c, m.status).Muted()
			flex.PopoverWith(c, menu, flex.PopoverSpec{Width: 220, Align: flex.Start}, &m.menuOpen, func() {
				if flex.Button(c, "Say hello to the page").Ghost().Clicked() && m.view != nil {
					m.view.PostMessage("hello from Viva")
					m.menuOpen = false
				}
			})
		})
		content := flex.Slot(c).Grow(1)
		if m.view != nil {
			content.NativeView(m.view.ID())
		}
	})
}

func main() {
	m := &model{}
	m.web.OnReady = m.open
	config := app.Config{Title: "Web view", Width: 760, Height: 520, Web: &m.web}
	if err := flex.Run(config, m.page); err != nil {
		log.Fatal(err)
	}
}
