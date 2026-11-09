package components

import (
	"slices"
	"testing"
	"time"

	"github.com/vivaui/viva/gfx"
)

func TestDropdownMenuCapturesOverlayContractAndItemStates(t *testing.T) {
	theme := gfx.LightTheme()
	var canvas gfx.Canvas
	canvas.SetInteraction(gfx.ID{}, gfx.ID{}, gfx.Item(14), true)
	menu := BeginDropdownMenu(&canvas, theme, gfx.Rect{X: 10, Y: 20, Width: 100, Height: 180}, DropdownMenuOptions{Layer: 7, Scope: gfx.Item(42), DismissID: gfx.Item(91)})
	if menu.Bounds().Width != 128 || canvas.TrappedFocusScope() != gfx.Item(42) || canvas.EscapeTarget() != gfx.Item(91) {
		t.Fatalf("overlay contract = %+v scope=%d escape=%d", menu.Bounds(), canvas.TrappedFocusScope(), canvas.EscapeTarget())
	}
	checkboxBounds := menu.AddItem(DropdownMenuItem{ID: gfx.Item(11), Label: "Status bar", Kind: DropdownMenuCheckbox, Checked: true})
	menu.AddItem(DropdownMenuItem{ID: gfx.Item(12), Label: "Comfortable", Value: "comfortable", Kind: DropdownMenuRadio, Selected: true})
	menu.AddItem(DropdownMenuItem{ID: gfx.Item(13), Label: "Unavailable", Disabled: true})
	menu.AddItem(DropdownMenuItem{ID: gfx.Item(14), Label: "Delete", Variant: DropdownMenuItemDestructive, Shortcut: "⌘⌫"})
	menu.AddItem(DropdownMenuItem{ID: gfx.Item(15), Label: "More", Kind: DropdownMenuSubmenu, Expanded: true})

	if checkboxBounds != (gfx.Rect{X: 14, Y: 24, Width: 120, Height: 32}) {
		t.Fatalf("first item bounds = %+v", checkboxBounds)
	}
	for _, rectangle := range canvas.Rectangles() {
		if rectangle.Layer != 7 {
			t.Fatalf("rectangle escaped overlay layer: %+v", rectangle)
		}
	}
	for _, text := range canvas.Texts() {
		if text.Layer != 7 {
			t.Fatalf("text escaped overlay layer: %+v", text)
		}
	}
	hits := nonBlockHits(canvas.Hits())
	if len(hits) != 5 {
		t.Fatalf("hits = %+v", hits)
	}
	want := []struct {
		checked, selected, disabled, expanded, focusable bool
	}{
		{checked: true, focusable: true},
		{selected: true, focusable: true},
		{disabled: true},
		{focusable: true},
		{expanded: true, focusable: true},
	}
	for index, hit := range hits {
		state := want[index]
		if hit.Role != gfx.RoleMenuItem || hit.Scope != gfx.Item(42) || hit.Checked != state.checked || hit.Selected != state.selected || hit.Disabled != state.disabled || hit.Expanded != state.expanded || hit.Focusable != state.focusable {
			t.Fatalf("hit %d = %+v", index, hit)
		}
	}
	deleteText := dropdownMenuTestText(canvas.Texts(), "Delete")
	if deleteText.Style.Color != theme.Danger {
		t.Fatalf("destructive text = %+v", deleteText.Style.Color)
	}
}

func TestDropdownMenuGeometryAndCommandStreamAreDeterministic(t *testing.T) {
	var canvas gfx.Canvas
	paint := func() {
		canvas.Reset()
		menu := BeginDropdownMenu(&canvas, gfx.DarkTheme(), gfx.Rect{X: 5, Y: 6, Width: 160, Height: 118}, DropdownMenuOptions{Layer: 3, Scope: gfx.Item(8), DismissID: gfx.Item(9)})
		if got := menu.NextItemBounds(); got != (gfx.Rect{X: 9, Y: 10, Width: 152, Height: 32}) {
			t.Fatalf("next item = %+v", got)
		}
		menu.AddLabel("View", false)
		menu.AddSeparator()
		menu.AddItem(DropdownMenuItem{ID: gfx.Item(1), Label: "Reload", Shortcut: "⌘R"})
	}
	paint()
	rectangles := slices.Clone(canvas.Rectangles())
	texts := slices.Clone(canvas.Texts())
	hits := slices.Clone(canvas.Hits())
	paint()
	if !slices.Equal(rectangles, canvas.Rectangles()) || !slices.Equal(texts, canvas.Texts()) || !slices.Equal(hits, canvas.Hits()) {
		t.Fatal("identical dropdown menus produced different command streams")
	}
	if allocations := testing.AllocsPerRun(100, paint); allocations != 0 {
		t.Fatalf("dropdown menu paint allocations = %v", allocations)
	}
}

func dropdownMenuTestText(texts []gfx.Text, value string) gfx.Text {
	for _, text := range texts {
		if text.Value == value {
			return text
		}
	}
	return gfx.Text{}
}

func TestDropdownMenuStateTransitionsAreAllocationFree(t *testing.T) {
	var state DropdownMenuState
	if state.IsOpen() {
		t.Fatal("zero state is open")
	}
	state = state.Open()
	if !state.IsOpen() || state.Toggle().IsOpen() || state.Close().IsOpen() || state.Dismiss().IsOpen() || state.Escape().IsOpen() || state.OutsidePress().IsOpen() {
		t.Fatal("dropdown-menu state transition failed")
	}
	transition := func() {
		state = state.Open().Toggle().Open().Dismiss().Open().Escape().Open().OutsidePress().Open().Close()
	}
	if allocations := testing.AllocsPerRun(1000, transition); allocations != 0 {
		t.Fatalf("dropdown-menu state allocations = %v", allocations)
	}
}

func TestDropdownMenuKeysReplaceTheShortcutText(t *testing.T) {
	theme := gfx.DarkTheme()
	var canvas gfx.Canvas
	menu := BeginDropdownMenu(&canvas, theme, gfx.Rect{Width: 240, Height: 80}, DropdownMenuOptions{Layer: 5})
	menu.AddItem(DropdownMenuItem{ID: gfx.Item(1), Label: "Reload", Keys: []string{"⌘", "R"}, Shortcut: "unused"})
	for _, text := range canvas.Texts() {
		if text.Value == "unused" {
			t.Fatal("Shortcut text was painted although Keys were given")
		}
	}
}

func TestDropdownMenuSlotHoldsCallerContent(t *testing.T) {
	var canvas gfx.Canvas
	menu := BeginDropdownMenu(&canvas, gfx.DarkTheme(), gfx.Rect{Width: 240, Height: 80}, DropdownMenuOptions{Layer: 5})
	row, slot := menu.AddItemSlot(DropdownMenuItem{ID: gfx.Item(1), Label: "Badge", TrailingWidth: 20})
	if slot.Width != 20 || slot.X+slot.Width != row.X+row.Width-8 {
		t.Fatalf("slot %+v in row %+v", slot, row)
	}
}

func TestDropdownMenuDismissBackdropCoversTheViewportBehindTheSurface(t *testing.T) {
	viewport := gfx.Rect{Width: 800, Height: 600}
	var canvas gfx.Canvas
	menu := BeginDropdownMenu(&canvas, gfx.DarkTheme(), gfx.Rect{X: 10, Y: 20, Width: 160, Height: 80}, DropdownMenuOptions{Layer: 7, Scope: gfx.Item(42), DismissID: gfx.Item(91), Viewport: viewport})
	menu.AddItem(DropdownMenuItem{ID: gfx.Item(11), Label: "One"})
	hits := canvas.Hits()
	if len(hits) != 4 {
		t.Fatalf("hits = %+v", hits)
	}
	backdrop := hits[0]
	if backdrop.ID != gfx.Item(91) || backdrop.Bounds != viewport || backdrop.Layer != 7 || backdrop.Scope != gfx.Item(42) || backdrop.Focusable || backdrop.Role != gfx.RoleButton {
		t.Fatalf("backdrop = %+v", backdrop)
	}
	if !hits[1].Block || !hits[2].Container || hits[3].ID != gfx.Item(11) {
		t.Fatalf("the item must come after the surface block to win the press: %+v", hits)
	}
}

func TestDropdownMenuWithoutViewportDeclaresNoBackdrop(t *testing.T) {
	var canvas gfx.Canvas
	BeginDropdownMenu(&canvas, gfx.DarkTheme(), gfx.Rect{Width: 160, Height: 80}, DropdownMenuOptions{Layer: 7, DismissID: gfx.Item(91)})
	if hits := canvas.Hits(); len(hits) != 2 || !hits[0].Block || !hits[1].Container || hits[1].Role != gfx.RoleMenu {
		t.Fatalf("hits = %+v, want the surface block and its menu container", hits)
	}
}

func BenchmarkDropdownMenuWithBackdrop(b *testing.B) {
	theme := gfx.DarkTheme()
	var canvas gfx.Canvas
	b.ReportAllocs()
	for b.Loop() {
		canvas.Reset()
		menu := BeginDropdownMenu(&canvas, theme, gfx.Rect{X: 10, Y: 20, Width: 160, Height: 112}, DropdownMenuOptions{Layer: 7, Scope: gfx.Item(42), DismissID: gfx.Item(91), Viewport: gfx.Rect{Width: 800, Height: 600}})
		menu.AddItem(DropdownMenuItem{ID: gfx.Item(11), Label: "One"})
		menu.AddItem(DropdownMenuItem{ID: gfx.Item(12), Label: "Two"})
		menu.AddItem(DropdownMenuItem{ID: gfx.Item(13), Label: "Three"})
	}
}

func TestDropdownMenuItemIconShiftsTheLabelAndStaysOnTheOverlayLayer(t *testing.T) {
	icon := gfx.Icon{ViewBox: 24, Layers: []gfx.IconLayer{{
		Fill: gfx.Color{Red: 1, Alpha: 1}, Commands: []gfx.PathCommand{gfx.MoveTo(2, 2), gfx.LineTo(22, 2), gfx.LineTo(12, 22), gfx.ClosePath()},
	}}}
	var plain, withIcon gfx.Canvas
	paint := func(canvas *gfx.Canvas, icon gfx.Icon, kind DropdownMenuItemKind) {
		canvas.Reset()
		menu := BeginDropdownMenu(canvas, gfx.DarkTheme(), gfx.Rect{Width: 200, Height: 80}, DropdownMenuOptions{Layer: 7, Scope: gfx.Item(42)})
		menu.AddItem(DropdownMenuItem{ID: gfx.Item(1), Label: "Claude", Icon: icon, Kind: kind, Checked: true})
	}
	for _, kind := range []DropdownMenuItemKind{DropdownMenuAction, DropdownMenuCheckbox} {
		paint(&plain, gfx.Icon{}, kind)
		paint(&withIcon, icon, kind)
		// A checked checkbox item has its check mark's triangles either way.
		if len(withIcon.Triangles()) <= len(plain.Triangles()) {
			t.Fatalf("kind %d: triangles without icon %d, with icon %d", kind, len(plain.Triangles()), len(withIcon.Triangles()))
		}
		for _, triangle := range withIcon.Triangles() {
			if triangle.Layer != 7 {
				t.Fatalf("kind %d: icon escaped the overlay layer: %+v", kind, triangle)
			}
		}
		label := len(withIcon.Texts()) - 1
		if got := withIcon.Texts()[label].Bounds.X - plain.Texts()[len(plain.Texts())-1].Bounds.X; got != menuIconSize+menuIconGap {
			t.Fatalf("kind %d: label moved %v, want %v", kind, got, menuIconSize+menuIconGap)
		}
	}
	paint(&withIcon, icon, DropdownMenuAction)
	if allocations := testing.AllocsPerRun(100, func() { paint(&withIcon, icon, DropdownMenuAction) }); allocations != 0 {
		t.Fatalf("menu item with an icon allocated %v times", allocations)
	}
}

func TestDropdownMenuTallerThanItsBoundsScrollsInsteadOfDroppingItems(t *testing.T) {
	var scroll MenuScroll
	bounds := gfx.Rect{X: 10, Y: 20, Width: 160, Height: 104} // room for exactly three rows
	paint := func() (*gfx.Canvas, []gfx.Rect) {
		canvas := &gfx.Canvas{}
		menu := BeginDropdownMenu(canvas, gfx.DarkTheme(), bounds, DropdownMenuOptions{Layer: 7, Scope: gfx.Item(42), Scroll: &scroll})
		var rows []gfx.Rect
		for index := range 6 {
			rows = append(rows, menu.AddItem(DropdownMenuItem{ID: gfx.Item(100 + index), Label: "Item"}))
		}
		menu.End()
		return canvas, rows
	}
	_, rows := paint()
	if scroll.Max() != 3*32 {
		t.Fatalf("scroll range = %v, want the three rows that do not fit", scroll.Max())
	}
	if rows[2] == (gfx.Rect{}) || rows[3] != (gfx.Rect{}) {
		t.Fatalf("at the top: row 3 = %+v (should show), row 4 = %+v (should not)", rows[2], rows[3])
	}
	if !scroll.Wheel(gfx.ScrollEvent{DeltaY: 1000}) || scroll.Offset != scroll.Max() {
		t.Fatalf("a long wheel gesture left the offset at %v of %v", scroll.Offset, scroll.Max())
	}
	canvas, rows := paint()
	if rows[0] != (gfx.Rect{}) || rows[5] == (gfx.Rect{}) {
		t.Fatalf("scrolled to the end: first row %+v, last row %+v", rows[0], rows[5])
	}
	var ids []gfx.ID
	for _, hit := range canvas.Hits() {
		ids = append(ids, hit.ID)
		if hit.Bounds.Y < bounds.Y || hit.Bounds.Y+hit.Bounds.Height > bounds.Y+bounds.Height {
			t.Fatalf("hit %+v leaves the menu %+v", hit, bounds)
		}
	}
	if !slices.Contains(ids, gfx.Item(105)) || slices.Contains(ids, gfx.Item(100)) {
		t.Fatalf("hits after scrolling = %v", ids)
	}
	if scroll.By(-10000); scroll.Offset != 0 {
		t.Fatalf("offset = %v after scrolling past the top", scroll.Offset)
	}
}

func TestDropdownMenuViewportShortensAMenuTallerThanTheWindow(t *testing.T) {
	var canvas gfx.Canvas
	menu := BeginDropdownMenu(&canvas, gfx.DarkTheme(), gfx.Rect{X: 10, Y: 20, Width: 160, Height: 900}, DropdownMenuOptions{
		Layer: 7, DismissID: gfx.Item(91), Viewport: gfx.Rect{Width: 800, Height: 600},
	})
	if bounds := menu.Bounds(); bounds.Y < 0 || bounds.Y+bounds.Height > 600 {
		t.Fatalf("menu %+v leaves the 800x600 window", bounds)
	}
}

func TestMenuSurfaceChecksSitOnTheRightExceptInTheMenubar(t *testing.T) {
	theme := gfx.DarkTheme()
	paint := func(canvas *gfx.Canvas, menubar bool) gfx.Rect {
		canvas.Reset()
		if menubar {
			content := BeginMenubarContent(canvas, theme, gfx.Rect{Width: 200, Height: 80}, MenubarOverlayOptions{Layer: 1})
			return content.AddItem(MenubarItem{ID: gfx.Item(1), Label: "Auto save", Kind: MenubarCheckbox, Checked: true})
		}
		menu := BeginDropdownMenu(canvas, theme, gfx.Rect{Width: 200, Height: 80}, DropdownMenuOptions{Layer: 1})
		return menu.AddItem(DropdownMenuItem{ID: gfx.Item(1), Label: "Auto save", Kind: DropdownMenuCheckbox, Checked: true})
	}
	var canvas gfx.Canvas
	for _, menubar := range []bool{false, true} {
		row := paint(&canvas, menubar)
		var label gfx.Text
		for _, text := range canvas.Texts() {
			label = text
		}
		onLeft := label.Bounds.X-row.X >= 32
		if onLeft != menubar {
			t.Errorf("menubar=%v: label starts %v in from the row", menubar, label.Bounds.X-row.X)
		}
		if len(canvas.Triangles()) == 0 {
			t.Errorf("menubar=%v: no check was painted", menubar)
		}
	}
}

// scrollingMenu paints a dropdown of n items in room for three rows, the way a
// caller does on every frame, and reports where each item's hit lies.
func scrollingMenu(scroll *MenuScroll, canvas *gfx.Canvas, n int, disabled map[int]bool) map[gfx.ID]gfx.Rect {
	return focusedScrollingMenu(scroll, canvas, n, disabled, gfx.ID{}, false)
}

func focusedScrollingMenu(scroll *MenuScroll, canvas *gfx.Canvas, n int, disabled map[int]bool, focus gfx.ID, visible bool) map[gfx.ID]gfx.Rect {
	canvas.Reset()
	canvas.SetInteraction(gfx.ID{}, gfx.ID{}, focus, visible)
	menu := BeginDropdownMenu(canvas, gfx.DarkTheme(), gfx.Rect{X: 10, Y: 20, Width: 160, Height: 104}, DropdownMenuOptions{Layer: 7, Scope: gfx.Item(42), Scroll: scroll})
	for index := range n {
		menu.AddItem(DropdownMenuItem{ID: gfx.Item(100 + index), Label: "Item", Disabled: disabled[index]})
	}
	menu.End()
	hits := map[gfx.ID]gfx.Rect{}
	for _, hit := range canvas.Hits() {
		hits[hit.ID] = hit.Bounds
	}
	return hits
}

func TestMenuNavigateScrollsTheItemItMovesToIntoView(t *testing.T) {
	var scroll MenuScroll
	var canvas gfx.Canvas
	scrollingMenu(&scroll, &canvas, 8, map[int]bool{4: true})
	menuTop, menuBottom := float32(20), float32(20+104)
	step := func(key gfx.Key, from int) int {
		t.Helper()
		id, ok := scroll.Navigate(key, gfx.Item(100+from), false)
		if !ok {
			t.Fatalf("key %v from %d: not a movement", key, from)
		}
		hits := scrollingMenu(&scroll, &canvas, 8, map[int]bool{4: true})
		bounds, shown := hits[id]
		if !shown || bounds.Y < menuTop || bounds.Y+bounds.Height > menuBottom {
			t.Fatalf("key %v from %d: item %v is at %+v, outside the menu", key, from, id, bounds)
		}
		return int(id.Name()) - 100
	}
	// Past the room of three rows, one step at a time; the disabled item is jumped.
	if got := step(gfx.KeyDown, 2); got != 3 {
		t.Fatalf("Down from 2 = %d", got)
	}
	if got := step(gfx.KeyDown, 3); got != 5 {
		t.Fatalf("Down from 3 = %d, want the disabled 4 skipped", got)
	}
	if got := step(gfx.KeyEnd, 5); got != 7 {
		t.Fatalf("End = %d", got)
	}
	if got := step(gfx.KeyPageUp, 7); got >= 7 || got < 4 {
		t.Fatalf("PageUp from 7 = %d, want a page back", got)
	}
	if got := step(gfx.KeyHome, 5); got != 0 || scroll.Offset != 0 {
		t.Fatalf("Home = %d at offset %v", got, scroll.Offset)
	}
	if id, _ := scroll.Navigate(gfx.KeyUp, gfx.Item(100), true); id != gfx.Item(107) {
		t.Fatalf("Up from the first item with loop = %v, want the last", id)
	}
	if id, _ := scroll.Navigate(gfx.KeyUp, gfx.Item(100), false); id != gfx.Item(100) {
		t.Fatalf("Up from the first item without loop = %v, want to stay", id)
	}
	if _, ok := scroll.Navigate(gfx.KeyEnter, gfx.Item(100), true); ok {
		t.Fatal("Enter is not a movement")
	}
}

func TestMenuScrollsToTheItemTabFocused(t *testing.T) {
	var scroll MenuScroll
	var canvas gfx.Canvas
	scrollingMenu(&scroll, &canvas, 8, nil)
	last := gfx.Item(107)
	focusedScrollingMenu(&scroll, &canvas, 8, nil, last, true)
	if scroll.Offset != scroll.Max() {
		t.Fatalf("offset %v after the keyboard focused the last item, want %v", scroll.Offset, scroll.Max())
	}
	// The wheel moves away from a focused item without being undone.
	scroll.By(-1000)
	focusedScrollingMenu(&scroll, &canvas, 8, nil, last, true)
	if scroll.Offset != 0 {
		t.Fatalf("offset %v: the focused item pulled the list back", scroll.Offset)
	}
	// A press that focuses a row is not the keyboard.
	scroll = MenuScroll{}
	scrollingMenu(&scroll, &canvas, 8, nil)
	focusedScrollingMenu(&scroll, &canvas, 8, nil, last, false)
	if scroll.Offset != 0 {
		t.Fatalf("offset %v after a pointer focus", scroll.Offset)
	}
}

func TestMenuScrollbarIsDraggableAndRowsLeaveItRoom(t *testing.T) {
	scroll := MenuScroll{BarID: gfx.Item(9)}
	var canvas gfx.Canvas
	scrollingMenu(&scroll, &canvas, 8, nil)
	hits := scrollingMenu(&scroll, &canvas, 8, nil)
	bar, ok := hits[scroll.BarID]
	if !ok {
		t.Fatal("a scrolling menu with a BarID declares no scrollbar hit")
	}
	for id, bounds := range hits {
		if id != scroll.BarID && id != gfx.Item(0) && bounds.X+bounds.Width > bar.X && bounds.Y < bar.Y+bar.Height && bounds.Y+bounds.Height > bar.Y && bounds.Width < 200 {
			t.Fatalf("item %v hit %+v overlaps the scrollbar %+v", id, bounds, bar)
		}
	}
	scroll.Drag(gfx.PointerEvent{Kind: gfx.PointerDown, Target: scroll.BarID, Y: bar.Y + 2})
	if !scroll.Dragging() {
		t.Fatal("pressing the thumb did not start a drag")
	}
	if !scroll.Drag(gfx.PointerEvent{Kind: gfx.PointerMove, Captured: true, Target: scroll.BarID, Y: bar.Y + bar.Height + 500}) || scroll.Offset != scroll.Max() {
		t.Fatalf("dragging far below left the offset at %v of %v", scroll.Offset, scroll.Max())
	}
	scroll.Drag(gfx.PointerEvent{Kind: gfx.PointerUp, Target: scroll.BarID})
	if scroll.Dragging() || scroll.Drag(gfx.PointerEvent{Kind: gfx.PointerMove, Y: 0}) {
		t.Fatal("moving after release still dragged")
	}
	// Without a BarID nothing but the rows is declared.
	plain := MenuScroll{}
	if _, ok := scrollingMenu(&plain, &canvas, 8, nil)[gfx.Item(9)]; ok || len(nonBlockHits(canvas.Hits())) != 3 {
		t.Fatalf("a menu without BarID declares %d control hits, want its three visible rows", len(nonBlockHits(canvas.Hits())))
	}
}

func TestScrollingMenuAllocatesNothingOncePainted(t *testing.T) {
	scroll := MenuScroll{BarID: gfx.Item(9)}
	var canvas gfx.Canvas
	paint := func() {
		canvas.Reset()
		menu := BeginDropdownMenu(&canvas, gfx.DarkTheme(), gfx.Rect{X: 10, Y: 20, Width: 160, Height: 104}, DropdownMenuOptions{Layer: 7, Scope: gfx.Item(42), Scroll: &scroll})
		for index := range 8 {
			menu.AddItem(DropdownMenuItem{ID: gfx.Item(100 + index), Label: "Item"})
		}
		menu.End()
		scroll.Navigate(gfx.KeyDown, gfx.Item(101), true)
	}
	paint()
	paint()
	if allocations := testing.AllocsPerRun(100, paint); allocations != 0 {
		t.Fatalf("a scrolling menu allocated %v times a frame", allocations)
	}
}

func TestMenuOpensAtTheOffsetTheCallerSetBeforeItsFirstPaint(t *testing.T) {
	scroll := MenuScroll{Offset: 64}
	var canvas gfx.Canvas
	hits := scrollingMenu(&scroll, &canvas, 8, nil)
	if scroll.Offset != 64 {
		t.Fatalf("offset %v after the first paint, want the 64 it opened at", scroll.Offset)
	}
	if _, shown := hits[gfx.Item(100)]; shown {
		t.Fatal("the first item shows in a menu opened two rows down")
	}
	scroll.By(-1000)
	if scrollingMenu(&scroll, &canvas, 8, nil); scroll.Offset != 0 {
		t.Fatalf("offset %v after scrolling to the top", scroll.Offset)
	}
}

// A submenu owns its level: opening a sibling replaces it, closing it leaves the
// parent menu open, and the horizontal keys say which level movement to make.
func TestSubmenuStateTracksOneOpenLevel(t *testing.T) {
	var state SubmenuState
	if state.IsAnyOpen() || state.IsOpen(gfx.Item(1)) {
		t.Fatal("the zero state is open")
	}
	state = state.Open(gfx.Item(1)).Open(gfx.Item(2))
	if !state.IsOpen(gfx.Item(2)) || state.IsOpen(gfx.Item(1)) || state.Trigger() != gfx.Item(2) {
		t.Fatalf("state = %+v, want item 2 open", state)
	}
	if state.Open(gfx.ID{}).Trigger() != gfx.Item(2) {
		t.Fatal("a zero trigger replaced the open one")
	}
	if state.Close().IsAnyOpen() {
		t.Fatal("Close left a submenu open")
	}
	for key, want := range map[gfx.Key]SubmenuCommand{gfx.KeyRight: SubmenuCommandOpen, gfx.KeyLeft: SubmenuCommandClose} {
		if command, ok := SubmenuCommandForKey(key); !ok || command != want {
			t.Fatalf("key %v = (%v, %v), want %v", key, command, ok, want)
		}
	}
	if _, ok := SubmenuCommandForKey(gfx.KeyDown); ok {
		t.Fatal("Down is not a level movement")
	}
}

// The grace area keeps a submenu open while the pointer travels from its row to
// the panel, and ends outside the pair.
func TestSubmenuSafeKeepsTheDiagonalTravel(t *testing.T) {
	trigger := gfx.Rect{X: 10, Y: 20, Width: 100, Height: 32}
	submenu := gfx.Rect{X: 110, Y: 20, Width: 100, Height: 120}
	if !SubmenuSafe(trigger, submenu, gfx.Point{X: 60, Y: 30}) {
		t.Fatal("the trigger is not safe")
	}
	if !SubmenuSafe(trigger, submenu, gfx.Point{X: 150, Y: 100}) {
		t.Fatal("the panel is not safe")
	}
	if !SubmenuSafe(trigger, submenu, gfx.Point{X: 108, Y: 90}) {
		t.Fatal("the gap between trigger and panel is not safe")
	}
	if SubmenuSafe(trigger, submenu, gfx.Point{X: 400, Y: 300}) {
		t.Fatal("a far point is safe")
	}
}

// Hovering another submenu row switches after the rest delay, the grace area
// keeps the open panel, and leaving both closes it.
func TestSubmenuHoverSwitchesKeepsAndCloses(t *testing.T) {
	now := time.Unix(0, 0)
	var intent HoverIntent
	state := SubmenuState{}.Open(gfx.Item(1))
	rowA := gfx.Rect{X: 10, Y: 10, Width: 100, Height: 32}
	rowB := gfx.Rect{X: 10, Y: 42, Width: 100, Height: 32}
	panel := gfx.Rect{X: 110, Y: 10, Width: 140, Height: 120}
	if next, wait := SubmenuHover(state, gfx.Item(2), gfx.Point{X: 50, Y: 50}, rowB, rowA, panel, now, &intent, 100*time.Millisecond); !next.IsOpen(gfx.Item(1)) || wait != 100*time.Millisecond {
		t.Fatalf("first hover = (%v, %v)", next.Trigger(), wait)
	}
	if next, wait := SubmenuHover(state, gfx.Item(2), gfx.Point{X: 50, Y: 50}, rowB, rowA, panel, now.Add(120*time.Millisecond), &intent, 100*time.Millisecond); !next.IsOpen(gfx.Item(2)) || wait != 0 {
		t.Fatalf("rested hover = (%v, %v)", next.Trigger(), wait)
	}
	if next, _ := SubmenuHover(state, gfx.ID{}, gfx.Point{X: 108, Y: 60}, gfx.Rect{}, rowA, panel, now, &intent, 100*time.Millisecond); !next.IsOpen(gfx.Item(1)) {
		t.Fatal("the grace area closed the submenu")
	}
	if next, _ := SubmenuHover(state, gfx.ID{}, gfx.Point{X: 400, Y: 300}, gfx.Rect{}, rowA, panel, now, &intent, 100*time.Millisecond); next.IsAnyOpen() {
		t.Fatal("leaving the pair kept the submenu open")
	}
}

// The pointer moves the menu's current row, as Radix does, so the tint and the
// focused item never name two rows; a pointer that rests in place never takes
// the row back from the keyboard.
func TestDropdownMenuPointerMovesTheCurrentRowOnlyWhenItEntersOne(t *testing.T) {
	theme := gfx.LightTheme()
	var canvas gfx.Canvas
	paint := func(hovered, focused gfx.ID, visible bool) int {
		canvas.Reset()
		canvas.SetInteraction(hovered, gfx.ID{}, focused, visible)
		menu := BeginDropdownMenu(&canvas, theme, gfx.Rect{X: 10, Y: 20, Width: 100, Height: 180}, DropdownMenuOptions{Layer: 7, Scope: gfx.Item(42), DismissID: gfx.Item(91)})
		menu.AddItem(DropdownMenuItem{ID: gfx.Item(11), Label: "One"})
		menu.AddItem(DropdownMenuItem{ID: gfx.Item(12), Label: "Two"})
		menu.AddItem(DropdownMenuItem{ID: gfx.Item(13), Label: "Three"})
		menu.AddItem(DropdownMenuItem{ID: gfx.Item(14), Label: "Off", Disabled: true})
		menu.End()
		tinted := 0
		for _, rectangle := range canvas.Rectangles() {
			if rectangle.Color == theme.Accent && rectangle.Radius == theme.Metrics.RadiusItem() {
				tinted++
			}
		}
		return tinted
	}

	if got := paint(gfx.Item(12), gfx.Item(11), true); got != 1 {
		t.Fatalf("hovering the second row tinted %d rows", got)
	}
	if canvas.FocusRequest() != gfx.Item(12) {
		t.Fatalf("hovering the second row requested focus %v", canvas.FocusRequest())
	}
	if got := paint(gfx.Item(12), gfx.Item(12), true); got != 1 {
		t.Fatalf("the row the pointer entered tinted %d rows", got)
	}
	// The pointer has not moved; a keyboard move must keep the row it chose.
	if got := paint(gfx.Item(12), gfx.Item(13), true); got != 1 {
		t.Fatalf("a resting pointer tinted %d rows", got)
	}
	if request := canvas.FocusRequest(); !request.IsZero() {
		t.Fatalf("a resting pointer requested focus %v", request)
	}
	// Crossing onto a disabled row moves nothing, and the focused row returns.
	paint(gfx.Item(14), gfx.Item(13), true)
	if request := canvas.FocusRequest(); !request.IsZero() {
		t.Fatalf("a disabled row requested focus %v", request)
	}
	if got := paint(gfx.Item(14), gfx.Item(13), true); got != 1 {
		t.Fatalf("after crossing a disabled row %d rows were tinted", got)
	}
	// Focus that the pointer moved is not "focus visible": the row the pointer
	// entered must stay tinted on the next paint, not flash and go dark.
	if got := paint(gfx.Item(12), gfx.Item(12), false); got != 1 {
		t.Fatalf("a pointer-focused row tinted %d rows on the next paint", got)
	}
}
