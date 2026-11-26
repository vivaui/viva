// Package empty is the opt-in page control for an empty state: what a list, a
// table or a page shows when there is nothing in it, with the next step to take.
// A page declares it with a Spec and, optionally, the actions under the words:
//
//	empty.View(c, empty.Spec{
//		Icon:        m.folderIcon,
//		Boxed:       true,
//		Title:       "No projects yet",
//		Description: "Create a project to get started.",
//	}, func() {
//		ui.Button(c, "Create project")
//	}).Fill()
//
// The media is the icon, bare or Boxed in a muted rounded square; the title is
// medium weight and the description is muted and wraps. The actions closure
// declares page elements, so a button there has its own identity and is read
// where it is declared. The state is centered in the room it is given: give it
// Fill, or a size.
//
// It is a separate package so an application that does not import it pays
// nothing for it, and it depends only on the public gfx and ui packages.
package empty

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// maxWidth is the widest the words go, so a long description wraps into a
// readable column in a wide window.
const maxWidth = 384

// Spec declares an empty state.
type Spec struct {
	// Icon is the media above the title. A zero icon leaves it out.
	Icon gfx.Icon
	// Boxed draws the icon in a muted rounded square instead of bare.
	Boxed bool
	// Title says what is missing; Description says what to do about it. Either may
	// be empty.
	Title, Description string
}

// View declares the empty state at c, with actions, when not nil, declaring what
// the user can do under the words. Declare the view on its own, not inside a Memo.
func View(c ui.C, spec Spec, actions func()) ui.Box {
	theme := c.Theme()
	metrics := theme.Metrics
	root := ui.Column(c).Center().Gap(metrics.GapItem).Pad(metrics.PadSurface * 2)
	root.Children(func() {
		if spec.Icon.ViewBox > 0 {
			if spec.Boxed {
				side := metrics.ControlHeight + metrics.GapItem
				ui.Column(c).Size(side, side).Center().Radius(metrics.RadiusSurface).Background(theme.Muted).Children(func() {
					ui.Icon(c, spec.Icon).Size(24, 24)
				})
			} else {
				ui.Icon(c, spec.Icon).Size(32, 32).Muted()
			}
		}
		if spec.Title != "" {
			ui.Text(c, spec.Title).FontSize(metrics.FontSizeM + 5).Weight(gfx.WeightMedium).TextCenter().MaxWidth(maxWidth)
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
