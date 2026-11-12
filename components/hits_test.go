package components

import "github.com/vivaui/viva/gfx"

// nonBlockHits drops the inert surface block that a popup paints inside its
// dismiss backdrop, so a test can talk about its controls and rows.
func nonBlockHits(hits []gfx.Hit) []gfx.Hit {
	out := make([]gfx.Hit, 0, len(hits))
	for _, hit := range hits {
		if !hit.Block && !hit.Container {
			out = append(out, hit)
		}
	}
	return out
}
