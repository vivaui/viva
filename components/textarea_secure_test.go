package components

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
)

// painted is what the canvas was asked to draw: its bullets, by row (a selected row
// is drawn once more in the selection's color, and counted once), and any rune that is
// neither a bullet nor a line break.
func painted(canvas *gfx.Canvas) (rows []string, leaked []string) {
	type label struct {
		value  string
		bounds gfx.Rect
	}
	seen := map[label]bool{}
	for _, text := range canvas.Texts() {
		key := label{text.Value, text.Bounds}
		if seen[key] {
			continue
		}
		seen[key] = true
		rows = append(rows, text.Value)
		for _, r := range text.Value {
			if r != '•' && r != '\n' && r != '\r' {
				leaked = append(leaked, text.Value)
				break
			}
		}
	}
	return rows, leaked
}

// bullets is how many bullets the rows hold.
func bullets(rows []string) (count int) {
	for _, row := range rows {
		count += strings.Count(row, "•")
	}
	return count
}

// A Secure Textarea paints its value as bullets, one a rune, and its sign as a
// password field: nothing of the value is on the canvas or in the hit, with the
// rows a Layout keeps and without them.
func TestTextareaSecurePaintsBulletsOnly(t *testing.T) {
	theme := gfx.LightTheme()
	value := "-----BEGIN OPENSSH PRIVATE KEY-----\nbG9uZyBsaW5lIHRoYXQgd3JhcHMgaW4gdGhlIGJveA\n-----END OPENSSH PRIVATE KEY-----\n"
	for _, with := range []string{"layout", "no layout"} {
		var layout TextareaLayout
		var canvas gfx.Canvas
		snapshot := TextareaSnapshot{ID: gfx.Item(1), Label: "Private key", Value: value, Secure: true, Focused: true, ShowCaret: true, HideEditorText: true, Caret: 10, SelectionStart: 5, SelectionEnd: len(value)}
		bounds := gfx.Rect{Width: 180, Height: 190}
		if with == "layout" {
			snapshot.Layout = &layout
			snapshot.DisplayValue = value // ignored while Secure: the value is what the bullets are
		}
		PaintTextareaContent(&canvas, theme, bounds, bounds, snapshot)
		rows, leaked := painted(&canvas)
		if len(leaked) != 0 {
			t.Fatalf("%s: a secure textarea painted its value: %q", with, leaked)
		}
		if got, want := bullets(rows), utf8.RuneCountInString(value)-strings.Count(value, "\n"); got != want {
			t.Fatalf("%s: %d bullets over %d rows, want %d", with, got, len(rows), want)
		}
		hits := canvas.Hits()
		if len(hits) != 1 || !hits[0].Secure || hits[0].Value != "" || !hits[0].Multiline {
			t.Fatalf("%s: hits = %+v, want one password field that tells no value", with, hits)
		}
		// The caret and the selection were not those of the value's glyphs: they
		// were those of its bullets.
		if with == "layout" && layout.RowsText() != text.Bullets(value) {
			t.Fatalf("the rows of a secure textarea are of %q", layout.RowsText())
		}
	}
}

// A Secure multiline input group paints bullets for its rows and signs its hit
// as a password field's, as a single-line one does.
func TestInputGroupMultilineSecurePaintsBullets(t *testing.T) {
	var canvas gfx.Canvas
	theme := gfx.LightTheme()
	value := "one line of a secret\nanother one"
	PaintInputGroup(&canvas, theme, gfx.Rect{Width: 220, Height: 64}, InputGroupSnapshot{
		ID: gfx.Item(2), Label: "Secret", Value: value, Multiline: true, Secure: true, HideEditorText: true,
	})
	rows, leaked := painted(&canvas)
	if len(leaked) != 0 || bullets(rows) != utf8.RuneCountInString(value)-strings.Count(value, "\n") {
		t.Fatalf("a secure input group painted %d bullets and %q", bullets(rows), leaked)
	}
	hits := canvas.Hits()
	if len(hits) != 1 || !hits[0].Secure || hits[0].Value != "" || !hits[0].Multiline {
		t.Fatalf("hits = %+v, want one multiline password field", hits)
	}
}

// The steady paint of a secret builds nothing: the bullets are the layout's once
// the text is there, as the rows are.
func TestTextareaSecureSteadyPaintAllocatesNothing(t *testing.T) {
	value := longSecret()
	for _, secure := range []bool{false, true} {
		var layout TextareaLayout
		var canvas gfx.Canvas
		snapshot := TextareaSnapshot{ID: gfx.Item(1), Value: value, Secure: secure, Layout: &layout, Focused: true, ShowCaret: true, HideEditorText: true, Caret: len(value)}
		bounds := gfx.Rect{Width: 420, Height: 150}
		for range 3 {
			PaintTextareaContent(&canvas, gfx.LightTheme(), bounds, bounds, snapshot)
		}
		allocs := testing.AllocsPerRun(50, func() {
			PaintTextareaContent(&canvas, gfx.LightTheme(), bounds, bounds, snapshot)
		})
		if allocs != 0 {
			t.Fatalf("secure=%v: a steady paint allocates %v objects", secure, allocs)
		}
	}
	// An edit builds the bullets of the new text, and of it alone.
	var layout TextareaLayout
	var canvas gfx.Canvas
	snapshot := TextareaSnapshot{ID: gfx.Item(1), Value: "old", Secure: true, Layout: &layout}
	bounds := gfx.Rect{Width: 420, Height: 150}
	PaintTextareaContent(&canvas, gfx.LightTheme(), bounds, bounds, snapshot)
	snapshot.Value = "new text"
	PaintTextareaContent(&canvas, gfx.LightTheme(), bounds, bounds, snapshot)
	if layout.RowsText() != "••••••••" {
		t.Fatalf("after an edit the rows are of %q, want the new text's bullets", layout.RowsText())
	}
}

// longSecret is the size of a 4096-bit RSA key in the OpenSSH format.
func longSecret() string {
	var b strings.Builder
	b.WriteString("-----BEGIN OPENSSH PRIVATE KEY-----\n")
	for range 70 {
		b.WriteString("bG9uZyBsaW5lIG9mIGEgc2VjcmV0IHRoYXQgd3JhcHMgaW4gdGhlIGJveA\n")
	}
	return b.String()
}

// BenchmarkTextareaSecurePaint is the steady paint of a box of eight rows over a
// secret the size of a key, beside the paint of the same box for a text: what
// concealing costs a frame.
func BenchmarkTextareaSecurePaint(b *testing.B) {
	for _, secure := range []bool{false, true} {
		name := "text"
		if secure {
			name = "secure"
		}
		b.Run(name, func(b *testing.B) {
			value := longSecret()
			var layout TextareaLayout
			var canvas gfx.Canvas
			theme := gfx.LightTheme()
			snapshot := TextareaSnapshot{ID: gfx.Item(1), Value: value, Secure: secure, Layout: &layout, Focused: true, ShowCaret: true, HideEditorText: true, Caret: len(value)}
			bounds := gfx.Rect{Width: 420, Height: 150}
			PaintTextareaContent(&canvas, theme, bounds, bounds, snapshot)
			b.ReportAllocs()
			for b.Loop() {
				canvas.Reset()
				PaintTextareaContent(&canvas, theme, bounds, bounds, snapshot)
			}
		})
	}
}
