package ui

import "github.com/vivaui/viva/gfx"

// hitText is the text geometry of a hit a test just read from canvas.
func hitText(Surface *gfx.Canvas, hit *gfx.Hit) gfx.HitText {
	text, _ := Surface.HitText(hit)
	return text
}

// hitTextAt is the text geometry of the hit at index in canvas.Hits.
func hitTextAt(Surface *gfx.Canvas, index int) gfx.HitText {
	hits := Surface.Hits()
	if index < 0 || index >= len(hits) {
		return gfx.HitText{}
	}
	return hitText(Surface, &hits[index])
}
