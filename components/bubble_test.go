package components

import (
	"slices"
	"strings"
	"testing"

	"github.com/vivaui/viva/gfx"
)

func TestBubbleMirrorsOfficialVariantsWithoutSpeechTail(t *testing.T) {
	theme := gfx.LightTheme()
	variants := []BubbleVariant{BubbleDefault, BubbleSecondary, BubbleMuted, BubbleTinted, BubbleOutline, BubbleGhost, BubbleDestructive}
	var canvas gfx.Canvas
	for index, variant := range variants {
		Bubble(&canvas, theme, gfx.Rect{X: 0, Y: float32(index * 44), Width: 200, Height: 40}, BubbleEnd, variant, "Hello")
	}
	if len(canvas.Texts()) != len(variants) {
		t.Fatalf("bubble texts = %d", len(canvas.Texts()))
	}
	if canvas.Rectangles()[0].Color != theme.Primary {
		t.Fatalf("default bubble = %+v", canvas.Rectangles()[0])
	}
	if canvas.Texts()[4].Style.Color != theme.Foreground || canvas.Texts()[6].Style.Color != theme.Danger {
		t.Fatalf("bubble variant colors = %+v", canvas.Texts())
	}
	if canvas.Texts()[0].Bounds.X <= 0 {
		t.Fatal("end-aligned bubble stayed at start")
	}
}

func TestBubbleReactionAndCommandsAreStable(t *testing.T) {
	var canvas gfx.Canvas
	paint := func() {
		canvas.Reset()
		Bubble(&canvas, gfx.DarkTheme(), gfx.Rect{Width: 220, Height: 40}, BubbleStart, BubbleOutline, "Review")
		BubbleReaction(&canvas, gfx.DarkTheme(), gfx.Item(9), gfx.Rect{Width: 176, Height: 40}, BubbleReactionBottom, BubbleEnd, "👍 2")
	}
	paint()
	rectangles := slices.Clone(canvas.Rectangles())
	texts := slices.Clone(canvas.Texts())
	hits := slices.Clone(canvas.Hits())
	paint()
	if len(canvas.Hits()) != 1 || !slices.Equal(rectangles, canvas.Rectangles()) || !slices.Equal(texts, canvas.Texts()) || !slices.Equal(hits, canvas.Hits()) {
		t.Fatal("bubble reaction commands are unstable")
	}
}

func TestBubbleShrinksToShortTextAndWrapsLongText(t *testing.T) {
	theme := gfx.LightTheme()
	var canvas gfx.Canvas
	row := gfx.Rect{Width: 400}
	short := Bubble(&canvas, theme, row, BubbleStart, BubbleMuted, "Hi")
	if short.Width >= row.Width*0.5 || len(canvas.Texts()) != 1 {
		t.Fatalf("short bubble %+v with %d lines", short, len(canvas.Texts()))
	}
	canvas.Reset()
	long := Bubble(&canvas, theme, row, BubbleEnd, BubbleMuted, strings.Repeat("a long sentence that has to wrap ", 12))
	if long.Width != row.Width*0.80 || long.X+long.Width != row.Width || len(canvas.Texts()) < 2 {
		t.Fatalf("long bubble %+v with %d lines", long, len(canvas.Texts()))
	}
	if long.Height <= short.Height {
		t.Fatalf("a wrapped bubble is not taller: %v vs %v", long.Height, short.Height)
	}
	if size := MeasureBubble(&canvas, theme, row.Width, BubbleMuted, strings.Repeat("a long sentence that has to wrap ", 12)); size.Width != long.Width || size.Height != long.Height {
		t.Fatalf("MeasureBubble %+v disagrees with the painted bubble %+v", size, long)
	}
}

func TestBubbleSecondaryUsesTheSecondaryTokens(t *testing.T) {
	theme := gfx.LightTheme()
	var canvas gfx.Canvas
	Bubble(&canvas, theme, gfx.Rect{Width: 200}, BubbleStart, BubbleSecondary, "Hello")
	if canvas.Rectangles()[0].Color != theme.Secondary || canvas.Texts()[0].Style.Color != theme.SecondaryForeground {
		t.Fatalf("secondary bubble paints %+v / %+v", canvas.Rectangles()[0].Color, canvas.Texts()[0].Style.Color)
	}
}
