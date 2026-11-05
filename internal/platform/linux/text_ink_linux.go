package linux

import (
	"math"
	"strings"
	"unicode/utf8"

	"github.com/vivaui/viva/internal/platform"
)

// inkKey names a glyph run as drawRuns draws it in a cell span: what decides how far its ink goes
// past the cells.
type inkKey struct {
	family string
	size   float32
	weight float32
	span   float32 // the width of the cells it is drawn in, in device pixels
	italic bool
	text   string
}

// inkCacheLimit bounds the glyph runs whose reach is kept; the cache starts over when it is full.
const inkCacheLimit = 4096

// inkMargin is the pixels past a glyph's outline that its antialiasing and the LCD filter touch.
const inkMargin = 1

// ownedColumns plans the columns each run of a grid label owns: its cells, and the blank cells
// beside them that its glyphs' ink reaches into (see platform.GridOwner).
func (services *textServices) ownedColumns(label platform.TextLabel, family string, scale float32, pitch float64, width int) []platform.GridSpan {
	reaches := false
	offset := 0
	for index, run := range label.Runs {
		end := offset + int(run.Bytes)
		if run.Bytes < 0 || end > len(label.Value) {
			break
		}
		piece := label.Value[offset:end]
		offset = end
		// ASCII ink stays inside its cell in every font a terminal is used with, and a
		// ligature run is ASCII; only the other characters are measured.
		if run.Cells <= 0 || run.Ligatures || piece == "" || printableASCII(piece) || !utf8.ValidString(piece) {
			continue
		}
		reach, ok := services.inkReach(inkKey{family: family, size: label.FontSize * scale, weight: run.Weight, span: float32(float64(run.Cells) * pitch), italic: run.Italic, text: piece})
		if !ok || (reach.Left <= 0 && reach.Right <= 0) {
			continue
		}
		if !reaches {
			reaches = true
			if cap(services.ink) < len(label.Runs) {
				services.ink = make([]platform.GridInk, len(label.Runs))
			}
			services.ink = services.ink[:len(label.Runs)]
			clear(services.ink)
		}
		services.ink[index] = reach
	}
	var ink []platform.GridInk
	if reaches {
		ink = services.ink
	}
	return services.owner.Plan(label.Runs, label.Value, pitch, width, ink)
}

// inkReach is how far the ink of a non-ASCII piece goes past its cells, drawn where drawRuns puts
// it, from Pango's extent of the glyphs.
func (services *textServices) inkReach(key inkKey) (platform.GridInk, bool) {
	if reach, known := services.inkCache[key]; known {
		return reach, true
	}
	metrics, err := services.configureLayout(key.text, key.family, key.size, key.weight, key.italic)
	if err != nil {
		return platform.GridInk{}, false
	}
	var ink pangoRectangle
	services.layoutGetExtents(services.layout, &ink, nil)
	x := 0.0
	if isSingleCharacter(key.text) {
		x = (float64(key.span) - float64(metrics.Width)) / 2
	}
	inkLeft := x + float64(ink.X)/pangoScale
	inkRight := inkLeft + float64(ink.Width)/pangoScale
	var reach platform.GridInk
	if ink.Width > 0 {
		if beyond := -inkLeft; beyond > 0 {
			reach.Left = float32(math.Ceil(beyond)) + inkMargin
		}
		if beyond := inkRight - float64(key.span); beyond > 0 {
			reach.Right = float32(math.Ceil(beyond)) + inkMargin
		}
	}
	if services.inkCache == nil || len(services.inkCache) >= inkCacheLimit {
		services.inkCache = make(map[inkKey]platform.GridInk, 64)
	}
	key.text = strings.Clone(key.text) // the label's own string would keep the whole line alive
	services.inkCache[key] = reach
	return reach, true
}
