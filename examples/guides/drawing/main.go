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
