// Package metriccardview is the opt-in page control for a dashboard metric: a
// title, a value and a trend, with an optional description and icon. A page
// declares the visible data with chained options:
//
//	metriccardview.View(c, "Active sessions", metriccardview.New().
//		Title("Active sessions").Value("12,840").
//		Trend("+18.2% from last week").Up()).
//		FillWidth().Height(150)
//
// It composes the page's own primitives, so the card participates in the page
// layout, theme and density. The caller supplies every string, the trend's
// direction and any icon; Content paints additional page content below the
// card's rows, such as a sparkline or a row of links.
//
// It is a separate package so an application that does not import it pays
// nothing for it, and it depends only on the public gfx and ui packages. A
// Metric is a value built with chained options; keep no state in it, build it
// from the model in each paint, and View allocates nothing on a warm paint.
package metriccardview

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// trendKind selects the trend row's color.
type trendKind uint8

const (
	trendNeutral trendKind = iota
	trendUp
	trendDown
)

// Metric is one card's presentation. Its zero value is an empty card; New is
// its readable starting point. Every option returns the value, so a call site
// chains only what it shows.
type Metric struct {
	title       string
	value       string
	trend       string
	description string
	icon        gfx.Icon
	kind        trendKind
	content     func(ui.C)
}

// New returns an empty card.
func New() Metric { return Metric{} }

// Title is the muted caption above the value.
func (m Metric) Title(title string) Metric { m.title = title; return m }

// Value is the card's large numeric reading.
func (m Metric) Value(value string) Metric { m.value = value; return m }

// Trend is the delta line below the value, such as "+18.2% from last week".
func (m Metric) Trend(trend string) Metric { m.trend = trend; return m }

// Up paints the trend in the theme's success color: the metric moved the
// favorable way.
func (m Metric) Up() Metric { m.kind = trendUp; return m }

// Down paints the trend in the theme's danger color: the metric moved the
// unfavorable way.
func (m Metric) Down() Metric { m.kind = trendDown; return m }

// Neutral paints the trend in the muted foreground color. It is the zero
// value's color, so it is only needed to undo Up or Down.
func (m Metric) Neutral() Metric { m.kind = trendNeutral; return m }

// Description is an optional muted line below the trend, such as the period the
// value covers.
func (m Metric) Description(description string) Metric {
	m.description = description
	return m
}

// Icon paints a 16 pixel icon before the trailing edge of the title row. A zero
// icon paints none.
func (m Metric) Icon(icon gfx.Icon) Metric { m.icon = icon; return m }

// Content paints caller content below the card's rows, inside its padding. The
// callback declares page elements, so their identities and handlers are the
// caller's own. It is not painted when nil.
func (m Metric) Content(content func(ui.C)) Metric { m.content = content; return m }

// View declares the card at c. label names the card for assistive technology.
// Give it a size; the card is as tall as its rows and, when the caller gives a
// height, fills it. Declare the view on its own, not inside a Memo.
func View(c ui.C, label string, metric Metric) ui.Box {
	if c == nil {
		panic("metriccardview: View needs a page")
	}
	theme := c.Theme()
	metrics := theme.Metrics
	card := ui.Column(c).Pad(metrics.PadSurface).Gap(metrics.GapItem*0.75).
		Background(theme.Card).Border(1, theme.Border).Radius(metrics.RadiusSurface)
	if label != "" {
		card = card.HitWith(ui.ID(c), label, gfx.HitOptions{Role: gfx.RoleGroup, SkipFocus: true})
	}
	card.Children(func() {
		if metric.title != "" || metric.icon.ViewBox > 0 {
			ui.Row(c).FillWidth().Items(ui.Center).Gap(metrics.GapItem).Children(func() {
				if metric.title != "" {
					ui.Text(c, metric.title).
						FontSize(metrics.FontSizeS).Weight(gfx.WeightMedium).Muted().Grow(1)
				} else {
					ui.Spacer(c).Grow(1)
				}
				if metric.icon.ViewBox > 0 {
					ui.Icon(c, metric.icon).Size(16, 16).Muted()
				}
			})
		}
		if metric.value != "" {
			ui.Text(c, metric.value).
				FontSize(metrics.FontSizeL - 2).Weight(gfx.WeightSemibold)
		}
		if metric.trend != "" {
			trend := ui.Text(c, metric.trend).
				FontSize(metrics.FontSizeS).Weight(gfx.WeightMedium)
			switch metric.kind {
			case trendUp:
				trend.Color(theme.Success)
			case trendDown:
				trend.Color(theme.Danger)
			default:
				trend.Muted()
			}
		}
		if metric.description != "" {
			ui.Text(c, metric.description).FontSize(metrics.FontSizeS).Muted()
		}
		if metric.content != nil {
			metric.content(c)
		}
	})
	return card
}
