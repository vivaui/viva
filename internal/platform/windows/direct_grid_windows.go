package windows

import (
	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/ui"
)

// gridPiece is one run of a fixed-pitch label: its text, how it is drawn, and where it
// stands, in device pixels from the label's left edge.
type gridPiece struct {
	text       string
	run        ui.TextRun
	weight     float32
	color      d2dColor
	left, span float32
	font       uintptr // the GDI font that draws the piece, set by useGDI; 0 for DirectWrite
}

// gridPieces splits a fixed-pitch label into its runs. A label without runs is one piece
// of the label's own color and weight.
func (rasterizer *directRasterizer) gridPieces(label platform.TextLabel, pitch float32) []gridPiece {
	rasterizer.pieces = rasterizer.pieces[:0]
	if len(label.Runs) == 0 {
		rasterizer.pieces = append(rasterizer.pieces, gridPiece{
			text: label.Value, weight: label.Weight,
			color: d2dColor{R: float32(label.Color.Red), G: float32(label.Color.Green), B: float32(label.Color.Blue), A: float32(label.Color.Alpha)},
		})
		return rasterizer.pieces
	}
	offset := 0
	for _, run := range label.Runs {
		end := offset + int(run.Bytes)
		if run.Bytes < 0 || end > len(label.Value) {
			break
		}
		text := label.Value[offset:end]
		offset = end
		if text == "" {
			continue
		}
		rasterizer.pieces = append(rasterizer.pieces, gridPiece{
			text: text, run: run, weight: run.Weight,
			color: d2dColor{R: run.Color.Red, G: run.Color.Green, B: run.Color.Blue, A: run.Color.Alpha},
			left:  float32(run.Column) * pitch, span: float32(run.Cells) * pitch,
		})
	}
	return rasterizer.pieces
}

// colorSpan is the columns of a ClearType label texture, left to right, that are text of
// one color. Its coverage is the same for every color, so the renderer draws the one
// texture once per span, with the span's color, scissored to its columns.
type colorSpan struct {
	left, right int // device pixels from the texture's left edge
	color       d2dColor
}

// gridLCD draws a fixed-pitch label into pixels as ClearType coverage per channel, as the
// text of an ordinary label is (see rasterize), and records the color of each span of
// columns in rasterizer.spans. Each run is drawn by GDI or by Direct2D (see useGDI), and
// the coverage of its columns is read from the drawing made for it. Direct2D thins light text
// and thickens dark text, and GDI does too, so a run is drawn white on black or black on white
// by the brightness of its color. A line takes one drawing per renderer and brightness it
// holds, and each run is drawn once: unlike an ordinary label's single color a line has many,
// and drawing a saturated color twice for the brightness of each channel would double the
// cost of the terminal for a difference nobody sees. Runs stand on cells of the grid, so the
// columns of one run never hold the glyphs of another except for the last pixel the
// ClearType filter spills over.
func (rasterizer *directRasterizer) gridLCD(pixels []byte, label platform.TextLabel, size float32, width, height int) error {
	family := monoFamily(label.MonoFace)
	fallback := rasterizer.fontFallback(label.MonoFace)
	pitch := label.CellWidth * float32(size/label.FontSize)
	baseline := float32(platform.LabelBaseline(height, float64(size)))
	pieces := rasterizer.gridPieces(label, pitch)
	if err := rasterizer.useGDI(pieces, family, size, width, height); err != nil {
		return err
	}
	for _, viaGDI := range [2]bool{false, true} {
		for _, drawLight := range [2]bool{true, false} {
			if !passDraws(pieces, drawLight, viaGDI) {
				continue
			}
			var scratch *textBitmap
			var err error
			if viaGDI {
				scratch, err = rasterizer.gdi.draw(pieces, drawLight, pitch, int(baseline), width, height)
			} else {
				scratch, err = rasterizer.drawDirect(pieces, drawLight, family, fallback, size, pitch, baseline, width, height)
			}
			if err != nil {
				return err
			}
			// Neighbouring cells share the pixel their edge falls in, and each has drawn its own
			// part of it. Columns of cells drawn in this pass are read once, whichever cells
			// share them; what the other passes left in a shared column is joined with them.
			var left, right int
			for _, piece := range pieces {
				if !inPass(piece, drawLight, viaGDI) {
					continue
				}
				pieceLeft, pieceRight := columnsOf(piece, width, len(pieces) == 1)
				if pieceLeft == pieceRight {
					continue
				}
				if right > left && pieceLeft <= right {
					right = max(right, pieceRight)
					continue
				}
				if right > left {
					coverageOfColumns(pixels, scratch, width, height, left, right, drawLight)
				}
				left, right = pieceLeft, pieceRight
			}
			if right > left {
				coverageOfColumns(pixels, scratch, width, height, left, right, drawLight)
			}
		}
	}
	for index := 0; index+3 < len(pixels); index += 4 {
		pixels[index+3] = luminance(pixels[index+2], pixels[index+1], pixels[index])
	}
	for _, piece := range pieces {
		left, right := columnsOf(piece, width, len(pieces) == 1)
		if left == right {
			continue
		}
		if n := len(rasterizer.spans); n > 0 && rasterizer.spans[n-1].color == piece.color && rasterizer.spans[n-1].right >= left {
			rasterizer.spans[n-1].right = max(rasterizer.spans[n-1].right, right)
			continue
		}
		rasterizer.spans = append(rasterizer.spans, colorSpan{left: left, right: right, color: piece.color})
	}
	return nil
}

// inPass reports whether a piece is drawn by the pass of a brightness and a renderer.
func inPass(piece gridPiece, light, viaGDI bool) bool {
	return isLight(piece.color) == light && (piece.font != 0) == viaGDI
}

// passDraws reports whether any piece is drawn by the pass of a brightness and a renderer.
func passDraws(pieces []gridPiece, light, viaGDI bool) bool {
	for _, piece := range pieces {
		if inPass(piece, light, viaGDI) {
			return true
		}
	}
	return false
}

// drawDirect draws the pieces of the pass of a brightness that Direct2D draws, white on black
// for light and black on white for dark, and returns the bitmap they are on.
func (rasterizer *directRasterizer) drawDirect(pieces []gridPiece, light bool, family string, fallback uintptr, size, pitch, baseline float32, width, height int) (*textBitmap, error) {
	background, foreground := d2dColor{A: 1}, d2dColor{R: 1, G: 1, B: 1, A: 1}
	if !light {
		background, foreground = foreground, background
	}
	scratch, err := rasterizer.opaque.bind(width, height)
	if err != nil {
		return nil, err
	}
	fill(scratch, width, height, background)
	rasterizer.opaque.begin(foreground, d2dAntialiasDefault)
	var drawErr error
	for _, piece := range pieces {
		if !inPass(piece, light, false) {
			continue
		}
		if err := rasterizer.drawRun(rasterizer.opaque, family, fallback, size, piece.weight, piece.run, piece.text, piece.left, piece.span, pitch, baseline); err != nil && drawErr == nil {
			drawErr = err
		}
	}
	if err := rasterizer.opaque.end(); err != nil && drawErr == nil {
		drawErr = err
	}
	return scratch, drawErr
}

// columnsOf is the device columns of a label texture width pixels wide that a piece owns:
// those of its cells, which no other piece owns, or all of them for the one piece of a label
// without runs. A run that holds no cell owns none, so it cannot take the coverage of its
// neighbours.
func columnsOf(piece gridPiece, width int, only bool) (left, right int) {
	if only && piece.span <= 0 {
		return 0, width
	}
	return platform.GridColumns(float64(piece.left), float64(piece.span), width)
}

// isLight reports whether text of a color is drawn white on black: its luminance is above
// half.
func isLight(color d2dColor) bool {
	return 0.299*color.R+0.587*color.G+0.114*color.B > 0.5
}

// coverageOfColumns takes the coverage of all channels within the columns left to right
// from a drawing of white text on black (drawnLight) or black text on white, and joins it with
// the coverage already there: a column that two neighbouring cells of different brightness
// share has part of each cell's glyph, drawn in separate passes, and replacing one with the
// other would leave a gap between them.
func coverageOfColumns(pixels []byte, bitmap *textBitmap, width, height, left, right int, drawnLight bool) {
	var flip byte
	if !drawnLight {
		flip = 255
	}
	for row := 0; row < height; row++ {
		source := bitmap.Pixels[row*bitmap.Width*4:]
		out := pixels[row*width*4 : (row+1)*width*4]
		for index := left * 4; index < right*4; index += 4 {
			for channel := 0; channel < 3; channel++ {
				drawn, have := int(source[index+channel]^flip), int(out[index+channel])
				out[index+channel] = byte(have + drawn - have*drawn/255)
			}
		}
	}
}
