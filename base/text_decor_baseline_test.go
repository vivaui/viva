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

// firstDifferentLine names the first line of got that differs from want, so a
// failure says which primitive moved.
func firstDifferentLine(got, want string) string {
	gotLines, wantLines := strings.Split(got, "\n"), strings.Split(want, "\n")
	for index := 0; index < len(gotLines) || index < len(wantLines); index++ {
		var gotLine, wantLine string
		if index < len(gotLines) {
			gotLine = gotLines[index]
		}
		if index < len(wantLines) {
			wantLine = wantLines[index]
		}
		if gotLine != wantLine {
			return fmt.Sprintf("line %d:\n got: %s\nwant: %s", index+1, gotLine, wantLine)
		}
	}
	return "no line differs"
}

// A page that sets neither Italic nor Strike is painted exactly as it was painted
// before either existed: same rectangles, same labels, same styles, in the same
// order. Italic and Strike are flags of an element's own, and an element that does
// not set them must cost a renderer nothing and change no pixel.
//
// plainPagePrimitives was recorded from the tree before this change.
func TestAPageWithoutItalicOrStrikePaintsTheSamePrimitivesAsBefore(t *testing.T) {
	got := plainPage(runeCanvas())
	if got != plainPagePrimitives {
		t.Fatalf("the primitives of a plain page changed: %s", firstDifferentLine(got, plainPagePrimitives))
	}
}

// plainPagePrimitives is what plainPage painted before Italic and Strike existed:
// every primitive of that frame, recorded from origin/main at 75a5070.
var plainPagePrimitives = strings.Join([]string{
	"rect {Bounds:{X:0 Y:0 Width:400 Height:293} Color:{Red:0 Green:0 Blue:0 Alpha:0.25} Radius:4 Layer:0 ClipOf:{X:0 Y:0 Width:0 Height:0} HasClip:false Order:1}",
	"rect {Bounds:{X:1 Y:1 Width:398 Height:291} Color:{Red:0 Green:0 Blue:0.1 Alpha:1} Radius:3 Layer:0 ClipOf:{X:0 Y:0 Width:0 Height:0} HasClip:false Order:2}",
	"text {Value:plain text Bounds:{X:9 Y:9 Width:382 Height:19} Style:{Size:12 Weight:0 Italic:false Color:{Red:0.039215688 Green:0.039215688 Blue:0.039215688 Alpha:1} Mono:false MonoFace:0 Direction:0} CellWidth:0 Baseline:0 RunFirst:0 RunCount:0 Identity:{Owner:0 Item:0} Quad:[{X:0 Y:0} {X:0 Y:0} {X:0 Y:0} {X:0 Y:0}] Tint:[{Red:0 Green:0 Blue:0 Alpha:0} {Red:0 Green:0 Blue:0 Alpha:0} {Red:0 Green:0 Blue:0 Alpha:0} {Red:0 Green:0 Blue:0 Alpha:0}] HasQuad:false Layer:0 ClipOf:{X:0 Y:0 Width:0 Height:0} HasClip:false Order:3}",
	"text {Value:weighted Bounds:{X:9 Y:32 Width:382 Height:21} Style:{Size:14 Weight:0.23 Italic:false Color:{Red:0.039215688 Green:0.039215688 Blue:0.039215688 Alpha:1} Mono:false MonoFace:0 Direction:0} CellWidth:0 Baseline:0 RunFirst:0 RunCount:0 Identity:{Owner:0 Item:0} Quad:[{X:0 Y:0} {X:0 Y:0} {X:0 Y:0} {X:0 Y:0}] Tint:[{Red:0 Green:0 Blue:0 Alpha:0} {Red:0 Green:0 Blue:0 Alpha:0} {Red:0 Green:0 Blue:0 Alpha:0} {Red:0 Green:0 Blue:0 Alpha:0}] HasQuad:false Layer:0 ClipOf:{X:0 Y:0 Width:0 Height:0} HasClip:false Order:4}",
	"text {Value:muted Bounds:{X:9 Y:57 Width:382 Height:19} Style:{Size:12 Weight:0 Italic:false Color:{Red:0.4509804 Green:0.4509804 Blue:0.4509804 Alpha:1} Mono:false MonoFace:0 Direction:0} CellWidth:0 Baseline:0 RunFirst:0 RunCount:0 Identity:{Owner:0 Item:0} Quad:[{X:0 Y:0} {X:0 Y:0} {X:0 Y:0} {X:0 Y:0}] Tint:[{Red:0 Green:0 Blue:0 Alpha:0} {Red:0 Green:0 Blue:0 Alpha:0} {Red:0 Green:0 Blue:0 Alpha:0} {Red:0 Green:0 Blue:0 Alpha:0}] HasQuad:false Layer:0 ClipOf:{X:0 Y:0 Width:0 Height:0} HasClip:false Order:5}",
	"text {Value:centered Bounds:{X:152 Y:80 Width:382 Height:19} Style:{Size:12 Weight:0 Italic:false Color:{Red:0.039215688 Green:0.039215688 Blue:0.039215688 Alpha:1} Mono:false MonoFace:0 Direction:0} CellWidth:0 Baseline:0 RunFirst:0 RunCount:0 Identity:{Owner:0 Item:0} Quad:[{X:0 Y:0} {X:0 Y:0} {X:0 Y:0} {X:0 Y:0}] Tint:[{Red:0 Green:0 Blue:0 Alpha:0} {Red:0 Green:0 Blue:0 Alpha:0} {Red:0 Green:0 Blue:0 Alpha:0} {Red:0 Green:0 Blue:0 Alpha:0}] HasQuad:false Layer:0 ClipOf:{X:0 Y:0 Width:0 Height:0} HasClip:false Order:6}",
	"text {Value:ended Bounds:{X:331 Y:103 Width:382 Height:19} Style:{Size:12 Weight:0 Italic:false Color:{Red:0.039215688 Green:0.039215688 Blue:0.039215688 Alpha:1} Mono:false MonoFace:0 Direction:0} CellWidth:0 Baseline:0 RunFirst:0 RunCount:0 Identity:{Owner:0 Item:0} Quad:[{X:0 Y:0} {X:0 Y:0} {X:0 Y:0} {X:0 Y:0}] Tint:[{Red:0 Green:0 Blue:0 Alpha:0} {Red:0 Green:0 Blue:0 Alpha:0} {Red:0 Green:0 Blue:0 Alpha:0} {Red:0 Green:0 Blue:0 Alpha:0}] HasQuad:false Layer:0 ClipOf:{X:0 Y:0 Width:0 Height:0} HasClip:false Order:7}",
	"text {Value:mono Bounds:{X:9 Y:126 Width:382 Height:15} Style:{Size:12 Weight:0 Italic:false Color:{Red:0.039215688 Green:0.039215688 Blue:0.039215688 Alpha:1} Mono:true MonoFace:0 Direction:0} CellWidth:7.2000003 Baseline:0 RunFirst:0 RunCount:1 Identity:{Owner:0 Item:0} Quad:[{X:0 Y:0} {X:0 Y:0} {X:0 Y:0} {X:0 Y:0}] Tint:[{Red:0 Green:0 Blue:0 Alpha:0} {Red:0 Green:0 Blue:0 Alpha:0} {Red:0 Green:0 Blue:0 Alpha:0} {Red:0 Green:0 Blue:0 Alpha:0}] HasQuad:false Layer:0 ClipOf:{X:0 Y:0 Width:0 Height:0} HasClip:false Order:8}",
	"text {Value:tall Bounds:{X:9 Y:145 Width:382 Height:30} Style:{Size:12 Weight:0 Italic:false Color:{Red:0.039215688 Green:0.039215688 Blue:0.039215688 Alpha:1} Mono:false MonoFace:0 Direction:0} CellWidth:0 Baseline:0 RunFirst:0 RunCount:0 Identity:{Owner:0 Item:0} Quad:[{X:0 Y:0} {X:0 Y:0} {X:0 Y:0} {X:0 Y:0}] Tint:[{Red:0 Green:0 Blue:0 Alpha:0} {Red:0 Green:0 Blue:0 Alpha:0} {Red:0 Green:0 Blue:0 Alpha:0} {Red:0 Green:0 Blue:0 Alpha:0}] HasQuad:false Layer:0 ClipOf:{X:0 Y:0 Width:0 Height:0} HasClip:false Order:9}",
	"text {Value:a paragraph of plain text that Bounds:{X:9 Y:179 Width:382 Height:19} Style:{Size:12 Weight:0 Italic:false Color:{Red:0.039215688 Green:0.039215688 Blue:0.039215688 Alpha:1} Mono:false MonoFace:0 Direction:1} CellWidth:0 Baseline:0 RunFirst:0 RunCount:0 Identity:{Owner:0 Item:0} Quad:[{X:0 Y:0} {X:0 Y:0} {X:0 Y:0} {X:0 Y:0}] Tint:[{Red:0 Green:0 Blue:0 Alpha:0} {Red:0 Green:0 Blue:0 Alpha:0} {Red:0 Green:0 Blue:0 Alpha:0} {Red:0 Green:0 Blue:0 Alpha:0}] HasQuad:false Layer:0 ClipOf:{X:0 Y:0 Width:0 Height:0} HasClip:false Order:10}",
	"text {Value:its container does not stretch, Bounds:{X:9 Y:198 Width:382 Height:19} Style:{Size:12 Weight:0 Italic:false Color:{Red:0.039215688 Green:0.039215688 Blue:0.039215688 Alpha:1} Mono:false MonoFace:0 Direction:1} CellWidth:0 Baseline:0 RunFirst:0 RunCount:0 Identity:{Owner:0 Item:0} Quad:[{X:0 Y:0} {X:0 Y:0} {X:0 Y:0} {X:0 Y:0}] Tint:[{Red:0 Green:0 Blue:0 Alpha:0} {Red:0 Green:0 Blue:0 Alpha:0} {Red:0 Green:0 Blue:0 Alpha:0} {Red:0 Green:0 Blue:0 Alpha:0}] HasQuad:false Layer:0 ClipOf:{X:0 Y:0 Width:0 Height:0} HasClip:false Order:11}",
	"text {Value:so it asks for its widest line Bounds:{X:9 Y:217 Width:382 Height:19} Style:{Size:12 Weight:0 Italic:false Color:{Red:0.039215688 Green:0.039215688 Blue:0.039215688 Alpha:1} Mono:false MonoFace:0 Direction:1} CellWidth:0 Baseline:0 RunFirst:0 RunCount:0 Identity:{Owner:0 Item:0} Quad:[{X:0 Y:0} {X:0 Y:0} {X:0 Y:0} {X:0 Y:0}] Tint:[{Red:0 Green:0 Blue:0 Alpha:0} {Red:0 Green:0 Blue:0 Alpha:0} {Red:0 Green:0 Blue:0 Alpha:0} {Red:0 Green:0 Blue:0 Alpha:0}] HasQuad:false Layer:0 ClipOf:{X:0 Y:0 Width:0 Height:0} HasClip:false Order:12}",
	"text {Value:שלום עולם Bounds:{X:283 Y:240 Width:382 Height:19} Style:{Size:12 Weight:0 Italic:false Color:{Red:0.039215688 Green:0.039215688 Blue:0.039215688 Alpha:1} Mono:false MonoFace:0 Direction:0} CellWidth:0 Baseline:0 RunFirst:0 RunCount:0 Identity:{Owner:0 Item:0} Quad:[{X:0 Y:0} {X:0 Y:0} {X:0 Y:0} {X:0 Y:0}] Tint:[{Red:0 Green:0 Blue:0 Alpha:0} {Red:0 Green:0 Blue:0 Alpha:0} {Red:0 Green:0 Blue:0 Alpha:0} {Red:0 Green:0 Blue:0 Alpha:0}] HasQuad:false Layer:0 ClipOf:{X:0 Y:0 Width:0 Height:0} HasClip:false Order:13}",
	"text {Value:in a row Bounds:{X:10 Y:264 Width:96 Height:19} Style:{Size:12 Weight:0 Italic:false Color:{Red:0.039215688 Green:0.039215688 Blue:0.039215688 Alpha:1} Mono:false MonoFace:0 Direction:0} CellWidth:0 Baseline:0 RunFirst:0 RunCount:0 Identity:{Owner:0 Item:0} Quad:[{X:0 Y:0} {X:0 Y:0} {X:0 Y:0} {X:0 Y:0}] Tint:[{Red:0 Green:0 Blue:0 Alpha:0} {Red:0 Green:0 Blue:0 Alpha:0} {Red:0 Green:0 Blue:0 Alpha:0} {Red:0 Green:0 Blue:0 Alpha:0}] HasQuad:false Layer:0 ClipOf:{X:0 Y:0 Width:0 Height:0} HasClip:false Order:22}",
	"text {Value:stretched Bounds:{X:106 Y:264 Width:284 Height:19} Style:{Size:12 Weight:0 Italic:false Color:{Red:0.039215688 Green:0.039215688 Blue:0.039215688 Alpha:1} Mono:false MonoFace:0 Direction:0} CellWidth:0 Baseline:0 RunFirst:0 RunCount:0 Identity:{Owner:0 Item:0} Quad:[{X:0 Y:0} {X:0 Y:0} {X:0 Y:0} {X:0 Y:0}] Tint:[{Red:0 Green:0 Blue:0 Alpha:0} {Red:0 Green:0 Blue:0 Alpha:0} {Red:0 Green:0 Blue:0 Alpha:0} {Red:0 Green:0 Blue:0 Alpha:0}] HasQuad:false Layer:0 ClipOf:{X:0 Y:0 Width:0 Height:0} HasClip:false Order:23}",
	"triangle {Points:[{X:9 Y:263} {X:391 Y:263} {X:390 Y:264}] Color:{Red:0 Green:0 Blue:0 Alpha:0.5} Shade:[{Red:0 Green:0 Blue:0 Alpha:0} {Red:0 Green:0 Blue:0 Alpha:0}] Shaded:false Layer:0 EdgeMask:1 ClipOf:{X:0 Y:0 Width:0 Height:0} HasClip:false Order:14}",
	"triangle {Points:[{X:9 Y:263} {X:390 Y:264} {X:10 Y:264}] Color:{Red:0 Green:0 Blue:0 Alpha:0.5} Shade:[{Red:0 Green:0 Blue:0 Alpha:0} {Red:0 Green:0 Blue:0 Alpha:0}] Shaded:false Layer:0 EdgeMask:2 ClipOf:{X:0 Y:0 Width:0 Height:0} HasClip:false Order:15}",
	"triangle {Points:[{X:391 Y:263} {X:391 Y:284} {X:390 Y:283}] Color:{Red:0 Green:0 Blue:0 Alpha:0.5} Shade:[{Red:0 Green:0 Blue:0 Alpha:0} {Red:0 Green:0 Blue:0 Alpha:0}] Shaded:false Layer:0 EdgeMask:1 ClipOf:{X:0 Y:0 Width:0 Height:0} HasClip:false Order:16}",
	"triangle {Points:[{X:391 Y:263} {X:390 Y:283} {X:390 Y:264}] Color:{Red:0 Green:0 Blue:0 Alpha:0.5} Shade:[{Red:0 Green:0 Blue:0 Alpha:0} {Red:0 Green:0 Blue:0 Alpha:0}] Shaded:false Layer:0 EdgeMask:2 ClipOf:{X:0 Y:0 Width:0 Height:0} HasClip:false Order:17}",
	"triangle {Points:[{X:391 Y:284} {X:9 Y:284} {X:10 Y:283}] Color:{Red:0 Green:0 Blue:0 Alpha:0.5} Shade:[{Red:0 Green:0 Blue:0 Alpha:0} {Red:0 Green:0 Blue:0 Alpha:0}] Shaded:false Layer:0 EdgeMask:1 ClipOf:{X:0 Y:0 Width:0 Height:0} HasClip:false Order:18}",
	"triangle {Points:[{X:391 Y:284} {X:10 Y:283} {X:390 Y:283}] Color:{Red:0 Green:0 Blue:0 Alpha:0.5} Shade:[{Red:0 Green:0 Blue:0 Alpha:0} {Red:0 Green:0 Blue:0 Alpha:0}] Shaded:false Layer:0 EdgeMask:2 ClipOf:{X:0 Y:0 Width:0 Height:0} HasClip:false Order:19}",
	"triangle {Points:[{X:9 Y:284} {X:9 Y:263} {X:10 Y:264}] Color:{Red:0 Green:0 Blue:0 Alpha:0.5} Shade:[{Red:0 Green:0 Blue:0 Alpha:0} {Red:0 Green:0 Blue:0 Alpha:0}] Shaded:false Layer:0 EdgeMask:1 ClipOf:{X:0 Y:0 Width:0 Height:0} HasClip:false Order:20}",
	"triangle {Points:[{X:9 Y:284} {X:10 Y:264} {X:10 Y:283}] Color:{Red:0 Green:0 Blue:0 Alpha:0.5} Shade:[{Red:0 Green:0 Blue:0 Alpha:0} {Red:0 Green:0 Blue:0 Alpha:0}] Shaded:false Layer:0 EdgeMask:2 ClipOf:{X:0 Y:0 Width:0 Height:0} HasClip:false Order:21}",
	"images 0",
}, "\n") + "\n"
