package componentdemo

import "github.com/vivaui/viva/gfx"

// The demos draw a few Lucide-style icons of their own; Viva ships no icon set.

func lucide(commands ...gfx.PathCommand) gfx.Icon {
	return gfx.Icon{ViewBox: 24, Stroke: 2, Cap: gfx.CapRound, Join: gfx.JoinRound, Commands: commands}
}

func circle(cx, cy, r float32) []gfx.PathCommand {
	const k = 0.5523
	return []gfx.PathCommand{
		gfx.MoveTo(cx+r, cy),
		gfx.CubicTo(cx+r, cy+r*k, cx+r*k, cy+r, cx, cy+r),
		gfx.CubicTo(cx-r*k, cy+r, cx-r, cy+r*k, cx-r, cy),
		gfx.CubicTo(cx-r, cy-r*k, cx-r*k, cy-r, cx, cy-r),
		gfx.CubicTo(cx+r*k, cy-r, cx+r, cy-r*k, cx+r, cy),
		gfx.ClosePath(),
	}
}

var (
	// infoIcon is a circled "i".
	infoIcon = lucide(append(circle(12, 12, 10),
		gfx.MoveTo(12, 16), gfx.LineTo(12, 12), gfx.MoveTo(12, 8), gfx.LineTo(12.01, 8))...)
	// alertIcon is a circled "!".
	alertIcon = lucide(append(circle(12, 12, 10),
		gfx.MoveTo(12, 8), gfx.LineTo(12, 12), gfx.MoveTo(12, 16), gfx.LineTo(12.01, 16))...)
	// folderIcon is an outlined folder.
	folderIcon = lucide(
		gfx.MoveTo(4, 20), gfx.LineTo(20, 20), gfx.CubicTo(21.1, 20, 22, 19.1, 22, 18), gfx.LineTo(22, 8),
		gfx.CubicTo(22, 6.9, 21.1, 6, 20, 6), gfx.LineTo(12.1, 6), gfx.LineTo(10.5, 3.9),
		gfx.CubicTo(10.1, 3.4, 9.6, 3.1, 9, 3.1), gfx.LineTo(4, 3.1),
		gfx.CubicTo(2.9, 3.1, 2, 4, 2, 5.1), gfx.LineTo(2, 18), gfx.CubicTo(2, 19.1, 2.9, 20, 4, 20), gfx.ClosePath())
)

// moreIcon is three dots.
var moreIcon = lucide(
	gfx.MoveTo(5, 12), gfx.LineTo(5.01, 12), gfx.MoveTo(12, 12), gfx.LineTo(12.01, 12), gfx.MoveTo(19, 12), gfx.LineTo(19.01, 12))
