package app

import "github.com/vivaui/viva/gfx"

// onClick makes config's paint report every click the way an application does:
// by asking Clicked about the controls it declared, in the event pass. sink runs
// once per click, in that pass, with the identity of the control clicked. Set
// config.Paint first; onClick wraps it.
func onClick(config *Config, sink func(id gfx.ID)) {
	paint := config.Paint
	config.Paint = func(canvas *gfx.Canvas, size gfx.Size) {
		if paint != nil {
			paint(canvas, size)
		}
		for _, hit := range canvas.Hits() {
			if canvas.Clicked(hit.ID) {
				sink(hit.ID)
			}
		}
	}
}
