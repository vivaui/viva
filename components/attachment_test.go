package components

import (
	"slices"
	"testing"

	"github.com/vivaui/viva/gfx"
)

func TestAttachmentMirrorsStateSizeOrientationAndTrigger(t *testing.T) {
	theme := gfx.LightTheme()
	var canvas gfx.Canvas
	data := AttachmentData{Name: "cover.png", Description: "128 KB", Kind: AttachmentImage, ResourceID: 99, State: AttachmentError, Size: AttachmentSizeSmall, Orientation: AttachmentVertical}
	BeginAttachment(&canvas, theme, gfx.Item(8), gfx.Rect{Width: 120, Height: 110}, data).Paint()
	if len(canvas.Hits()) != 1 || canvas.Hits()[0].Label != "cover.png" {
		t.Fatalf("attachment hits = %+v", canvas.Hits())
	}
	if canvas.Rectangles()[0].Color.Alpha != 0.30 || canvas.Texts()[1].Style.Color.Red != theme.Danger.Red {
		t.Fatalf("error attachment commands = %+v / %+v", canvas.Rectangles(), canvas.Texts())
	}
}

func TestAttachmentVerticalMediaIsASquareAsWideAsTheCard(t *testing.T) {
	theme := gfx.LightTheme()
	var canvas gfx.Canvas
	attachment := BeginAttachment(&canvas, theme, gfx.Item(1), gfx.Rect{}, AttachmentData{Name: "cover.png", Description: "128 KB", Kind: AttachmentImage, Orientation: AttachmentVertical})
	size := attachment.Size()
	attachment.Paint()
	var media gfx.Rect
	for _, rectangle := range canvas.Rectangles() {
		if rectangle.Color == theme.Muted {
			media = rectangle.Bounds
		}
	}
	if media.Width != media.Height || media.Width <= size.Width/2 || media.Width >= size.Width {
		t.Fatalf("media %+v in a %+v card", media, size)
	}
}

func TestAttachmentActionsSitOnTopOfTheTrigger(t *testing.T) {
	theme := gfx.LightTheme()
	for _, orientation := range []AttachmentOrientation{AttachmentHorizontal, AttachmentVertical} {
		var canvas gfx.Canvas
		bounds := gfx.Rect{X: 10, Y: 20}
		BeginAttachment(&canvas, theme, gfx.Item(1), bounds, AttachmentData{Name: "report.pdf", Description: "2 MB", Orientation: orientation}).
			Action(AttachmentAction{ID: gfx.Item(2), Label: "Retry"}).
			Action(AttachmentAction{ID: gfx.Item(3), Label: "Remove"}).Paint()
		hits := canvas.Hits()
		if len(hits) != 3 || hits[0].ID != gfx.Item(1) || hits[1].Label != "Retry" || hits[2].Label != "Remove" {
			t.Fatalf("orientation %d: hits %+v", orientation, hits)
		}
		card := hits[0].Bounds
		for _, hit := range hits[1:] {
			inside := hit.Bounds.X >= card.X && hit.Bounds.Y >= card.Y &&
				hit.Bounds.X+hit.Bounds.Width <= card.X+card.Width && hit.Bounds.Y+hit.Bounds.Height <= card.Y+card.Height
			if !inside {
				t.Fatalf("orientation %d: action %+v outside the card %+v", orientation, hit.Bounds, card)
			}
		}
		if hits[1].Bounds.X+hits[1].Bounds.Width > hits[2].Bounds.X {
			t.Fatalf("orientation %d: actions overlap: %+v %+v", orientation, hits[1].Bounds, hits[2].Bounds)
		}
	}
}

func TestAttachmentUploadingShowsTheRealProgress(t *testing.T) {
	theme := gfx.LightTheme()
	bar := func(progress float32) (width float32, found bool) {
		var canvas gfx.Canvas
		BeginAttachment(&canvas, theme, gfx.ID{}, gfx.Rect{Width: 200}, AttachmentData{Name: "a.mov", State: AttachmentUploading, Progress: progress}).Paint()
		for _, rectangle := range canvas.Rectangles() {
			if rectangle.Bounds.Height == 2 {
				return rectangle.Bounds.Width, true
			}
		}
		return 0, false
	}
	half, ok := bar(0.5)
	full, _ := bar(1)
	if !ok || half <= 0 || full < half*1.9 || full > half*2.1 {
		t.Fatalf("half = %v, full = %v", half, full)
	}
	if _, found := bar(0); found {
		t.Fatal("an upload that has sent nothing drew a bar")
	}
}

func TestAttachmentGroupPlacesCardsInARowAndMeasuresTheScroll(t *testing.T) {
	theme := gfx.LightTheme()
	var canvas gfx.Canvas
	group := BeginAttachmentGroup(theme, gfx.Rect{X: 5, Y: 10, Width: 300})
	first := BeginAttachment(&canvas, theme, gfx.ID{}, gfx.Rect{}, AttachmentData{Name: "a"})
	second := BeginAttachment(&canvas, theme, gfx.ID{}, gfx.Rect{}, AttachmentData{Name: "a much longer file name than the first one has.txt"})
	firstSlot := group.Next(first.Size())
	secondSlot := group.Next(second.Size())
	if secondSlot.X <= firstSlot.X+firstSlot.Width || firstSlot.X != 5 {
		t.Fatalf("slots %+v %+v", firstSlot, secondSlot)
	}
	if second.Size().Width <= first.Size().Width {
		t.Fatal("a longer name did not widen its card")
	}
	if group.Extent() != secondSlot.X+secondSlot.Width-5 {
		t.Fatalf("extent %v", group.Extent())
	}
}

func TestAttachmentIdleAndProcessingCommandsAreStable(t *testing.T) {
	var canvas gfx.Canvas
	paint := func() {
		canvas.Reset()
		data := AttachmentData{Name: "report.pdf", Description: "2.4 MB", Kind: AttachmentFile, State: AttachmentIdle, Size: AttachmentSizeExtraSmall}
		BeginAttachment(&canvas, gfx.DarkTheme(), gfx.Item(3), gfx.Rect{Width: 200, Height: 38}, data).Disabled().Paint()
	}
	paint()
	rectangles := slices.Clone(canvas.Rectangles())
	texts := slices.Clone(canvas.Texts())
	paint()
	if len(canvas.Hits()) != 0 || len(canvas.Rectangles()) < 8 {
		t.Fatal("idle dashed attachment did not remain inert and explicit")
	}
	if !slices.Equal(rectangles, canvas.Rectangles()) || !slices.Equal(texts, canvas.Texts()) {
		t.Fatal("identical attachments produced different commands")
	}
}
