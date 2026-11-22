package base

import (
	"testing"

	"github.com/vivaui/viva/gfx"
)

// The window size class is a function of the page's width alone, and AtLeast
// agrees with it at every class's first pixel and the one before it.
func TestSizeClassFollowsThePageWidth(t *testing.T) {
	theme := gfx.LightTheme()
	page := NewPage(PageConfig{Theme: &theme})
	canvas := testCanvas()
	classes := []Breakpoint{Compact, Medium, Expanded, Large, ExtraLarge}
	for index, class := range classes {
		for _, width := range []float32{class.minimum(), class.minimum() - 1} {
			if width < 0 {
				continue
			}
			want := class
			if width < class.minimum() {
				want = classes[index-1]
			}
			var got Breakpoint
			var reached []bool
			page.Draw(canvas, gfx.Rect{Width: width, Height: 100}, func(c C) {
				got = c.SizeClass()
				reached = reached[:0]
				for _, other := range classes {
					reached = append(reached, c.AtLeast(other))
				}
			})
			if got != want {
				t.Fatalf("a page %v wide is in class %d, want %d", width, got, want)
			}
			for i, other := range classes {
				if reached[i] != (other <= want) {
					t.Fatalf("a page %v wide: AtLeast(%d) = %v, but its class is %d", width, other, reached[i], want)
				}
			}
		}
	}
}

// From changes an element from its breakpoint on: a column of two blocks below
// Medium stacks them, and from Medium lays them side by side.
func TestFromTurnsAColumnIntoARowAtItsBreakpoint(t *testing.T) {
	theme := gfx.LightTheme()
	page := NewPage(PageConfig{Theme: &theme})
	canvas := testCanvas()
	var first, second Box
	view := func(c C) {
		Column(c).Gap(8).From(Medium, func(b Box) { b.Row().Gap(16) }).Children(func() {
			first = Column(c).Size(100, 40)
			second = Column(c).Size(100, 40)
		})
	}
	page.Draw(canvas, gfx.Rect{Width: Medium.minimum() - 1, Height: 300}, view)
	a, b := first.Rect(), second.Rect()
	if b.Y < a.Y+a.Height || b.X != a.X {
		t.Fatalf("below Medium the blocks are at %v and %v, want the second under the first", a, b)
	}
	page.Draw(canvas, gfx.Rect{Width: Medium.minimum(), Height: 300}, view)
	a, b = first.Rect(), second.Rect()
	if b.X < a.X+a.Width || b.Y != a.Y {
		t.Fatalf("from Medium the blocks are at %v and %v, want them side by side", a, b)
	}
}

// A container query reads the Container's width, not the window's, and is
// answered right in the first paint: the same card stacks in a narrow sidebar
// and lies in a row in the wide main area. Later paints declare the page once.
func TestContainerQueryIsAnsweredInTheFirstPaint(t *testing.T) {
	theme := gfx.LightTheme()
	page := NewPage(PageConfig{Theme: &theme})
	canvas := testCanvas()
	type card struct{ first, second Box }
	var narrow, wide card
	declarations := 0
	cardIn := func(c C, into *card) {
		Column(c).From(Width(480), func(b Box) { b.Row() }).Children(func() {
			into.first = Column(c).Size(100, 40)
			into.second = Column(c).Size(100, 40)
		})
	}
	view := func(c C) {
		declarations++
		Row(c).Fill().Children(func() {
			Container(c).Width(240).Children(func() { cardIn(c, &narrow) })
			Container(c).Grow(1).Children(func() { cardIn(c, &wide) })
		})
	}
	bounds := gfx.Rect{Width: 1000, Height: 400}
	page.Draw(canvas, bounds, view)
	stacked := func(k card) bool { return k.second.Rect().Y >= k.first.Rect().Y+k.first.Rect().Height }
	if !stacked(narrow) || stacked(wide) {
		t.Fatalf("first paint: narrow card stacked=%v, wide card stacked=%v; want the narrow one stacked and the wide one in a row", stacked(narrow), stacked(wide))
	}
	declarations = 0
	page.Draw(canvas, bounds, view)
	if declarations != 1 {
		t.Fatalf("a steady paint declared the page %d times", declarations)
	}
	if !stacked(narrow) || stacked(wide) {
		t.Fatal("a steady paint changed the answers")
	}
}

// A Container's width comes from where it is placed, never from what it holds,
// so what a query changes inside it cannot change what the query reads.
func TestContainerWidthDoesNotFollowItsContent(t *testing.T) {
	theme := gfx.LightTheme()
	page := NewPage(PageConfig{Theme: &theme})
	canvas := testCanvas()
	var container Box
	inner := float32(100)
	view := func(c C) {
		Row(c).Fill().Children(func() {
			container = Container(c).Pad(4).Children(func() { Column(c).Size(inner, 20) })
		})
	}
	bounds := gfx.Rect{Width: 800, Height: 100}
	page.Draw(canvas, bounds, view)
	before := container.Rect().Width
	inner = 500
	page.Draw(canvas, bounds, view)
	if after := container.Rect().Width; after != before {
		t.Fatalf("the container is %v wide holding 100 and %v holding 500", before, after)
	}
}

// Responsive changes cost nothing once warm.
func TestResponsivePaintDoesNotAllocate(t *testing.T) {
	theme := gfx.LightTheme()
	page := NewPage(PageConfig{Theme: &theme})
	canvas := testCanvas()
	view := func(c C) {
		Row(c).Fill().Children(func() {
			Container(c).Grow(1).Children(func() {
				Column(c).Gap(8).From(Medium, func(b Box) { b.Row().Gap(16) }).
					From(Width(480), func(b Box) { b.Pad(12) }).Children(func() {
					Column(c).Size(100, 40)
				})
			})
		})
	}
	bounds := gfx.Rect{Width: 900, Height: 300}
	for range 3 {
		page.Draw(canvas, bounds, view)
	}
	if allocations := testing.AllocsPerRun(50, func() { page.Draw(canvas, bounds, view) }); allocations != 0 {
		t.Fatalf("a steady responsive paint allocated %v times", allocations)
	}
}
