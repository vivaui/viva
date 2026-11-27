package windows

import (
	"unicode/utf16"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/grapheme"
)

type fittedCluster struct {
	end               uint32
	leading, trailing float32
}

// appendFittedClusterCarets maps DirectWrite clusters to editor graphemes. A
// ligature can cover several graphemes, and a grapheme can cover several native
// clusters; only real grapheme boundaries become caret stops.
func appendFittedClusterCarets(dst []gfx.FittedMonoCaret, value string, clusters []fittedCluster, origin, scale float32) ([]gfx.FittedMonoCaret, bool) {
	if value == "" || len(clusters) == 0 {
		return dst, false
	}
	base := len(dst)
	var units uint32
	var scratch [64]uint32
	boundaries := scratch[:0]
	for off := 0; off < len(value); {
		boundaries = append(boundaries, units)
		dst = append(dst, gfx.FittedMonoCaret{Offset: off})
		next := grapheme.Next(value, off)
		for _, r := range value[off:next] {
			units += uint32(utf16.RuneLen(r))
		}
		off = next
	}
	if clusters[len(clusters)-1].end != units {
		return dst[:base], false
	}
	boundaries = append(boundaries, units)
	dst = append(dst, gfx.FittedMonoCaret{Offset: len(value)})
	carets := dst[base:]
	var start uint32
	at := 0
	for _, cluster := range clusters {
		if cluster.end <= start || cluster.end > units {
			return dst[:base], false
		}
		lead, trail := (cluster.leading-origin)*scale, (cluster.trailing-origin)*scale
		if boundaries[at] == start {
			carets[at].Leading = lead
			at++
		}
		end := at
		for boundaries[end] < cluster.end {
			end++
		}
		// DirectWrite reports a single visual span for a ligature. Give each
		// grapheme inside that span a share, in the cluster's own direction.
		for i := at; i < end; i++ {
			x := lead + (trail-lead)*float32(i-at+1)/float32(end-at+1)
			carets[i].Leading, carets[i].Trailing = x, x
		}
		if boundaries[end] == cluster.end {
			carets[end].Trailing = trail
		}
		at, start = end, cluster.end
	}
	carets[0].Trailing = carets[0].Leading
	carets[at].Leading = carets[at].Trailing
	return dst, true
}
