package main

import (
	"fmt"
	"log"
	"runtime"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// Identities are names in a tree, not numbers: the application declares the
// tree once and every control it paints is a name inside a feature of it.
var (
	tree = gfx.Tree{}
	page = tree.Feature("mail")
	rows = tree.Sub(page, tree.Name("rows"))

	divider = tree.ID(page, "divider")
	list    = tree.ID(page, "list")
)

var mailboxes = []string{"Inbox", "Drafts", "Sent", "Archive", "Trash"}

type model struct {
	split    ui.SplitterState
	selected int
	palette  bool
	context  string
	view     ui.View
}

// primary is the modifier that means "the command key" on this platform.
func primary() gfx.Modifiers {
	if runtime.GOOS == "darwin" {
		return gfx.ModifierMeta
	}
	return gfx.ModifierControl
}

func newConfig(m *model) app.Config {
	theme := gfx.LightTheme()
	config := app.Config{Tree: &tree, Title: "Input", Width: 640, Height: 360}

	config.Paint = func(canvas *gfx.Canvas, size gfx.Size) {
		bounds := gfx.Rect{Width: size.Width, Height: size.Height}
		canvas.Fill(bounds, theme.Background)

		m.view.Draw(canvas, theme, bounds, func(f *ui.Frame) {
			f.Row().Fill().Children(func() {
				f.ScrollArea(list).Width(m.split.Pos).Background(theme.Card).Children(func() {
					for i, name := range mailboxes {
						row := f.Row().Pad(8).PadX(12).Hover(ui.Look{}.Background(theme.Hover)).Hit(rows.At(i), name)
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
	config.OnPointer = func(event gfx.PointerEvent) gfx.EventResult {
		if event.Kind == gfx.PointerDown && event.Button == gfx.PointerButtonSecondary && event.Target.Scope() == rows {
			row, _ := event.Target.Name().Index()
			m.context = "Context menu for " + mailboxes[row]
			return gfx.EventResult{Handled: true, Repaint: true}
		}
		return gfx.EventResult{}
	}

	// OnKey is called for the control that has keyboard focus: a row of the list,
	// or the divider, which moves with the arrow keys.
	config.OnKey = func(event gfx.KeyEvent) gfx.EventResult {
		if result := m.split.Key(event); result.Handled {
			return result
		}
		if event.Target.Scope() != rows {
			return gfx.EventResult{}
		}
		switch event.Key {
		case gfx.KeyDown:
			m.selected = min(m.selected+1, len(mailboxes)-1)
		case gfx.KeyUp:
			m.selected = max(m.selected-1, 0)
		default:
			return gfx.EventResult{}
		}
		return gfx.EventResult{Handled: true, Repaint: true, Focus: rows.At(m.selected), FocusVisible: true}
	}

	config.OnShortcut = func(event gfx.KeyEvent) gfx.EventResult {
		if event.Modifiers == primary() && event.Text == "k" {
			m.palette = !m.palette
			return gfx.EventResult{Handled: true, Repaint: true}
		}
		return gfx.EventResult{}
	}

	// The wheel goes to the scroll area under the pointer.
	config.OnScroll = m.view.Wheel
	return config
}

func main() {
	m := &model{split: ui.SplitterState{Pos: 220}}
	var canvas gfx.Canvas
	if err := app.RunCanvas(newConfig(m), &canvas); err != nil {
		log.Fatal(err)
	}
}
