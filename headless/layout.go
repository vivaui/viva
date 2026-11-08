//go:build darwin || windows || linux

package headless

import (
	"fmt"
	"math"
	"sort"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/internal/textmetrics"
	"github.com/vivaui/viva/ui"
)

// LayoutKind names one way a frame's geometry is probably wrong.
type LayoutKind string

const (
	// LayoutOffscreen is text whose visible part reaches outside the window.
	LayoutOffscreen LayoutKind = "offscreen"
	// LayoutClipped is text a clip cuts off at its end: a label that is truncated.
	LayoutClipped LayoutKind = "clipped"
	// LayoutOverlap is two texts on one layer that are drawn over each other.
	LayoutOverlap LayoutKind = "overlap"
	// LayoutSpill is text that reaches out of the control it is inside.
	LayoutSpill LayoutKind = "spill"
	// LayoutCollide is two focusable controls on one layer that cross each other
	// without one containing the other.
	LayoutCollide LayoutKind = "collide"
)

// LayoutProblem is one suspicious spot in a frame. Bounds is where it is; Text
// is the painted text or the control's label.
type LayoutProblem struct {
	Kind   LayoutKind
	Bounds ui.Rect
	Text   string
	Detail string
}

// layoutTolerance is how far, in logical pixels, geometry may differ before it
// counts: shapes are laid out on sub-pixel positions and text boxes round up.
const layoutTolerance = 1

// CheckLayout finds geometry in snapshot that is almost surely a bug: text that
// is cut off, runs out of its button or field, lies over other text, or leaves
// the window, and focusable controls that cross each other. It reads the frame
// the application painted and decides nothing about intent, so a deliberate
// overlap (an overlay drawn on its own layer is not one) still shows. Problems
// come from the top of the window down.
func CheckLayout(snapshot app.Snapshot) []LayoutProblem {
	window := ui.Rect{Width: float32(snapshot.Width), Height: float32(snapshot.Height)}
	type visibleText struct {
		text    ui.Text
		visible ui.Rect
	}
	var texts []visibleText
	var problems []LayoutProblem
	for index, text := range snapshot.Texts {
		// A terminal's grid is laid out by cells and checked by its own tests.
		if text.Value == "" || text.CellWidth > 0 || text.RunCount > 0 || text.Style.Mono {
			continue
		}
		width := paintedTextWidth(snapshot, index, text)
		extent := lineExtent(text, width)
		if extent.Width <= 0 || extent.Height <= 0 {
			continue
		}
		visible := extent
		if text.HasClip {
			visible = intersection(visible, text.ClipOf)
		}
		if visible.Width <= 0 || visible.Height <= 0 {
			continue
		}
		texts = append(texts, visibleText{text, visible})
		if !contains(window, visible, layoutTolerance) {
			problems = append(problems, LayoutProblem{LayoutOffscreen, visible, text.Value,
				fmt.Sprintf("window is %dx%d", snapshot.Width, snapshot.Height)})
		}
		// Only a cut across the line is reported: a line that a scrolled area shows
		// half of is cut along its height at the edge of the area, as it should be.
		if cutW := extent.Width - visible.Width; cutW > layoutTolerance {
			problems = append(problems, LayoutProblem{LayoutClipped, extent, text.Value,
				fmt.Sprintf("%g of %g px wide are cut off", cutW, extent.Width)})
		}
		if container, found := innermostControl(snapshot.Hits, text, width); found &&
			!contains(container.Bounds, visible, layoutTolerance) {
			problems = append(problems, LayoutProblem{LayoutSpill, visible, text.Value,
				fmt.Sprintf("reaches out of %q at x=%g y=%g w=%g h=%g", container.Label, container.Bounds.X, container.Bounds.Y, container.Bounds.Width, container.Bounds.Height)})
		}
	}
	for first := range texts {
		for second := first + 1; second < len(texts); second++ {
			a, b := texts[first], texts[second]
			if a.text.Layer != b.text.Layer {
				continue
			}
			common := intersection(a.visible, b.visible)
			if common.Width > layoutTolerance && common.Height > layoutTolerance {
				problems = append(problems, LayoutProblem{LayoutOverlap, common, a.text.Value,
					fmt.Sprintf("is drawn over %q", b.text.Value)})
			}
		}
	}
	var focusable []ui.Hit
	for _, hit := range snapshot.Hits {
		if hit.Focusable && !hit.Disabled && !hit.Block && !hit.IsLabel && !hit.Backdrop {
			focusable = append(focusable, hit)
		}
	}
	for first := range focusable {
		for second := first + 1; second < len(focusable); second++ {
			a, b := focusable[first], focusable[second]
			if a.Layer != b.Layer {
				continue
			}
			common := intersection(a.Bounds, b.Bounds)
			if common.Width > layoutTolerance && common.Height > layoutTolerance &&
				!contains(a.Bounds, b.Bounds, layoutTolerance) && !contains(b.Bounds, a.Bounds, layoutTolerance) {
				problems = append(problems, LayoutProblem{LayoutCollide, common, a.Label,
					fmt.Sprintf("crosses %q", b.Label)})
			}
		}
	}
	sort.SliceStable(problems, func(i, j int) bool {
		a, b := problems[i].Bounds, problems[j].Bounds
		if a.Y != b.Y {
			return a.Y < b.Y
		}
		return a.X < b.X
	})
	return problems
}

// paintedTextWidth prefers the width the canvas measured with its font engine.
// Snapshots built by hand (or by an older producer) fall back to the approximate
// reference metrics, keeping CheckLayout's previous behaviour.
func paintedTextWidth(snapshot app.Snapshot, index int, text ui.Text) float32 {
	if index < len(snapshot.TextWidths) && snapshot.TextWidths[index] > 0 {
		return snapshot.TextWidths[index]
	}
	return textmetrics.Width(text.Value, text.Style.Size, text.Style.Weight)
}

// lineExtent is where the line of text is drawn: Text.Bounds holds the box the
// caller gave, in which the line starts at Bounds.X and is as wide as the
// painted (or reference) metrics measure it.
func lineExtent(text ui.Text, width float32) ui.Rect {
	box := text.Bounds
	box.Width = width
	return box
}

// innermostControl is the smallest control on text's layer that holds the middle
// of the line. A line that runs out of its control by more than half of itself
// belongs to no control and is not found. Opaque panels count; labels of a
// control, which are the text's own extent, and backdrops do not.
func innermostControl(hits []ui.Hit, text ui.Text, width float32) (ui.Hit, bool) {
	line := lineExtent(text, width)
	middleY := line.Y + line.Height/2
	var best ui.Hit
	found := false
	for _, hit := range hits {
		if hit.Layer != text.Layer || hit.IsLabel || hit.Backdrop || hit.MovesWindow || hit.Disabled && !hit.Block {
			continue
		}
		bounds := hit.Bounds
		if !pointIn(bounds, line.X+line.Width/2, middleY) {
			continue
		}
		if !found || bounds.Width*bounds.Height < best.Bounds.Width*best.Bounds.Height {
			best, found = hit, true
		}
	}
	return best, found
}

func pointIn(bounds ui.Rect, x, y float32) bool {
	return x >= bounds.X && x <= bounds.X+bounds.Width && y >= bounds.Y && y <= bounds.Y+bounds.Height
}

func intersection(a, b ui.Rect) ui.Rect {
	left := float32(math.Max(float64(a.X), float64(b.X)))
	top := float32(math.Max(float64(a.Y), float64(b.Y)))
	right := float32(math.Min(float64(a.X+a.Width), float64(b.X+b.Width)))
	bottom := float32(math.Min(float64(a.Y+a.Height), float64(b.Y+b.Height)))
	if right <= left || bottom <= top {
		return ui.Rect{}
	}
	return ui.Rect{X: left, Y: top, Width: right - left, Height: bottom - top}
}

// contains reports whether inner lies inside outer, give or take slack.
func contains(outer, inner ui.Rect, slack float32) bool {
	return inner.X >= outer.X-slack && inner.Y >= outer.Y-slack &&
		inner.X+inner.Width <= outer.X+outer.Width+slack && inner.Y+inner.Height <= outer.Y+outer.Height+slack
}
