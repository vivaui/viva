package components

import (
	"slices"
	"testing"

	"github.com/vivaui/viva/ui"
)

func TestCommandCapturesPaletteContractAndItemState(t *testing.T) {
	var canvas ui.Canvas
	canvas.SetInteraction(ui.ID{}, ui.ID{}, ui.Item(12), true)
	command := BeginCommand(&canvas, ui.LightTheme(), ui.Rect{X: 10, Y: 20, Width: 320, Height: 220}, CommandOptions{
		InputID: ui.Item(9), Label: "Command Palette", Query: "set", Variant: CommandDialog,
		Layer: 8, Scope: ui.Item(44), DismissID: ui.Item(90),
	})
	if canvas.TrappedFocusScope() != ui.Item(44) || canvas.EscapeTarget() != ui.Item(90) {
		t.Fatalf("overlay contract scope=%d escape=%d", canvas.TrappedFocusScope(), canvas.EscapeTarget())
	}
	command.AddGroup("Settings")
	itemBounds := command.AddItem(CommandItem{ID: ui.Item(12), Label: "Open settings", Value: "settings", Shortcut: "⌘,", Selected: true})
	command.AddItem(CommandItem{ID: ui.Item(13), Label: "Toggle panel", Checked: true})
	command.AddItem(CommandItem{ID: ui.Item(14), Label: "Unavailable", Disabled: true})
	if itemBounds != (ui.Rect{X: 14, Y: 96, Width: 312, Height: 44}) {
		t.Fatalf("first item bounds = %+v", itemBounds)
	}
	hits := nonBlockHits(canvas.Hits())
	if len(hits) != 4 {
		t.Fatalf("hits = %+v", hits)
	}
	if input := hits[0]; input.Role != ui.RoleCombobox || input.Scope != ui.Item(44) || input.Value != "set" || !input.Expanded {
		t.Fatalf("input = %+v", input)
	}
	if item := hits[1]; item.Role != ui.RoleOption || !item.Selected || item.Scope != ui.Item(44) {
		t.Fatalf("selected item = %+v", item)
	}
	if checked := hits[2]; !checked.Checked || checked.Role != ui.RoleOption {
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
	var idle, pointer, keyboard, selected ui.Canvas
	paint := func(canvas *ui.Canvas, selectedRow bool) {
		command := BeginCommand(canvas, ui.LightTheme(), ui.Rect{Width: 260, Height: 150}, CommandOptions{InputID: ui.Item(1), Label: "Commands"})
		command.AddItem(CommandItem{ID: ui.Item(2), Label: "New window", Selected: selectedRow})
	}
	paint(&idle, false)
	pointer.SetInteraction(ui.ID{}, ui.ID{}, ui.Item(2), false)
	paint(&pointer, false)
	if !slices.Equal(pointer.Rectangles(), idle.Rectangles()) {
		t.Fatal("pointer focus tinted a row the caller did not select")
	}
	keyboard.SetInteraction(ui.ID{}, ui.ID{}, ui.Item(2), true)
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
	var canvas ui.Canvas
	paint := func() {
		canvas.Reset()
		command := BeginCommand(&canvas, ui.DarkTheme(), ui.Rect{X: 4, Y: 5, Width: 260, Height: 150}, CommandOptions{InputID: ui.Item(1), Label: "Commands", Placeholder: "Type a command", Layer: 3, Scope: ui.Item(7), DismissID: ui.Item(8)})
		command.AddGroup("General")
		command.AddItem(CommandItem{ID: ui.Item(2), Label: "New window", Shortcut: "⌘N"})
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

func commandAssertLayer(t *testing.T, rectangles []ui.Rectangle, texts []ui.Text, layer uint16) {
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
	icon := ui.Icon{ViewBox: 24, Stroke: 2, Commands: []ui.PathCommand{ui.MoveTo(4, 12), ui.LineTo(20, 12)}}
	bounds := ui.Rect{X: 20, Y: 30, Width: 300, Height: 36 + 4 + 3*32}
	paint := func() (*ui.Canvas, []ui.Rect) {
		canvas := &ui.Canvas{}
		command := BeginCommand(canvas, ui.LightTheme(), bounds, CommandOptions{InputID: ui.Item(1), Layer: 4, Scope: ui.Item(2), Scroll: &scroll})
		var rows []ui.Rect
		for index := range 8 {
			rows = append(rows, command.AddItem(CommandItem{ID: ui.Item(10 + index), Label: "Command", Icon: icon}))
		}
		command.End()
		return canvas, rows
	}
	canvas, rows := paint()
	if rows[0] == (ui.Rect{}) || rows[7] != (ui.Rect{}) || scroll.Max() <= 0 {
		t.Fatalf("first row %+v, last row %+v, range %v: want the list cut at the bounds", rows[0], rows[7], scroll.Max())
	}
	for _, hit := range nonBlockHits(canvas.Hits()) {
		if hit.ID != ui.Item(1) && hit.Bounds.Y < bounds.Y+36 {
			t.Fatalf("result %+v reaches up into the input row", hit)
		}
	}
	scroll.By(10000)
	_, rows = paint()
	if rows[7] == (ui.Rect{}) || rows[0] != (ui.Rect{}) {
		t.Fatalf("scrolled to the end: first %+v last %+v", rows[0], rows[7])
	}
}

func TestCommandRevealScrollsAResultTheCallerHighlighted(t *testing.T) {
	var scroll MenuScroll
	var canvas ui.Canvas
	bounds := ui.Rect{X: 20, Y: 30, Width: 300, Height: 36 + 4 + 3*32}
	paint := func() map[ui.ID]ui.Rect {
		canvas.Reset()
		command := BeginCommand(&canvas, ui.LightTheme(), bounds, CommandOptions{InputID: ui.Item(1), Scope: ui.Item(2), Scroll: &scroll})
		for index := range 8 {
			command.AddItem(CommandItem{ID: ui.Item(10 + index), Label: "Command", Selected: index == 6})
		}
		command.End()
		hits := map[ui.ID]ui.Rect{}
		for _, hit := range canvas.Hits() {
			hits[hit.ID] = hit.Bounds
		}
		return hits
	}
	paint()
	if !scroll.Reveal(ui.Item(16)) {
		t.Fatal("revealing the seventh of eight results moved nothing")
	}
	row, ok := paint()[ui.Item(16)]
	if !ok || row.Y < bounds.Y+36 || row.Y+row.Height > bounds.Y+bounds.Height {
		t.Fatalf("revealed result at %+v (shown %v), outside the list", row, ok)
	}
	if scroll.Reveal(ui.Item(99)) {
		t.Fatal("revealing a result that was not painted moved the list")
	}
}
