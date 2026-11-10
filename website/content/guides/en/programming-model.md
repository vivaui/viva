---
title: Programming model
summary: Application-owned state, declared hit regions, typed events and how background work reaches the window.
order: 2
group: concepts
---

## State is yours

A Viva application keeps its own state in ordinary Go values. The page function `func (m *model) page(c ui.C)` reads that state and declares the interface; Viva reconciles each declaration with a retained scene. If nothing changed, the repaint costs no layout, no scene rebuild and no GPU submission. If one node changed, one small range of instance data is uploaded.

Components follow the same rule. `ui.Button`, `ui.Modal` and the rest do not own a value, a checked flag or an open flag. You pass the current value in and handle the click that should change it.

## Hit regions and identities

A page names its elements by their place among their siblings, and a row by `c.Key` or `c.KeyInt`, so focus, hover and press follow the row as items come, go and move. A control of your own takes `ui.ID(c)`. Read state where you declare: `Box.State().Focused`, `Hovered` and `Pressed`, and `Clicked()`. A page does not name controls in a `gfx.Tree`; a component written with a `ui.Frame` does.

## Events

A button is handled where it is declared: `ui.Button(c, "Save").Clicked()` is true in the one paint that delivers its click, and only there may a paint change state. For a handler that changes the list being declared, use `c.After(func() { ... })`; it runs after the paint, may block or open a native dialog, and repaints. The page wires the window's pointer, key, wheel and text input itself through `Page.Config`; a `ui.Control`'s `ui.Handlers` takes the events for a control of your own. For input the page does not model, `app.Config` still has typed callbacks, each returning a `gfx.EventResult`:

| Callback | Receives |
| --- | --- |
| `OnPointer` | Down, move, up and exit events with the target and logical coordinates. A press keeps its original target through the matching move and up events. |
| `OnKey` | Keyboard events for the focused target. |
| `OnShortcut` | Control- or Command-modified printable chords, before the native text system sees them. A focused ordinary text field on Windows takes the standard editing chords first. |
| `OnScroll` | Pixel deltas with the window position, modifiers and target. |

A `gfx.EventResult` says what the application did with the event. `Handled` suppresses the built-in behavior for that event, `Repaint` asks for one synchronous paint, and `Focus`, `Blur` and `FocusVisible` move focus. Returning the zero value from an event that changed nothing performs no repaint.

```go
config.OnKey = func(event gfx.KeyEvent) gfx.EventResult {
	if event.Key == gfx.KeyEscape && state.open {
		state.open = false
		return gfx.EventResult{Handled: true, Repaint: true}
	}
	return gfx.EventResult{}
}
```

All callbacks run on the window thread and must not block.

## Background work

Other goroutines never touch the window. A goroutine publishes its result under your own synchronization, then calls `Invalidator.Invalidate`. The window thread wakes once, runs `OnWake`, and repaints.

<!-- example: background/main.go -->
```go
package main

import (
	"log"
	"sync"
	"time"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// slowWork stands in for anything that takes a while: a request, a disk scan.
var slowWork = func() string {
	time.Sleep(time.Second)
	return "done"
}

type model struct {
	mu          sync.Mutex
	pending     string // written by the worker goroutine, under mu
	status      string // read by the page, written only on the window thread
	invalidator *app.Invalidator
}

func (m *model) page(c ui.C) {
	ui.Row(c).Fill().Pad(24).Gap(12).Items(ui.Start).Children(func() {
		if ui.Button(c, "Refresh").Clicked() {
			m.status = "working"
			go func() {
				result := slowWork()
				m.mu.Lock()
				m.pending = result
				m.mu.Unlock()
				m.invalidator.Invalidate() // wakes the window thread once
			}()
		}
		ui.Text(c, m.status)
	})
}

func newConfig(m *model, invalidator *app.Invalidator) app.Config {
	theme := gfx.LightTheme()
	m.invalidator = invalidator
	page := ui.NewPage(ui.PageConfig{View: m.page, Theme: &theme})
	config := page.Config(app.Config{Title: "Background", Width: 480, Height: 240, Invalidator: invalidator})

	// OnWake runs on the window thread right before the repaint Invalidate asked
	// for: move what the goroutine published into state that the page reads.
	config.OnWake = func() {
		m.mu.Lock()
		m.status, m.pending = m.pending, ""
		m.mu.Unlock()
	}
	return config
}

func main() {
	var invalidator app.Invalidator
	var canvas gfx.Canvas
	if err := app.RunCanvas(newConfig(&model{status: "idle"}, &invalidator), &canvas); err != nil {
		log.Fatal(err)
	}
}
```

`Invalidator` owns no goroutine, timer or channel. An application that never calls it pays nothing for it, and the page only ever reads state that `OnWake` already moved onto the window thread.

### Scoped effects

Work that belongs to a page — a directory listing for the current directory, a request for the selected host — is a `ui.Effect` instead of a goroutine of your own. Create its state once with the model and declare it in the page:

```go
var load = ui.NewEffect[dirListing, string](func(ctx context.Context) (dirListing, error) {
	return readDir(ctx) // no C, Box or Frame: this runs off the paint
})

func (m *model) page(c ui.C) {
	ui.Effect(c, m.load, m.dir) // the key: a new directory cancels the old run
	if listing, err, ok := m.load.Take(); ok {
		m.apply(listing, err)
	}
	// ...
}
```

The key names one run. The same key does not start the work again; a different key cancels the run and starts a new one; a paint that no longer declares the effect cancels it; and closing the window cancels all of them. A result that arrives after its key changed, after the effect disappeared or after the window closed is dropped. `Take` returns the result once, on the window thread, after the effect is declared; the page wakes itself when the result lands. One state belongs to one page, and an effect is not declared inside a `Memo`.

The run function receives a context and must return when it is done; the framework does not kill goroutines or poll. A page that only draws into a canvas of your own gives `PageConfig.Invalidator` the invalidator of that window, and calls `Page.Close` when the page goes away; a page given no invalidator keeps the result for the next paint but cannot wake the window by itself.

A goroutine of your own, for something with no page key to cancel it, still publishes under its own lock and calls `Invalidator.Invalidate`.

## Text input

Text belongs to the application too. Give `Config.TextInput` a provider that returns the `text.Editor` for the focused field, and Viva connects it to the platform text system, including selection and input-method composition. Only the focused field is handed to the native editor; when focus leaves, the current text is painted again as a static label in the same frame.

## Windows and lifecycle

`app.RunCanvas` runs one window. `app.RunApplication` runs several: each window owns its own paint, focus, input and close state, and the loop ends when the last one closes. `OnCloseRequest` can hold a window open, for example to confirm unsaved work. Status-area icons, popup panels and hidden windows are covered in [Windows and the status area](/en/docs/windows/).
