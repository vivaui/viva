package base_test

import "github.com/vivaui/viva/gfx"

func hitText(Surface *gfx.Canvas, hit *gfx.Hit) gfx.HitText {
	text, _ := Surface.HitText(hit)
	return text
}

func hitTextAt(Surface *gfx.Canvas, index int) gfx.HitText {
	hits := Surface.Hits()
	if index < 0 || index >= len(hits) {
		return gfx.HitText{}
	}
	return hitText(Surface, &hits[index])
}
