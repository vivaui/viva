package base

import (
	"slices"
	"testing"
	"unicode/utf8"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
)

// concealedFixture fits a text into two layouts, one that conceals it and one of
// its bullets, and shows the same rows of each, as a paint does. What the two
// answer is then compared: the concealed layout must do by the secret what the
// plain one does by the bullets it paints.
func concealedFixture(t *testing.T, canvas *gfx.Canvas, value string) (secret, bullets TextareaLayout) {
	t.Helper()
	secret.Conceal(true)
	for _, fitted := range []*TextareaLayout{&secret, &bullets} {
		shown := value
		if fitted == &bullets {
			shown = text.Bullets(value)
		}
		fitted.Fit(canvas, shown, fitTestSize, fitTestWidth, true)
		fitted.Content, fitted.LineHeight = gfx.Rect{Width: fitTestWidth, Height: 4 * fitTestLine}, fitTestLine
		fitted.Show(0)
	}
	if secret.RowsText() != bullets.RowsText() {
		t.Fatalf("a concealed layout shows %q, a layout of the bullets %q", secret.RowsText(), bullets.RowsText())
	}
	return secret, bullets
}

// runes is where an offset stands in its text, in characters.
func runes(value string, offset int) int { return utf8.RuneCountInString(value[:offset]) }

// A layout that conceals keeps the rows of the secret's bullets, and answers in
// the secret's own offsets: every caret it places and every move it makes stands
// at the character the rows of the bullets put it at, as a layout of the bullets
// itself would, and never outside the text or inside a rune of it.
func TestTextareaConcealedAnswersInTheSecretsOffsets(t *testing.T) {
	canvas := shapedCanvas(nil)
	value := "-----BEGIN OPENSSH PRIVATE KEY-----\nbG9uZyBsaW5lIHRoYXQgd3JhcHMgaW4gdGhlIGJveA//\nshort\n-----END OPENSSH PRIVATE KEY-----\n"
	secret, bullets := concealedFixture(t, canvas, value)
	if !slices.Equal(secret.Rows, bullets.Rows) {
		t.Fatalf("the rows of a secret are not the rows of its bullets:\n got %v\nwant %v", secret.Rows, bullets.Rows)
	}
	// A press anywhere on a row.
	for index := range bullets.Rows {
		y := bullets.Content.Y + (float32(index)+0.5)*bullets.LineHeight - bullets.Scroll
		for _, x := range []float32{-5, 0, 1, 17, 40, 81, fitTestWidth / 2, fitTestWidth - 1, fitTestWidth + 9} {
			point := gfx.Point{X: x, Y: y}
			got, gotOK := secret.OffsetAt(point)
			want, wantOK := bullets.OffsetAt(point)
			if gotOK != wantOK || got < 0 || got > len(value) || got < len(value) && value[got]&0xc0 == 0x80 {
				t.Fatalf("OffsetAt%v of the secret = %d, %v (want ok %v), outside its text or inside a rune", point, got, gotOK, wantOK)
			}
			if gotOK && runes(value, got) != runes(bullets.RowsText(), want) {
				t.Fatalf("OffsetAt%v: character %d of the secret, %d of its bullets", point, runes(value, got), runes(bullets.RowsText(), want))
			}
		}
	}
	// Every move from a boundary of every rune.
	for caret := range value {
		for _, movement := range []TextareaMovement{TextareaMoveDown, TextareaMoveUp, TextareaMoveLineStart, TextareaMoveLineEnd, TextareaMovePageDown, TextareaMoveTextStart, TextareaMoveTextEnd} {
			got, want := secret.Move(value, caret, movement), bullets.Move(bullets.RowsText(), text.BulletOffset(value, caret), movement)
			if got < 0 || got > len(value) || got < len(value) && value[got]&0xc0 == 0x80 {
				t.Fatalf("move %d from %d of the secret led to %d, outside its text or inside a rune", movement, caret, got)
			}
			if runes(value, got) != runes(bullets.RowsText(), want) {
				t.Fatalf("move %d from character %d led to character %d of the secret, %d of its bullets",
					movement, runes(value, caret), runes(value, got), runes(bullets.RowsText(), want))
			}
		}
	}
}

// Conceal(false) is the layout of the text again: the rows and the offsets are the
// text's own, and nothing of the secret is kept.
func TestTextareaRevealedIsTheTextsLayoutAgain(t *testing.T) {
	canvas := shapedCanvas(nil)
	value := "line one wraps in this box\nline two\n\nlast line"
	secret, _ := concealedFixture(t, canvas, value)
	var plain TextareaLayout
	secret.Conceal(false)
	for _, fitted := range []*TextareaLayout{&secret, &plain} {
		fitted.Fit(canvas, value, fitTestSize, fitTestWidth, true)
		fitted.Content, fitted.LineHeight = gfx.Rect{Width: fitTestWidth, Height: 3 * fitTestLine}, fitTestLine
		fitted.Show(0)
	}
	if secret.RowsText() != value || !slices.Equal(secret.Rows, plain.Rows) {
		t.Fatalf("a revealed layout is not one of its text:\n got %q %v\nwant %q %v", secret.RowsText(), secret.Rows, plain.RowsText(), plain.Rows)
	}
	for _, movement := range []TextareaMovement{TextareaMoveDown, TextareaMoveUp, TextareaMoveLineEnd, TextareaMoveTextEnd} {
		if got, want := secret.Move(value, 3, movement), plain.Move(value, 3, movement); got != want {
			t.Fatalf("move %d from 3: revealed %d, plain %d", movement, got, want)
		}
	}
	point := gfx.Point{X: 12, Y: plain.Content.Y + 1.5*plain.LineHeight - plain.Scroll}
	got, gotOK := secret.OffsetAt(point)
	want, wantOK := plain.OffsetAt(point)
	if got != want || gotOK != wantOK {
		t.Fatalf("OffsetAt%v: revealed %d, %v; plain %d, %v", point, got, gotOK, want, wantOK)
	}
	if secret.concealed.on || secret.concealed.value != "" || secret.concealed.bullets != "" {
		t.Fatal("a revealed layout keeps the secret or its bullets")
	}
}

// A concealed layout builds the bullets once per text: a steady paint, and the
// caret it places and the moves it makes, do not allocate.
func TestTextareaConcealedSteadyPaintAllocatesNothing(t *testing.T) {
	canvas := shapedCanvas(nil)
	value := longText(200)
	var secret TextareaLayout
	secret.Conceal(true)
	paint := func(caret int) {
		secret.Fit(canvas, value, fitTestSize, fitTestWidth, true)
		secret.Content, secret.LineHeight = gfx.Rect{Width: fitTestWidth, Height: 20 * fitTestLine}, fitTestLine
		secret.Show(caret)
	}
	paint(0)
	if allocs := testing.AllocsPerRun(50, func() { paint(1234) }); allocs != 0 {
		t.Fatalf("a steady paint of a secret allocates %v objects", allocs)
	}
	if allocs := testing.AllocsPerRun(50, func() {
		secret.Move(value, 1234, TextareaMoveDown)
		secret.OffsetAt(gfx.Point{X: 30, Y: 7})
	}); allocs != 0 {
		t.Fatalf("moving the caret of a secret allocates %v objects", allocs)
	}
	end := len(value)
	paint(end)
	if secret.RowOf(len(secret.RowsText())) != len(secret.Rows)-1 {
		t.Fatal("the caret at the end of a secret is not on its last row")
	}
}

// BenchmarkTextareaConcealedSteady is a long secret's paint that changes nothing
// but the caret, beside the same paint of a text: what concealing costs a frame.
func BenchmarkTextareaConcealedSteady(b *testing.B) {
	canvas := shapedCanvas(nil)
	value := longText(2000)
	for _, concealed := range []bool{false, true} {
		name := "text"
		if concealed {
			name = "concealed"
		}
		b.Run(name, func(b *testing.B) {
			var layout TextareaLayout
			layout.Conceal(concealed)
			paint := func(caret int) {
				layout.Fit(canvas, value, fitTestSize, fitTestWidth, true)
				layout.Content, layout.LineHeight = gfx.Rect{Width: fitTestWidth, Height: 20 * fitTestLine}, fitTestLine
				layout.Show(caret)
			}
			paint(0)
			b.ReportAllocs()
			for b.Loop() {
				paint(1234)
			}
		})
	}
}
