package base

import (
	"fmt"
	"strings"
	"testing"

	"github.com/vivaui/viva/gfx"
)

// plainPageProse is long enough that the paragraph in the reference page breaks.
const plainPageProse = "a paragraph of plain text that its container does not stretch, so it asks for its widest line"

// plainPage draws a page of Texts and nothing of its own that Italic or Strike
// touches, and returns every primitive it painted. It is the page the guide's
// text attributes describe: sizes, weights, colors, alignment, Mono, LineHeight,
// wrapping, right-to-left text, background, border and radius.
func plainPage(canvas *gfx.Canvas) string {
	var view View
	f := view.Begin(canvas, gfx.LightTheme(), gfx.Rect{Width: 400, Height: 500})
	f.Column().Pad(8).Gap(4).Background(gfx.Color{Blue: 0.1, Alpha: 1}).Radius(4).Border(1, gfx.Color{Alpha: 0.25}).Children(func() {
		f.Text("plain text").FontSize(12)
		f.Text("weighted").FontSize(14).Weight(gfx.WeightSemibold)
		f.Text("muted").FontSize(12).Muted()
		f.Text("centered").FontSize(12).TextCenter()
		f.Text("ended").FontSize(12).TextEnd()
		f.Text("mono").FontSize(12).Mono()
		f.Text("tall").FontSize(12).LineHeight(30)
		f.Wrap(plainPageProse).FontSize(12).MaxLines(3)
		f.Text("שלום עולם").FontSize(12)
		f.Row().Border(1, gfx.Color{Alpha: 0.5}).Children(func() {
			f.Text("in a row").FontSize(12)
			f.Text("stretched").FontSize(12).Grow(1)
		})
	})
	f.End()
	return dumpPrimitives(canvas)
}

// dumpPrimitives writes every primitive a paint pass left on the canvas, in paint
// order. The primitives are what a renderer rasterizes, so two frames whose
// primitives are equal are two frames that are the same pixels.
func dumpPrimitives(canvas *gfx.Canvas) string {
	var out strings.Builder
	for _, rectangle := range canvas.Rectangles() {
		fmt.Fprintf(&out, "rect %+v\n", rectangle)
	}
	for _, text := range canvas.Texts() {
		fmt.Fprintf(&out, "text %+v\n", text)
	}
	for _, triangle := range canvas.Triangles() {
		fmt.Fprintf(&out, "triangle %+v\n", triangle)
	}
	for _, shadow := range canvas.Shadows() {
		fmt.Fprintf(&out, "shadow %+v\n", shadow)
	}
	fmt.Fprintf(&out, "images %d\n", len(canvas.Images()))
	return out.String()
}

// A page that sets neither Italic nor Strike paints no italic run and no strike
// fill. That is the invariant this change must keep, and it holds whatever else
// moves in the layout or in gfx.Text: a page that never asks for a face cannot
// get one.
//
// An earlier version of this test pinned a dump of every primitive; it failed on
// any unrelated layout change, so it now asserts the property instead.
func TestAPageWithoutItalicOrStrikePaintsNeitherFace(t *testing.T) {
	canvas := runeCanvas()
	plainPage(canvas)

	texts := canvas.Texts()
	if len(texts) == 0 {
		t.Fatal("the reference page painted no text")
	}
	for _, text := range texts {
		if text.Style.Italic {
			t.Fatalf("a page that sets no Italic painted an italic run: %q", text.Value)
		}
	}

	// Strike is a one-pixel fill through the middle of each line. The reference
	// page draws no rule of its own, so any one-pixel rectangle is a strike.
	for _, rectangle := range canvas.Rectangles() {
		if rectangle.Bounds.Height == 1 {
			t.Fatalf("a page that sets no Strike painted a strike fill: %+v", rectangle.Bounds)
		}
	}
}
