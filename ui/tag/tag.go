// Package tag is the opt-in page control for a tag: a short label in a small
// rounded box that classifies something, and optionally can be ticked on and off
// or removed. It keeps no state of its own: a page passes what it knows, as for a
// button, and reads what the user did from the result:
//
//	for i, host := range m.tags {
//		result := tag.View(c.Key(host.Name), host.Name, tag.Spec{Closable: true})
//		if result.Closed {
//			m.removeTag(i)
//		}
//	}
//
// A checkable tag is a toggle: the page gives it Checked and flips it when the
// result says Toggled.
//
//	result := tag.View(c, "production", tag.Spec{Checkable: true, Checked: m.production})
//	if result.Toggled {
//		m.production = !m.production
//	}
//
// Status colors the tag in the shared ui.Status vocabulary, Outlined draws only a
// rim, and Size is Small, Default or Large. Name each tag of a loop with c.Key before
// View, by something that stays with the tag when others are removed, not by its
// index.
//
// It is a separate package so an application that does not import it pays
// nothing for it. A tag allocates nothing in a warm paint.
package tag

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// Size is how large the tag is.
type Size uint8

const (
	// Default is the zero value.
	Default Size = iota
	// Small is the compact tag.
	Small
	// Large is the roomy tag.
	Large
)

var closeIcon = gfx.Icon{ViewBox: 24, Stroke: 2.5, Cap: gfx.CapRound, Join: gfx.JoinRound, Commands: []gfx.PathCommand{
	gfx.MoveTo(18, 6), gfx.LineTo(6, 18), gfx.MoveTo(6, 6), gfx.LineTo(18, 18),
}}

// Spec declares one tag.
type Spec struct {
	// Status colors the tag; ui.StatusDefault is the neutral tag.
	Status ui.Status
	// Icon is drawn before the label. A zero icon draws none.
	Icon gfx.Icon
	// Outlined draws only a rim, with no fill.
	Outlined bool
	// Size is how large the tag is.
	Size Size
	// Closable adds a close button; Closed reports a press on it.
	Closable bool
	// CloseLabel is the accessible name of the close button, "Remove" when empty.
	CloseLabel string
	// Checkable makes the tag a toggle that is on while Checked is set; Toggled
	// reports a press on it. A checked tag takes the theme's primary color.
	Checkable, Checked bool
}

// Result is what the user did to the tag in the paint just made.
type Result struct {
	// Box is the tag, for the page to place: Grow, Margin and the rest.
	Box ui.Box
	// Closed is true in the paint that delivers a press on the close button.
	Closed bool
	// Toggled is true in the paint that delivers a press on a checkable tag.
	Toggled bool
}

func tint(color gfx.Color, alpha float32) gfx.Color {
	color.Alpha *= alpha
	return color
}

// View declares the tag at c. Declare it on its own, not inside a Memo.
func View(c ui.C, label string, spec Spec) Result {
	theme := c.Theme()
	metrics := theme.Metrics
	size, padX, padY := metrics.FontSizeS, float32(8), float32(2)
	switch spec.Size {
	case Small:
		size, padX, padY = metrics.FontSizeS-1, 6, 1
	case Large:
		size, padX, padY = metrics.FontSizeM, 10, 4
	}
	ink, fill, rim := theme.SecondaryForeground, theme.Secondary, theme.Border
	switch spec.Status {
	case ui.StatusSuccess:
		ink = theme.Success
		fill, rim = tint(ink, 0.12), tint(ink, 0.4)
	case ui.StatusWarning:
		ink = theme.WarningColor()
		fill, rim = tint(ink, 0.12), tint(ink, 0.4)
	case ui.StatusDanger:
		ink = theme.Danger
		fill, rim = tint(ink, 0.12), tint(ink, 0.4)
	}
	if spec.Checkable && spec.Checked {
		ink, fill, rim = theme.PrimaryForeground, theme.Primary, theme.Primary
	}
	if spec.Outlined && !(spec.Checkable && spec.Checked) {
		fill = gfx.Color{}
		if spec.Status == ui.StatusDefault {
			ink = theme.Foreground
		}
	}
	id := ui.ID(c)
	box := ui.Row(c).Items(ui.Center).Gap(4).Shrink(0).PadSides(padY, padX, padY, padX).
		Radius(metrics.RadiusControl).Background(fill).Border(1, rim)
	var result Result
	if spec.Checkable {
		box = box.HitWith(id, label, gfx.HitOptions{Role: gfx.RoleCheckbox, Checked: spec.Checked})
	}
	box.Children(func() {
		if spec.Icon.ViewBox > 0 {
			ui.Icon(c, spec.Icon).Size(size, size).Color(ink)
		}
		ui.Text(c, label).FontSize(size).Weight(gfx.WeightMedium).Color(ink)
		if spec.Closable {
			name := spec.CloseLabel
			if name == "" {
				name = "Remove"
			}
			closeID := ui.ID(c)
			button := ui.Row(c).Size(size+2, size+2).Shrink(0).Center().HitWith(closeID, name, gfx.HitOptions{Role: gfx.RoleButton})
			button.Children(func() { ui.Icon(c, closeIcon).Size(size-1, size-1).Color(ink) })
			result.Closed = button.Clicked()
		}
	})
	result.Box = box
	result.Toggled = spec.Checkable && box.Clicked()
	return result
}
