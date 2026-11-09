package components

import (
	"slices"
	"testing"

	"github.com/vivaui/viva/gfx"
)

func TestPaginationUsesGhostLinksAndOutlinedCurrentPage(t *testing.T) {
	theme := gfx.LightTheme()
	var canvas gfx.Canvas
	pagination := BeginPagination(&canvas, theme, gfx.Rect{Width: 340})
	pagination.AddPrevious(gfx.Item(1), "Previous", true)
	pagination.AddPage(gfx.Item(2), "1", false)
	pagination.AddPage(gfx.Item(3), "2", true)
	pagination.AddEllipsis()
	pagination.AddNext(gfx.Item(4), "Next", false)
	if len(canvas.Hits()) != 3 || canvas.Hits()[0].ID != gfx.Item(2) || canvas.Hits()[1].ID != gfx.Item(3) || canvas.Hits()[2].ID != gfx.Item(4) {
		t.Fatalf("pagination hits = %+v", canvas.Hits())
	}
	if len(canvas.Rectangles()) != 2 || canvas.Rectangles()[0].Color != theme.Border || canvas.Rectangles()[1].Color != theme.Background {
		t.Fatalf("current outline commands = %+v", canvas.Rectangles())
	}
	if canvas.Texts()[0].Value != "Previous" || canvas.Texts()[len(canvas.Texts())-1].Value != "Next" || len(canvas.Triangles()) == 0 {
		t.Fatalf("pagination controls = %+v", canvas.Texts())
	}
	if want := theme.Metrics.ControlHeight; canvas.Hits()[0].Bounds.Height != want || canvas.Hits()[0].Bounds.Width != want {
		t.Fatalf("page item is %+v, want a default-button-high square (%v)", canvas.Hits()[0].Bounds, want)
	}
}

func TestPaginationCommandStreamIsStable(t *testing.T) {
	var canvas gfx.Canvas
	paint := func() {
		canvas.Reset()
		pagination := BeginPagination(&canvas, gfx.DarkTheme(), gfx.Rect{Width: 280})
		pagination.AddPrevious(gfx.Item(1), "Prev", false)
		pagination.AddPage(gfx.Item(2), "1", true)
		pagination.AddPage(gfx.Item(3), "2", false)
		pagination.AddNext(gfx.Item(4), "Next", false)
	}
	paint()
	rectangles := slices.Clone(canvas.Rectangles())
	texts := slices.Clone(canvas.Texts())
	hits := slices.Clone(canvas.Hits())
	paint()
	if !slices.Equal(rectangles, canvas.Rectangles()) || !slices.Equal(texts, canvas.Texts()) || !slices.Equal(hits, canvas.Hits()) {
		t.Fatal("identical pagination produced different command streams")
	}
}
