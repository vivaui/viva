package components

import (
	"slices"
	"testing"

	"github.com/vivaui/viva/gfx"
)

func TestMessageSeparatesStructureFromBubbleContent(t *testing.T) {
	theme := gfx.LightTheme()
	var canvas gfx.Canvas
	regions := Message(&canvas, theme, gfx.Rect{Width: 280, Height: 96}, MessageData{Header: "Ada · 09:41", Footer: "Delivered", Initials: "AL", Alignment: MessageStart, ShowAvatar: true})
	if regions.Content.Width >= 280 || regions.Avatar.Width != theme.Metrics.ControlHeight {
		t.Fatalf("message regions = %+v", regions)
	}
	if len(canvas.Texts()) != 3 || len(canvas.Rectangles()) != 1 {
		t.Fatalf("message metadata commands = %d rectangles, %d texts", len(canvas.Rectangles()), len(canvas.Texts()))
	}
	Bubble(&canvas, theme, regions.Content, BubbleStart, BubbleMuted, "Ready")
	if canvas.Texts()[3].Value != "Ready" {
		t.Fatal("message content slot did not compose with bubble")
	}
}

func TestMessageEndAlignmentAndCommandsAreStable(t *testing.T) {
	var canvas gfx.Canvas
	paint := func() MessageRegions {
		canvas.Reset()
		return Message(&canvas, gfx.DarkTheme(), gfx.Rect{Width: 260, Height: 80}, MessageData{Header: "You", Footer: "Sent", Initials: "Y", Alignment: MessageEnd, ShowAvatar: true})
	}
	regions := paint()
	if regions.Avatar.X <= regions.Content.X {
		t.Fatalf("end-aligned avatar/content = %+v", regions)
	}
	rectangles := slices.Clone(canvas.Rectangles())
	texts := slices.Clone(canvas.Texts())
	paint()
	if !slices.Equal(rectangles, canvas.Rectangles()) || !slices.Equal(texts, canvas.Texts()) {
		t.Fatal("identical messages produced different command streams")
	}
}

func TestMessageEndAlignsFooterToTheEnd(t *testing.T) {
	theme := gfx.LightTheme()
	for _, alignment := range []MessageAlignment{MessageStart, MessageEnd} {
		var canvas gfx.Canvas
		regions := Message(&canvas, theme, gfx.Rect{Width: 300, Height: 96}, MessageData{Footer: "Sent", Alignment: alignment})
		texts := canvas.Texts()
		if len(texts) != 1 {
			t.Fatalf("texts = %d", len(texts))
		}
		left := texts[0].Bounds.X
		right := left + canvas.TextWidthWeight("Sent", theme.Metrics.FontSizeS, gfx.WeightMedium)
		inset := theme.Metrics.PadControlX
		if alignment == MessageStart && left != regions.Footer.X+inset {
			t.Fatalf("start footer begins at %v, want %v", left, regions.Footer.X+inset)
		}
		if alignment == MessageEnd && absDiff(right, regions.Footer.X+regions.Footer.Width-inset) > 0.5 {
			t.Fatalf("end footer ends at %v, want %v", right, regions.Footer.X+regions.Footer.Width-inset)
		}
	}
}

func TestMessageReservedAvatarLinesUpConsecutiveMessages(t *testing.T) {
	theme := gfx.LightTheme()
	var canvas gfx.Canvas
	first := Message(&canvas, theme, gfx.Rect{Width: 300, Height: 64}, MessageData{ShowAvatar: true, Initials: "AL"})
	canvas.Reset()
	next := Message(&canvas, theme, gfx.Rect{Width: 300, Height: 64}, MessageData{ReserveAvatar: true})
	if next.Content != first.Content {
		t.Fatalf("reserved content %+v, avatar content %+v", next.Content, first.Content)
	}
	if len(canvas.Rectangles()) != 0 {
		t.Fatal("a reserved avatar slot was painted")
	}
}

func TestMessageGroupStacksRowsWithTheGroupGap(t *testing.T) {
	theme := gfx.LightTheme()
	group := BeginMessageGroup(theme, gfx.Rect{X: 10, Y: 20, Width: 300})
	a, b := group.Row(40), group.Row(50)
	if b.Y != a.Y+a.Height+MessageGroupGap(theme.Metrics) || a.X != 10 || group.Height() != 40+MessageGroupGap(theme.Metrics)+50 {
		t.Fatalf("rows %+v %+v height %v", a, b, group.Height())
	}
}

func absDiff(a, b float32) float32 {
	if a > b {
		return a - b
	}
	return b - a
}
