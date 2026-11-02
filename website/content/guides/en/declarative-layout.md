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

Controls are the ones in `components`, with the same look: `Button` (with `Outline`, `Ghost`, `Secondary`, `Destructive`, `Small` and the other variants), `Checkbox`, `Switch`, `Icon`, `Divider`. `Clicked()` is read where the control is declared: it is true in the one paint that delivers the click. Any element can be made a pointer target with `Hit(id, label)`, and `HoverBackground` paints under the pointer.

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
