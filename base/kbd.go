package base

import "github.com/vivaui/viva/gfx"

// Kbd adds the keycaps of a shortcut side by side, one per key, as shadcn's KbdGroup
// does: each a hairline-bordered chip on the muted fill, as wide as its text and
// at least as wide as it is tall. Keys come from a chord's Keycaps.
//
//	ui.Kbd(c, "⌘", "W")
func Kbd(c C, keys ...string) Box {
	th := c.f.Palette
	m := th.Metrics
	height := m.FontSizeS + 9
	radius := m.RadiusControl * 0.75
	return Row(c).Gap(4).Items(Center).Shrink(0).Children(func() {
		for _, key := range keys {
			Text(c, key).FontSize(m.FontSizeS).Weight(gfx.WeightMedium).Color(th.MutedForeground).
				Background(th.Muted).Border(1, th.Border).Radius(radius).
				Height(height).MinWidth(height).PadX(4).TextCenter().Shrink(0)
		}
	})
}
