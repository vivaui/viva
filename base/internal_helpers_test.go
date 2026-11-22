package base

import (
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/style"
)

var (
	tableScope  = tree.Sub(page, tree.Name("table-rows"))
	headerScope = tree.Sub(page, tree.Name("table-headers"))
)

func sameCanvas(t *testing.T, Name string, flexed, direct *gfx.Canvas) {
	t.Helper()
	if len(flexed.Rectangles()) != len(direct.Rectangles()) || len(flexed.Texts()) != len(direct.Texts()) {
		t.Fatalf("%s: the flexed page and the direct one painted differently", Name)
	}
}

var wideSixColumns = []style.TableColumn{{Width: 120}, {Width: 120}, {Width: 120}, {Width: 120}, {Width: 120}, {Width: 120}}

var sixTitles = []string{"A", "B", "C", "D", "E", "F"}

var allSortableSix = []bool{true, true, true, true, true, true}

func hitByID(Surface *gfx.Canvas, ID gfx.ID) gfx.Hit {
	for _, hit := range Surface.Hits() {
		if hit.ID == ID {
			return hit
		}
	}
	return gfx.Hit{}
}

type countingRows struct {
	n     int
	reads int
}

func (c *countingRows) Len() int { return c.n }
func (c *countingRows) Cell(row, column int) string {
	c.reads++
	return "r"
}
