---
title: Text input and forms
summary: How an application-owned text.Editor connects to the platform's text system, with a working form field.
order: 5
group: guides
---

Text is the one place where Viva and the operating system have to share a document: the platform's text service handles selection, the caret, dead keys and input-method composition, while your application needs to know what the text is. Viva splits the work so that the application always owns the text.

- **You own the text.** A `ui.TextField` edits a `string` of your model: what the user types is in it the next paint, and what you assign to it is what the field shows. The page keeps the field's `text.Editor` (selection, caret, input-method composition). When your code also reads or moves the caret or selection (a command line, a find bar), keep a `ui.Field` instead: it is that editor, read with `Text()` and changed with `SetText`, and `ui.TextField` takes it the same way.
- **The platform drives it.** When a text field has focus, Viva asks the window's `Config.TextInput` provider which editor belongs to that field, connects the platform's text service to it, and applies every native event to it. A page (`ui.Run`, `ui.NewPage`) is that provider for the TextFields it declares.
- **Painting shows the result.** `ui.TextField` draws the field from the editor, with focus and caret already filled in.

<!-- example: textfield/main.go -->
```go
package main

import (
	"log"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/ui"
)

type model struct {
	name  string
	saved string
}

func (m *model) page(c ui.C) {
	ui.Column(c).Fill().Pad(24).Gap(12).Items(ui.Start).Children(func() {
		// m.name is the text: the field writes what is typed into it, and shows
		// what the program puts there. Typing, the caret, selection, the clipboard
		// and input methods need nothing more; the page keeps the field's editor.
		ui.TextField(c, &m.name, "Name", "Your name").Width(320)
		if ui.Button(c, "Save").Clicked() {
			m.saved = m.name
		}
		if m.saved != "" {
			ui.Text(c, "Saved "+m.saved).Muted()
		}
	})
}

func main() {
	m := &model{}
	if err := ui.Run(app.Config{Title: "Form", Width: 480, Height: 240}, m.page); err != nil {
		log.Fatal(err)
	}
}
```

## The provider

A page's `Config` answers `TextInput` for every TextField the last paint declared, by the identity it gave the field, and passes any other focused field (a terminal of yours, say) to the provider the `app.Config` already had. `TextInput` is called with an `app.TextInputContext` describing the focused hit: its ID, bounds and flags such as `Multiline`, `ReadOnly` and `Secure`. Return an `app.TextInput` whose `Editor` is yours, or `nil` to give a field no native editing. When `Editor` is set the runtime mirrors it to the platform and applies each native event to it, so there is no event handling to write. Leave `Editor` nil and set `OnEvent` instead if you need to filter or transform edits yourself.

The text geometry the field declared, its bounds, font size and color, fills in whatever `TextInput` leaves unset. That is what keeps the text from jumping when the field gains focus.

## Painting a field

A field the application already names, and whose editor it keeps, is `ui.TextFieldID(c, id, &m.searchField, label, placeholder)`, with `m.searchField.Bind(&m.search)` called once where the model is made (the `ui.Field` lives in the model, as for any field): the id is the field's identity in the page's Tree, so the window's own `TextInput` routing, an initial focus and a test address it, while the editor is the one the application reads and moves. `.NavigationKeys()` leaves Up, Down, PageUp and PageDown to the application instead of moving the caret, for a search box whose arrows move a list's selection. `.OnKey(fn)` gives the field a handler for the keys pressed while it has the keyboard, before the page routes them anywhere else: returning `Handled` claims one, and returning an empty `EventResult` leaves it to the page, so Enter can still submit. Keys that type text are never offered and always reach the field. `OnKey` implies `NavigationKeys`, since Up and Down have to reach the application for the field to claim them.

`ui.TextField(c, &m.name, label, placeholder)` takes the snapshot from the field, so there is nothing to build; `Changed` reports the first paint after the user edited it, not an assignment or `SetText` of yours; `.Secure()` makes it a password field. Components that paint your own field take an `InputSnapshot` instead, and it is one frame's view of the field: `Value`, `Placeholder`, `Invalid`, `Disabled`, `ReadOnly` and `Secure`, plus `Focused` and `PlatformEditing`. The page's field sets the last one for you. While the platform's editor is on screen it draws the text, the caret and the selection itself, and the static label must not paint underneath it. So `PlatformEditing` is true only while the field is focused and has text, and when focus leaves, the next frame paints the current text again as an ordinary label. The example tests exactly that: after the click on Save, the native editor is gone.

The snapshot's selection and composition offsets are UTF-8 byte offsets into the painted text. The platform's text events use rune indices. They are different things; do not pass one where the other is expected.

## What the editor provides

A `ui.Field` creates its editor on first use; `text.NewEditor(initial)` makes one by hand. `Insert`, `DeleteBackward`, `DeleteForward`, `MoveLeft` and `MoveRight` edit it, `Selection` and `SetSelection` read and set the selection by rune index, and `Composition` reports the marked range during input-method composition. `Restrict(accept, maxRunes)` limits what an editor takes, which is how a one-time code field accepts only digits, six at most. `SetOverwrite` switches to overwrite mode.

The editor also owns a bounded undo history. Command+Z and Command+Shift+Z on macOS, and Control+Z plus Control+Y or Control+Shift+Z on Windows and Linux, reach `Undo` and `Redo` after the application declines the shortcut. Adjacent typing and deletion coalesce; a paste, line break, selection replacement or completed input-method composition is one separate unit. `BreakUndoGroup` adds an application-defined boundary and `ClearHistory` discards retained history. An `Editor`-backed secure field clears history after every native edit, so an old password is not retained for undo. Terminals do not use this history: Control+Z still reaches the program running in the terminal.

An `Editor` is not safe for concurrent use: touch it only on the window thread.

## Password fields

Call `.Secure()` on the `ui.TextField` (or set `Secure` on a `ui.Field`, or `InputSnapshot.Secure`). The component paints one bullet per character, never exposes the value to the accessibility tree, and the runtime masks what the platform text service draws.

## Several lines and addons

`components.PaintInputGroup` takes the same kind of snapshot with `Multiline`, and four addon slots, `InlineStart`, `InlineEnd`, `BlockStart` and `BlockEnd`, for icons, text prefixes and buttons. `components.PaintTextarea` and the other fields build on the same pieces; each has a page in the [component catalog](/en/ui/input/) with runnable examples.

## Multiline text areas

A page writes one with the field it edits, as a single-line one: `ui.TextArea(c, &m.notes, "Notes", "").Height(120)`. Enter starts a new line, long lines wrap, and the rows scroll to follow the caret; the page moves the caret by rows (Up, Down, Home, End) and gives the platform the editor. What follows is what those do, for a field built from the component itself.

`components.PaintTextarea` wraps long lines at the width of the box by default (`NoWrap` turns that off). The platform editor knows one line, so a multiline field sets `HideEditorText` and the component paints the text, selection, composition underline and caret itself. For the caret behavior that wrapping needs, keep a `components.TextareaLayout` in the application next to its editor and pass it to every frame through `TextareaSnapshot.Layout`:

- It keeps the wrapped visual rows and the scroll position, so a steady paint allocates nothing; it scrolls to the caret when the caret moves, and `ScrollBy` answers the wheel.
- Arrows, Home, End and the page keys go from the application's key handler to `components.TextareaMovementForKey` and `layout.Move`, and the byte offset that comes back is given to the editor (`Editor.MoveTo(index, extend)`, with Shift extending the selection). Up and Down keep the column through a run of presses; Home and End go to the ends of the visual row.
- The runtime places the caret by visual row on a press or drag (the layout tells it where the rows are through the hit region), and the input method's candidate window follows the row the caret is on.
- `layout.ContentHeight` gives the height of a box that grows with its text (shadcn's `field-sizing-content`); the application decides where it stops.

Offsets stay UTF-8 byte offsets into the text the application holds. While an input method is composing, the arrow keys are its.

## What is verified where

Native editing is implemented on macOS, with real Chinese input-method composition, grapheme-aware editing and click, drag and select-all handling. Windows has a native editor with Microsoft Pinyin composition and its candidate window. On Linux an input method running under IBus, or Fcitx5 with its IBus frontend, composes in the field, and the engine's own panel draws the candidates. In the browser, the catalog's Input example is wired to the same editor and everything else is visual only. See [Platforms](/en/docs/platforms/).

The wrapping Textarea described above was exercised on macOS, through the headless driver; the matching Windows and Linux paths (no platform caret placement, the input-method window at the painted caret) are written but not yet run on real windows, and the browser still uses its own text box for it.
