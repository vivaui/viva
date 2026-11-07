---
title: Handling input
summary: Pointer, keyboard, shortcuts, scrolling and focus, with a complete example you can drag, type into and scroll.
order: 4
group: guides
---

Input arrives as typed callbacks on `app.Config`. Each takes an event and returns a `ui.EventResult`, which says whether the application used the event and whether the window needs to repaint. The example below uses all of them: a panel with a draggable divider, a list you click and move through with the arrow keys, a secondary press, a keyboard shortcut and wheel scrolling. Layout is `flex`, so the example only handles what the page cannot declare itself.

<!-- example: input/main.go -->
```go
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
```

## Identities

A hit region is identified by an `ui.ID`: a place (`ui.Scope`) and a name inside it. The zero ID means "no target". Nothing is numbered by hand. A program declares one `ui.Tree`, names a feature for each page or overlay, and a name for each control; a list is a scope of its own whose rows are `ui.Index(i)`.

```go
var (
	tree    ui.Tree
	mail    = tree.Feature("mail")
	rows    = tree.Sub(mail, tree.Name("rows"))
	hitSend = mail.ID(tree.Name("send"))   // one control
)

canvas.Hit(rows.ID(ui.Index(row)), bounds) // while painting, one ID per row

if id.Scope() == rows {                    // when the click arrives
	row, _ := id.Name().Index()
	open(row)
}
```

Two different places or names can never be confused, because they are interned rather than hashed or numbered, so features written by different people cannot collide and there is no range to run out of. `tree.Within(id.Scope(), mail)` says whether a click happened anywhere on a page, however deep. Give the tree to the window with `Config.Tree` so identities print as paths such as `mail/rows/#3` in logs and in the headless driver. Naming happens once, when a package-level variable is initialized or a list first appears; painting an ID that already exists allocates nothing.

## Pointer

A click is handled where the control is declared: `f.Button(...).Clicked()`, or `Clicked()` on a `Hit` box, as the rows of the list do. A drag between two panes needs no handler either: `f.Splitter` captures the pointer, shows the resize cursor and keeps both panes inside their minimum sizes, and the example only gives the first pane `m.split.Pos` wide.

`OnPointer` is for what is left. It receives down, move, up and exit events in logical window pixels, and `Target` is the identity under the pointer. A press keeps its original target through the move and up events that follow, and `Captured` is true while that is the case, which is what makes a drag of your own work even when the pointer leaves the control. The example uses it for a secondary press on a row, the start of a context menu.

The moves of a drag may be coalesced to the latest queued position, as the platform does natively, so a handler depends on the coordinates rather than on the number of calls: a drag that follows the displacement from its press still ends where the pointer is.

`Button` tells primary from secondary presses. `ClickCount` numbers presses of one button close together, 1 for a lone press and 2 for the second press of a double click. `HitOptions.Cursor` sets the pointer shape while a hit is hovered or dragged (`f.Splitter` already uses `PointerCursorResizeHorizontal`).

## Keyboard and focus

`OnKey` receives key events for the focused hit. `Target` says which one, so a handler checks it before acting: the example checks that it is a row, and hands the divider's arrow keys to `m.split.Key`. `Key` names navigation, editing and function keys (`KeyDown`, `KeyEscape`, `KeyEnter`, `KeyTab`…). For a chord such as Ctrl+C, `Key` is `KeyUnknown` and `Text` holds the character without modifiers.

Any hit declared with `Hit`, `HitWithLabel` or `HitWithOptions` can take focus. Tab moves between them, Enter and Space activate a focused button, and a hit marked `SkipFocus` or `Disabled` is skipped. The window tracks focus for you:

- `Canvas.Focused` returns the hit that has it, and `Canvas.FocusVisible` returns it only when a focus ring should be drawn. A mouse click focuses a control without showing a ring; pressing Tab reveals it.
- `Canvas.Hovered` and `Canvas.Pressed` report the pointer state the same way.

Paint code reads these rather than keeping its own copy, so hover, press and focus can never disagree with what the runtime believes.

To move focus yourself, return `Focus` (and `FocusVisible`) in the `EventResult`, or `Blur` to clear it.

## Shortcuts

`OnShortcut` receives printable chords, chiefly Control- or Command-modified ones, before the native text system sees them. Return `Handled` only for chords your application owns: an unhandled chord continues through normal text editing, so Command-C still copies inside a text field. On Windows, a focused ordinary text field claims the standard editing and clipboard chords (copy, cut, paste, select all, undo, redo, Home/End, word and document navigation) before `OnShortcut`; terminals and `Placed` editors do not. The example compares `Modifiers` against a `primary()` helper because the conventional key is Command on macOS and Control elsewhere.

## Typing into a list

A list, tree or table that searches as you type, or a control with single-letter commands, declares it takes typed text: set `ui.HitOptions{TypeAhead: true}` on the hit that holds focus and return an `app.TextInput` for it from `config.TextInput` (`context.TypeAhead` is true for that hit). While it has focus the keyboard is raw, as in a terminal: printable characters and input-method input, pinyin included, arrive as `app.TextInsert` events in `OnEvent`, and every other key (arrows, Enter, Escape, Backspace, Tab) arrives as a `ui.KeyEvent` in `OnKey`. The application owns the query: append the inserted text, and return the input method's composition in the `TextState` (`Composing`, `Composition`) so it keeps the keys it needs while composing; move the selection, and show your own search popup. Set `TextInput.Bounds` where the input method should draw its composition. Hits that do not declare it are unaffected, and a window with no `TextInput` provider pays nothing. `Session.Keys("f")` types into such a hit in headless tests.

`OnShortcut` is for chords the application owns, not for typing: it runs before the input method, so a key it takes never composes.

`OnModifierTap` receives a modifier key that was pressed and released on its own: nothing else, whether a key, typed text, a pointer press or a scroll, came between, and the window kept the keyboard. `OnKey` never sees a lone modifier outside a terminal that asked for the whole keyboard, so this is how an application builds a gesture such as pressing Shift twice. Viva reports each tap and the application times the second one, so there is no timer inside the framework. Setting it makes the window report modifier presses and releases to the runtime and changes nothing else: input methods still see a bare Shift, and `OnKey` still gets no modifier events. A terminal that asked for the whole keyboard receives the modifier itself as well. In headless tests `Session.KeyDown` and `Session.KeyUp` on a modifier produce the tap.

## Scrolling

A `f.ScrollArea(id)` keeps its offset in the `flex.View`, so scrolling needs no state of yours: `config.OnScroll = m.view.Wheel` gives the wheel to the area under the pointer. `OnScroll` receives pixel deltas with the window position, modifiers, `Viewport` (the current window size) and the hit under the pointer, so a handler of your own does not depend on a preceding pointer move. Positive deltas reveal content below. Content you scroll yourself should be clipped with `Canvas.Clip` so it does not paint over its neighbors; clipped hit regions stop responding too.

## Repainting

Return `Repaint: true` only when state changed. An event that changed nothing returns the zero `EventResult`, which performs no repaint.
