package components

import (
	"slices"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
)

func TestContextMenuUsesCallerPositionAndTypedMenuStates(t *testing.T) {
	var canvas gfx.Canvas
	menu := BeginContextMenu(&canvas, gfx.LightTheme(), gfx.Rect{X: 240, Y: 180, Width: 150, Height: 150}, ContextMenuOptions{Layer: 12, Scope: gfx.Item(64), DismissID: gfx.Item(88)})
	first := menu.AddItem(ContextMenuItem{ID: gfx.Item(21), Label: "Grid", Kind: ContextMenuCheckbox, Checked: true})
	menu.AddItem(ContextMenuItem{ID: gfx.Item(22), Label: "Large", Kind: ContextMenuRadio, Selected: true})
	menu.AddItem(ContextMenuItem{ID: gfx.Item(23), Label: "Inspect", Hint: "Open inspector", Disabled: true})
	menu.AddItem(ContextMenuItem{ID: gfx.Item(24), Label: "Services", Kind: ContextMenuSubmenu, Expanded: true})

	if first != (gfx.Rect{X: 244, Y: 184, Width: 142, Height: 32}) || menu.NextItemBounds().Y != 312 {
		t.Fatalf("context geometry = first %+v next %+v", first, menu.NextItemBounds())
	}
	if canvas.TrappedFocusScope() != gfx.Item(64) || canvas.EscapeTarget() != gfx.Item(88) {
		t.Fatalf("scope=%d escape=%d", canvas.TrappedFocusScope(), canvas.EscapeTarget())
	}
	hits := nonBlockHits(canvas.Hits())
	if len(hits) != 4 || !hits[0].Checked || !hits[1].Selected || !hits[2].Disabled || hits[2].Focusable || !hits[3].Expanded {
		t.Fatalf("context hits = %+v", hits)
	}
	for _, hit := range hits {
		if hit.Role != gfx.RoleMenuItem || hit.Scope != gfx.Item(64) {
			t.Fatalf("context hit = %+v", hit)
		}
	}
	for _, rectangle := range canvas.Rectangles() {
		if rectangle.Layer != 12 {
			t.Fatalf("context rectangle layer = %+v", rectangle)
		}
	}
}

func TestContextMenuVariantsAndRepeatedPaintStayStable(t *testing.T) {
	theme := gfx.DarkTheme()
	var canvas gfx.Canvas
	canvas.SetInteraction(gfx.ID{}, gfx.ID{}, gfx.Item(31), true)
	paint := func() {
		canvas.Reset()
		menu := BeginContextMenu(&canvas, theme, gfx.Rect{Width: 144, Height: 84}, ContextMenuOptions{Layer: 5, Scope: gfx.Item(7), DismissID: gfx.Item(8)})
		menu.AddItem(ContextMenuItem{ID: gfx.Item(31), Label: "Remove", Variant: ContextMenuItemDestructive})
		menu.AddItem(ContextMenuItem{ID: gfx.Item(32), Label: "Disabled", Disabled: true, Inset: true})
	}
	paint()
	remove := contextMenuTestText(canvas.Texts(), "Remove")
	disabled := contextMenuTestText(canvas.Texts(), "Disabled")
	if remove.Style.Color != theme.Danger || disabled.Style.Color.Alpha != theme.PopoverForeground.Alpha*0.50 {
		t.Fatalf("variant colors = remove %+v disabled %+v", remove.Style.Color, disabled.Style.Color)
	}
	rectangles := slices.Clone(canvas.Rectangles())
	texts := slices.Clone(canvas.Texts())
	hits := slices.Clone(canvas.Hits())
	paint()
	if !slices.Equal(rectangles, canvas.Rectangles()) || !slices.Equal(texts, canvas.Texts()) || !slices.Equal(hits, canvas.Hits()) {
		t.Fatal("identical context menus produced different command streams")
	}
}

func contextMenuTestText(texts []gfx.Text, value string) gfx.Text {
	for _, text := range texts {
		if text.Value == value {
			return text
		}
	}
	return gfx.Text{}
}

func TestContextMenuStateTransitionsAreAllocationFree(t *testing.T) {
	var state ContextMenuState
	if state.IsOpen() {
		t.Fatal("zero state is open")
	}
	state = state.Open()
	if !state.IsOpen() || state.Toggle().IsOpen() || state.Close().IsOpen() || state.Dismiss().IsOpen() || state.Escape().IsOpen() || state.OutsidePress().IsOpen() {
		t.Fatal("context-menu state transition failed")
	}
	transition := func() {
		state = state.Open().Toggle().Open().Dismiss().Open().Escape().Open().OutsidePress().Open().Close()
	}
	if allocations := testing.AllocsPerRun(1000, transition); allocations != 0 {
		t.Fatalf("context-menu state allocations = %v", allocations)
	}
}

func TestContextMenuKeysPaintKeycapsAtTheTrailingEdge(t *testing.T) {
	theme := gfx.DarkTheme()
	var canvas gfx.Canvas
	bounds := gfx.Rect{X: 10, Y: 10, Width: 240, Height: 80}
	menu := BeginContextMenu(&canvas, theme, bounds, ContextMenuOptions{Layer: 5})
	row := menu.AddItem(ContextMenuItem{ID: gfx.Item(3), Label: "Split Right", Keys: []string{"⌘", "⌥", "H"}})
	var caps []gfx.Rect
	for _, rectangle := range canvas.Rectangles() {
		if rectangle.Color == theme.Border && rectangle.Radius == kbdRadius(theme) {
			caps = append(caps, rectangle.Bounds)
		}
	}
	if len(caps) != 3 {
		t.Fatalf("keycap rectangles = %+v", canvas.Rectangles())
	}
	if last := caps[2]; last.X+last.Width != row.X+row.Width-8 {
		t.Fatalf("last keycap ends at %v, want %v", last.X+last.Width, row.X+row.Width-8)
	}
	if caps[0].X >= caps[1].X || caps[1].X >= caps[2].X || caps[1].X != caps[0].X+caps[0].Width+menuKeysGap {
		t.Fatalf("keycaps are not spaced left to right: %+v", caps)
	}
}

func TestContextMenuSlotHoldsCallerContentInsideTheMenu(t *testing.T) {
	theme := gfx.DarkTheme()
	var canvas gfx.Canvas
	menu := BeginContextMenu(&canvas, theme, gfx.Rect{X: 10, Y: 10, Width: 240, Height: 120}, ContextMenuOptions{Layer: 7, Scope: gfx.Item(9)})
	row, slot := menu.AddItemSlot(ContextMenuItem{ID: gfx.Item(1), Label: "Monitor", TrailingWidth: 36})
	if slot.Width != 36 || slot.X+slot.Width != row.X+row.Width-8 || slot.Y != row.Y || slot.Height != row.Height {
		t.Fatalf("slot %+v in row %+v", slot, row)
	}
	menu.BeginContent()
	BeginSwitch(&canvas, theme, gfx.Item(2), gfx.Rect{X: slot.X, Y: slot.Y + 8, Width: 36, Height: 18}, true).Small().Paint()
	menu.EndContent()

	var found bool
	for _, hit := range canvas.Hits() {
		if hit.ID == gfx.Item(2) {
			found = true
			if hit.Layer != 7 || hit.Scope != gfx.Item(9) {
				t.Fatalf("custom content hit is on layer %d scope %d, want the menu's 7/9", hit.Layer, hit.Scope)
			}
		}
	}
	if !found {
		t.Fatal("the switch painted no hit")
	}
	for _, text := range canvas.Texts() {
		if text.Value == "Monitor" && text.Bounds.X+text.Bounds.Width > slot.X {
			t.Fatalf("the label %+v runs under the slot %+v", text.Bounds, slot)
		}
	}
}

func TestContextMenuWithoutSlotReturnsAnEmptySlot(t *testing.T) {
	var canvas gfx.Canvas
	menu := BeginContextMenu(&canvas, gfx.DarkTheme(), gfx.Rect{Width: 240, Height: 80}, ContextMenuOptions{Layer: 5})
	if _, slot := menu.AddItemSlot(ContextMenuItem{ID: gfx.Item(1), Label: "Plain"}); slot != (gfx.Rect{}) {
		t.Fatalf("slot = %+v, want empty", slot)
	}
}

func TestContextMenuDismissBackdropCoversTheViewportBehindTheSurface(t *testing.T) {
	viewport := gfx.Rect{Width: 800, Height: 600}
	var canvas gfx.Canvas
	menu := BeginContextMenu(&canvas, gfx.DarkTheme(), gfx.Rect{X: 10, Y: 20, Width: 160, Height: 80}, ContextMenuOptions{Layer: 7, Scope: gfx.Item(42), DismissID: gfx.Item(91), Viewport: viewport})
	menu.AddItem(ContextMenuItem{ID: gfx.Item(11), Label: "One"})
	hits := canvas.Hits()
	if len(hits) != 4 || hits[0].ID != gfx.Item(91) || hits[0].Bounds != viewport || hits[0].Layer != 7 || hits[0].Scope != gfx.Item(42) || hits[0].Focusable || !hits[1].Block || !hits[2].Container || hits[2].Role != gfx.RoleMenu || hits[3].ID != gfx.Item(11) {
		t.Fatalf("hits = %+v", hits)
	}
}

func TestContextMenuRowHeightOptionPacksItemsDenser(t *testing.T) {
	var canvas gfx.Canvas
	menu := BeginContextMenu(&canvas, gfx.LightTheme(), gfx.Rect{X: 10, Y: 20, Width: 160, Height: 4*24 + 9 + 8}, ContextMenuOptions{Layer: 1, Scope: gfx.Item(2), DismissID: gfx.Item(3), RowHeight: 24})
	first := menu.AddItem(ContextMenuItem{ID: gfx.Item(11), Label: "One"})
	menu.AddSeparator()
	second := menu.AddItem(ContextMenuItem{ID: gfx.Item(12), Label: "Two"})
	third := menu.AddItem(ContextMenuItem{ID: gfx.Item(13), Label: "Three"})
	if first.Height != 24 || second.Y != first.Y+24+menuSeparatorHeight || third.Y != second.Y+24 || menu.NextItemBounds().Height != 24 {
		t.Fatalf("rows = %+v %+v %+v next %+v", first, second, third, menu.NextItemBounds())
	}
	for _, hit := range nonBlockHits(canvas.Hits()) {
		if hit.Bounds.Height != 24 {
			t.Fatalf("hit %+v is not a 24px row", hit)
		}
	}
}

func TestContextMenuItemWidthKeepsTheLabelWhole(t *testing.T) {
	var canvas gfx.Canvas
	theme := gfx.LightTheme()
	item := ContextMenuItem{ID: gfx.Item(1), Label: "A rather long menu label that needs more than the minimum", Keys: []string{"⇧", "⌘", "S"}, Icon: gfx.Icon{ViewBox: 24, Stroke: 2, Commands: []gfx.PathCommand{gfx.MoveTo(4, 12), gfx.LineTo(20, 12)}}}
	width := ContextMenuItemWidth(&canvas, theme, item)
	label := func(width float32) gfx.Text {
		canvas.Reset()
		menu := BeginContextMenu(&canvas, theme, gfx.Rect{Width: width, Height: 40}, ContextMenuOptions{Layer: 1, Scope: gfx.Item(2), DismissID: gfx.Item(3)})
		menu.AddItem(item)
		return contextMenuTestText(canvas.Texts(), item.Label)
	}
	need := canvas.TextWidth(item.Label, theme.Metrics.FontSizeM)
	if got := label(width).Bounds.Width; got < need {
		t.Fatalf("at the computed width %v the label gets %v of the %v it needs", width, got, need)
	}
	if got := label(width - 12).Bounds.Width; got >= need {
		t.Fatalf("12px under the computed width the label still gets %v of %v: the width is not tight", got, need)
	}
}

func TestMenuContentKeepsTheFocusedRowFromTinting(t *testing.T) {
	var tree gfx.Tree
	feature := tree.Feature("menu-content")
	scope := gfx.Item(64)
	rowID := tree.ID(feature, "row")
	contentID := tree.ID(feature, "content")
	theme := gfx.DarkTheme()
	var rowBounds, contentBounds gfx.Rect
	driver, err := app.NewDriver(app.Config{
		Width: 320, Height: 240, Tree: &tree,
		Paint: func(c *gfx.Canvas, size gfx.Size) {
			menu := BeginContextMenu(c, theme, gfx.Rect{X: 20, Y: 20, Width: 220, Height: 160}, ContextMenuOptions{Layer: 7, Scope: scope})
			rowBounds = menu.AddItem(ContextMenuItem{ID: rowID, Label: "Row"})
			menu.BeginContent()
			contentBounds = menu.AddItem(ContextMenuItem{})
			c.HitWithOptions(contentID, contentBounds, "Content", gfx.HitOptions{Role: gfx.RoleButton})
			menu.EndContent()
			menu.End()
		},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	move := func(x, y float32) {
		t.Helper()
		must(driver.Move(x, y))
	}
	rowTinted := func() bool {
		t.Helper()
		for _, primitive := range driver.Snapshot().Primitives {
			if primitive.Color == theme.Accent && primitive.Bounds == rowBounds {
				return true
			}
		}
		return false
	}
	rowCenter := func() (float32, float32) {
		return rowBounds.X + rowBounds.Width/2, rowBounds.Y + rowBounds.Height/2
	}
	contentCenter := func() (float32, float32) {
		return contentBounds.X + contentBounds.Width/2, contentBounds.Y + contentBounds.Height/2
	}
	// Hovering the row makes it the menu's current row.
	x, y := rowCenter()
	move(x, y)
	move(x+1, y)
	if !rowTinted() {
		t.Fatal("hovering the row did not tint it: the test cannot see the state it checks")
	}
	// The pointer then rests on caller content inside the menu. The focused row
	// must not light up again.
	x, y = contentCenter()
	move(x, y)
	move(x+1, y)
	if rowTinted() {
		t.Fatal("the row is tinted while the pointer rests on the menu's content")
	}
	// Returning to the row lights it again.
	x, y = rowCenter()
	move(x, y)
	move(x+1, y)
	if !rowTinted() {
		t.Fatal("the row is no longer tinted after the pointer returned to it")
	}
}
