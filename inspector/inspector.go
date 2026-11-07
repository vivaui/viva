package inspector

import (
	"fmt"
	"strings"

	"github.com/vivaui/viva/ui"
)

// Mode says what an Inspector shows.
type Mode uint8

const (
	// Off shows nothing. It is the zero value.
	Off Mode = iota
	// Hovered outlines the region under the pointer and describes it.
	Hovered
	// All also outlines every other region of the frame.
	All
)

// overlayLayer is above any layer an application's popups use.
const overlayLayer uint16 = 1<<16 - 1

// Inspector is the state of the overlay, owned by the application. Its zero
// value is off.
type Inspector struct {
	mode Mode
}

// Mode is what the inspector shows.
func (inspector *Inspector) Mode() Mode { return inspector.mode }

// Enabled reports whether the overlay is painted.
func (inspector *Inspector) Enabled() bool { return inspector.mode != Off }

// Set chooses what the inspector shows.
func (inspector *Inspector) Set(mode Mode) { inspector.mode = mode }

// Cycle moves to the next mode: off, then the hovered region, then every
// region, then off again. One key is enough to drive it.
func (inspector *Inspector) Cycle() {
	inspector.mode = (inspector.mode + 1) % (All + 1)
}

var (
	interactiveOutline = ui.Color{Red: 0.20, Green: 0.52, Blue: 1, Alpha: 0.55}
	regionOutline      = ui.Color{Red: 0.20, Green: 0.78, Blue: 0.45, Alpha: 0.55}
	disabledOutline    = ui.Color{Red: 0.60, Green: 0.60, Blue: 0.60, Alpha: 0.55}
	hoverFill          = ui.Color{Red: 0.20, Green: 0.52, Blue: 1, Alpha: 0.16}
	hoverOutline       = ui.Color{Red: 0.20, Green: 0.52, Blue: 1, Alpha: 0.95}
	chipFill           = ui.Color{Red: 0.07, Green: 0.08, Blue: 0.10, Alpha: 0.94}
	chipBorder         = ui.Color{Red: 1, Green: 1, Blue: 1, Alpha: 0.22}
	chipText           = ui.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1}
	chipMuted          = ui.Color{Red: 0.72, Green: 0.76, Blue: 0.82, Alpha: 1}
)

const (
	chipSize    = 12
	chipLine    = 16
	chipPad     = 8
	chipGap     = 6
	chipMaxText = 64
)

// Paint draws the overlay over everything painted so far on canvas, inside
// viewport, which keeps the description on screen. Call it last in the paint
// function, after every region has been declared. It does nothing while the
// inspector is off.
func (inspector *Inspector) Paint(canvas *ui.Canvas, viewport ui.Rect) {
	if inspector.mode == Off {
		return
	}
	canvas.PushLayer(overlayLayer)
	defer canvas.PopLayer()
	hits := canvas.Hits()
	if inspector.mode == All {
		for index := range hits {
			hit := &hits[index]
			if !outlined(hit) {
				continue
			}
			canvas.Border(hit.Bounds, 1, outlineColor(hit), ui.Color{})
		}
	}
	id := canvas.Hovered()
	if id.IsZero() {
		return
	}
	for index := len(hits) - 1; index >= 0; index-- {
		if hits[index].ID != id || !outlined(&hits[index]) {
			continue
		}
		hit := &hits[index]
		canvas.Border(hit.Bounds, 2, hoverOutline, hoverFill)
		paintChip(canvas, viewport, hit)
		return
	}
}

// outlined says whether a declared region is one a person looks for: not an
// opaque block, a window-drag area or a label helper, which are no controls.
func outlined(hit *ui.Hit) bool {
	return !hit.Block && !hit.MovesWindow && !hit.IsLabel && hit.Bounds.Width > 0 && hit.Bounds.Height > 0
}

func outlineColor(hit *ui.Hit) ui.Color {
	switch {
	case hit.Disabled:
		return disabledOutline
	case hit.Container:
		return regionOutline
	}
	return interactiveOutline
}

// paintChip describes hit in a small box beside it: under it when there is room,
// else over it, kept inside viewport.
func paintChip(canvas *ui.Canvas, viewport ui.Rect, hit *ui.Hit) {
	var lines [4]string
	count := 0
	add := func(line string) {
		lines[count] = clip(line)
		count++
	}
	add(fmt.Sprintf("%s  %s", hit.Role, canvas.Tree().Text(hit.ID)))
	if hit.Label != "" {
		add(fmt.Sprintf("%q", hit.Label))
	}
	add(fmt.Sprintf("%g × %g at %g, %g · layer %d", hit.Bounds.Width, hit.Bounds.Height, hit.Bounds.X, hit.Bounds.Y, hit.Layer))
	if state := states(canvas, hit); state != "" {
		add(state)
	}
	width := float32(0)
	for _, line := range lines[:count] {
		width = max(width, canvas.TextWidthWeight(line, chipSize, ui.WeightMedium))
	}
	// A viewport narrower than the longest line shortens the lines, which the
	// label ends with an ellipsis, instead of letting the box run off the window.
	box := ui.Rect{Width: min(width+2*chipPad, max(viewport.Width, 2*chipPad)), Height: float32(count)*chipLine + 2*chipPad}
	width = box.Width - 2*chipPad
	box.X = min(max(hit.Bounds.X, viewport.X), max(viewport.X+viewport.Width-box.Width, viewport.X))
	box.Y = hit.Bounds.Y + hit.Bounds.Height + chipGap
	if box.Y+box.Height > viewport.Y+viewport.Height {
		box.Y = hit.Bounds.Y - chipGap - box.Height
	}
	box.Y = min(max(box.Y, viewport.Y), max(viewport.Y+viewport.Height-box.Height, viewport.Y))
	canvas.RoundedBorder(box, 6, 1, chipBorder, chipFill)
	for index, line := range lines[:count] {
		color := chipText
		if index > 0 {
			color = chipMuted
		}
		canvas.Label(line, ui.Rect{X: box.X + chipPad, Y: box.Y + chipPad + float32(index)*chipLine, Width: width, Height: chipLine},
			ui.TextStyle{Size: chipSize, Weight: ui.WeightMedium, Color: color})
	}
}

// states lists the state worth reading at a glance, as the headless tree does.
func states(canvas *ui.Canvas, hit *ui.Hit) string {
	var state []string
	add := func(on bool, name string) {
		if on {
			state = append(state, name)
		}
	}
	add(canvas.Focused() == hit.ID, "focused")
	add(hit.Disabled, "disabled")
	add(hit.Checked, "checked")
	add(hit.Mixed, "mixed")
	add(hit.Selected, "selected")
	add(hit.Expandable && hit.Expanded, "expanded")
	add(hit.Expandable && !hit.Expanded, "collapsed")
	add(hit.ReadOnly, "readonly")
	add(hit.Focusable && !hit.Disabled, "focusable")
	return strings.Join(state, " ")
}

// clip keeps a long label from stretching the description across the window.
func clip(line string) string {
	runes := []rune(line)
	if len(runes) <= chipMaxText {
		return line
	}
	return string(runes[:chipMaxText-1]) + "…"
}
