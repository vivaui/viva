---
title: Declarative layout
summary: Describe a page with rows, columns, padding and growth instead of coordinates; package ui lays it out.
order: 7
group: guides
---

Slicing rectangles by hand (see Drawing and layout) is the lowest level of Viva, and it is right for a custom control. For a page of ordinary parts (a header, a form, a list row, a footer) it is a lot of arithmetic, and the arithmetic goes wrong when a label gets longer. The `ui` package lays a page out the way CSS flexbox does, from a description.

```go
view.Draw(canvas, theme, bounds, func(f *ui.Frame) {
	f.Column().Pad(24).Gap(12).Children(func() {
		f.Text("Title").FontSize(18).Bold()
		f.Row().Gap(8).Justify(ui.End).Children(func() {
			f.Button(cancel, "Cancel").Outline()
			if f.Button(save, "Save").Clicked() {
				model.save()
			}
		})
	})
})
```

## The model

- `Row` lays children out left to right, `Column` top to bottom, `Stack` over one another.
- A child is as big as its content, or as `Width`, `Height` or `Size` say. `FillWidth`, `FillHeight` and `Fill` take all the room across a container, or share what is left along it.
- `Grow(n)` shares the free room along the container between the children that grow, in proportion. `Spacer()` is an empty child that grows.
- `Pad` and `Margin` take one number, or `PadX`, `PadY`, `PadSides` for more. `Gap` is the space between children.
- Across a container children stretch by default; `Items(ui.Center)` centers them, `Self` does it for one child. Along it, `Justify(ui.Center)`, `ui.End` and `ui.Between` place what is left. `Center()` does both.
- `Basis(px)` is what an element starts from before free room is shared, and `WidthPercent` and `HeightPercent` take a share of the container. `MinWidth`, `MaxWidth`, `MinHeight` and `MaxHeight` bound a size; room a capped element cannot take goes to the others, as in CSS, so `Grow(1).MaxWidth(470)` beside a `Spacer()` keeps what follows against the far edge.
- `Items(ui.Baseline)` lines a Row's children up on their first line of text, so a large heading, a small caption and a button sit on one baseline. A `Text` shrinks and clips before a fixed-size sibling does.
- `Background`, `Radius`, `Border`, `Shadow` and `Clip` style a container. On a control such as an `Input`, `Select` or `Button`, `Radius` and `FontSize` give that control its own corner radius and text size, and a `Height` on a `Button` is how tall the button is painted.
- `Paragraph(text, maxLines)` is text that wraps to the width it gets, so a longer label makes the row taller and pushes what follows down. `LineHeight` sets its leading.
- `Grid(minCell)` fits as many equal columns as the width allows and wraps onto new rows.
- `Scroll(offset)` is a clipped column taller than itself. You own the offset (the wheel arrives in `Config.OnScroll`); after `End`, `Content()` is how tall the children are, so the offset can be clamped.

Controls are the ones in `components`, with the same look: `Button` (with `Outline`, `Ghost`, `Secondary`, `Destructive`, `Small` and the other variants), `Checkbox`, `Switch`, `Icon`, `Divider`. `Clicked()` is read where the control is declared: it is true in the one paint that delivers the click. Any element can be made a pointer target with `Hit(id, label)`; what it looks like under the pointer, pressed and focused is declared with `Hover`, `Pressed` and `Focus` (below).

## Pages: ui.C

The page API is the same layout with the bookkeeping of a page done for you. Every element is a function of `ui` that takes the page's `c` first; containers take their children in a function, and controls take the value they change. Nothing is named by you, and a page imports `ui` alone: colors, geometry and the theme are re-exported there (`ui.RGB`, `ui.Rect`, `ui.Theme`), and package `gfx` underneath holds the canvas, input events and keys:

```go
func (m *model) page(c ui.C) {
	ui.Column(c).Fill().Pad(24).Gap(12).Children(func() {
		ui.TextField(c, &m.name, "Name", "Your name")
		ui.Switch(c, &m.sync, "Sync over SSH")
		ui.Choice(c, &m.theme, themes, nil)
		if ui.Button(c, "Save").Clicked() {
			m.save()
		}
	})
}

ui.Run(app.Config{Title: "Settings", Width: 480, Height: 320}, m.page)
```

A click flips `m.sync` in the paint that delivers it, the first time the control is asked (`Clicked`, `Changed`) or else when the page ends, so a `Disabled` written after the control still holds. `Choice`, `Tabs` and `Select` take a value of any comparable type and write it as they are declared: disable them with an `if`, not a `Disabled` after them. `TextField` needs no `Config.TextInput`: the page's `Config` gives the platform the editor of each field it declared, and passes any other focused field to the provider the configuration already had. `Popover` and `Modal` open while a `*bool` is true and set it to false when dismissed (see [Dialogs, menus and popups](/en/docs/overlays/)). `Box.NativeView` gives a page of the system's web engine the room of an element.

An element is named by where it is: its kind and its number among the elements of that kind in its container. Three things refine that:

- A control bound to a value is named by that value's address, and the name follows the value when it moves: a field shown above a password field does not take the password's focus, and filtering a list keeps each checkbox's focus on its row. A copy (`v := m.items[k]; ui.Checkbox(c, &v, ...)`) works too: it keeps the name of its place, so write the copy back after the call.
- Inside `c.Key(item.ID)` or `c.KeyInt(id)`, elements are numbered within that row, so a row's buttons follow it when rows come, go or move. Keys are reused two paints after their row is gone. A loop whose rows hold a `Button` keys its rows; `viva vet` reports one that does not.
- While the user holds a button (presses it, or it has the focus), a paint in which another button took its place does not deliver the press, and the focus is given up.

`ui.List(c, &state, n, row)` declares only the rows on screen of a list of `n`, each as tall as its content, and scrolls it; `ListState.Key` names rows by an item's ID. `ui.State[T](c)` is an element's own small state (whether its menu is open), kept while its container is declared. `ui.Memo(c, version, fn)` copies the part `fn` declares from the last paint while `version` is unchanged, declaring it again when it is hovered, focused, animated or resized; see Cost below.

A settings struct can have its form written for it. Tag the fields and run `viva form` (from `//go:generate go tool viva form`): it writes `viva_form.go` with `func (v *Settings) Form(c ui.C) ui.Box`, plain page code with no reflection:

```go
type Settings struct {
	Name   ui.Field       `form:"Name" placeholder:"Your name"`
	Sync   bool             `form:"Sync over SSH"`            // a Switch; "Sync,checkbox" for a Checkbox
	Theme  Theme            `form:"Theme" choices:"themes,Theme.String"`
	Size   string           `form:"Size,select" choices:"sizes"`
	Volume ui.SliderState `form:"Volume"`
}
```

`choices` names a package-level slice and, for options that are not strings, the function that names one. A field with no `form` tag gets nothing; a tag on a type it has no control for is an error that says which types it knows.

The fleet page of `examples/guides/page` is a complete program: a filter, a scrolling list of hosts each with a checkbox bound to the host, a progress bar and a button, and not one identity declared.

<!-- example: page/main.go -->
```go
// Fleet check written as a page: the page examples/guides/fleet declares with a
// Frame and a tree of identities, written without naming a control.
//
// Each checkbox writes into the host it shows, and is named by that host's field,
// so filtering the list moves no focus or click from one host to another.
package main

import (
	"log"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/ui"
)

type host struct {
	name    string
	addr    string
	item    string
	handled bool
}

type model struct {
	hosts  []host
	filter string
	scans  int
}

var filters = []string{"All", "Open", "Handled"}

func newModel() *model {
	return &model{filter: "All", hosts: []host{
		{name: "edge-01", addr: "10.0.4.11", item: "Disable password auth"},
		{name: "edge-02", addr: "10.0.4.12", item: "Rotate host key"},
		{name: "build-01", addr: "10.0.8.3", item: "Pin to ed25519"},
		{name: "build-02", addr: "10.0.8.4", item: "Restrict agent forwarding", handled: true},
		{name: "db-primary", addr: "10.0.9.2", item: "Close port 22 to the world"},
		{name: "db-replica", addr: "10.0.9.3", item: "Enable key-only login", handled: true},
		{name: "jump-01", addr: "10.0.1.5", item: "Set a login grace time"},
		{name: "nas-01", addr: "10.0.12.7", item: "Upgrade OpenSSH"},
	}}
}

func (m *model) matches(h *host) bool {
	switch m.filter {
	case "Open":
		return !h.handled
	case "Handled":
		return h.handled
	}
	return true
}

func (m *model) handled() int {
	n := 0
	for i := range m.hosts {
		if m.hosts[i].handled {
			n++
		}
	}
	return n
}

func (m *model) page(c ui.C) {
	ui.Column(c).Fill().Pad(22).Gap(14).Children(func() {
		ui.Text(c, "Fleet check").FontSize(26).Bold()
		ui.Text(c, "SSH posture across the hosts this workstation reaches.").FontSize(13).Muted()
		ui.Choice(c, &m.filter, filters, nil)

		// The list takes the room left over, so the footer stays on screen.
		ui.Scroll(c).Grow(1).Gap(8).Children(func() {
			for i := range m.hosts {
				if h := &m.hosts[i]; m.matches(h) {
					m.row(c, h)
				}
			}
		})

		handled, total := m.handled(), len(m.hosts)
		ui.Progress(c, float32(handled)/float32(max(total, 1))).FillWidth()
		ui.Row(c).FillWidth().Gap(10).Items(ui.Center).Children(func() {
			ui.Textf(c, "%d of %d handled", handled, total).FontSize(12).Muted().Mono()
			ui.Spacer(c)
			ui.Textf(c, "scanned %dx", m.scans).FontSize(12).Muted().Mono()
			if ui.Button(c, "Scan again").Clicked() {
				m.scans++
			}
		})
	})
}

// row is one host: a component of the page's own, written as Viva's elements are.
func (m *model) row(c ui.C, h *host) {
	theme := c.Theme()
	ui.Row(c).FillWidth().Pad(12).Gap(12).Radius(10).Items(ui.Center).Background(theme.Card).Children(func() {
		if h.handled {
			ui.Text(c, "ok").FontSize(11).Color(theme.Success)
		} else {
			ui.Text(c, "open").FontSize(11).Color(theme.Danger)
		}
		ui.Column(c).Grow(1).Gap(2).Children(func() {
			ui.Row(c).Gap(8).Items(ui.Center).Children(func() {
				ui.Text(c, h.name).FontSize(14).Bold()
				ui.Text(c, h.addr).FontSize(12).Muted().Mono()
			})
			item := ui.Text(c, h.item).FontSize(12)
			if h.handled {
				item.Muted()
			}
		})
		// The checkbox writes into the host; the bar above derives itself from the
		// hosts on the next paint.
		ui.Checkbox(c, &h.handled, "").Label(h.name)
	})
}

func main() {
	if err := ui.Run(app.Config{Title: "Viva fleet", Width: 460, Height: 560}, newModel().page); err != nil {
		log.Fatal(err)
	}
}
```

`c.Frame()` is the Frame underneath, for anything the page does not declare itself (`Textarea`, `InputGroup`, the `Base` controls); `c.Part("word")` gives the next element a name of yours, which a test or the headless driver can use. A page painted into part of a window (a pane) is `page.Draw(canvas, bounds, view)` on a `ui.NewPage` given the window's Tree and a root name. A window-level handler that switches on identities, or a custom control, keeps using a Frame.

The fleet page of `examples/guides/page` (a choice, a scroll area with eight rows each holding a checkbox, a progress bar and a button) declares, lays out and paints in 9.8 µs on an Apple M2 Pro against 9.0 µs for the same page written with a Frame, and allocates nothing once warm; a list of a thousand keyed rows costs 0.75 ms against 0.67 ms. With each row in a `Memo` a list of two hundred rows costs 87 µs instead of 271 µs, and 37 µs with the whole list in one; the copies keep the last paint's commands, about 0.5 MB for those two hundred rows (`go test ./ui -bench 'Fleet$|Rows1000|Rows200'`).

## States and effects

An element that has an identity looks different while the pointer is over it, while it is held down and while the keyboard has it. Each state is a `Look`: a value that lists only what changes, built with the same words as the element (`Background`, `Border`, `Color`, `Ring`, `Shadow`, `Opacity`).

```go
f.Row().Pad(8).Radius(8).Hit(id, "Open").Transition().
	Hover(ui.Look{}.Background(theme.Hover)).
	Pressed(ui.Look{}.Background(theme.Muted)).
	Focus(ui.Look{}.Ring(3, theme.Ring))
```

`Focus` applies when the focus is meant to be seen (after a key, not after a click); a `Hit` is focusable unless `HitWith` says `SkipFocus`. The looks combine field by field in the order Focus, Hover, Pressed, so a focused row that is hovered shows the ring of one and the fill of the other. `Transition()` fades between them over the theme's motion (`TransitionWith` takes another); while nothing changes it schedules no frames. `StateFrom(row)` makes an element follow the state of another, so what is inside a row can change with the row: `f.Icon(edit).Opacity(0).StateFrom(row).Hover(ui.Look{}.Opacity(1))` is an icon that appears when its row is hovered.

Four effects have no state of their own. `Ring(width, color)` draws a line around the outside of the element without taking room, as a focus ring does. `Opacity(v)` fades the element and everything in it, controls included; it stays laid out and clickable at 0, and an `Image` is not faded. `Gradient(angle, from, to)` and `GradientStops(angle, stops...)` fill the element, corners included, with a linear gradient, over its `Background` and under its `Border`; the angle is in degrees clockwise from up, as in CSS, and `ui.ToRight`, `ui.ToBottomRight` and the others name the common ones.

## Mixing with hand-placed painting

`Slot()` reserves a rectangle for something you paint yourself, and `Box.Rect()` says where it ended up. Use `view.Begin(...)` and `f.End()` instead of `Draw` when you need the rectangles before painting: declare, call `End`, then paint into the slots. This is how a custom control sits inside a laid-out page.

## Cost

Declaring, measuring, arranging and painting a page of about sixty elements takes about 8 microseconds and allocates nothing once the `View` has seen its largest page. Keep one `View` per page that is painted every frame, in your state; the zero value is ready. The function you give `Children` is called at once and never stored, so it costs nothing either.

A single-line `Text` is not wrapped: a line that does not fit is clipped, and it shrinks before a fixed-size sibling does. Use `Paragraph` for text that should wrap.

## Starting a project

`viva init my-app` writes a runnable project: a window laid out with `ui`, a test that drives it without a window, `viva.toml`, and `AGENTS.md`, a one-page guide to layout, identities, clicks and tests that a coding assistant reads before it changes the UI. `viva dev .` then runs it with hot reload.

<!-- example: layout/main.go -->
```go
package main

import (
	"log"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

var (
	tree = gfx.Tree{}
	page = tree.Feature("layout")

	save   = tree.ID(page, "save")
	cancel = tree.ID(page, "cancel")
)

type model struct {
	saved bool
	notes bool
	view  ui.View
}

func newConfig(m *model) app.Config {
	theme := gfx.LightTheme()
	config := app.Config{Tree: &tree, Title: "Layout", Width: 420, Height: 220}

	config.Paint = func(canvas *gfx.Canvas, size gfx.Size) {
		canvas.Fill(gfx.Rect{Width: size.Width, Height: size.Height}, theme.Background)
		bounds := gfx.Rect{Width: size.Width, Height: size.Height}

		m.view.Draw(canvas, theme, bounds, func(f *ui.Frame) {
			f.Column().Fill().Pad(24).Gap(12).Children(func() {
				f.Text("Deploy settings").FontSize(18).Bold()
				f.Text("Where the build goes when it passes.").Muted()
				f.Divider()
				f.Row().Items(ui.Center).Gap(8).Children(func() {
					f.Text("Notify the team")
					f.Spacer()
					f.Switch(tree.ID(page, "notes"), m.notes).Label("Notify the team")
				})
				f.Spacer()
				f.Row().Gap(8).Justify(ui.End).Children(func() {
					f.Button(cancel, "Cancel").Outline()
					if f.Button(save, "Save").Clicked() {
						m.saved = true
					}
				})
			})
		})
	}
	return config
}

func main() {
	m := &model{}
	var canvas gfx.Canvas
	if err := app.RunCanvas(newConfig(m), &canvas); err != nil {
		log.Fatal(err)
	}
}
```
