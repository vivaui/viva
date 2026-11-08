---
title: Declarative layout
summary: Describe a page with rows, columns, padding and growth instead of coordinates; the flex package lays it out.
order: 7
group: guides
---

Slicing rectangles by hand (see Drawing and layout) is the lowest level of Viva, and it is right for a custom control. For a page of ordinary parts (a header, a form, a list row, a footer) it is a lot of arithmetic, and the arithmetic goes wrong when a label gets longer. The `flex` package lays a page out the way CSS flexbox does, from a description.

```go
view.Draw(canvas, theme, bounds, func(f *flex.Frame) {
	f.Column().Pad(24).Gap(12).Children(func() {
		f.Text("Title").FontSize(18).Bold()
		f.Row().Gap(8).Justify(flex.End).Children(func() {
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
- Across a container children stretch by default; `Items(flex.Center)` centers them, `Self` does it for one child. Along it, `Justify(flex.Center)`, `flex.End` and `flex.Between` place what is left. `Center()` does both.
- `Basis(px)` is what an element starts from before free room is shared, and `WidthPercent` and `HeightPercent` take a share of the container. `MinWidth`, `MaxWidth`, `MinHeight` and `MaxHeight` bound a size; room a capped element cannot take goes to the others, as in CSS, so `Grow(1).MaxWidth(470)` beside a `Spacer()` keeps what follows against the far edge.
- `Items(flex.Baseline)` lines a Row's children up on their first line of text, so a large heading, a small caption and a button sit on one baseline. A `Text` shrinks and clips before a fixed-size sibling does.
- `Background`, `Radius`, `Border`, `Shadow` and `Clip` style a container. On a control such as an `Input`, `Select` or `Button`, `Radius` and `FontSize` give that control its own corner radius and text size, and a `Height` on a `Button` is how tall the button is painted.
- `Paragraph(text, maxLines)` is text that wraps to the width it gets, so a longer label makes the row taller and pushes what follows down. `LineHeight` sets its leading.
- `Grid(minCell)` fits as many equal columns as the width allows and wraps onto new rows.
- `Scroll(offset)` is a clipped column taller than itself. You own the offset (the wheel arrives in `Config.OnScroll`); after `End`, `Content()` is how tall the children are, so the offset can be clamped.

Controls are the ones in `components`, with the same look: `Button` (with `Outline`, `Ghost`, `Secondary`, `Destructive`, `Small` and the other variants), `Checkbox`, `Switch`, `Icon`, `Divider`. `Clicked()` is read where the control is declared: it is true in the one paint that delivers the click. Any element can be made a pointer target with `Hit(id, label)`; what it looks like under the pointer, pressed and focused is declared with `Hover`, `Pressed` and `Focus` (below).

## Pages without identities: flex.Screen

`flex.Screen` is the same layout with the bookkeeping of a page done for you. A Frame's controls take identities you declare in a `ui.Tree` and report clicks you write back yourself; a Screen's controls take the value they change, and the Screen names them:

```go
func (m *model) page(s flex.Screen) {
	s.Column().Fill().Pad(24).Gap(12).Children(func() {
		s.Field(&m.name, "Name", "Your name")
		s.Switch(&m.sync, "Sync over SSH")
		s.Choice(&m.theme, themes)
		if s.Button("Save").Clicked() {
			m.save()
		}
	})
}

flex.Run(app.Config{Title: "Settings", Width: 480, Height: 320}, m.page)
```

A click flips `m.sync` and sets `m.theme` in the paint that delivers it, the first time the control is asked (`Clicked`, `Changed`) or else when the page ends, so a `Disabled` written after the control still holds. `Field` needs no `Config.TextInput`: the Screen's `Config` gives the platform the editor of each Field it declared, and passes any other focused field to the provider the configuration already had. `Popover` and `Modal` open while a `*bool` is true and set it to false when dismissed (see [Dialogs, menus and popups](/en/docs/overlays/)). `Box.NativeView` gives a page of the system's web engine the room of an element.

The Screen names its controls so that a name never moves to another control without the user knowing:

- A control bound to a value is named by that value's address. A field shown above a password field does not take the password's focus and typing, and filtering a list keeps each checkbox's focus on its row. Bind fields of the model, not copies: a local copy has a new address in every paint, and its click is lost (the Screen reports it through `diagnostics`).
- A control bound to nothing, a `Button`, is numbered among the buttons of its scope. While the user holds one (presses it, or it has the focus), a paint in which another button took its number does not deliver the press, and the focus is given up.
- Inside `s.Key(item.ID)` or `s.KeyIndex(id)`, controls are numbered within that row, so a list whose rows have buttons keys its rows: the row's buttons then follow it when rows come, go or move. Keys are reused two paints after their row is gone, so a list costs what its rows on screen at once cost. Two buttons with one label and no Key in one scope are reported, since nothing can tell them apart.

The fleet page of `examples/guides/screen` is a complete program: a filter, a scrolling list of hosts each with a checkbox bound to the host, a progress bar and a button, and not one identity declared.

<!-- example: screen/main.go -->
```go
// Fleet check with flex.Screen: the page examples/guides/fleet declares with a
// Frame and a tree of identities, written without naming a control.
//
// Each checkbox writes into the host it shows, and is named by that host's field,
// so filtering the list moves no focus or click from one host to another.
package main

import (
	"log"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/flex"
)

type host struct {
	name    string
	addr    string
	item    string
	handled bool
}

type model struct {
	hosts  []host
	filter int
	scans  int
}

var filters = []string{"All", "Open", "Handled"}

func newModel() *model {
	return &model{hosts: []host{
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
	switch filters[m.filter] {
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

func (m *model) page(s flex.Screen) {
	s.Column().Fill().Pad(22).Gap(14).Children(func() {
		s.Text("Fleet check").FontSize(26).Bold()
		s.Text("SSH posture across the hosts this workstation reaches.").FontSize(13).Muted()
		s.Choice(&m.filter, filters)

		// The list takes the room left over, so the footer stays on screen.
		s.ScrollArea("hosts").Grow(1).Gap(8).Children(func() {
			for i := range m.hosts {
				if h := &m.hosts[i]; m.matches(h) {
					m.row(s, h)
				}
			}
		})

		handled, total := m.handled(), len(m.hosts)
		s.Progress(float32(handled) / float32(max(total, 1))).FillWidth()
		s.Row().FillWidth().Gap(10).Items(flex.Center).Children(func() {
			s.Textf("%d of %d handled", handled, total).FontSize(12).Muted().Mono()
			s.Spacer()
			s.Textf("scanned %dx", m.scans).FontSize(12).Muted().Mono()
			if s.Button("Scan again").Clicked() {
				m.scans++
			}
		})
	})
}

func (m *model) row(s flex.Screen, h *host) {
	theme := s.Theme()
	s.Row().FillWidth().Pad(12).Gap(12).Radius(10).Items(flex.Center).Background(theme.Card).Children(func() {
		if h.handled {
			s.Text("ok").FontSize(11).Color(theme.Success)
		} else {
			s.Text("open").FontSize(11).Color(theme.Danger)
		}
		s.Column().Grow(1).Gap(2).Children(func() {
			s.Row().Gap(8).Items(flex.Center).Children(func() {
				s.Text(h.name).FontSize(14).Bold()
				s.Text(h.addr).FontSize(12).Muted().Mono()
			})
			item := s.Text(h.item).FontSize(12)
			if h.handled {
				item.Muted()
			}
		})
		// The checkbox writes into the host; the bar above derives itself from the
		// hosts on the next paint.
		s.Checkbox(&h.handled, "").Label(h.name)
	})
}

func main() {
	if err := flex.Run(app.Config{Title: "Viva fleet", Width: 460, Height: 560}, newModel().page); err != nil {
		log.Fatal(err)
	}
}
```

`s.Raw()` is the Frame underneath, for anything the Screen does not forward (`Textarea`, `InputGroup`, the `Base` controls); `s.ID("word")` gives a control declared there a name of yours, which a test or the headless driver can use. A page painted into part of a window (a pane) is `screen.Draw(canvas, bounds, body)` on a `flex.NewScreen` given the window's Tree. A window-level handler that switches on identities, or a custom control, keeps using a Frame.

The fleet page of `examples/guides/screen` (a choice, a scroll area with eight rows each holding a checkbox, a progress bar and a button) declares, lays out and paints in 9.8 µs on an Apple M2 Pro against 9.0 µs for the same page written with a Frame, and allocates nothing once warm; a list of a thousand keyed rows costs 0.68 ms against 0.57 ms (`go test ./flex -bench 'Fleet$|Keyed1000$'`).

## States and effects

An element that has an identity looks different while the pointer is over it, while it is held down and while the keyboard has it. Each state is a `Look`: a value that lists only what changes, built with the same words as the element (`Background`, `Border`, `Color`, `Ring`, `Shadow`, `Opacity`).

```go
f.Row().Pad(8).Radius(8).Hit(id, "Open").Transition().
	Hover(flex.Look{}.Background(theme.Hover)).
	Pressed(flex.Look{}.Background(theme.Muted)).
	Focus(flex.Look{}.Ring(3, theme.Ring))
```

`Focus` applies when the focus is meant to be seen (after a key, not after a click); a `Hit` is focusable unless `HitWith` says `SkipFocus`. The looks combine field by field in the order Focus, Hover, Pressed, so a focused row that is hovered shows the ring of one and the fill of the other. `Transition()` fades between them over the theme's motion (`TransitionWith` takes another); while nothing changes it schedules no frames. `StateFrom(row)` makes an element follow the state of another, so what is inside a row can change with the row: `f.Icon(edit).Opacity(0).StateFrom(row).Hover(flex.Look{}.Opacity(1))` is an icon that appears when its row is hovered.

Four effects have no state of their own. `Ring(width, color)` draws a line around the outside of the element without taking room, as a focus ring does. `Opacity(v)` fades the element and everything in it, controls included; it stays laid out and clickable at 0, and an `Image` is not faded. `Gradient(angle, from, to)` and `GradientStops(angle, stops...)` fill the element, corners included, with a linear gradient, over its `Background` and under its `Border`; the angle is in degrees clockwise from up, as in CSS, and `flex.ToRight`, `flex.ToBottomRight` and the others name the common ones.

## Mixing with hand-placed painting

`Slot()` reserves a rectangle for something you paint yourself, and `Box.Rect()` says where it ended up. Use `view.Begin(...)` and `f.End()` instead of `Draw` when you need the rectangles before painting: declare, call `End`, then paint into the slots. This is how a custom control sits inside a laid-out page.

## Cost

Declaring, measuring, arranging and painting a page of about sixty elements takes about 8 microseconds and allocates nothing once the `View` has seen its largest page. Keep one `View` per page that is painted every frame, in your state; the zero value is ready. The function you give `Children` is called at once and never stored, so it costs nothing either.

A single-line `Text` is not wrapped: a line that does not fit is clipped, and it shrinks before a fixed-size sibling does. Use `Paragraph` for text that should wrap.

## Starting a project

`viva init my-app` writes a runnable project: a window laid out with `flex`, a test that drives it without a window, `viva.toml`, and `AGENTS.md`, a one-page guide to layout, identities, clicks and tests that a coding assistant reads before it changes the UI. `viva dev .` then runs it with hot reload.

<!-- example: layout/main.go -->
```go
package main

import (
	"log"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/flex"
	"github.com/vivaui/viva/ui"
)

var (
	tree = ui.Tree{}
	page = tree.Feature("layout")

	save   = tree.ID(page, "save")
	cancel = tree.ID(page, "cancel")
)

type model struct {
	saved bool
	notes bool
	view  flex.View
}

func newConfig(m *model) app.Config {
	theme := ui.LightTheme()
	config := app.Config{Tree: &tree, Title: "Layout", Width: 420, Height: 220}

	config.Paint = func(canvas *ui.Canvas, size ui.Size) {
		canvas.Fill(ui.Rect{Width: size.Width, Height: size.Height}, theme.Background)
		bounds := ui.Rect{Width: size.Width, Height: size.Height}

		m.view.Draw(canvas, theme, bounds, func(f *flex.Frame) {
			f.Column().Fill().Pad(24).Gap(12).Children(func() {
				f.Text("Deploy settings").FontSize(18).Bold()
				f.Text("Where the build goes when it passes.").Muted()
				f.Divider()
				f.Row().Items(flex.Center).Gap(8).Children(func() {
					f.Text("Notify the team")
					f.Spacer()
					f.Switch(tree.ID(page, "notes"), m.notes).Label("Notify the team")
				})
				f.Spacer()
				f.Row().Gap(8).Justify(flex.End).Children(func() {
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
	var canvas ui.Canvas
	if err := app.RunCanvas(newConfig(m), &canvas); err != nil {
		log.Fatal(err)
	}
}
```
