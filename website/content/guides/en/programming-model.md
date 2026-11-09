---
title: Programming model
summary: Application-owned state, declared hit regions, typed events and how background work reaches the window.
order: 2
group: concepts
---

## State is yours

A Viva application keeps its own state in ordinary Go values. The `Paint` callback reads that state and describes the interface; Viva reconciles each description with a retained scene. If nothing changed, the repaint costs no layout, no scene rebuild and no GPU submission. If one node changed, one small range of instance data is uploaded.

Components follow the same rule. `f.Button`, `f.Dialog` and the rest do not own a value, a checked flag or an open flag. You pass the current value in and handle the click that should change it.

## Hit regions and identities

Anything interactive is a hit region declared while painting, with an identity you name in a `gfx.Tree`. Keep identities stable between frames: focus, hover and press state are tracked by ID, and `Canvas.Focused`, `Canvas.FocusVisible` and `Canvas.Hovered` report them back so painting code reads them rather than copying them.

```go
var (
	tree = gfx.Tree{}
	page = tree.Feature("dashboard")

	hitRefresh = page.ID(tree.Name("refresh"))
	hitPanel   = page.ID(tree.Name("panel"))
)
```

## Events

A button is handled where it is declared: `f.Button(save, "Save").Clicked()` is true in the one paint that delivers its click, the event pass, and only there may a paint change state. The runtime adds that pass only when a paint asks `Clicked`, so an application that does not pays nothing. An application that keeps one central click handler reads `canvas.ClickTarget()` once at the start of its paint and passes the identity on through `canvas.AfterPaint(func() { ... })`, which runs the handler after the paint has returned (outside it, so it may block or open a native dialog) and then repaints; it marks the paint as listening just as `Clicked` does. Prefer `Clicked` at each control. Controls that need more use typed callbacks, each returning a `gfx.EventResult`:

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
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/gfx"
)

// Identities are names in a tree, not numbers: the application declares the
// tree once and every control it paints is a name inside a feature of it.
var (
	tree = gfx.Tree{}
	page = tree.Feature("background")

	refresh = tree.ID(page, "refresh")
)

// slowWork stands in for anything that takes a while: a request, a disk scan.
var slowWork = func() string {
	time.Sleep(time.Second)
	return "done"
}

type model struct {
	mu      sync.Mutex
	pending string // written by the worker goroutine, under mu
	status  string // read by Paint, written only on the window thread
	view    ui.View
}

func newConfig(m *model, invalidator *app.Invalidator) app.Config {
	theme := gfx.LightTheme()
	config := app.Config{Tree: &tree, Title: "Background", Width: 480, Height: 240, Invalidator: invalidator}

	config.Paint = func(canvas *gfx.Canvas, size gfx.Size) {
		bounds := gfx.Rect{Width: size.Width, Height: size.Height}
		canvas.Fill(bounds, theme.Background)

		m.view.Draw(canvas, theme, bounds, func(f *ui.Frame) {
			f.Row().Fill().Pad(24).Gap(12).Items(ui.Start).Children(func() {
				if f.Button(refresh, "Refresh").Clicked() {
					m.status = "working"
					go func() {
						result := slowWork()
						m.mu.Lock()
						m.pending = result
						m.mu.Unlock()
						invalidator.Invalidate() // wakes the window thread once
					}()
				}
				f.Text(m.status)
			})
		})
	}

	// OnWake runs on the window thread right before the repaint Invalidate asked
	// for: move what the goroutine published into state that Paint reads.
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

`Invalidator` owns no goroutine, timer or channel. An application that never calls it pays nothing for it, and `Paint` only ever reads state that `OnWake` already moved onto the window thread.

## Text input

Text belongs to the application too. Give `Config.TextInput` a provider that returns the `text.Editor` for the focused field, and Viva connects it to the platform text system, including selection and input-method composition. Only the focused field is handed to the native editor; when focus leaves, the current text is painted again as a static label in the same frame.

## Windows and lifecycle

`app.RunCanvas` runs one window. `app.RunApplication` runs several: each window owns its own paint, focus, input and close state, and the loop ends when the last one closes. `OnCloseRequest` can hold a window open, for example to confirm unsaved work. Status-area icons, popup panels and hidden windows are covered in [Windows and the status area](/en/docs/windows/).
