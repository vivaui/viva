package datepicker

import "github.com/vivaui/viva/gfx"

// The picker's glyphs are Lucide's icon geometry (<https://lucide.dev>, ISC
// License), compiled from its SVG sources with the repository's `viva icons`
// compiler and kept unexported so the package's API stays the two components.
// Sources: calendar, chevron-left, chevron-right, chevrons-left,
// chevrons-right and x.
var (
	calendarIcon = gfx.Icon{
		ViewBox: 24.0,
		Stroke:  2.0,
		Cap:     gfx.CapRound,
		Join:    gfx.JoinRound,
		Commands: []gfx.PathCommand{
			gfx.MoveTo(8.0, 2.0),
			gfx.LineTo(8.0, 5.0),
			gfx.MoveTo(16.0, 2.0),
			gfx.LineTo(16.0, 5.0),
			gfx.MoveTo(19.0, 3.0),
			gfx.CubicTo(20.1046, 3.0, 21.0, 3.89543, 21.0, 5.0),
			gfx.LineTo(21.0, 19.0),
			gfx.CubicTo(21.0, 20.1046, 20.1046, 21.0, 19.0, 21.0),
			gfx.LineTo(5.0, 21.0),
			gfx.CubicTo(3.89543, 21.0, 3.0, 20.1046, 3.0, 19.0),
			gfx.LineTo(3.0, 5.0),
			gfx.CubicTo(3.0, 3.89543, 3.89543, 3.0, 5.0, 3.0),
			gfx.ClosePath(),
			gfx.MoveTo(3.0, 9.0),
			gfx.LineTo(21.0, 9.0),
		},
	}
	chevronLeftIcon = gfx.Icon{
		ViewBox: 24.0,
		Stroke:  2.0,
		Cap:     gfx.CapRound,
		Join:    gfx.JoinRound,
		Commands: []gfx.PathCommand{
			gfx.MoveTo(15.0, 18.0),
			gfx.LineTo(9.0, 12.0),
			gfx.LineTo(15.0, 6.0),
		},
	}
	chevronRightIcon = gfx.Icon{
		ViewBox: 24.0,
		Stroke:  2.0,
		Cap:     gfx.CapRound,
		Join:    gfx.JoinRound,
		Commands: []gfx.PathCommand{
			gfx.MoveTo(9.0, 18.0),
			gfx.LineTo(15.0, 12.0),
			gfx.LineTo(9.0, 6.0),
		},
	}
	chevronsLeftIcon = gfx.Icon{
		ViewBox: 24.0,
		Stroke:  2.0,
		Cap:     gfx.CapRound,
		Join:    gfx.JoinRound,
		Commands: []gfx.PathCommand{
			gfx.MoveTo(11.0, 17.0),
			gfx.LineTo(6.0, 12.0),
			gfx.LineTo(11.0, 7.0),
			gfx.MoveTo(18.0, 17.0),
			gfx.LineTo(13.0, 12.0),
			gfx.LineTo(18.0, 7.0),
		},
	}
	chevronsRightIcon = gfx.Icon{
		ViewBox: 24.0,
		Stroke:  2.0,
		Cap:     gfx.CapRound,
		Join:    gfx.JoinRound,
		Commands: []gfx.PathCommand{
			gfx.MoveTo(6.0, 17.0),
			gfx.LineTo(11.0, 12.0),
			gfx.LineTo(6.0, 7.0),
			gfx.MoveTo(13.0, 17.0),
			gfx.LineTo(18.0, 12.0),
			gfx.LineTo(13.0, 7.0),
		},
	}
	closeIcon = gfx.Icon{
		ViewBox: 24.0,
		Stroke:  2.0,
		Cap:     gfx.CapRound,
		Join:    gfx.JoinRound,
		Commands: []gfx.PathCommand{
			gfx.MoveTo(18.0, 6.0),
			gfx.LineTo(6.0, 18.0),
			gfx.MoveTo(6.0, 6.0),
			gfx.LineTo(18.0, 18.0),
		},
	}
)
