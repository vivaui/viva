//go:build js && wasm

package web

import (
	"fmt"
	"strings"
	"syscall/js"

	"github.com/vivaui/viva/gfx"
)

func cssFont(fontSize, weight float32) string {
	return cssFontStyle(fontSize, weight, false)
}

func cssFontStyle(fontSize, weight float32, italic bool) string {
	cssWeight := 400
	switch {
	case weight >= 0.34:
		cssWeight = 700
	case weight >= 0.20:
		cssWeight = 600
	case weight >= 0.10:
		cssWeight = 500
	}
	slant := ""
	if italic {
		slant = "italic "
	}
	family := "system-ui, -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif"
	if interfaceFamily != "" {
		family = cssFamily(interfaceFamily) + ", " + family
	}
	return fmt.Sprintf("%s%d %gpx %s", slant, cssWeight, fontSize, family)
}

// baselineMetrics mirrors the CoreText typographic values the native label
// rasterizer uses to vertically center a line inside its bounds.
type baselineMetrics struct {
	Ascent  float32
	Descent float32
}

// cssMonoFont is the browser's fixed-pitch font.
func cssMonoFont(fontSize, weight float32, italic bool, face gfx.MonoFace) string {
	cssWeight := 400
	switch {
	case weight >= 0.34:
		cssWeight = 700
	case weight >= 0.20:
		cssWeight = 600
	case weight >= 0.10:
		cssWeight = 500
	}
	slant := ""
	if italic {
		slant = "italic "
	}
	return fmt.Sprintf("%s%d %gpx %s", slant, cssWeight, fontSize, cssMonoFamilies(face))
}

// cssMonoFamilies is the font-family list of a face: its primary font, then the
// families that draw what the primary lacks, then the generic ones.
func cssMonoFamilies(face gfx.MonoFace) string {
	var list strings.Builder
	switch name := gfx.MonoFamilyName(face); {
	case name != "":
		list.WriteString(cssFamily(name))
		list.WriteString(", ")
	case gfx.MonoIsClassic(face):
		list.WriteString("'Courier New', Courier, ")
	}
	for _, fallback := range gfx.MonoFallbacks(face) {
		list.WriteString(cssFamily(fallback))
		list.WriteString(", ")
	}
	switch {
	case gfx.MonoFamilyName(face) != "":
		list.WriteString("monospace")
	case gfx.MonoIsClassic(face):
		list.WriteString("monospace")
	default:
		list.WriteString("ui-monospace, SFMono-Regular, Menlo, Consolas, monospace")
	}
	return list.String()
}

// monoBaseline reports the fixed-pitch font's ascent and descent at a size.
func monoBaseline(context js.Value, fontSize float32, face gfx.MonoFace) baselineMetrics {
	context.Set("font", cssMonoFont(fontSize, 0, false, face))
	metrics := context.Call("measureText", "Ag")
	ascent := float32(metrics.Get("fontBoundingBoxAscent").Float())
	descent := float32(metrics.Get("fontBoundingBoxDescent").Float())
	if ascent <= 0 {
		ascent = float32(metrics.Get("actualBoundingBoxAscent").Float())
		descent = float32(metrics.Get("actualBoundingBoxDescent").Float())
	}
	return baselineMetrics{Ascent: ascent, Descent: descent}
}

// cssFamily quotes a font family name for a CSS font shorthand. Quotes and
// backslashes are dropped: a family name has none, and one that did could close
// the quotes.
func cssFamily(name string) string {
	return "'" + strings.NewReplacer("'", "", `"`, "", `\`, "").Replace(name) + "'"
}
