package components

import (
	"slices"
	"testing"

	"github.com/vivaui/viva/ui"
)

func TestSelectPaintsTypedTriggerAndScopedOptions(t *testing.T) {
	theme := ui.LightTheme()
	var canvas ui.Canvas
	selectControl := BeginSelect(&canvas, theme, ui.Rect{X: 10, Y: 12, Width: 160}, SelectOptions{
		TriggerID: ui.Item(1), Label: "Density", Value: "Comfortable", Open: true,
		Content: ui.Rect{X: 10, Y: 48, Width: 120, Height: 132},
		Layer:   9, Scope: ui.Item(31), DismissID: ui.Item(7),
	})
	selectControl.AddLabel("Density")
	selectedBounds := selectControl.AddItem(SelectItem{ID: ui.Item(2), Label: "Comfortable", Value: "comfortable", Selected: true})
	selectControl.AddItem(SelectItem{ID: ui.Item(3), Label: "Compact", Value: "compact", Disabled: true})
	selectControl.AddScrollDown(ui.Item(4), false)

	if selectControl.TriggerBounds().Height != theme.Metrics.ControlHeight || selectControl.ContentBounds().Width != 160 {
		t.Fatalf("select geometry = trigger %+v content %+v", selectControl.TriggerBounds(), selectControl.ContentBounds())
	}
	if selectedBounds != (ui.Rect{X: 14, Y: 80, Width: 152, Height: 32}) {
		t.Fatalf("selected bounds = %+v", selectedBounds)
	}
	if canvas.TrappedFocusScope() != ui.Item(31) || canvas.EscapeTarget() != ui.Item(7) {
		t.Fatalf("scope=%d escape=%d", canvas.TrappedFocusScope(), canvas.EscapeTarget())
	}
	hits := nonBlockHits(canvas.Hits())
	if len(hits) != 4 {
		t.Fatalf("select hits = %+v", hits)
	}
	trigger := hits[0]
	if !trigger.Scope.IsZero() || trigger.Role != ui.RoleCombobox || trigger.Value != "Comfortable" || !trigger.Expanded {
		t.Fatalf("select trigger = %+v", trigger)
	}
	selected := hits[1]
	disabled := hits[2]
	if selected.Scope != ui.Item(31) || !selected.Selected || selected.Value != "comfortable" || disabled.Scope != ui.Item(31) || !disabled.Disabled || disabled.Focusable {
		t.Fatalf("select item states = selected %+v disabled %+v", selected, disabled)
	}
	for _, rectangle := range canvas.Rectangles()[:2] {
		if rectangle.Layer != 0 {
			t.Fatalf("trigger rectangle layer = %+v", rectangle)
		}
	}
	foundOverlay := false
	for _, rectangle := range canvas.Rectangles()[2:] {
		if rectangle.Layer == 9 {
			foundOverlay = true
		}
	}
	if !foundOverlay {
		t.Fatal("select content emitted no overlay-layer rectangles")
	}
}

func TestSelectClosedSmallDisabledAndInvalidTrigger(t *testing.T) {
	var canvas ui.Canvas
	selectControl := BeginSelect(&canvas, ui.DarkTheme(), ui.Rect{Width: 140}, SelectOptions{
		TriggerID: ui.Item(5), Label: "Theme", Placeholder: "Choose theme", Disabled: true,
		Invalid: true, Size: SelectSizeSmall,
	})
	if selectControl.TriggerBounds().Height != ui.DarkTheme().Metrics.ControlHeight-4 || selectControl.ContentBounds() != (ui.Rect{}) {
		t.Fatalf("closed select geometry = trigger %+v content %+v", selectControl.TriggerBounds(), selectControl.ContentBounds())
	}
	if !canvas.TrappedFocusScope().IsZero() || !canvas.EscapeTarget().IsZero() {
		t.Fatal("closed select installed overlay routing")
	}
	if len(canvas.Hits()) != 1 || !canvas.Hits()[0].Disabled || canvas.Hits()[0].Focusable || canvas.Hits()[0].Expanded {
		t.Fatalf("disabled trigger = %+v", canvas.Hits())
	}
	// The border and fill, with no halo; the border is the destructive one
	// faded by the disabled state.
	rectangles := canvas.Rectangles()
	if len(rectangles) != 2 || rectangles[0].Color == ui.DarkTheme().Input.Over(ui.DarkTheme().Background) || rectangles[0].Color.Red <= rectangles[0].Color.Green {
		t.Fatalf("invalid trigger rectangles = %+v", rectangles)
	}
}

func TestSelectExtraSmallTriggerStepsTheTypeDown(t *testing.T) {
	theme := ui.DarkTheme()
	var canvas ui.Canvas
	control := BeginSelect(&canvas, theme, ui.Rect{Width: 168}, SelectOptions{
		TriggerID: ui.Item(1), Label: "Target", Value: "本机-2", Open: true,
		Content: ui.Rect{Y: 32, Width: 168, Height: 72}, Scope: ui.Item(9), DismissID: ui.Item(10),
		Size: SelectSizeXSmall,
	})
	control.AddItem(SelectItem{ID: ui.Item(2), Label: "本机-2"})
	control.AddItem(SelectItem{ID: ui.Item(3), Label: "Staging"})
	if height := control.TriggerBounds().Height; height != theme.Metrics.ControlHeight-8 {
		t.Fatalf("extra-small trigger height = %v", height)
	}
	texts := canvas.Texts()
	if len(texts) != 3 || texts[0].Value != "本机-2" || texts[0].Style.Size != theme.Metrics.FontSizeS {
		t.Fatalf("extra-small trigger text = %+v", texts)
	}
	// A compact trigger still opens the body-size menu a desktop select opens.
	for _, item := range texts[1:] {
		if item.Style.Size != theme.Metrics.FontSizeM {
			t.Fatalf("extra-small popup item text = %+v", item)
		}
	}
}

func TestSelectGeometryAndCommandStreamAreDeterministic(t *testing.T) {
	var canvas ui.Canvas
	paint := func() {
		canvas.Reset()
		selectControl := BeginSelect(&canvas, ui.DarkTheme(), ui.Rect{Width: 150}, SelectOptions{
			TriggerID: ui.Item(1), Label: "Language", Placeholder: "Choose", Open: true,
			Content: ui.Rect{Y: 36, Width: 150, Height: 101}, Layer: 6, Scope: ui.Item(10), DismissID: ui.Item(11),
		})
		if next := selectControl.NextItemBounds(); next != (ui.Rect{X: 4, Y: 40, Width: 142, Height: 32}) {
			t.Fatalf("next select item = %+v", next)
		}
		selectControl.AddScrollUp(ui.Item(7), true)
		selectControl.AddItem(SelectItem{ID: ui.Item(8), Label: "English", Value: "en", Selected: true})
		selectControl.AddSeparator()
	}
	paint()
	rectangles := slices.Clone(canvas.Rectangles())
	texts := slices.Clone(canvas.Texts())
	hits := slices.Clone(canvas.Hits())
	paint()
	if !slices.Equal(rectangles, canvas.Rectangles()) || !slices.Equal(texts, canvas.Texts()) || !slices.Equal(hits, canvas.Hits()) {
		t.Fatal("identical selects produced different command streams")
	}
	if allocations := testing.AllocsPerRun(100, paint); allocations != 0 {
		t.Fatalf("select paint allocations = %v", allocations)
	}
}

func TestSelectDismissBackdropSitsBehindThePopupAndOnlyWhenOpen(t *testing.T) {
	viewport := ui.Rect{Width: 800, Height: 600}
	options := SelectOptions{
		TriggerID: ui.Item(1), Label: "Density", Value: "Comfortable", Open: true,
		Content: ui.Rect{X: 10, Y: 48, Width: 160, Height: 80},
		Layer:   9, Scope: ui.Item(31), DismissID: ui.Item(7), Viewport: viewport,
	}
	var canvas ui.Canvas
	selectControl := BeginSelect(&canvas, ui.DarkTheme(), ui.Rect{X: 10, Y: 12, Width: 160}, options)
	selectControl.AddItem(SelectItem{ID: ui.Item(2), Label: "Comfortable"})
	hits := canvas.Hits()
	if len(hits) != 5 || hits[0].ID != ui.Item(1) || hits[1].ID != ui.Item(7) || !hits[2].Block || !hits[3].Container || hits[3].Role != ui.RoleListBox || hits[4].ID != ui.Item(2) {
		t.Fatalf("hits = %+v", hits)
	}
	if backdrop := hits[1]; backdrop.Bounds != viewport || backdrop.Layer != 9 || backdrop.Scope != ui.Item(31) || backdrop.Focusable {
		t.Fatalf("backdrop = %+v", backdrop)
	}

	canvas.Reset()
	options.Open = false
	BeginSelect(&canvas, ui.DarkTheme(), ui.Rect{X: 10, Y: 12, Width: 160}, options)
	if len(canvas.Hits()) != 1 {
		t.Fatalf("a closed select declares only its trigger: %+v", canvas.Hits())
	}

	canvas.Reset()
	options.Open, options.Viewport = true, ui.Rect{}
	BeginSelect(&canvas, ui.DarkTheme(), ui.Rect{X: 10, Y: 12, Width: 160}, options)
	if hits := canvas.Hits(); len(hits) != 3 || hits[0].ID != ui.Item(1) || !hits[1].Block || !hits[2].Container {
		t.Fatalf("without a viewport the trigger, surface block and listbox are declared: %+v", hits)
	}
}

func BenchmarkSelectWithBackdrop(b *testing.B) {
	theme := ui.DarkTheme()
	var canvas ui.Canvas
	b.ReportAllocs()
	for b.Loop() {
		canvas.Reset()
		selectControl := BeginSelect(&canvas, theme, ui.Rect{X: 10, Y: 12, Width: 160}, SelectOptions{
			TriggerID: ui.Item(1), Label: "Density", Value: "Comfortable", Open: true,
			Content: ui.Rect{X: 10, Y: 48, Width: 160, Height: 112},
			Layer:   9, Scope: ui.Item(31), DismissID: ui.Item(7), Viewport: ui.Rect{Width: 800, Height: 600},
		})
		selectControl.AddItem(SelectItem{ID: ui.Item(2), Label: "Comfortable"})
		selectControl.AddItem(SelectItem{ID: ui.Item(3), Label: "Compact"})
	}
}

func TestSelectIconsShiftLabelsAndStayAllocationFree(t *testing.T) {
	icon := ui.Icon{ViewBox: 24, Layers: []ui.IconLayer{{
		Fill: ui.Color{Red: 1, Alpha: 1}, Commands: []ui.PathCommand{ui.MoveTo(2, 2), ui.LineTo(22, 2), ui.LineTo(12, 22), ui.ClosePath()},
	}}}
	var plain, withIcon ui.Canvas
	paint := func(canvas *ui.Canvas, icon ui.Icon) {
		canvas.Reset()
		selectControl := BeginSelect(canvas, ui.DarkTheme(), ui.Rect{Width: 200}, SelectOptions{
			TriggerID: ui.Item(1), Label: "Provider", Value: "Claude", Icon: icon, Open: true,
			Content: ui.Rect{Y: 36, Width: 200, Height: 80}, Layer: 6, Scope: ui.Item(10),
		})
		selectControl.AddItem(SelectItem{ID: ui.Item(2), Label: "Claude", Selected: true, Icon: icon})
	}
	paint(&plain, ui.Icon{})
	paint(&withIcon, icon)
	// Both draw the trigger's chevron as triangles; the icons add their own.
	if len(withIcon.Triangles()) <= len(plain.Triangles()) {
		t.Fatalf("triangles without icon %d, with icon %d", len(plain.Triangles()), len(withIcon.Triangles()))
	}
	// Texts are the trigger value, the item label and its check.
	for _, index := range []int{0, 1} {
		got := withIcon.Texts()[index].Bounds.X - plain.Texts()[index].Bounds.X
		if got != selectIconSize+selectIconGap {
			t.Fatalf("text %d moved %v, want %v", index, got, selectIconSize+selectIconGap)
		}
	}
	paint(&withIcon, icon)
	if allocations := testing.AllocsPerRun(100, func() { paint(&withIcon, icon) }); allocations != 0 {
		t.Fatalf("select with icons allocations = %v", allocations)
	}
}

func TestSelectTextIconUsesTheIconSlot(t *testing.T) {
	var canvas ui.Canvas
	paint := func() {
		canvas.Reset()
		control := BeginSelect(&canvas, ui.LightTheme(), ui.Rect{Width: 200}, SelectOptions{
			TriggerID: ui.Item(1), Label: "Language", Value: "繁體中文", IconText: "繁", Open: true,
			Content: ui.Rect{Y: 36, Width: 200, Height: 80}, Layer: 6, Scope: ui.Item(10),
		})
		control.AddItem(SelectItem{ID: ui.Item(2), Label: "繁體中文", IconText: "繁"})
		control.End()
	}
	paint()
	texts := canvas.Texts()
	if len(texts) != 4 || texts[0].Value != "繁" || texts[2].Value != "繁" {
		t.Fatalf("missing text icons: %+v", texts)
	}
	if texts[1].Bounds.X != testInset+selectIconSize+selectIconGap || texts[3].Bounds.X != 4+8+selectIconSize+selectIconGap {
		t.Fatalf("text icons did not reserve the icon slot: %+v", texts)
	}
	if allocations := testing.AllocsPerRun(100, paint); allocations != 0 {
		t.Fatalf("select with text icons allocations = %v", allocations)
	}
}

func TestSelectImageIconPreservesItsAspectRatioAndStaysAllocationFree(t *testing.T) {
	bitmap := ui.Bitmap{Pix: make([]byte, 4*3*4), Width: 4, Height: 3, Stride: 16, ID: 100}
	var canvas ui.Canvas
	paint := func() {
		canvas.Reset()
		control := BeginSelect(&canvas, ui.LightTheme(), ui.Rect{Width: 200}, SelectOptions{
			TriggerID: ui.Item(1), Label: "Language", Value: "English", IconImage: bitmap, Open: true,
			Content: ui.Rect{Y: 36, Width: 200, Height: 80}, Layer: 6, Scope: ui.Item(10),
		})
		control.AddItem(SelectItem{ID: ui.Item(2), Label: "English", IconImage: bitmap})
		control.End()
	}
	paint()
	images := canvas.Images()
	if len(images) != 2 || images[0].Bounds != (ui.Rect{X: testInset, Y: 10, Width: 16, Height: 12}) || images[1].Bounds != (ui.Rect{X: 12, Y: 50, Width: 16, Height: 12}) {
		t.Fatalf("image icon geometry = %+v", images)
	}
	if images[0].Layer != 0 || images[1].Layer != 6 || images[0].Bitmap.ID != bitmap.ID || images[1].Bitmap.ID != bitmap.ID {
		t.Fatalf("image icon identity or layer was lost: %+v", images)
	}
	if allocations := testing.AllocsPerRun(100, paint); allocations != 0 {
		t.Fatalf("select with image icons allocations = %v", allocations)
	}
}

func TestSelectInvalidImageKeepsItsTextFallback(t *testing.T) {
	var canvas ui.Canvas
	BeginSelect(&canvas, ui.LightTheme(), ui.Rect{Width: 200}, SelectOptions{
		TriggerID: ui.Item(1), Value: "Traditional Chinese", IconText: "繁",
		IconImage: ui.Bitmap{Pix: []byte{0, 0, 0, 0}, Width: 2, Height: 2, Stride: 8},
	})
	if len(canvas.Images()) != 0 || len(canvas.Texts()) != 2 || canvas.Texts()[0].Value != "繁" {
		t.Fatal("an image with short pixel storage hid the text icon")
	}
}

// The arrow of a select must sit on the vertical middle of its trigger. It was a
// text glyph, which sits low in its line box, so it hung below the middle.
func TestSelectChevronIsVerticallyCenteredInItsTrigger(t *testing.T) {
	for _, size := range []SelectSize{SelectSizeDefault, SelectSizeSmall} {
		var canvas ui.Canvas
		selectControl := BeginSelect(&canvas, ui.DarkTheme(), ui.Rect{X: 10, Y: 20, Width: 200}, SelectOptions{TriggerID: ui.Item(1), Label: "Provider", Value: "Claude", Size: size})
		trigger := selectControl.TriggerBounds()
		if len(canvas.Triangles()) == 0 {
			t.Fatal("the closed select drew no arrow")
		}
		top, bottom := float32(1e9), float32(-1e9)
		for _, triangle := range canvas.Triangles() {
			for _, point := range triangle.Points {
				top, bottom = min(top, point.Y), max(bottom, point.Y)
			}
		}
		if got, want := (top+bottom)/2, trigger.Y+trigger.Height/2; got < want-0.5 || got > want+0.5 {
			t.Fatalf("size %d: the arrow's middle is at %v, the trigger's at %v", size, got, want)
		}
	}
}

// A select with more options than its popup has room for scrolls, as a dropdown
// menu does, instead of dropping the options below the last one that fits.
func TestSelectWithMoreOptionsThanFitScrollsToTheLastOne(t *testing.T) {
	var scroll MenuScroll
	paint := func() (*ui.Canvas, []ui.Rect) {
		canvas := &ui.Canvas{}
		control := BeginSelect(canvas, ui.DarkTheme(), ui.Rect{X: 10, Y: 10, Width: 160}, SelectOptions{
			TriggerID: ui.Item(1), Open: true, Layer: 7, Scope: ui.Item(2),
			Content: ui.Rect{X: 10, Y: 46, Width: 160, Height: 104}, Scroll: &scroll,
		})
		var rows []ui.Rect
		for index := range 8 {
			rows = append(rows, control.AddItem(SelectItem{ID: ui.Item(100 + index), Label: "Item"}))
		}
		control.End()
		return canvas, rows
	}
	if _, rows := paint(); rows[2] == (ui.Rect{}) || rows[3] != (ui.Rect{}) || scroll.Max() <= 0 {
		t.Fatalf("at the top rows 2 and 3 are %+v and %+v, scroll range %v", rows[2], rows[3], scroll.Max())
	}
	scroll.Wheel(ui.ScrollEvent{DeltaY: 10000})
	canvas, rows := paint()
	if rows[0] != (ui.Rect{}) || rows[7] == (ui.Rect{}) {
		t.Fatalf("scrolled to the end the first and last rows are %+v and %+v", rows[0], rows[7])
	}
	reached := map[ui.ID]bool{}
	for _, hit := range canvas.Hits() {
		reached[hit.ID] = true
	}
	if !reached[ui.Item(107)] || reached[ui.Item(100)] {
		t.Fatalf("hits after scrolling: last %v, first %v", reached[ui.Item(107)], reached[ui.Item(100)])
	}
}

// A select open inside a dialog is the deeper overlay: its own trap and Escape
// target survive the dialog finishing its paint, and the dialog takes them back
// on the frame after the list closes.
func TestSelectInsideADialogKeepsItsTrapAndEscape(t *testing.T) {
	paint := func(withSelect bool) *ui.Canvas {
		var canvas ui.Canvas
		dialog := BeginDialog(&canvas, ui.LightTheme(), ui.Rect{Width: 800, Height: 600}, DialogOptions{
			Layer: 7, Scope: ui.Item(91), DismissID: ui.Item(11), CloseID: ui.Item(12), ContentSize: ui.Size{Width: 400, Height: 220},
		})
		layout := dialog.Begin("Preset", "Choose a preset.")
		if withSelect {
			selectControl := BeginSelect(&canvas, ui.LightTheme(), ui.Rect{X: layout.Body.X, Y: layout.Body.Y, Width: 200}, SelectOptions{
				TriggerID: ui.Item(20), Label: "Preset", Open: true,
				Content: ui.Rect{X: layout.Body.X, Y: layout.Body.Y + 36, Width: 200, Height: 80},
				Layer:   8, Scope: ui.Item(31), DismissID: ui.Item(7),
			})
			selectControl.AddItem(SelectItem{ID: ui.Item(21), Label: "Fast"})
			selectControl.End()
		}
		dialog.End()
		return &canvas
	}
	open := paint(true)
	if open.TrappedFocusScope() != ui.Item(31) || open.EscapeTarget() != ui.Item(7) {
		t.Fatalf("open select: trap=%d escape=%d, want the list's", open.TrappedFocusScope(), open.EscapeTarget())
	}
	closed := paint(false)
	if closed.TrappedFocusScope() != ui.Item(91) || closed.EscapeTarget() != ui.Item(11) {
		t.Fatalf("closed select: trap=%d escape=%d, want the dialog's", closed.TrappedFocusScope(), closed.EscapeTarget())
	}
}
