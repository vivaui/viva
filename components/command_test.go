package components

import (
	"slices"
	"testing"

	"github.com/vivaui/viva/gfx"
)

func TestCommandCapturesPaletteContractAndItemState(t *testing.T) {
	var canvas gfx.Canvas
	canvas.SetInteraction(gfx.ID{}, gfx.ID{}, gfx.Item(12), true)
	command := BeginCommand(&canvas, gfx.LightTheme(), gfx.Rect{X: 10, Y: 20, Width: 320, Height: 220}, CommandOptions{
		InputID: gfx.Item(9), Label: "Command Palette", Query: "set", Variant: CommandDialog,
		Layer: 8, Scope: gfx.Item(44), DismissID: gfx.Item(90),
	})
	if canvas.TrappedFocusScope() != gfx.Item(44) || canvas.EscapeTarget() != gfx.Item(90) {
		t.Fatalf("overlay contract scope=%d escape=%d", canvas.TrappedFocusScope(), canvas.EscapeTarget())
	}
	command.AddGroup("Settings")
	itemBounds := command.AddItem(CommandItem{ID: gfx.Item(12), Label: "Open settings", Value: "settings", Shortcut: "⌘,", Selected: true})
	command.AddItem(CommandItem{ID: gfx.Item(13), Label: "Toggle panel", Checked: true})
	command.AddItem(CommandItem{ID: gfx.Item(14), Label: "Unavailable", Disabled: true})
	if itemBounds != (gfx.Rect{X: 14, Y: 96, Width: 312, Height: 44}) {
		t.Fatalf("first item bounds = %+v", itemBounds)
	}
	hits := nonBlockHits(canvas.Hits())
	if len(hits) != 4 {
		t.Fatalf("hits = %+v", hits)
	}
	if input := hits[0]; input.Role != gfx.RoleCombobox || input.Scope != gfx.Item(44) || input.Value != "set" || !input.Expanded {
		t.Fatalf("input = %+v", input)
	}
	if item := hits[1]; item.Role != gfx.RoleOption || !item.Selected || item.Scope != gfx.Item(44) {
		t.Fatalf("selected item = %+v", item)
	}
	if checked := hits[2]; !checked.Checked || checked.Role != gfx.RoleOption {
		t.Fatalf("checked item = %+v", checked)
	}
	if disabled := hits[3]; !disabled.Disabled || disabled.Focusable {
		t.Fatalf("disabled item = %+v", disabled)
	}
	commandAssertLayer(t, canvas.Rectangles(), canvas.Texts(), 8)
}

// The caller names the active row: neither a pointer nor a keyboard focus tints
// one on its own, so a row the caller did not select can never be lit beside the
// row Enter runs.
func TestCommandItemTintsOnlyTheSelectedRow(t *testing.T) {
	var idle, pointer, keyboard, selected gfx.Canvas
	paint := func(canvas *gfx.Canvas, selectedRow bool) {
		command := BeginCommand(canvas, gfx.LightTheme(), gfx.Rect{Width: 260, Height: 150}, CommandOptions{InputID: gfx.Item(1), Label: "Commands"})
		command.AddItem(CommandItem{ID: gfx.Item(2), Label: "New window", Selected: selectedRow})
	}
	paint(&idle, false)
	pointer.SetInteraction(gfx.ID{}, gfx.ID{}, gfx.Item(2), false)
	paint(&pointer, false)
	if !slices.Equal(pointer.Rectangles(), idle.Rectangles()) {
		t.Fatal("pointer focus tinted a row the caller did not select")
	}
	keyboard.SetInteraction(gfx.ID{}, gfx.ID{}, gfx.Item(2), true)
	paint(&keyboard, false)
	if !slices.Equal(keyboard.Rectangles(), idle.Rectangles()) {
		t.Fatal("keyboard focus tinted a row the caller did not select")
	}
	paint(&selected, true)
	if slices.Equal(selected.Rectangles(), idle.Rectangles()) {
		t.Fatal("the selected row was not tinted")
	}
}

func TestCommandPaintIsDeterministicAndAllocationFreeAfterWarmup(t *testing.T) {
	var canvas gfx.Canvas
	paint := func() {
		canvas.Reset()
		command := BeginCommand(&canvas, gfx.DarkTheme(), gfx.Rect{X: 4, Y: 5, Width: 260, Height: 150}, CommandOptions{InputID: gfx.Item(1), Label: "Commands", Placeholder: "Type a command", Layer: 3, Scope: gfx.Item(7), DismissID: gfx.Item(8)})
		command.AddGroup("General")
		command.AddItem(CommandItem{ID: gfx.Item(2), Label: "New window", Shortcut: "⌘N"})
		command.AddSeparator()
		command.AddEmpty("No results found.")
	}
	paint()
	rectangles := slices.Clone(canvas.Rectangles())
	texts := slices.Clone(canvas.Texts())
	hits := slices.Clone(canvas.Hits())
	paint()
	if !slices.Equal(rectangles, canvas.Rectangles()) || !slices.Equal(texts, canvas.Texts()) || !slices.Equal(hits, canvas.Hits()) {
		t.Fatal("identical command snapshots produced different streams")
	}
	if allocations := testing.AllocsPerRun(100, paint); allocations != 0 {
		t.Fatalf("command paint allocations = %v", allocations)
	}
}

func commandAssertLayer(t *testing.T, rectangles []gfx.Rectangle, texts []gfx.Text, layer uint16) {
	t.Helper()
	for _, rectangle := range rectangles {
		if rectangle.Layer != layer {
			t.Fatalf("rectangle layer = %+v", rectangle)
		}
	}
	for _, text := range texts {
		if text.Layer != layer {
			t.Fatalf("text layer = %+v", text)
		}
	}
}

func TestCommandListScrollsUnderTheInputAndItemsCarryIcons(t *testing.T) {
	var scroll MenuScroll
	icon := gfx.Icon{ViewBox: 24, Stroke: 2, Commands: []gfx.PathCommand{gfx.MoveTo(4, 12), gfx.LineTo(20, 12)}}
	bounds := gfx.Rect{X: 20, Y: 30, Width: 300, Height: 36 + 4 + 3*32}
	paint := func() (*gfx.Canvas, []gfx.Rect) {
		canvas := &gfx.Canvas{}
		command := BeginCommand(canvas, gfx.LightTheme(), bounds, CommandOptions{InputID: gfx.Item(1), Layer: 4, Scope: gfx.Item(2), Scroll: &scroll})
		var rows []gfx.Rect
		for index := range 8 {
			rows = append(rows, command.AddItem(CommandItem{ID: gfx.Item(10 + index), Label: "Command", Icon: icon}))
		}
		command.End()
		return canvas, rows
	}
	canvas, rows := paint()
	if rows[0] == (gfx.Rect{}) || rows[7] != (gfx.Rect{}) || scroll.Max() <= 0 {
		t.Fatalf("first row %+v, last row %+v, range %v: want the list cut at the bounds", rows[0], rows[7], scroll.Max())
	}
	for _, hit := range nonBlockHits(canvas.Hits()) {
		if hit.ID != gfx.Item(1) && hit.Bounds.Y < bounds.Y+36 {
			t.Fatalf("result %+v reaches up into the input row", hit)
		}
	}
	scroll.By(10000)
	_, rows = paint()
	if rows[7] == (gfx.Rect{}) || rows[0] != (gfx.Rect{}) {
		t.Fatalf("scrolled to the end: first %+v last %+v", rows[0], rows[7])
	}
}

func TestCommandRevealScrollsAResultTheCallerHighlighted(t *testing.T) {
	var scroll MenuScroll
	var canvas gfx.Canvas
	bounds := gfx.Rect{X: 20, Y: 30, Width: 300, Height: 36 + 4 + 3*32}
	paint := func() map[gfx.ID]gfx.Rect {
		canvas.Reset()
		command := BeginCommand(&canvas, gfx.LightTheme(), bounds, CommandOptions{InputID: gfx.Item(1), Scope: gfx.Item(2), Scroll: &scroll})
		for index := range 8 {
			command.AddItem(CommandItem{ID: gfx.Item(10 + index), Label: "Command", Selected: index == 6})
		}
		command.End()
		hits := map[gfx.ID]gfx.Rect{}
		for _, hit := range canvas.Hits() {
			hits[hit.ID] = hit.Bounds
		}
		return hits
	}
	paint()
	if !scroll.Reveal(gfx.Item(16)) {
		t.Fatal("revealing the seventh of eight results moved nothing")
	}
	row, ok := paint()[gfx.Item(16)]
	if !ok || row.Y < bounds.Y+36 || row.Y+row.Height > bounds.Y+bounds.Height {
		t.Fatalf("revealed result at %+v (shown %v), outside the list", row, ok)
	}
	if scroll.Reveal(gfx.Item(99)) {
		t.Fatal("revealing a result that was not painted moved the list")
	}
}
