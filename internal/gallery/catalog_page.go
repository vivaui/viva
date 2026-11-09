package gallery

import (
	"strconv"

	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/componentdemo"
)

// catalog is the component list the gallery browses, in the order the demo
// package publishes it: by category, then by ID.
var catalog = componentdemo.Catalog()

// categorySpan is one run of components that share a category.
type categorySpan struct {
	name         string
	first, count int
}

var (
	categories      = buildCategories()
	exampleCounts   = buildExampleCounts()
	componentLines  = buildComponentLines()
	catalogSubtitle = buildCatalogSubtitle()
)

func buildCategories() []categorySpan {
	var spans []categorySpan
	for index := range catalog {
		if last := len(spans) - 1; last >= 0 && spans[last].name == catalog[index].Category {
			spans[last].count++
			continue
		}
		spans = append(spans, categorySpan{name: catalog[index].Category, first: index, count: 1})
	}
	return spans
}

// buildExampleCounts and the builders below prepare every string the paint path
// shows, so painting concatenates and formats nothing.
func buildExampleCounts() []string {
	counts := make([]string, len(catalog))
	for index := range catalog {
		count := len(catalog[index].Examples)
		counts[index] = strconv.Itoa(count) + " examples"
		if count == 1 {
			counts[index] = "1 example"
		}
	}
	return counts
}

func buildComponentLines() []string {
	lines := make([]string, len(catalog))
	for index := range catalog {
		lines[index] = catalog[index].Category + " · " + catalog[index].Summary
	}
	return lines
}

func buildCatalogSubtitle() string {
	examples := 0
	for index := range catalog {
		examples += len(catalog[index].Examples)
	}
	return strconv.Itoa(len(catalog)) + " components, " + strconv.Itoa(examples) + " live examples. Open one to see every variant and state."
}

const (
	sectionHeaderHeight = float32(30)
	sectionGap          = float32(26)
)

// catalogGrid sizes the component cards: as many columns as fit at the minimum
// card width, stretched to fill the page, so the grid tracks the window.
var catalogGrid = gfx.GridSpec{MinCellWidth: 240, CellHeight: 120, Gap: 14}

// catalogHeight returns the total catalog content height for one page width.
func catalogHeight(width float32) float32 {
	height := pageHeaderHeight
	for _, section := range categories {
		height += sectionHeaderHeight + catalogGrid.Height(width, section.count) + sectionGap
	}
	return height - sectionGap
}

// paintCatalog lays every component out as a card, grouped by category.
func (state *State) paintCatalog(canvas *gfx.Canvas, theme gfx.Theme, content gfx.Rect, viewport gfx.Rect) {
	body := pageHeader(canvas, theme, content, "Components", catalogSubtitle)
	y := body.Y
	open := 0 // the page of the card clicked in this paint, if any
	for _, section := range categories {
		canvas.Label(section.name, gfx.Rect{X: body.X, Y: y, Width: body.Width, Height: 20}, gfx.TextStyle{
			Size: 12, Weight: gfx.WeightSemibold, Color: theme.MutedForeground,
		})
		y += sectionHeaderHeight
		grid := gfx.BeginGrid(gfx.Rect{X: body.X, Y: y, Width: body.Width}, catalogGrid)
		for index := section.first; index < section.first+section.count; index++ {
			bounds := grid.Next()
			if bounds.Y+bounds.Height <= viewport.Y || bounds.Y >= viewport.Y+viewport.Height {
				continue // scrolled out of sight
			}
			if paintComponentCard(canvas, theme, index, bounds) {
				open = index + 1
			}
		}
		y += catalogGrid.Height(body.Width, section.count) + sectionGap
	}
	if open != 0 {
		state.openPage(open)
	}
}

// paintComponentCard paints one card and reports whether this paint is
// delivering a click on it.
func paintComponentCard(canvas *gfx.Canvas, theme gfx.Theme, index int, bounds gfx.Rect) bool {
	component := &catalog[index]
	id := cardScope.ID(gfx.Index(index))
	canvas.HitWithLabel(id, bounds, component.Title)
	components.Card(canvas, theme, bounds)
	if canvas.Hovered() == id {
		// RoundedBorder paints the ring color solid and then the fill over its
		// inside, so the fill must be the card surface, not transparent.
		canvas.RoundedBorder(bounds, theme.Metrics.RadiusSurface+4, 1.5, theme.Primary, theme.Card)
	}

	textWidth := bounds.Width - 32
	canvas.Label(component.Title, gfx.Rect{X: bounds.X + 16, Y: bounds.Y + 14, Width: textWidth, Height: 22}, gfx.TextStyle{
		Size: 14, Weight: gfx.WeightSemibold, Color: theme.Foreground,
	})
	first, rest := wrapLine(component.Summary, 12, textWidth)
	second, _ := wrapLine(rest, 12, textWidth)
	canvas.Label(first, gfx.Rect{X: bounds.X + 16, Y: bounds.Y + 40, Width: textWidth, Height: 18}, gfx.TextStyle{
		Size: 12, Color: theme.MutedForeground,
	})
	if second != "" {
		canvas.Label(second, gfx.Rect{X: bounds.X + 16, Y: bounds.Y + 58, Width: textWidth, Height: 18}, gfx.TextStyle{
			Size: 12, Color: theme.MutedForeground,
		})
	}
	canvas.Label(exampleCounts[index], gfx.Rect{X: bounds.X + 16, Y: bounds.Y + bounds.Height - 30, Width: textWidth, Height: 16}, gfx.TextStyle{
		Size: 11, Weight: gfx.WeightMedium, Color: theme.MutedForeground,
	})
	return canvas.Clicked(id)
}
