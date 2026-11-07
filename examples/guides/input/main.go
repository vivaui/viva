package main

import (
	"fmt"
	"log"
	"runtime"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/flex"
	"github.com/vivaui/viva/ui"
)

// Identities are names in a tree, not numbers: the application declares the
// tree once and every control it paints is a name inside a feature of it.
var (
	tree = ui.Tree{}
	page = tree.Feature("mail")
	rows = tree.Sub(page, tree.Name("rows"))

	divider = tree.ID(page, "divider")
	list    = tree.ID(page, "list")
)

var mailboxes = []string{"Inbox", "Drafts", "Sent", "Archive", "Trash"}

type model struct {
	split    flex.SplitterState
	selected int
	palette  bool
	context  string
	view     flex.View
}

// primary is the modifier that means "the command key" on this platform.
func primary() ui.Modifiers {
	if runtime.GOOS == "darwin" {
		return ui.ModifierMeta
	}
	return ui.ModifierControl
}

func newConfig(m *model) app.Config {
	theme := ui.LightTheme()
	config := app.Config{Tree: &tree, Title: "Input", Width: 640, Height: 360}

	config.Paint = func(canvas *ui.Canvas, size ui.Size) {
		bounds := ui.Rect{Width: size.Width, Height: size.Height}
		canvas.Fill(bounds, theme.Background)

		m.view.Draw(canvas, theme, bounds, func(f *flex.Frame) {
			f.Row().Fill().Children(func() {
				f.ScrollArea(list).Width(m.split.Pos).Background(theme.Card).Children(func() {
					for i, name := range mailboxes {
						row := f.Row().Pad(8).PadX(12).Hover(flex.Look{}.Background(theme.Hover)).Hit(rows.At(i), name)
						if i == m.selected {
							row.Background(theme.Hover)
						}
						if row.Children(func() { f.Text(name) }).Clicked() {
							m.selected = i
						}
					}
				})
				// The splitter is the draggable divider: it captures the pointer, shows
				// the resize cursor and keeps both panes within their minimum sizes.
				f.Splitter(divider, &m.split).Mins(120, 200)
				f.Column().Grow(1).Pad(16).Gap(8).Children(func() {
					detail := fmt.Sprintf("%s selected", mailboxes[m.selected])
					if m.palette {
						detail = "Command palette"
					}
					f.Text(detail)
					f.Text(m.context).Muted()
				})
			})
		})
	}

	// OnPointer sees presses, releases and moves; Target is the identity under
	// the pointer. A drag may be coalesced to its latest position, so a handler
	// must follow the coordinates, not count moves. Here a secondary press on a
	// row is the start of a context menu.
	config.OnPointer = func(event ui.PointerEvent) ui.EventResult {
		if event.Kind == ui.PointerDown && event.Button == ui.PointerButtonSecondary && event.Target.Scope() == rows {
			row, _ := event.Target.Name().Index()
			m.context = "Context menu for " + mailboxes[row]
			return ui.EventResult{Handled: true, Repaint: true}
		}
		return ui.EventResult{}
	}

	// OnKey is called for the control that has keyboard focus: a row of the list,
	// or the divider, which moves with the arrow keys.
	config.OnKey = func(event ui.KeyEvent) ui.EventResult {
		if result := m.split.Key(event); result.Handled {
			return result
		}
		if event.Target.Scope() != rows {
			return ui.EventResult{}
		}
		switch event.Key {
		case ui.KeyDown:
			m.selected = min(m.selected+1, len(mailboxes)-1)
		case ui.KeyUp:
			m.selected = max(m.selected-1, 0)
		default:
			return ui.EventResult{}
		}
		return ui.EventResult{Handled: true, Repaint: true, Focus: rows.At(m.selected), FocusVisible: true}
	}

	config.OnShortcut = func(event ui.KeyEvent) ui.EventResult {
		if event.Modifiers == primary() && event.Text == "k" {
			m.palette = !m.palette
			return ui.EventResult{Handled: true, Repaint: true}
		}
		return ui.EventResult{}
	}

	// The wheel goes to the scroll area under the pointer.
	config.OnScroll = m.view.Wheel
	return config
}

func main() {
	m := &model{split: flex.SplitterState{Pos: 220}}
	var canvas ui.Canvas
	if err := app.RunCanvas(newConfig(m), &canvas); err != nil {
		log.Fatal(err)
	}
}
