package platform

import "testing"

// The highlight and the caret share one band, and the composition underline lies below
// the baseline of the text yet inside the field, for short and tall fields at the scales
// displays use.
func TestTextInputBandsStayInsideTheField(t *testing.T) {
	for _, height := range []float32{24, 32, 80} {
		for _, scale := range []float32{0, 1, 1.25, 2} {
			const fontSize, y = 14, 10
			top, band := TextCaretBand(y, height, fontSize)
			if top < y || top+band > y+height {
				t.Errorf("height %v: caret band [%v, %v] leaves the field", height, top, top+band)
			}
			effective := scale
			if effective == 0 {
				effective = 1
			}
			baseline := y + float32(LabelBaseline(int(height*effective+0.999), float64(fontSize*effective)))/effective
			underline := CompositionUnderlineTop(y, height, fontSize, scale, 3)
			if underline < baseline || underline+3 > y+height {
				t.Errorf("height %v scale %v: underline %v not within [baseline %v, bottom %v]", height, scale, underline, baseline, y+height-3)
			}
		}
	}
}
