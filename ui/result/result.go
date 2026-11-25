// Package result is the opt-in page control for the outcome of a task, shown as
// the whole content of a page or a dialog: a large status icon, a title, a
// description and the next steps. A page declares it with a Spec and, optionally,
// the actions under the words:
//
//	result.View(c, result.Spec{
//		Status:      ui.StatusSuccess,
//		Title:       "Key imported",
//		Description: "The key is now in your vault.",
//	}, func() {
//		ui.Button(c, "Done")
//	}).Fill()
//
// Status picks the icon and its color in the shared ui.Status vocabulary:
// ui.StatusDefault is an information result, and success, warning and danger
// carry a check, a triangle and a cross. Icon replaces it. It is the large
// counterpart of an alert: an alert sits in a page beside other content, a result
// is the content.
//
// It is a separate package so an application that does not import it pays
// nothing for it, and it depends only on the public gfx and ui packages.
package result

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

const maxWidth = 440

var circle = []gfx.PathCommand{
	gfx.MoveTo(22, 12), gfx.CubicTo(22, 17.52, 17.52, 22, 12, 22), gfx.CubicTo(6.48, 22, 2, 17.52, 2, 12),
	gfx.CubicTo(2, 6.48, 6.48, 2, 12, 2), gfx.CubicTo(17.52, 2, 22, 6.48, 22, 12),
}

func icon(extra ...gfx.PathCommand) gfx.Icon {
	return gfx.Icon{ViewBox: 24, Stroke: 1.5, Cap: gfx.CapRound, Join: gfx.JoinRound, Commands: append(append([]gfx.PathCommand(nil), circle...), extra...)}
}

var (
	infoIcon    = icon(gfx.MoveTo(12, 16), gfx.LineTo(12, 12), gfx.MoveTo(12, 8), gfx.LineTo(12.01, 8))
	successIcon = icon(gfx.MoveTo(8.5, 12.5), gfx.LineTo(11, 15), gfx.LineTo(15.5, 9.5))
	dangerIcon  = icon(gfx.MoveTo(15, 9), gfx.LineTo(9, 15), gfx.MoveTo(9, 9), gfx.LineTo(15, 15))
	warningIcon = gfx.Icon{ViewBox: 24, Stroke: 1.5, Cap: gfx.CapRound, Join: gfx.JoinRound, Commands: []gfx.PathCommand{
		gfx.MoveTo(10.29, 3.86), gfx.LineTo(1.82, 18), gfx.QuadTo(1, 20, 3, 20), gfx.LineTo(21, 20), gfx.QuadTo(23, 20, 22.18, 18),
		gfx.LineTo(13.71, 3.86), gfx.QuadTo(12, 2, 10.29, 3.86),
		gfx.MoveTo(12, 9), gfx.LineTo(12, 13), gfx.MoveTo(12, 17), gfx.LineTo(12.01, 17),
	}}
)

// Spec declares a result.
type Spec struct {
	// Status picks the icon and its color; an information result by default.
	Status ui.Status
	// Icon replaces the icon of the status. A zero icon keeps it.
	Icon gfx.Icon
	// Title says how it went; Description says more. Either may be empty.
	Title, Description string
}

// View declares the result at c, with actions, when not nil, declaring the next
// steps under the words. It is centered in the room it is given: give it Fill, or
// a size. Declare the view on its own, not inside a Memo.
func View(c ui.C, spec Spec, actions func()) ui.Box {
	theme := c.Theme()
	metrics := theme.Metrics
	ink := theme.MutedForeground
	glyph := infoIcon
	switch spec.Status {
	case ui.StatusSuccess:
		ink, glyph = theme.Success, successIcon
	case ui.StatusWarning:
		ink, glyph = theme.WarningColor(), warningIcon
	case ui.StatusDanger:
		ink, glyph = theme.Danger, dangerIcon
	}
	if spec.Icon.ViewBox > 0 {
		glyph = spec.Icon
	}
	root := ui.Column(c).Center().Gap(metrics.GapItem).Pad(metrics.PadSurface * 2)
	root = root.HitWith(ui.ID(c), spec.Title, gfx.HitOptions{Role: gfx.RoleStatus, SkipFocus: true})
	root.Children(func() {
		ui.Icon(c, glyph).Size(64, 64).Color(ink)
		if spec.Title != "" {
			ui.Text(c, spec.Title).FontSize(metrics.FontSizeL).Weight(gfx.WeightSemibold).TextCenter().MaxWidth(maxWidth)
		}
		if spec.Description != "" {
			ui.Wrap(c, spec.Description).Muted().TextCenter().MaxWidth(maxWidth)
		}
		if actions != nil {
			ui.Row(c).Gap(metrics.GapItem).MarginSides(metrics.GapItem, 0, 0, 0).Items(ui.Center).Children(actions)
		}
	})
	return root
}
