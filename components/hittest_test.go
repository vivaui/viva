package components

import "github.com/vivaui/viva/gfx"

// hitText is the text geometry of a hit a test just read from canvas.
func hitText(canvas *gfx.Canvas, hit *gfx.Hit) gfx.HitText {
	text, _ := canvas.HitText(hit)
	return text
}

// hitTextAt is the text geometry of the hit at index in canvas.Hits.
func hitTextAt(canvas *gfx.Canvas, index int) gfx.HitText {
	hits := canvas.Hits()
	if index < 0 || index >= len(hits) {
		return gfx.HitText{}
	}
	return hitText(canvas, &hits[index])
}
