---
title: Handling input
summary: Pointer, keyboard, shortcuts, scrolling and focus, with a complete example you can drag, type into and scroll.
order: 4
group: guides
---

Input in a page is declared where it acts. A click is read where the control is declared: `ui.Button(c, "Save").Clicked()` is true in the paint that delivers it. `ui.List` chooses the row clicked and moves the choice with the arrow keys by itself; `ui.Splitter` captures the drag and keeps the panes in their limits by itself; a secondary press is `ui.ContextMenu` on the row. The page routes the window's pointer, keys, wheel and text input to the controls it declared, and `app.Config` keeps typed callbacks for input the page does not model, such as a custom-painted control or a host that routes another page's events. This guide covers both layers. The example below is a panel with a draggable divider, a list you click and move through with the arrow keys, a context menu, a keyboard shortcut and wheel scrolling.

<!-- example: input/main.go -->
```go
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
	// ListState.Label is the row's name for assistive technology, for the text
	// type-ahead matches, and for a test looking a row up by name.
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
		// The page routes pointer, wheel, keys and text input to the controls it
		// declares; the focused splitter gets its arrow keys from the page.
	})
}

func main() {
	var canvas gfx.Canvas
	if err := app.RunCanvas(newConfig(newModel()), &canvas); err != nil {
		log.Fatal(err)
	}
}
```

## Identities

A page does not name its elements by hand. It names each by its place among the elements of its kind in its parent, and a row by `c.Key`, `c.KeyInt` or, in a `ui.List`, `ListState.Key`, so focus, hover and press follow a row as rows are inserted above it. The names are also accessible names: `ListState.Label` names a row for assistive technology, for the type-ahead that moves to it, and for a test that finds it, and `Box.Label` names a container such as the list. A test finds a control by the same name a user hears, with no coordinates or tree identifiers of its own.

A control of your own takes `ui.ID(c)`. A component written against a `ui.Frame` declares its hits in a `gfx.Tree` itself, and a page drawn into a canvas adopts the canvas's tree; an application page writes neither.

## Pointer

A click is handled where the control is declared: `ui.Button(c, "Save").Clicked()`, or `ListRow.Selected()` as the list shows the choice the click made. A drag between two panes needs no handler either: `ui.Splitter(c, &m.split)` captures the pointer, shows the resize cursor and keeps both panes within their minimum and maximum sizes (`Mins`, `Maxs`), and the example only gives the first pane `m.split.Pos` wide. The context menu is declared on its target as well: `ui.ContextMenu(c, row.Element, ...)` opens at the pointer on a secondary press, and Shift+F10 opens it from the keyboard.

`OnPointer` is for what the page does not model: a custom-painted control or a host that routes another page's pointer events. It receives down, move, up and exit events in logical window pixels, and `Target` is the identity under the pointer. A press keeps its original target through the move and up events that follow, and `Captured` is true while that is the case, which is what makes a drag of your own work even when the pointer leaves the control. The moves of a drag may be coalesced to the latest queued position, as the platform does natively, so a handler depends on the coordinates rather than on the number of calls: a drag that follows the displacement from its press still ends where the pointer is.

`PointerEvent.Button` tells primary from secondary presses. `ClickCount` numbers presses of one button close together, 1 for a lone press and 2 for the second press of a double click. `HitOptions.Cursor` sets the pointer shape while a hit is hovered or dragged (`ui.Splitter` already uses `PointerCursorResizeHorizontal`). A control of your own that needs several interactive parts declares them through `ui.Handlers`.

## Keyboard and focus

The page routes keys to the control with the focus. A `ui.List` uses them itself: Up and Down move the choice, Home and End go to the first and last row, Page Up and Down by a screen, Enter or a double click is `ListState.Submitted()`, and typing the start of a name given by `ListState.Label` moves to that row. A menu takes the arrows and Escape while it is open, and a text field takes the editing keys, all without a handler.

A `ui.Splitter` declared in a page gets its arrow keys from the page: the focused divider moves with Left and Right by 16 pixels, and Home and End take it to its limits. The behavior is in `SplitterState.Key`, which a splitter declared on a `Frame` gives to the window's key callback:

```go
page.Config(app.Config{OnKey: m.split.Key /* a Frame-level splitter */})
```

`OnKey` receives the key events the page did not take, for the focused target. `Target` says which one, so a handler checks it before acting. `Key` names navigation, editing and function keys (`KeyDown`, `KeyEscape`, `KeyEnter`, `KeyTab`…). For a chord such as Ctrl+C, `Key` is `KeyUnknown` and `Text` holds the character without modifiers. A control of your own takes its keys through `ui.Handlers` instead; `OnKey` is the fallback below the page.

Focus is tracked for you. Where an element is declared, `Box.State().Focused`, `Hovered` and `Pressed` report the pointer and focus state; a custom painter reads `Canvas.Focused` and `Canvas.FocusVisible`. Reading these rather than keeping copies means hover, press and focus can never disagree with what the runtime believes. Tab moves between the page's focusable hits, Enter and Space activate a focused button, and a hit marked `SkipFocus` or `Disabled` is skipped. A mouse click focuses a control without showing a ring; pressing Tab reveals it.

To keep the focus in the model, use `ui.FocusBind` on the control. Setting the field moves the focus there; a focus the user moves writes back into the field; `ui.FocusedValue` reads the focus that is committed, not a request waiting for a hidden element. The request is edge-triggered and waits while the control is not declared:

```go
ui.FocusBind(ui.TextField(c, &m.query, "Search", ""), &m.focus, focusSearch)
// m.focus = focusSearch moves the focus there; FocusedValue reports where it is.
```

The control needs an identity (a control, or a container given `Hit`), and a binding is not declared inside a `Memo`.

## Shortcuts

A page declares its shortcuts where they act: the example's `if ui.Shortcut(c, ui.Primary, "k") { m.palette = !m.palette }`. `ui.Primary` is Command on macOS and Control on Windows and Linux, so no helper is needed. `Box.Shortcut` on a container or control acts only while the focus is in it, the innermost of nested ones winning. A chord without Primary, Ctrl or Alt is left to a field or terminal being typed in, and while a Popover, Modal, Menu, or Select or Combobox list is open the page's own shortcuts wait.

A handler of your own that matches a chord and wants it to act as a click on a control returns `EventResult.Activate` with that control's identity: the paint runs as the click's event pass, as for Enter on a focused button.

`OnShortcut` receives printable chords, chiefly Control- or Command-modified ones, before the native text system sees them. Return `Handled` only for chords your application owns: an unhandled chord continues through normal text editing, so Command-C still copies inside a text field. On Windows, a focused ordinary text field claims the standard editing and clipboard chords (copy, cut, paste, select all, undo, redo, Home/End, word and document navigation) before `OnShortcut`; terminals and `Placed` editors do not. It is the fallback for a host or a custom control; a page's own chords are `ui.Shortcut`.

## Typing into a list

`ui.List` with `ListState.Label` set moves to a row as the user types its first letters. A control of your own that searches as you type declares that it takes typed text: set `gfx.HitOptions{TypeAhead: true}` on the hit that holds focus and return an `app.TextInput` for it from `config.TextInput` (`context.TypeAhead` is true for that hit). While it has focus the keyboard is raw, as in a terminal: printable characters and input-method input, pinyin included, arrive as `app.TextInsert` events in `OnEvent`, and every other key (arrows, Enter, Escape, Backspace, Tab) arrives as a `gfx.KeyEvent` in `OnKey`. The application owns the query: append the inserted text, and return the input method's composition in the `TextState` (`Composing`, `Composition`) so it keeps the keys it needs while composing; move the selection, and show your own search popup. Set `TextInput.Bounds` where the input method should draw its composition. Hits that do not declare it are unaffected, and a window with no `TextInput` provider pays nothing. `Session.Keys("f")` types into such a hit in headless tests.

`OnShortcut` is for chords the application owns, not for typing: it runs before the input method, so a key it takes never composes.

`OnModifierTap` receives a modifier key that was pressed and released on its own: nothing else, whether a key, typed text, a pointer press or a scroll, came between, and the window kept the keyboard. `OnKey` never sees a lone modifier outside a terminal that asked for the whole keyboard, so this is how an application builds a gesture such as pressing Shift twice. Viva reports each tap and the application times the second one, so there is no timer inside the framework. Setting it makes the window report modifier presses and releases to the runtime and changes nothing else: input methods still see a bare Shift, and `OnKey` still gets no modifier events. A terminal that asked for the whole keyboard receives the modifier itself as well. In headless tests `Session.KeyDown` and `Session.KeyUp` on a modifier produce the tap.

## Scrolling

`ui.List` and `ui.Scroll` keep their offsets themselves, so the wheel goes to the scroll area under the pointer and the page needs no offset field and no `OnScroll` handler; the example's wheel is the list's. `ListState.ScrollTo`, `ScrollIntoView`, `ScrollToEnd`, `Visible` and `AtEnd` move and read a list, and a `ui.Scroll` reports `ScrollState()` in the paint for the same. Content the page scrolls is clipped; a custom painter clips its own with `Canvas.Clip`, so it does not paint over its neighbors, and clipped hit regions stop responding.

`OnScroll` receives the events the page does not take: pixel deltas with the window position, modifiers, `Viewport` (the current window size) and the hit under the pointer, so a handler of your own does not depend on a preceding pointer move. Positive deltas reveal content below.

## Repainting

Return `Repaint: true` from a Config callback only when state changed. An event that changed nothing returns the zero `EventResult`, which performs no repaint. A page's click is delivered in a paint that declares the page again, so a painter reading model state after the control that changes it sees the new value; declare what depends on a change after the control that makes it, as the example's detail pane follows the list and the shortcut.
