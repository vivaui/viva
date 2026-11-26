// Package divider is the opt-in page control for a rule with a word in it: the
// line that parts two sections and says what the second one is. A plain rule is
// ui.Divider; this is for a rule that carries a label, as "or" between two ways to
// sign in does:
//
//	divider.View(c, "or", divider.Spec{})
//	divider.View(c, "Advanced", divider.Spec{Align: divider.Start})
//
// The label is centered by default; Start and End put it near an end, with a short
// run of line before it. The rule is horizontal and as wide as the container it is
// in.
//
// It is a separate package so an application that does not import it pays
// nothing for it, and it depends only on the public gfx and ui packages.
package divider

import (
	"github.com/vivaui/viva/ui"
)

// Align is where the label sits along the rule.
type Align uint8

const (
	// Center is the zero value: the label in the middle.
	Center Align = iota
	// Start puts the label near the start of the rule.
	Start
	// End puts the label near the end of the rule.
	End
)

// Spec declares the rule.
type Spec struct {
	// Align is where the label sits.
	Align Align
}

// run is the short length of line before a label at an end.
const run = 24

// View declares the rule with label at c. It is as wide as the container it is in and
// as tall as the label. Declare the view on its own, not inside a Memo.
func View(c ui.C, label string, spec Spec) ui.Box {
	theme := c.Theme()
	metrics := theme.Metrics
	row := ui.Row(c).FillWidth().Items(ui.Center).Gap(metrics.GapItem)
	row.Children(func() {
		if spec.Align == Start {
			ui.Spacer(c).Width(run).Height(1).Grow(0).Shrink(0).Background(theme.Border)
		} else {
			ui.Spacer(c).Height(1).Grow(1).Background(theme.Border)
		}
		ui.Text(c, label).FontSize(metrics.FontSizeS).Muted().Shrink(0)
		if spec.Align == End {
			ui.Spacer(c).Width(run).Height(1).Grow(0).Shrink(0).Background(theme.Border)
		} else {
			ui.Spacer(c).Height(1).Grow(1).Background(theme.Border)
		}
	})
	return row
}
