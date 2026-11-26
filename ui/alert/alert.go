// Package alert is the opt-in page control for a notice that stays in the page:
// a status icon, a title, a description, and optionally a button for what to do
// about it and a close button. A page keeps an Alert in its model, because it
// remembers whether the user closed it:
//
//	m.notice = alert.New()
//	// in the page:
//	m.notice.View(c, alert.Spec{
//		Status:      ui.StatusWarning,
//		Title:       "Host key changed",
//		Description: "The key of build-01 differs from the one you trusted.",
//		Action:      "Review",
//		Closable:    true,
//	}).FillWidth()
//	if m.notice.ActionClicked() {
//		// the user pressed Review
//	}
//
// Status colors the icon and the rim: ui.StatusDefault is an information notice
// in the neutral look, and success, warning and danger are tinted. Icon gives a
// notice an icon of its own in place of the status icon. Banner makes the notice
// a full-width strip with square corners and no side rims, for the top of a page.
//
// A closed alert stays closed until Reopen; View of a closed alert declares an
// empty box, so a page that wants no trace of it checks Closed first.
//
// It is a separate package so an application that does not import it pays
// nothing for it. Keep an Alert in the model: its state and handlers are made
// once, so a warm paint allocates nothing. Do not copy an Alert after it has been
// declared.
package alert

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// circle is the outline of a circle of radius 10 around (12, 12) as four cubic
// curves, the frame of the status icons.
var circle = []gfx.PathCommand{
	gfx.MoveTo(22, 12), gfx.CubicTo(22, 17.52, 17.52, 22, 12, 22), gfx.CubicTo(6.48, 22, 2, 17.52, 2, 12),
	gfx.CubicTo(2, 6.48, 6.48, 2, 12, 2), gfx.CubicTo(17.52, 2, 22, 6.48, 22, 12),
}

func icon(extra ...gfx.PathCommand) gfx.Icon {
	commands := append(append([]gfx.PathCommand(nil), circle...), extra...)
	return gfx.Icon{ViewBox: 24, Stroke: 2, Cap: gfx.CapRound, Join: gfx.JoinRound, Commands: commands}
}

var (
	infoIcon    = icon(gfx.MoveTo(12, 16), gfx.LineTo(12, 12), gfx.MoveTo(12, 8), gfx.LineTo(12.01, 8))
	successIcon = icon(gfx.MoveTo(9, 12), gfx.LineTo(11, 14), gfx.LineTo(15, 10))
	dangerIcon  = icon(gfx.MoveTo(15, 9), gfx.LineTo(9, 15), gfx.MoveTo(9, 9), gfx.LineTo(15, 15))
	warningIcon = gfx.Icon{ViewBox: 24, Stroke: 2, Cap: gfx.CapRound, Join: gfx.JoinRound, Commands: []gfx.PathCommand{
		gfx.MoveTo(10.29, 3.86), gfx.LineTo(1.82, 18), gfx.QuadTo(1, 20, 3, 20), gfx.LineTo(21, 20), gfx.QuadTo(23, 20, 22.18, 18),
		gfx.LineTo(13.71, 3.86), gfx.QuadTo(12, 2, 10.29, 3.86),
		gfx.MoveTo(12, 9), gfx.LineTo(12, 13), gfx.MoveTo(12, 17), gfx.LineTo(12.01, 17),
	}}
	closeIcon = gfx.Icon{ViewBox: 24, Stroke: 2, Cap: gfx.CapRound, Join: gfx.JoinRound, Commands: []gfx.PathCommand{
		gfx.MoveTo(18, 6), gfx.LineTo(6, 18), gfx.MoveTo(6, 6), gfx.LineTo(18, 18),
	}}
)

// statusIcon is the icon a status carries.
func statusIcon(status ui.Status) gfx.Icon {
	switch status {
	case ui.StatusSuccess:
		return successIcon
	case ui.StatusWarning:
		return warningIcon
	case ui.StatusDanger:
		return dangerIcon
	}
	return infoIcon
}

// Spec declares one notice.
type Spec struct {
	// Status is the outcome the notice reports; the neutral information notice by
	// default.
	Status ui.Status
	// Title is the notice's heading; Description is the line or lines under it.
	// Either may be empty.
	Title, Description string
	// Icon replaces the icon of the status. A zero icon keeps it.
	Icon gfx.Icon
	// NoIcon leaves the icon out.
	NoIcon bool
	// Action is the label of a button at the end of the notice; ActionClicked
	// reports a press on it. It is not drawn when empty.
	Action string
	// Closable adds a close button; Closed reports that it was pressed.
	Closable bool
	// Banner makes the notice a full-width strip with square corners and no side
	// rims.
	Banner bool
	// CloseLabel is the accessible name of the close button, "Close" when empty.
	CloseLabel string
}

// Alert is one notice and what the user did to it. Its zero value is an open
// alert; New is the readable way to make one.
type Alert struct {
	closed bool
	acted  bool // the action was pressed in the paint being declared
}

// New returns an open alert.
func New() *Alert { return &Alert{} }

// Closed reports whether the user closed the notice.
func (a *Alert) Closed() bool { return a.closed }

// Reopen shows a closed notice again.
func (a *Alert) Reopen() { a.closed = false }

// ActionClicked reports whether the paint just made delivered a press on the
// action button.
func (a *Alert) ActionClicked() bool { return a.acted }

// tint is color at the given opacity, the fill and the rim of a status.
func tint(color gfx.Color, alpha float32) gfx.Color {
	color.Alpha *= alpha
	return color
}

// View declares the notice at c. The notice is as tall as its text; give it
// FillWidth. Declare the view on its own, not inside a Memo.
func (a *Alert) View(c ui.C, spec Spec) ui.Box {
	if a == nil {
		panic("alert: View needs its Alert")
	}
	a.acted = false
	if a.closed {
		return ui.Spacer(c).Size(0, 0)
	}
	theme := c.Theme()
	metrics := theme.Metrics
	ink := theme.Foreground
	fill, rim := theme.Card, theme.Border
	switch spec.Status {
	case ui.StatusSuccess:
		ink = theme.Success
	case ui.StatusWarning:
		ink = theme.WarningColor()
	case ui.StatusDanger:
		ink = theme.Danger
	}
	if spec.Status != ui.StatusDefault {
		fill, rim = tint(ink, 0.08), tint(ink, 0.4)
	}
	root := ui.Row(c).Items(ui.Start).Gap(metrics.GapItem).Background(fill)
	root = root.Pad(metrics.PadSurface * 0.75)
	if spec.Banner {
		root = root.Radius(0).Border(0, rim)
	} else {
		root = root.Radius(metrics.RadiusSurface).Border(1, rim)
	}
	label := spec.Title
	if label == "" {
		label = spec.Description
	}
	root = root.HitWith(ui.ID(c), label, gfx.HitOptions{Role: gfx.RoleStatus, SkipFocus: true})
	root.Children(func() {
		if !spec.NoIcon {
			glyph := spec.Icon
			if glyph.ViewBox == 0 {
				glyph = statusIcon(spec.Status)
			}
			ui.Icon(c, glyph).Size(16, 16).Color(ink).MarginSides(2, 0, 0, 0).Shrink(0)
		}
		ui.Column(c).Grow(1).Basis(0).Gap(2).Children(func() {
			if spec.Title != "" {
				ui.Text(c, spec.Title).Weight(gfx.WeightMedium)
			}
			if spec.Description != "" {
				ui.Wrap(c, spec.Description).FontSize(metrics.FontSizeS).Muted()
			}
		})
		if spec.Action != "" {
			if ui.Button(c, spec.Action).Outline().Small().Shrink(0).Clicked() {
				a.acted = true
			}
		}
		if spec.Closable {
			name := spec.CloseLabel
			if name == "" {
				name = "Close"
			}
			if ui.Button(c, name).Icon(closeIcon).IconOnly().Ghost().Shrink(0).Clicked() {
				a.closed = true
			}
		}
	})
	return root
}
