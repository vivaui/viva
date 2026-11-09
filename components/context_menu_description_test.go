package components

import (
	"strings"
	"testing"

	"github.com/vivaui/viva/gfx"
)

func TestContextMenuFitsAtFractionalOrigins(t *testing.T) {
	for _, scale := range []float32{1, 1.25, 1.5, 1.75, 2} {
		for _, origin := range []float32{0, 100.4, 100.9, 2048.1} {
			var canvas gfx.Canvas
			canvas.SetPixelScale(scale)
			scroll := MenuScroll{BarID: gfx.Item(99)}
			for range 3 {
				canvas.Reset()
				menu := BeginContextMenu(&canvas, gfx.LightTheme(), gfx.Rect{X: 20.2, Y: origin, Width: 220, Height: 65}, ContextMenuOptions{Scroll: &scroll, RowHeight: 24})
				menu.AddItem(ContextMenuItem{ID: gfx.Item(1), Label: "Open"})
				menu.AddSeparator()
				menu.AddItem(ContextMenuItem{ID: gfx.Item(2), Label: "Upload"})
				menu.End()
				if scroll.max != 0 {
					t.Fatalf("scale=%v origin=%v: false overflow %v", scale, origin, scroll.max)
				}
				for _, hit := range canvas.Hits() {
					if hit.ID == scroll.BarID {
						t.Fatal("exactly fitting menu has a scrollbar")
					}
				}
			}
		}
	}
}

func TestContextMenuRetainsRealSubpixelOverflow(t *testing.T) {
	for _, overflow := range []float32{0.25, 1, 12} {
		var canvas gfx.Canvas
		scroll := MenuScroll{BarID: gfx.Item(99)}
		menu := BeginContextMenu(&canvas, gfx.DarkTheme(), gfx.Rect{Y: 100.4, Width: 220, Height: 65 - overflow}, ContextMenuOptions{Scroll: &scroll, RowHeight: 24})
		menu.AddItem(ContextMenuItem{ID: gfx.Item(1), Label: "Open"})
		menu.AddSeparator()
		menu.AddItem(ContextMenuItem{ID: gfx.Item(2), Label: "Upload"})
		menu.End()
		if scroll.max != overflow {
			t.Fatalf("overflow=%v: scroll range=%v", overflow, scroll.max)
		}
		found := false
		for _, hit := range canvas.Hits() {
			found = found || hit.ID == scroll.BarID
		}
		if !found {
			t.Fatal("real overflow lost its scrollbar")
		}
	}
}

func TestContextMenuDescriptionMeasuresPaintsAndKeepsOneAction(t *testing.T) {
	for _, theme := range []gfx.Theme{gfx.LightTheme(), gfx.DarkTheme()} {
		var canvas gfx.Canvas
		item := ContextMenuItem{ID: gfx.Item(2), Label: "Upload", Description: "/home/test/" + strings.Repeat("very-long-folder/", 12), Hint: "full remote path"}
		height := ContextMenuItemHeight(theme, item, 24)
		if height <= 24 || ContextMenuItemHeight(theme, ContextMenuItem{Label: "Open"}, 24) != 24 {
			t.Fatal("description row height is wrong")
		}
		if ContextMenuItemWidth(&canvas, theme, item) <= ContextMenuItemWidth(&canvas, theme, ContextMenuItem{Label: "Upload"}) {
			t.Fatal("measurement ignored the description")
		}
		scroll := MenuScroll{BarID: gfx.Item(99)}
		menu := BeginContextMenu(&canvas, theme, gfx.Rect{Y: 100.4, Width: 220, Height: 8 + height + 24}, ContextMenuOptions{Scroll: &scroll, RowHeight: 24})
		row := menu.AddItem(item)
		next := menu.AddItem(ContextMenuItem{ID: gfx.Item(3), Label: "Next"})
		menu.End()
		if next.Y != row.Y+row.Height || scroll.max != 0 {
			t.Fatalf("description overlaps or scrolls: row=%+v next=%+v max=%v", row, next, scroll.max)
		}
		count, description := 0, false
		for _, hit := range canvas.Hits() {
			if hit.ID == item.ID {
				count++
				if hit.Hint != item.Hint {
					t.Fatal("full path hint lost")
				}
			}
		}
		for _, text := range canvas.Texts() {
			if strings.HasPrefix(text.Value, "/home/test/") {
				description = true
				if !strings.HasSuffix(text.Value, "…") || text.Bounds.Y <= row.Y || text.Bounds.Y+text.Bounds.Height > row.Y+row.Height {
					t.Fatalf("description not fitted below title: %+v", text)
				}
			}
		}
		if count != 1 || !description {
			t.Fatalf("action count=%v description=%v", count, description)
		}
	}
}

func TestContextMenuDescriptionRowGroupsTitleAndDetail(t *testing.T) {
	for _, theme := range []gfx.Theme{gfx.LightTheme(), gfx.DarkTheme()} {
		var canvas gfx.Canvas
		item := ContextMenuItem{ID: gfx.Item(2), Label: "Medium risk", Description: "Deletes still ask."}
		height := ContextMenuItemHeight(theme, item, 0)
		menu := BeginContextMenu(&canvas, theme, gfx.Rect{Y: 40, Width: 280, Height: 8 + height}, ContextMenuOptions{})
		row := menu.AddItem(item)
		menu.End()
		var titleY, titleHeight, detailY, detailHeight float32
		var title, detail bool
		for _, text := range canvas.Texts() {
			switch text.Value {
			case item.Label:
				title, titleY, titleHeight = true, text.Bounds.Y, text.Bounds.Height
			case item.Description:
				detail, detailY, detailHeight = true, text.Bounds.Y, text.Bounds.Height
			}
		}
		if !title || !detail {
			t.Fatalf("title=%v description=%v were not painted", title, detail)
		}
		if titleHeight != menuLineHeight(theme.Metrics.FontSizeM) || detailHeight != menuLineHeight(theme.Metrics.FontSizeS) {
			t.Fatalf("line heights title=%v detail=%v", titleHeight, detailHeight)
		}
		if gap := detailY - (titleY + titleHeight); gap != menuDescriptionGap {
			t.Fatalf("title-to-detail gap=%v, want %v", gap, menuDescriptionGap)
		}
		top := titleY - row.Y
		bottom := row.Y + row.Height - (detailY + detailHeight)
		if top != bottom || top <= 0 {
			t.Fatalf("row pads are not symmetric: top=%v bottom=%v", top, bottom)
		}
	}
}

func BenchmarkContextMenuDescription(b *testing.B) {
	var canvas gfx.Canvas
	theme := gfx.LightTheme()
	item := ContextMenuItem{ID: gfx.Item(1), Label: "Upload to current server folder", Description: "/home/test/a/long/path/that/needs/to/be/ellipsized"}
	paint := func() {
		canvas.Reset()
		menu := BeginContextMenu(&canvas, theme, gfx.Rect{Y: 100.4, Width: 240, Height: 80}, ContextMenuOptions{RowHeight: 24})
		menu.AddItem(item)
		menu.End()
	}
	paint()
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		paint()
	}
}
