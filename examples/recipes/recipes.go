// Package recipes is a set of small, finished components written the way a Viva
// page is written: one Box chain per element, colors and sizes taken from the
// theme, states declared as looks. Copy one and change it.
//
// Each recipe is a function of the page context it declares into and the few
// values it shows; it returns what the caller needs to react (a Box to read
// Clicked from). None keeps state, none allocates once the page is warm, and
// every one is checked in recipes_test.go against both themes, so a recipe that
// stops compiling or stops working fails there before it misleads.
//
// The rules they follow, which a new component should too:
//
//   - Colors come from the theme (theme.Card, theme.MutedForeground, ...) so the
//     component is right in light and dark; the only literal colors are the
//     accents a gradient is made of.
//   - A length is a multiple of 4, the way Tailwind's spacing scale is, and a
//     radius is theme.Metrics.RadiusControl or RadiusSurface, or half the size of
//     a circle.
//   - Anything that reacts has an identity (Hit) and says how in Hover, Pressed
//     and Focus looks, with Transition to fade between them.
//   - What appears or changes inside a row follows the row (StateFrom) instead of
//     reading canvas.Hovered by hand.
package recipes

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// ring is the color of a focus ring: the theme's ring color at the strength
// shadcn uses for ring-ring/50.
func ring(theme gfx.Theme) gfx.Color { return theme.Ring.WithAlpha(0.5) }

// Interactive makes a Box a pointer and keyboard target whose look under the
// pointer, pressed and focused is the usual one: a filled hover, a darker press
// and a 3 pixel focus ring, faded over the theme's motion. It is the base of the
// rows and buttons below; call it after the size and shape, before Children.
// The caller gives the box its Hit identity.
func Interactive(box ui.Box, theme gfx.Theme) ui.Box {
	return box.Transition().
		Hover(ui.Look{}.Background(theme.Hover)).
		Pressed(ui.Look{}.Background(theme.Muted)).
		Focus(ui.Look{}.Ring(3, ring(theme)))
}

// SettingsRow is a title with a muted description under it and a switch at the
// end, the row of a settings page. It returns the switch, so the caller reads
// Clicked from it.
func SettingsRow(c ui.C, theme gfx.Theme, title, description string, on *bool) ui.Box {
	var toggle ui.Box
	ui.Row(c).Gap(16).PadY(12).Items(ui.Center).Children(func() {
		ui.Column(c).Grow(1).Gap(2).Children(func() {
			ui.Text(c, title).Weight(gfx.WeightMedium)
			ui.Paragraph(c, description, 2).Muted().FontSize(theme.Metrics.FontSizeS)
		})
		toggle = ui.Switch(c, on, title)
	})
	return toggle
}

// ListRow is a clickable row of a list: an icon, a name, a muted detail, and a
// pencil that appears only while the row is hovered or focused. selected shades
// it with the accent. It returns the row, so the caller reads Clicked from it.
func ListRow(c ui.C, theme gfx.Theme, icon, edit gfx.Icon, name, detail string, selected bool) ui.Box {
	row := ui.Row(c).Gap(12).Pad(8).PadX(12).Items(ui.Center).Radius(theme.Metrics.RadiusControl).
		Hit(ui.ID(c), name)
	if selected {
		row.Background(theme.Accent)
	}
	row = Interactive(row, theme)
	return row.Children(func() {
		ui.Icon(c, icon).Size(16, 16).Color(theme.MutedForeground)
		ui.Text(c, name).Grow(1).Ellipsis()
		ui.Text(c, detail).Muted().FontSize(theme.Metrics.FontSizeS)
		ui.Icon(c, edit).Size(16, 16).Opacity(0).StateFrom(row).
			Hover(ui.Look{}.Opacity(1)).Focus(ui.Look{}.Opacity(1))
	})
}

// Avatar is a circle with initials on a tinted fill, and a presence dot at its
// bottom right corner when online is true. The dot is Absolute, so it takes no
// room, and a ring of the page color keeps it clear of the avatar.
func Avatar(c ui.C, theme gfx.Theme, size float32, initials string, online bool) ui.Box {
	avatar := ui.Stack(c).Size(size, size).Shrink(0)
	return avatar.Children(func() {
		ui.Row(c).Fill().Radius(size / 2).Background(theme.Muted).Center().Children(func() {
			ui.Text(c, initials).Weight(gfx.WeightMedium).FontSize(size * 0.4).Color(theme.MutedForeground)
		})
		if online {
			dot := size * 0.3
			ui.Row(c).Absolute().Right(0).Bottom(0).Size(dot, dot).Radius(dot/2).
				Background(theme.Success).Border(2, theme.Background)
		}
	})
}

// Tone picks the colors of a Badge from the theme.
type Tone uint8

const (
	ToneNeutral Tone = iota
	ToneAccent
	ToneDanger
)

// Badge is a small pill of text: neutral for a count or tag, accent for something
// selected, danger for an error.
func Badge(c ui.C, theme gfx.Theme, tone Tone, text string) ui.Box {
	background, foreground := theme.Secondary, theme.SecondaryForeground
	switch tone {
	case ToneAccent:
		background, foreground = theme.Primary, theme.PrimaryForeground
	case ToneDanger:
		background, foreground = theme.Danger.WithAlpha(0.12), theme.Danger
	}
	return ui.Text(c, text).FontSize(theme.Metrics.FontSizeS).Weight(gfx.WeightMedium).Color(foreground).
		Background(background).PadX(8).PadY(2).Radius(gfx.RadiusFull).Shrink(0)
}

// StatCard is a figure with a muted caption and a gradient accent bar on top, the
// tile of a dashboard. The accent runs between the two colors given.
func StatCard(c ui.C, theme gfx.Theme, figure, caption string, from, to gfx.Color) ui.Box {
	card := ui.Column(c).Pad(16).Gap(4).Radius(theme.Metrics.RadiusSurface).
		Background(theme.Card).Border(1, theme.Border).Shadow(gfx.ShadowXS)
	return card.Children(func() {
		ui.Row(c).Height(4).Width(32).Radius(2).Gradient(ui.ToRight, from, to).MarginBottom(8)
		ui.Text(c, figure).FontSize(28).Weight(gfx.WeightSemibold)
		ui.Text(c, caption).Muted().FontSize(theme.Metrics.FontSizeS)
	})
}

// Hero is a banner with a diagonal gradient behind a title, a line of text and a
// call to action, in white over the gradient. It returns the button.
func Hero(c ui.C, theme gfx.Theme, title, text, action string) ui.Box {
	white := gfx.RGB(255, 255, 255)
	var button ui.Box
	ui.Column(c).Pad(24).Gap(8).Radius(theme.Metrics.RadiusSurface).Shadow(gfx.ShadowMD).
		GradientStops(ui.ToBottomRight,
			gfx.Stop(0, gfx.RGB(79, 70, 229)), gfx.Stop(0.6, gfx.RGB(147, 51, 234)), gfx.Stop(1, gfx.RGB(219, 39, 119))).
		Children(func() {
			ui.Text(c, title).FontSize(22).Weight(gfx.WeightSemibold).Color(white)
			ui.Paragraph(c, text, 3).Color(white.WithAlpha(0.85))
			button = ui.Button(c, action).Secondary().Self(ui.Start).MarginTop(8)
		})
	return button
}

// IconButton is a square, quiet button for a toolbar: the glyph is muted until
// the pointer is on the button, then takes the foreground color. Disabled dims it
// and removes its looks. It returns the button.
func IconButton(c ui.C, theme gfx.Theme, icon gfx.Icon, label string, disabled bool) ui.Box {
	size := theme.Metrics.ControlHeight
	button := ui.Row(c).Size(size, size).Shrink(0).Center().Radius(theme.Metrics.RadiusControl)
	if disabled {
		return button.Opacity(0.5).HitWith(ui.ID(c), label, gfx.HitOptions{Disabled: true}).Children(func() {
			ui.Icon(c, icon).Size(16, 16).Color(theme.MutedForeground)
		})
	}
	button = Interactive(button.Hit(ui.ID(c), label), theme)
	return button.Children(func() {
		ui.Icon(c, icon).Size(16, 16).Color(theme.MutedForeground).StateFrom(button).
			Hover(ui.Look{}.Color(theme.Foreground))
	})
}

// EmptyState is what a list shows when it has nothing: a glyph in a tinted
// circle, a title, a line of explanation and one action, centered in the room
// it is given. It returns the button.
func EmptyState(c ui.C, theme gfx.Theme, icon gfx.Icon, title, text, action string) ui.Box {
	var button ui.Box
	ui.Column(c).Fill().Center().Gap(8).Children(func() {
		ui.Row(c).Size(48, 48).Radius(24).Background(theme.Muted).Center().Children(func() {
			ui.Icon(c, icon).Size(20, 20).Color(theme.MutedForeground)
		})
		ui.Text(c, title).Weight(gfx.WeightMedium).FontSize(theme.Metrics.FontSizeL)
		ui.Paragraph(c, text, 3).Muted().TextCenter().MaxWidth(320)
		button = ui.Button(c, action).Outline().MarginTop(8)
	})
	return button
}

// Section is a titled group whose content is dimmed and inert-looking when it is
// switched off: the opacity covers everything in it, controls included. The
// content declares its elements into the same page.
func Section(c ui.C, theme gfx.Theme, title string, enabled bool, content func()) ui.Box {
	section := ui.Column(c).Gap(8)
	if !enabled {
		section.Opacity(0.5)
	}
	return section.Children(func() {
		ui.Text(c, title).Muted().FontSize(theme.Metrics.FontSizeS).Weight(gfx.WeightMedium)
		ui.Column(c).Radius(theme.Metrics.RadiusSurface).Background(theme.Card).Border(1, theme.Border).PadX(16).Children(content)
	})
}
