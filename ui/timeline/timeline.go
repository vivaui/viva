// Package timeline is the opt-in page control for a timeline: events in the
// order they happened, joined by a line down the side. A page keeps the events
// in its model and declares the line with a Spec:
//
//	timeline.View(c, "History", timeline.Spec{Items: m.events, Pending: "Waiting for review"})
//	// m.events is a []timeline.Item kept in the model:
//	// {Title: "Connected", Description: "build-01 accepted the key", Label: "09:41"}
//
// An item has a Title, an optional Description under it and an optional Label,
// the time or date of the event, shown muted beside it. Its Tone colors the dot
// and the line after it: Default is the theme's primary color, Success and
// Danger say how the event ended and Muted is a quiet one. Pending, when set,
// ends the line with an open dot for what comes next.
//
// With LabelWidth the labels line up in a column before the line, as a log
// does; without it a label sits under its title. It composes the page's own
// primitives, so it takes the page's theme and density.
//
// It is a separate package so an application that does not import it pays
// nothing for it, and it depends only on the public gfx and ui packages.
package timeline

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// Tone is the color of an event's dot and of the line after it.
type Tone uint8

const (
	// Default is the theme's primary color: the zero value.
	Default Tone = iota
	// Success is the event ended well.
	Success
	// Danger is the event failed.
	Danger
	// Muted is a quiet event.
	Muted
)

// Item is one event.
type Item struct {
	// Title names the event.
	Title string
	// Description is an optional line under the title.
	Description string
	// Label is the time or date of the event, shown muted. It is not drawn when
	// empty.
	Label string
	// Tone colors the dot.
	Tone Tone
}

// Spec declares the timeline.
type Spec struct {
	// Items are the events, in order. Keep the slice in the model: a slice made
	// in each paint allocates in each paint.
	Items []Item
	// Pending ends the line with an open dot and these words, for what is still to
	// come. It is not drawn when empty.
	Pending string
	// LabelWidth puts the labels in a column of this width before the line, so
	// they line up; zero puts each under its title.
	LabelWidth float32
}

// View declares the timeline at c. label names it for assistive technology. The
// timeline is as tall as its events; give it FillWidth. Declare the view on its
// own, not inside a Memo.
func View(c ui.C, label string, spec Spec) ui.Box {
	if c == nil {
		panic("timeline: View needs a page")
	}
	theme := c.Theme()
	look := look{theme: theme, gap: theme.Metrics.GapItem, size: theme.Metrics.FontSizeM, detail: theme.Metrics.FontSizeS}
	root := ui.Column(c).HitWith(ui.ID(c), label, gfx.HitOptions{Role: gfx.RoleGroup, SkipFocus: true})
	root.Children(func() {
		for i := range spec.Items {
			look.event(c, spec, spec.Items[i], i < len(spec.Items)-1 || spec.Pending != "")
		}
		if spec.Pending != "" {
			look.pending(c, spec)
		}
	})
	return root
}

// look is what a timeline paints with: the page's theme.
type look struct {
	theme  *gfx.Theme
	gap    float32
	size   float32
	detail float32
}

const dotSize = 10

// ink is the color of a tone.
func (l look) ink(tone Tone) gfx.Color {
	switch tone {
	case Success:
		return l.theme.Success
	case Danger:
		return l.theme.Danger
	case Muted:
		return l.theme.MutedForeground
	}
	return l.theme.Primary
}

// event declares one event: its label column, its dot and line, and its words.
func (l look) event(c ui.C, spec Spec, item Item, joined bool) {
	row := ui.Row(c).Items(ui.Stretch).Gap(l.gap)
	row.Children(func() {
		if spec.LabelWidth > 0 {
			ui.Row(c).Width(spec.LabelWidth).Shrink(0).Justify(ui.End).Items(ui.Start).Children(func() {
				ui.Text(c, item.Label).FontSize(l.detail).Muted().LineHeight(l.size * 1.5)
			})
		}
		l.rail(c, l.ink(item.Tone), true, joined)
		column := ui.Column(c).Gap(2).Grow(1).Basis(0)
		if joined {
			column = column.PadBottom(l.gap * 2)
		}
		column.Children(func() {
			ui.Text(c, item.Title).FontSize(l.size).LineHeight(l.size * 1.5)
			if item.Description != "" {
				ui.Text(c, item.Description).FontSize(l.detail).Muted()
			}
			if item.Label != "" && spec.LabelWidth <= 0 {
				ui.Text(c, item.Label).FontSize(l.detail).Muted()
			}
		})
	})
}

// pending declares the open dot that ends a timeline and its words.
func (l look) pending(c ui.C, spec Spec) {
	row := ui.Row(c).Items(ui.Stretch).Gap(l.gap)
	row.Children(func() {
		if spec.LabelWidth > 0 {
			ui.Spacer(c).Width(spec.LabelWidth).Grow(0).Shrink(0)
		}
		l.rail(c, l.theme.Border, false, false)
		ui.Text(c, spec.Pending).FontSize(l.size).Muted().LineHeight(l.size * 1.5)
	})
}

// rail declares a dot and, when joined, the line that runs down from it. A filled
// dot is solid; an open one is a ring.
func (l look) rail(c ui.C, ink gfx.Color, filled, joined bool) {
	rail := ui.Column(c).Items(ui.Center).Shrink(0).Width(dotSize)
	rail.Children(func() {
		slot := ui.Row(c).Size(dotSize, l.size*1.5).Shrink(0).Center()
		slot.Children(func() {
			dot := ui.Spacer(c).Size(dotSize, dotSize).Grow(0).Shrink(0).Radius(dotSize / 2)
			if filled {
				dot.Background(ink)
			} else {
				dot.Background(l.theme.Background).Border(2, ink)
			}
		})
		if joined {
			ui.Spacer(c).Width(1).Grow(1).MinHeight(8).Background(l.theme.Border)
		}
	})
}
