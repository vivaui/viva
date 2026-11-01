//go:build darwin && vivahead

package darwin

import (
	"testing"

	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/text"
	"github.com/vivaui/viva/ui"
)

// inkRows is the first and last device row at or right of fromX that holds
// glyph pixels, or -1 for both when there are none.
func inkRows(pixels []byte, width, height, fromX int) (first, last int) {
	first, last = -1, -1
	for y := 0; y < height; y++ {
		for x := fromX; x < width; x++ {
			if b, g, r, _ := pixelAt(pixels, width, x, y); int(r)+int(g)+int(b) > 150 {
				if first < 0 {
					first = y
				}
				last = y
				break
			}
		}
	}
	return first, last
}

// A field's text must not move when focusing it hands the drawing from the
// label to the native editor: both stand on the baseline every label does,
// whatever the line metrics of the text's fallback fonts are.
func TestEditorTextStandsWhereTheLabelDoesInTheSameBox(t *testing.T) {
	white := platform.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1}
	// The leading spaces keep the caret at the start of the editor out of the
	// measured columns.
	const value = "      主机Hxg 1212"
	for _, box := range []struct{ height, size float32 }{{36, 13}, {36, 14}, {28, 12}, {24, 14}, {40, 15}} {
		render := func(label *TextLabel, input *TextInput) (first, last int) {
			offscreen, err := NewOffscreen(160, 60, 2, Color{Alpha: 1})
			if err != nil {
				t.Skipf("no Metal device: %v", err)
			}
			defer offscreen.Close()
			var labels []TextLabel
			if label != nil {
				labels = []TextLabel{*label}
			}
			if err := offscreen.Set(nil, labels, input); err != nil {
				t.Fatal(err)
			}
			pixels, width, height, err := offscreen.Render()
			if err != nil {
				t.Fatal(err)
			}
			return inkRows(pixels, width, height, 2*24)
		}
		labelFirst, labelLast := render(&TextLabel{
			Value: value, X: 0, Y: 10, Width: 160, Height: box.height, FontSize: box.size, Color: white,
		}, nil)
		editorFirst, editorLast := render(nil, &TextInput{
			X: 0, Y: 10, Width: 160, Height: box.height, FontSize: box.size, Color: white,
			State:   TextState{Value: value},
			OnEvent: func(TextEvent) (TextState, error) { return TextState{}, nil },
		})
		if labelFirst < 0 || editorFirst < 0 {
			t.Fatalf("height %v size %v: no glyph pixels (label %d, editor %d)", box.height, box.size, labelFirst, editorFirst)
		}
		if labelFirst != editorFirst || labelLast != editorLast {
			t.Errorf("height %v size %v: the label's ink spans rows %d-%d, the editor's %d-%d", box.height, box.size, labelFirst, labelLast, editorFirst, editorLast)
		}
	}
}

// inkColumns says, for every device column, whether it holds glyph pixels.
func inkColumns(pixels []byte, width, height int) []bool {
	columns := make([]bool, width)
	for x := 0; x < width; x++ {
		for y := 0; y < height && !columns[x]; y++ {
			b, g, r, _ := pixelAt(pixels, width, x, y)
			columns[x] = int(r)+int(g)+int(b) > 150
		}
	}
	return columns
}

// A terminal's preedit is committed to the grid, which draws every character on its cells.
// The preedit must stand where the grid will put the text, or it jumps when it is committed:
// the wide characters in the middle of their two cells, the narrow ones on their one. The
// same text is drawn both ways and the columns that hold ink are compared.
func TestTerminalPreeditStandsWhereTheGridDrawsTheSameText(t *testing.T) {
	white := platform.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1}
	const (
		value = "ni你好世ab"
		size  = float32(13)
		scale = 2
	)
	cell := platform.TerminalPreeditCellWidth(size)
	render := func(label *TextLabel, input *TextInput) []bool {
		offscreen, err := NewOffscreen(200, 40, scale, Color{Alpha: 1})
		if err != nil {
			t.Skipf("no Metal device: %v", err)
		}
		defer offscreen.Close()
		var labels []TextLabel
		if label != nil {
			labels = []TextLabel{*label}
		}
		if err := offscreen.Set(nil, labels, input); err != nil {
			t.Fatal(err)
		}
		pixels, width, height, err := offscreen.Render()
		if err != nil {
			t.Fatal(err)
		}
		return inkColumns(pixels, width, height)
	}
	runs := platform.TerminalPreeditRuns(nil, value, ui.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1})
	grid := render(&TextLabel{
		Value: value, X: 0, Y: 8, Width: 200, Height: 20, FontSize: size, Color: white,
		Mono: true, CellWidth: cell, Runs: runs,
	}, nil)
	preedit := render(nil, &TextInput{
		X: 0, Y: 8, Width: 200, Height: 20, FontSize: size, Color: white, Terminal: true,
		// Without a composition there is no underline to count among the glyphs.
		State:   TextState{Value: value},
		OnEvent: func(TextEvent) (TextState, error) { return TextState{}, nil },
	})
	inked, different := 0, 0
	for x := range grid {
		if grid[x] {
			inked++
		}
		if grid[x] != preedit[x] {
			different++
		}
	}
	if inked == 0 {
		t.Fatal("the grid drew no glyph pixels")
	}
	if different > 2 {
		t.Errorf("%d of the %d device columns the grid inks differ in the preedit: it does not stand where the grid draws the text", different, inked)
	}
}

// The composition is underlined, in a terminal's one-row surface too: the underline lies below
// the descenders, which in a row only as tall as its text is the edge of the row.
func TestTerminalPreeditIsUnderlined(t *testing.T) {
	offscreen, err := NewOffscreen(120, 40, 2, Color{Alpha: 1})
	if err != nil {
		t.Skipf("no Metal device: %v", err)
	}
	defer offscreen.Close()
	white := platform.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1}
	input := &TextInput{
		X: 0, Y: 8, Width: 120, Height: 16, FontSize: 13, Color: white, Terminal: true,
		State:   TextState{Value: "ni你好", Composing: true, Composition: text.Range{End: 4}},
		OnEvent: func(TextEvent) (TextState, error) { return TextState{}, nil },
	}
	if err := offscreen.Set(nil, nil, input); err != nil {
		t.Fatal(err)
	}
	pixels, width, height, err := offscreen.Render()
	if err != nil {
		t.Fatal(err)
	}
	underlined := 0
	for x := 0; x < width; x++ {
		for y := 0; y < height; y++ {
			// The underline is blue; the glyphs are white.
			if blue, green, red, _ := pixelAt(pixels, width, x, y); int(blue) > int(red)+60 && int(blue) > int(green) {
				underlined++
				break
			}
		}
	}
	// "ni" and two wide characters are six cells.
	if want := int(6 * platform.TerminalPreeditCellWidth(13) * 2); underlined < want*9/10 {
		t.Errorf("the underline covers %d device columns, want about %d", underlined, want)
	}
}

// A composition changes with every key, so the surface that draws it is rebuilt that often.
func BenchmarkRasterizeTerminalPreedit(b *testing.B) {
	offscreen, err := NewOffscreen(400, 40, 2, Color{Alpha: 1})
	if err != nil {
		b.Skipf("no Metal device: %v", err)
	}
	defer offscreen.Close()
	input := &TextInput{
		X: 0, Y: 8, Width: 400, Height: 16, FontSize: 13, Terminal: true,
		Color:   platform.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1},
		State:   TextState{Value: "wo'men你好世界ab", Composing: true, Composition: text.Range{End: 12}},
		OnEvent: func(TextEvent) (TextState, error) { return TextState{}, nil },
	}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if err := offscreen.Set(nil, nil, input); err != nil {
			b.Fatal(err)
		}
	}
}
