// Package card is the opt-in page control for a card: a raised surface that
// groups what belongs together, with an optional header (a title, a description
// and something at its end), a body and an optional footer. A page declares it
// with a Spec and the body:
//
//	card.View(c, card.Spec{
//		Title:       "Native preview",
//		Description: "Keeps scene nodes and repaints only when content changes.",
//		Extra:       func() { ui.Button(c, "Open").Outline().Small() },
//	}, func() {
//		ui.Text(c, "0").FontSize(28).Bold()
//	}).FillWidth()
//
// The body, Extra and Footer are page declarations, so what they hold keeps its
// own identity and handlers. HeaderBorder and FooterBorder draw a rule between a
// section and the body; Size: Small is the compact card; Flat leaves out the
// shadow. The body is the closure after the Spec, as for ui.Popover and ui.Modal.
//
// It is a separate package so an application that does not import it pays
// nothing for it, and it depends only on the public gfx and ui packages.
package card

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// Size is how roomy the card is.
type Size uint8

const (
	// Default is the zero value.
	Default Size = iota
	// Small is the compact card.
	Small
)

// Spec declares one card. Its zero value is a card with no header or footer.
type Spec struct {
	// Title and Description head the card; the header is not drawn when both are
	// empty and there is no Extra.
	Title, Description string
	// Extra declares what sits at the end of the header, such as a button or a
	// menu. It is not called when nil.
	Extra func()
	// Footer declares the card's last section. It is not called when nil.
	Footer func()
	// HeaderBorder draws a rule between the header and the body; FooterBorder one
	// between the body and the footer.
	HeaderBorder, FooterBorder bool
	// Size is how roomy the card is.
	Size Size
	// Flat leaves out the shadow.
	Flat bool
}

// View declares the card at c with body as its content. The card is as tall as
// its sections; give it FillWidth or a Width. Declare the view on its own, not
// inside a Memo.
func View(c ui.C, spec Spec, body func()) ui.Box {
	theme := c.Theme()
	metrics := theme.Metrics
	pad, gap := float32(24), float32(8)
	if spec.Size == Small {
		pad, gap = 16, 6
	}
	rim := theme.Foreground
	rim.Alpha *= 0.10
	box := ui.Column(c).Pad(pad).Gap(gap).Background(theme.Card).Border(1, rim).Radius(metrics.RadiusSurface + 4)
	if !spec.Flat {
		box = box.Shadow(gfx.ShadowXS)
	}
	box.Children(func() {
		if spec.Title != "" || spec.Description != "" || spec.Extra != nil {
			header(c, theme, spec)
			if spec.HeaderBorder {
				ui.Divider(c)
			}
		}
		if body != nil {
			body()
		}
		if spec.Footer != nil {
			if spec.FooterBorder {
				ui.Divider(c)
			}
			spec.Footer()
		}
	})
	return box
}

// header declares the title and description with the Extra at their end.
func header(c ui.C, theme *gfx.Theme, spec Spec) {
	metrics := theme.Metrics
	ui.Row(c).FillWidth().Items(ui.Start).Gap(metrics.GapItem).Children(func() {
		ui.Column(c).Grow(1).Basis(0).Gap(2).Children(func() {
			if spec.Title != "" {
				ui.Text(c, spec.Title).FontSize(metrics.FontSizeM + 2).Weight(gfx.WeightSemibold)
			}
			if spec.Description != "" {
				ui.Wrap(c, spec.Description).FontSize(metrics.FontSizeS).Muted()
			}
		})
		if spec.Extra != nil {
			spec.Extra()
		}
	})
}
