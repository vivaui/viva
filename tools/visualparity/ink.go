//go:build darwin || windows || linux

package main

import (
	"fmt"
	"image"
	"io"
	"math"
	"path/filepath"
)

// inkRows returns the first and last rows (device pixels, relative to the top of
// the text's bounds) holding ink in the text's box, and the ink's horizontal
// extent. Ink is any pixel that differs from the box's background by more than
// the threshold; ok is false when there is none.
func inkRows(img *image.RGBA, scale float64, b Box) (top, bottom float64, left, right float64, ok bool) {
	x0, y0 := int(math.Floor(float64(b.X)*scale)), int(math.Floor(float64(b.Y)*scale))
	x1, y1 := int(math.Ceil(float64(b.X+b.W)*scale)), int(math.Ceil(float64(b.Y+b.H)*scale))
	bounds := img.Bounds()
	x0, y0, x1, y1 = max(x0, bounds.Min.X), max(y0, bounds.Min.Y), min(x1, bounds.Max.X), min(y1, bounds.Max.Y)
	if x0 >= x1 || y0 >= y1 {
		return 0, 0, 0, 0, false
	}
	bg := img.Pix[img.PixOffset(x0, y0) : img.PixOffset(x0, y0)+3]
	firstRow, lastRow, firstCol, lastCol := -1, -1, -1, -1
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			o := img.PixOffset(x, y)
			d := 0
			for c := 0; c < 3; c++ {
				d = max(d, absInt(int(img.Pix[o+c])-int(bg[c])))
			}
			if d > 96 {
				if firstRow < 0 {
					firstRow = y
				}
				lastRow = y
				if firstCol < 0 || x < firstCol {
					firstCol = x
				}
				lastCol = max(lastCol, x)
			}
		}
	}
	if firstRow < 0 {
		return 0, 0, 0, 0, false
	}
	return float64(firstRow-y0) / scale, float64(lastRow+1-y0) / scale, float64(firstCol-x0) / scale, float64(lastCol+1-x0) / scale, true
}

// inkMass is the total darkness of the text's box against its background, in
// whole pixels of full ink: how heavy the text looks.
func inkMass(img *image.RGBA, scale float64, b Box) float64 {
	x0, y0 := int(math.Floor(float64(b.X)*scale)), int(math.Floor(float64(b.Y)*scale))
	x1, y1 := int(math.Ceil(float64(b.X+b.W)*scale)), int(math.Ceil(float64(b.Y+b.H)*scale))
	bounds := img.Bounds()
	x0, y0, x1, y1 = max(x0, bounds.Min.X), max(y0, bounds.Min.Y), min(x1, bounds.Max.X), min(y1, bounds.Max.Y)
	if x0 >= x1 || y0 >= y1 {
		return 0
	}
	bg := img.Pix[img.PixOffset(x0, y0) : img.PixOffset(x0, y0)+3]
	var sum float64
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			o := img.PixOffset(x, y)
			d := 0
			for c := 0; c < 3; c++ {
				d = max(d, absInt(int(img.Pix[o+c])-int(bg[c])))
			}
			sum += float64(d) / 255
		}
	}
	return sum / (scale * scale)
}

// compareInk reports, for the baseline probe, where each platform's glyphs
// stand in the line box Viva gave them: the baseline (bottom of the capitals),
// the cap height and the x-height, in logical pixels from the top of the box.
func compareInk(w io.Writer, a, b *side) {
	const key = "baseline.light"
	fa, fb := a.facts[key], b.facts[key]
	if fa == nil || fb == nil {
		return
	}
	load := func(s *side) *image.RGBA {
		img, err := loadPNG(filepath.Join(s.dir, key+"@1x.png"))
		if err != nil {
			return nil
		}
		return img
	}
	ia, ib := load(a), load(b)
	if ia == nil || ib == nil {
		return
	}
	fmt.Fprintf(w, "\n== Glyph placement inside the line box (baseline probe, 1x, logical px from the top of the box; %s | %s | difference) ==\n", a.platform.Name, b.platform.Name)
	fmt.Fprintf(w, "%-5s %-9s | %-22s | %-22s | %-22s | %s\n", "size", "weight", "baseline (box h)", "cap height", "x-height", "ink mass ratio")
	var sumBase, sumCap float64
	var n int
	for i := 0; i+2 < len(fa.Texts) && i+2 < len(fb.Texts); i += 4 {
		ta, tb := fa.Texts[i], fb.Texts[i]
		if ta.Value != "HHHH" || tb.Value != "HHHH" {
			continue
		}
		hta, hba, _, _, ok1 := inkRows(ia, 1, ta.Bounds)
		htb, hbb, _, _, ok2 := inkRows(ib, 1, tb.Bounds)
		xta, xba, _, _, ok3 := inkRows(ia, 1, fa.Texts[i+1].Bounds)
		xtb, xbb, _, _, ok4 := inkRows(ib, 1, fb.Texts[i+1].Bounds)
		if !(ok1 && ok2 && ok3 && ok4) {
			continue
		}
		name := "regular"
		switch ta.Weight {
		case 0.15:
			name = "medium"
		case 0.23:
			name = "semibold"
		}
		massA, massB := inkMass(ia, 1, fa.Texts[i+2].Bounds), inkMass(ib, 1, fb.Texts[i+2].Bounds)
		fmt.Fprintf(w, "%-5g %-9s | %5.1f %5.1f %+6.1f (%4.1f) | %5.1f %5.1f %+6.1f       | %5.1f %5.1f %+6.1f | %.2f\n", ta.Size, name,
			hba, hbb, hbb-hba, ta.Bounds.H, hba-hta, hbb-htb, (hbb-htb)-(hba-hta), xba-xta, xbb-xtb, (xbb-xtb)-(xba-xta), massB/massA)
		sumBase += math.Abs(hbb - hba)
		sumCap += math.Abs((hbb - htb) - (hba - hta))
		n++
	}
	if n > 0 {
		fmt.Fprintf(w, "mean absolute baseline difference %.2f px, cap-height difference %.2f px over %d styles\n", sumBase/float64(n), sumCap/float64(n), n)
	}
}
