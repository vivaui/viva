package ui

import (
	"github.com/vivaui/viva/base"
	"github.com/vivaui/viva/gfx"
)

// BubbleVariant is the surface of a Bubble, as shadcn's content variants.
type BubbleVariant uint8

const (
	// BubblePrimary is the primary surface with the primary foreground.
	BubblePrimary BubbleVariant = iota
	BubbleSecondary
	BubbleMuted
	BubbleTinted
	BubbleOutline
	BubbleGhost
	BubbleDestructive
)

// Bubble declares one message surface: its text wraps at 80% of the row and
// the box is as tall as its lines. Use Self(Start) or Self(End) for alignment;
// the page layer owns the conversation order.
//
//	ui.Bubble(c, "Ready when you are", ui.BubbleMuted).Self(ui.Start)
func Bubble(c C, text string, variant BubbleVariant) Box {
	theme := *c.Theme()
	fill := theme.Primary
	foreground := theme.PrimaryForeground
	border := gfx.Color{}
	bordered := false
	padX, padY := theme.Metrics.PadControlX+1, theme.Metrics.GapItem+1
	switch variant {
	case BubbleSecondary:
		fill, foreground = theme.Secondary, theme.SecondaryForeground
	case BubbleMuted:
		fill, foreground = theme.Muted, theme.Foreground
	case BubbleTinted:
		fill, foreground = theme.Primary, theme.Foreground
		fill.Alpha *= 0.12
	case BubbleOutline:
		fill, foreground, border, bordered = theme.Background, theme.Foreground, theme.Border, true
	case BubbleGhost:
		fill, foreground = gfx.Color{}, theme.Foreground
		padX, padY = 0, 0
	case BubbleDestructive:
		fill, foreground = theme.Danger, theme.Danger
		fill.Alpha *= 0.10
		if theme.Background.Red < 0.5 {
			fill.Alpha *= 2
		}
	}
	bubble := base.Column(c).MaxWidthPercent(80).Radius(theme.Metrics.RadiusSurface + 4)
	if variant != BubbleGhost {
		bubble = bubble.PadSides(padY, padX, padY, padX).Background(fill)
	}
	if bordered {
		bubble = bubble.Border(1, border)
	}
	bubble.Children(func() {
		base.Wrap(c, text).
			FontSize(theme.Metrics.FontSizeM).
			LineHeight(theme.Metrics.FontSizeM * 1.625).
			Color(foreground)
	})
	return bubble
}
