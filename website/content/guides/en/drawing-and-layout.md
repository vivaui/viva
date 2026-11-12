---
title: Drawing and layout
summary: Coordinates, slicing space with rows, columns and grids, and painting rectangles, text, icons and paths.
order: 6
group: guides
---

Everything on screen is painted into a `gfx.Canvas` from your `Paint` callback. There is no widget tree to build and no stylesheet: you compute rectangles and ask the canvas to fill them, draw text in them, or declare them interactive.

## Coordinates

Positions are logical pixels, measured from the window's top-left corner, so the same numbers are correct on a standard and a high-density display. `gfx.Rect` holds `X`, `Y`, `Width` and `Height`. `Rect.Inset` shrinks it by a `gfx.Insets` (per-edge padding), and `gfx.All(n)` makes equal insets.

`Paint` receives the current window size and runs on every resize, so a layout that depends on `size` follows the window.

## Slicing space

Three small value types hand out rectangles. None allocates.

| Type | Use |
| --- | --- |
| `gfx.BeginRow(bounds, gap)` | Slots left to right: `Next(width)` takes a fixed width, `NextFlex()` takes what is left, `Even(n)` splits evenly. |
| `gfx.BeginColumn(bounds, gap)` | The same, top to bottom. |
| `gfx.BeginGrid(bounds, spec)` | A wrapping grid of equal cells. `GridSpec.MinCellWidth` is the narrowest a cell may be; the grid fits as many columns as it can and stretches the cells to fill the width. |

When a screen is one header, one footer and a body, cutting strips off the edges reads better than a column of slots. `Rect.CutTop`, `CutBottom`, `CutLeft` and `CutRight` return the strip and what is left, clamped so neither is ever negative; `Centered`, `Offset`, `Right` and `Bottom` cover the rest of the edge arithmetic.

```go
header, body := page.CutTop(48)
sidebar, content := body.CutLeft(240)
footer, content := content.CutBottom(32)
```

<!-- example: drawing/main.go -->
```go
package main

import (
	"fmt"
	"log"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
)

// Identities are names in a tree, not numbers: the application declares the
// tree once and every control it paints is a name inside a feature of it.
var (
	tree = gfx.Tree{}
	page = tree.Feature("drawing")

	hitNew = page.ID(tree.Name("new"))
)

func paint(canvas *gfx.Canvas, size gfx.Size) {
	theme := gfx.LightTheme()

	// Lay out by slicing rectangles; none of this allocates.
	page := gfx.Rect{Width: size.Width, Height: size.Height}.Inset(gfx.All(24))
	column := gfx.BeginColumn(page, 16)

	canvas.Label("Dashboard", column.Next(32), gfx.TextStyle{Size: 20, Weight: gfx.WeightSemibold, Color: theme.Foreground})

	toolbar := gfx.BeginRow(column.Next(36), 8)
	components.BeginButton(canvas, theme, hitNew, toolbar.Next(100)).Default("New")

	cards := column.NextFlex() // whatever height is left
	grid := gfx.BeginGrid(cards, gfx.GridSpec{MinCellWidth: 160, CellHeight: 96, Gap: 12})
	for index := 1; index <= 6; index++ {
		cell := grid.Next()
		canvas.RoundedBorder(cell, 8, 1, theme.Border, theme.Card)
		canvas.Label(fmt.Sprintf("Card %d", index), cell.Inset(gfx.All(12)), gfx.TextStyle{Size: 14, Color: theme.CardForeground})
	}
}

func main() {
	var canvas gfx.Canvas
	config := app.Config{Tree: &tree, Title: "Layout", Width: 720, Height: 480, Paint: paint}
	if err := app.RunCanvas(config, &canvas); err != nil {
		log.Fatal(err)
	}
}
```

Resize the window narrower and the grid loses columns, down to one, without any code for it, because the column count comes from the width `Paint` was given. The example's test checks exactly that.

## Painting

The canvas paints in the order you call it; later calls cover earlier ones.

| Method | Paints |
| --- | --- |
| `Fill`, `Rounded` | A rectangle, plain or with rounded corners. |
| `Border`, `RoundedBorder` | A filled rectangle with a border. |
| `Elevate`, `Shadow` | A soft drop shadow under a rounded rectangle: `Elevate` takes a level of the Tailwind scale (`gfx.ShadowXS`, `ShadowSM`, `ShadowMD`, `ShadowLG`), `Shadow` one box-shadow layer. Call it before the surface; the rectangle itself is cut out of the shadow. |
| `Label`, `LabelCenter`, `LabelAligned` | One line of system-font text. |
| `DrawIcon` | A compiled `gfx.Icon`, tinted. Compile SVGs into Go with `viva icons`. |
| `DrawPath`, `DrawBitmap` | A vector path or an image. |
| `DrawBackdrop` | An image beneath everything else in the frame, on every layer, unclipped; translucent surfaces let it show through. |
| `Clip`, `Unclip` | Restrict painting, and hit regions, to a rectangle. They nest. |

Text is drawn with `gfx.TextStyle`: size, weight (`gfx.WeightRegular`, `WeightMedium`, `WeightSemibold`…), color and, for terminals and code, `Mono`. Text is single-line. To fit a paragraph into a width, call `Canvas.WrapLines`, which breaks where Unicode line breaking (UAX #14) allows — spaces, hyphens, between CJK characters, between Thai syllables — and draw one `Label` per line. For lines drawn in a `Mono` or italic style, `Canvas.WrapLinesStyle` measures each line as that style draws it, so a fixed-pitch paragraph breaks where its cells fill the width, a CJK ideograph taking two. A `Label` aligns Hebrew, Arabic and other right-to-left text to its right edge and shows mixed text in the order Unicode's bidirectional algorithm gives (`AlignStart` is the default, `AlignEnd` the other end).

To lay text out you often need its size. `Canvas.TextWidth` and `TextWidthWeight` measure with the system font at the weight you will draw it, because a heavier line is wider.

## Declaring interaction

Painting a rectangle does not make it clickable. Declare a hit region with an identity named in your `gfx.Tree`: `Hit`, `HitWithLabel` (the label is what assistive technology calls it) or `HitWithOptions` for a role, cursor, state and more. Components do this for you; when you draw your own control, you do it. [Handling input](/en/docs/handling-input/) covers what happens next.

## Animation

Viva paints only when something asks it to, so an animation has to ask. `Canvas.Animate(id, slot, target, motion)` returns the value to draw now, easing from where it currently is toward `target`, and keeps repainting until it arrives, which means a reversal in the middle of a transition is continuous. When the user has asked for reduced motion it returns `target` unchanged. For something that changes slowly, such as a spinner that steps every 125 ms, `Canvas.RequestFrameIn(delay)` asks for one more paint after the delay; a frame that does not ask leaves the application idle.
