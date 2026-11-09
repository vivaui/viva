package components

import (
	"testing"
	"time"

	"github.com/vivaui/viva/gfx"
)

// A menu's typeahead starts after the row the keyboard was on, ignores case and
// disabled rows, wraps, and scrolls the row it reaches into view. The caller owns
// the typed text and its 500ms session; Match only resolves one query.
func TestMenuScrollMatchFindsLabelsByPrefix(t *testing.T) {
	var scroll MenuScroll
	var canvas gfx.Canvas
	labels := []string{"Save", "Save As", "Open", "Close", "Delete", "Select All"}
	paint := func() {
		canvas.Reset()
		menu := BeginDropdownMenu(&canvas, gfx.DarkTheme(), gfx.Rect{X: 10, Y: 20, Width: 160, Height: 6*32 + 8}, DropdownMenuOptions{Layer: 7, Scope: gfx.Item(42), Scroll: &scroll})
		for index, label := range labels {
			menu.AddItem(DropdownMenuItem{ID: gfx.Item(100 + index), Label: label, Disabled: label == "Delete"})
		}
		menu.End()
	}
	paint()
	for _, test := range []struct {
		name   string
		prefix string
		from   int // -1 for no current row
		want   int // -1 for no match
	}{
		{"first matching row", "s", -1, 0},
		{"after the current row", "s", 0, 1},
		{"wraps past the last row", "s", 1, 5},
		{"ignores case", "SA", -1, 0},
		{"matches a longer prefix", "sel", -1, 5},
		{"a disabled row is not a match", "d", -1, -1},
		{"an empty query matches nothing", "", -1, -1},
		{"an unknown prefix matches nothing", "z", -1, -1},
	} {
		t.Run(test.name, func(t *testing.T) {
			from := gfx.ID{}
			if test.from >= 0 {
				from = gfx.Item(100 + test.from)
			}
			id, ok := scroll.Match(test.prefix, from)
			if test.want < 0 {
				if ok {
					t.Fatalf("Match(%q) = %v, want no match", test.prefix, id)
				}
				return
			}
			if !ok || id != gfx.Item(100+test.want) {
				t.Fatalf("Match(%q) = (%v, %v), want item %d", test.prefix, id, ok, test.want)
			}
		})
	}
}

func TestMenuScrollMatchScrollsTheRowIntoView(t *testing.T) {
	var scroll MenuScroll
	var canvas gfx.Canvas
	labels := []string{"Item 0", "Item 1", "Item 2", "Item 3", "Item 4", "Item 5", "Item 6", "Item 7"}
	paint := func() map[gfx.ID]gfx.Rect {
		canvas.Reset()
		menu := BeginDropdownMenu(&canvas, gfx.DarkTheme(), gfx.Rect{X: 10, Y: 20, Width: 160, Height: 104}, DropdownMenuOptions{Layer: 7, Scope: gfx.Item(42), Scroll: &scroll})
		for index, label := range labels {
			menu.AddItem(DropdownMenuItem{ID: gfx.Item(100 + index), Label: label})
		}
		menu.End()
		hits := map[gfx.ID]gfx.Rect{}
		for _, hit := range canvas.Hits() {
			hits[hit.ID] = hit.Bounds
		}
		return hits
	}
	if hits := paint(); hits[gfx.Item(107)].Height > 0 {
		t.Fatal("the last row shows before the menu scrolled")
	}
	id, ok := scroll.Match("Item 7", gfx.ID{})
	if !ok || id != gfx.Item(107) {
		t.Fatalf("Match = (%v, %v)", id, ok)
	}
	bounds, shown := paint()[gfx.Item(107)]
	if !shown || bounds.Y < 20 || bounds.Y+bounds.Height > 124 {
		t.Fatalf("the matched row is at %+v, outside the menu", bounds)
	}
}

// Typeahead compares runes, so a label in any script matches by its first
// character rather than by a byte prefix.
func TestMenuScrollMatchUsesRunes(t *testing.T) {
	var scroll MenuScroll
	var canvas gfx.Canvas
	menu := BeginDropdownMenu(&canvas, gfx.DarkTheme(), gfx.Rect{Width: 160, Height: 80}, DropdownMenuOptions{Layer: 7, Scroll: &scroll})
	menu.AddItem(DropdownMenuItem{ID: gfx.Item(1), Label: "设置"})
	menu.AddItem(DropdownMenuItem{ID: gfx.Item(2), Label: "关闭"})
	menu.End()
	if id, ok := scroll.Match("关", gfx.ID{}); !ok || id != gfx.Item(2) {
		t.Fatalf("Match(关) = (%v, %v)", id, ok)
	}
}

// A typeahead session appends keys for half a second, starts after the row the
// keyboard was on, cycles a repeated first letter, and treats Space as part of
// the query only while it is running.
func TestMenuTypeaheadTypesAndCycles(t *testing.T) {
	var scroll MenuScroll
	var canvas gfx.Canvas
	menu := BeginDropdownMenu(&canvas, gfx.DarkTheme(), gfx.Rect{Width: 160, Height: 5*32 + 8}, DropdownMenuOptions{Layer: 7, Scroll: &scroll})
	for index, label := range []string{"Save", "Save As", "Open", "Select All", "Delete"} {
		menu.AddItem(DropdownMenuItem{ID: gfx.Item(100 + index), Label: label, Disabled: label == "Delete"})
	}
	menu.End()
	now := time.Unix(0, 0)
	var typeahead MenuTypeahead
	typeKey := func(text string, at time.Time) (gfx.ID, bool) {
		t.Helper()
		return typeahead.Type(text, at, &scroll, gfx.ID{})
	}
	if id, ok := typeKey("s", now); !ok || id != gfx.Item(100) {
		t.Fatalf("s = (%v, %v), want Save", id, ok)
	}
	// A second letter narrows the same query rather than starting over.
	if id, ok := typeKey("e", now.Add(10*time.Millisecond)); !ok || id != gfx.Item(103) {
		t.Fatalf("se = (%v, %v), want Select All", id, ok)
	}
	if _, ok := typeKey(" ", now.Add(20*time.Millisecond)); ok {
		t.Fatal("a trailing Space was taken as typeahead")
	}
	if typeahead.Typing(now.Add(30 * time.Millisecond)) {
		t.Fatal("a query that matched nothing left the session running")
	}

	// A fresh session cycles through rows with the same initial.
	typeahead.Reset()
	for index, want := range []gfx.ID{gfx.Item(100), gfx.Item(101), gfx.Item(103), gfx.Item(100)} {
		id, ok := typeKey("s", now.Add(time.Duration(index)*time.Millisecond))
		if !ok || id != want {
			t.Fatalf("repeated s #%d = (%v, %v), want %v", index+1, id, ok, want)
		}
	}

	// Half a second later the same key starts a new session instead of cycling.
	typeahead.Reset()
	if id, _ := typeKey("s", now); id != gfx.Item(100) {
		t.Fatalf("first s = %v", id)
	}
	if id, _ := typeKey("s", now.Add(MenuTypeaheadReset+time.Millisecond)); id != gfx.Item(100) {
		t.Fatalf("s after the reset = %v, want a new session at Save", id)
	}
}

// Space alone is activation, not a one-character query.
func TestMenuTypeaheadLeavesASpaceToActivation(t *testing.T) {
	var scroll MenuScroll
	var canvas gfx.Canvas
	menu := BeginDropdownMenu(&canvas, gfx.DarkTheme(), gfx.Rect{Width: 160, Height: 40}, DropdownMenuOptions{Layer: 7, Scroll: &scroll})
	menu.AddItem(DropdownMenuItem{ID: gfx.Item(1), Label: "Open"})
	menu.End()
	var typeahead MenuTypeahead
	if id, ok := typeahead.Type(" ", time.Unix(0, 0), &scroll, gfx.ID{}); ok || !id.IsZero() {
		t.Fatalf("Space = (%v, %v), want activation", id, ok)
	}
	if id, ok := typeahead.Type("ab", time.Unix(0, 0), &scroll, gfx.ID{}); ok || !id.IsZero() {
		t.Fatalf("a multi-rune key = (%v, %v)", id, ok)
	}
}

// A doubled first letter is a real query when a row has one, so Base UI does
// not cycle: the second key keeps the row it already matched rather than
// jumping to another row with the same initial.
func TestMenuTypeaheadDoesNotCyclePastADoubledInitial(t *testing.T) {
	var scroll MenuScroll
	var canvas gfx.Canvas
	menu := BeginDropdownMenu(&canvas, gfx.DarkTheme(), gfx.Rect{Width: 160, Height: 3*32 + 8}, DropdownMenuOptions{Layer: 7, Scroll: &scroll})
	for index, label := range []string{"Aaron", "Alice"} {
		menu.AddItem(DropdownMenuItem{ID: gfx.Item(100 + index), Label: label})
	}
	menu.End()
	now := time.Unix(0, 0)
	var typeahead MenuTypeahead
	if id, ok := typeahead.Type("a", now, &scroll, gfx.ID{}); !ok || id != gfx.Item(100) {
		t.Fatalf("a = (%v, %v)", id, ok)
	}
	if id, ok := typeahead.Type("a", now.Add(time.Millisecond), &scroll, gfx.ID{}); !ok || id != gfx.Item(100) {
		t.Fatalf("second a = (%v, %v), want Aaron to stay matched", id, ok)
	}
}

// TypeItems matches a closed control's options without a painted list, which is
// how a select trigger commits a typed match.
func TestMenuTypeaheadTypeItemsMatchesAClosedList(t *testing.T) {
	now := time.Unix(0, 0)
	items := []MenuTypeaheadItem{
		{ID: gfx.Item(1), Label: "zsh"},
		{ID: gfx.Item(2), Label: "bash"},
		{ID: gfx.Item(3), Label: "fish"},
		{ID: gfx.Item(4), Label: "disabled", Disabled: true},
	}
	var typeahead MenuTypeahead
	if id, ok := typeahead.TypeItems("f", now, items, gfx.ID{}); !ok || id != gfx.Item(3) {
		t.Fatalf("f = (%v, %v), want fish", id, ok)
	}
	typeahead.Reset()
	if id, ok := typeahead.TypeItems("b", now, items, gfx.ID{}); !ok || id != gfx.Item(2) {
		t.Fatalf("b = (%v, %v), want bash", id, ok)
	}
	typeahead.Reset()
	if id, ok := typeahead.TypeItems("d", now, items, gfx.ID{}); ok || !id.IsZero() {
		t.Fatalf("disabled option matched: (%v, %v)", id, ok)
	}
	// Repeating the initial cycles through rows that share it.
	typeahead.Reset()
	items[0].Label = "sh"
	items[1].Label = "shell"
	items[2].Label = "fish"
	for index, want := range []gfx.ID{gfx.Item(1), gfx.Item(2), gfx.Item(1)} {
		id, ok := typeahead.TypeItems("s", now.Add(time.Duration(index)*time.Millisecond), items, gfx.ID{})
		if !ok || id != want {
			t.Fatalf("s #%d = (%v, %v), want %v", index+1, id, ok, want)
		}
	}
}
