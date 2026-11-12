package main

import (
	"log"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

var mailboxes = []string{"Inbox", "Drafts", "Sent", "Archive", "Trash"}

type model struct {
	split   ui.SplitterState
	list    ui.ListState
	palette bool
	context string
}

func newModel() *model {
	m := &model{split: ui.SplitterState{Pos: 220}}
	// ListState.Label is the row's name for assistive technology and the text
	// type-ahead matches; Name names it for assistive technology instead, and
	// NoTypeahead turns the typing off.
	m.list.Label = func(i int) string { return mailboxes[i] }
	return m
}

// selected is the mailbox the detail pane speaks of: the list's choice, or the
// first one until the user makes a choice.
func (m *model) selected() int {
	if i := m.list.Selected(); i >= 0 {
		return i
	}
	return 0
}

// page declares the window. Each control writes what it changed into the model,
// and the page reads that state as it declares the next paint.
func (m *model) page(c ui.C) {
	// A shortcut is declared where it acts, as a click is read where it acts.
	if ui.Shortcut(c, ui.Primary, "k") {
		m.palette = !m.palette
	}

	theme := c.Theme()
	detail := mailboxes[m.selected()] + " selected"
	if m.palette {
		detail = "Command palette"
	}
	ui.Row(c).Fill().Children(func() {
		// The first pane is as wide as the splitter's state says; the list itself
		// keeps its selection and scrolling.
		ui.Column(c).Width(m.split.Pos).Background(theme.Card).Children(func() {
			ui.List(c, &m.list, len(mailboxes), m.row).Label("Mailboxes").Grow(1)
		})
		// The splitter is the draggable divider: it captures the pointer, shows
		// the resize cursor and keeps both panes within their minimum sizes.
		ui.Splitter(c, &m.split).Mins(120, 200).Label("Resize panes")
		ui.Column(c).Grow(1).Pad(16).Gap(8).Children(func() {
			ui.Text(c, detail)
			ui.Text(c, m.context).Muted()
		})
	})
}

// row is one mailbox: a row a click chooses and the keys move through, with a
// context menu on a secondary press.
func (m *model) row(r ui.ListRow) {
	theme := r.C.Theme()
	r.Element.Pad(8).PadX(12).Hover(ui.Look{}.Background(theme.Hover))
	if r.Index == m.selected() {
		r.Element.Background(theme.Hover)
	}
	ui.Text(r.C, mailboxes[r.Index])
	ui.ContextMenu(r.C, r.Element, func(menu *ui.MenuItems) {
		if menu.Item("Open").Clicked() {
			m.context = "Opened " + mailboxes[r.Index]
		}
	})
}

func newConfig(m *model) app.Config {
	theme := gfx.LightTheme()
	page := ui.NewPage(ui.PageConfig{View: m.page, Theme: &theme})
	return page.Config(app.Config{
		Title: "Input", Width: 640, Height: 360,
		// The page routes pointer, wheel, shortcut and text input to the controls
		// it declares. A splitter keeps its arrow keys in its own state, so give
		// them to the window's key callback.
		OnKey: m.split.Key,
	})
}

func main() {
	var canvas gfx.Canvas
	if err := app.RunCanvas(newConfig(newModel()), &canvas); err != nil {
		log.Fatal(err)
	}
}
