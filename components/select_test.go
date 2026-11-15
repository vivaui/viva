package components

import (
	"slices"
	"testing"

	"github.com/vivaui/viva/gfx"
)

func TestSelectPaintsTypedTriggerAndScopedOptions(t *testing.T) {
	theme := gfx.LightTheme()
	var canvas gfx.Canvas
	selectControl := BeginSelect(&canvas, theme, gfx.Rect{X: 10, Y: 12, Width: 160}, SelectOptions{
		TriggerID: gfx.Item(1), Label: "Density", Value: "Comfortable", Open: true,
		Content: gfx.Rect{X: 10, Y: 48, Width: 120, Height: 132},
		Layer:   9, Scope: gfx.Item(31), DismissID: gfx.Item(7),
	})
	selectControl.AddLabel("Density")
	selectedBounds := selectControl.AddItem(SelectItem{ID: gfx.Item(2), Label: "Comfortable", Value: "comfortable", Selected: true})
	selectControl.AddItem(SelectItem{ID: gfx.Item(3), Label: "Compact", Value: "compact", Disabled: true})
	selectControl.AddScrollDown(gfx.Item(4), false)

	if selectControl.TriggerBounds().Height != theme.Metrics.ControlHeight || selectControl.ContentBounds().Width != 160 {
		t.Fatalf("select geometry = trigger %+v content %+v", selectControl.TriggerBounds(), selectControl.ContentBounds())
	}
	if selectedBounds != (gfx.Rect{X: 14, Y: 80, Width: 152, Height: 32}) {
		t.Fatalf("selected bounds = %+v", selectedBounds)
	}
	if canvas.TrappedFocusScope() != gfx.Item(31) || canvas.EscapeTarget() != gfx.Item(7) {
		t.Fatalf("scope=%d escape=%d", canvas.TrappedFocusScope(), canvas.EscapeTarget())
	}
	hits := nonBlockHits(canvas.Hits())
	if len(hits) != 4 {
		t.Fatalf("select hits = %+v", hits)
	}
	trigger := hits[0]
	if !trigger.Scope.IsZero() || trigger.Role != gfx.RoleCombobox || trigger.Value != "Comfortable" || !trigger.Expanded {
		t.Fatalf("select trigger = %+v", trigger)
	}
	selected := hits[1]
	disabled := hits[2]
	if selected.Scope != gfx.Item(31) || !selected.Selected || selected.Value != "comfortable" || disabled.Scope != gfx.Item(31) || !disabled.Disabled || disabled.Focusable {
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
	var canvas gfx.Canvas
	selectControl := BeginSelect(&canvas, gfx.DarkTheme(), gfx.Rect{Width: 140}, SelectOptions{
		TriggerID: gfx.Item(5), Label: "Theme", Placeholder: "Choose theme", Disabled: true,
		Invalid: true, Size: SelectSizeSmall,
	})
	if selectControl.TriggerBounds().Height != gfx.DarkTheme().Metrics.ControlHeight-4 || selectControl.ContentBounds() != (gfx.Rect{}) {
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
	if len(rectangles) != 2 || rectangles[0].Color == gfx.DarkTheme().Input.Over(gfx.DarkTheme().Background) || rectangles[0].Color.Red <= rectangles[0].Color.Green {
		t.Fatalf("invalid trigger rectangles = %+v", rectangles)
	}
}

func TestSelectExtraSmallTriggerStepsTheTypeDown(t *testing.T) {
	theme := gfx.DarkTheme()
	var canvas gfx.Canvas
	control := BeginSelect(&canvas, theme, gfx.Rect{Width: 168}, SelectOptions{
		TriggerID: gfx.Item(1), Label: "Target", Value: "本机-2", Open: true,
		Content: gfx.Rect{Y: 32, Width: 168, Height: 72}, Scope: gfx.Item(9), DismissID: gfx.Item(10),
		Size: SelectSizeXSmall,
	})
	control.AddItem(SelectItem{ID: gfx.Item(2), Label: "本机-2"})
	control.AddItem(SelectItem{ID: gfx.Item(3), Label: "Staging"})
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
	var canvas gfx.Canvas
	paint := func() {
		canvas.Reset()
		selectControl := BeginSelect(&canvas, gfx.DarkTheme(), gfx.Rect{Width: 150}, SelectOptions{
			TriggerID: gfx.Item(1), Label: "Language", Placeholder: "Choose", Open: true,
			Content: gfx.Rect{Y: 36, Width: 150, Height: 101}, Layer: 6, Scope: gfx.Item(10), DismissID: gfx.Item(11),
		})
		if next := selectControl.NextItemBounds(); next != (gfx.Rect{X: 4, Y: 40, Width: 142, Height: 32}) {
			t.Fatalf("next select item = %+v", next)
		}
		selectControl.AddScrollUp(gfx.Item(7), true)
		selectControl.AddItem(SelectItem{ID: gfx.Item(8), Label: "English", Value: "en", Selected: true})
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
	viewport := gfx.Rect{Width: 800, Height: 600}
	options := SelectOptions{
		TriggerID: gfx.Item(1), Label: "Density", Value: "Comfortable", Open: true,
		Content: gfx.Rect{X: 10, Y: 48, Width: 160, Height: 80},
		Layer:   9, Scope: gfx.Item(31), DismissID: gfx.Item(7), Viewport: viewport,
	}
	var canvas gfx.Canvas
	selectControl := BeginSelect(&canvas, gfx.DarkTheme(), gfx.Rect{X: 10, Y: 12, Width: 160}, options)
	selectControl.AddItem(SelectItem{ID: gfx.Item(2), Label: "Comfortable"})
	hits := canvas.Hits()
	if len(hits) != 5 || hits[0].ID != gfx.Item(1) || hits[1].ID != gfx.Item(7) || !hits[2].Block || !hits[3].Container || hits[3].Role != gfx.RoleListBox || hits[4].ID != gfx.Item(2) {
		t.Fatalf("hits = %+v", hits)
	}
	if backdrop := hits[1]; backdrop.Bounds != viewport || backdrop.Layer != 9 || backdrop.Scope != gfx.Item(31) || backdrop.Focusable {
		t.Fatalf("backdrop = %+v", backdrop)
	}

	canvas.Reset()
	options.Open = false
	BeginSelect(&canvas, gfx.DarkTheme(), gfx.Rect{X: 10, Y: 12, Width: 160}, options)
	if len(canvas.Hits()) != 1 {
		t.Fatalf("a closed select declares only its trigger: %+v", canvas.Hits())
	}

	canvas.Reset()
	options.Open, options.Viewport = true, gfx.Rect{}
	BeginSelect(&canvas, gfx.DarkTheme(), gfx.Rect{X: 10, Y: 12, Width: 160}, options)
	if hits := canvas.Hits(); len(hits) != 3 || hits[0].ID != gfx.Item(1) || !hits[1].Block || !hits[2].Container {
		t.Fatalf("without a viewport the trigger, surface block and listbox are declared: %+v", hits)
	}
}

func BenchmarkSelectWithBackdrop(b *testing.B) {
	theme := gfx.DarkTheme()
	var canvas gfx.Canvas
	b.ReportAllocs()
	for b.Loop() {
		canvas.Reset()
		selectControl := BeginSelect(&canvas, theme, gfx.Rect{X: 10, Y: 12, Width: 160}, SelectOptions{
			TriggerID: gfx.Item(1), Label: "Density", Value: "Comfortable", Open: true,
			Content: gfx.Rect{X: 10, Y: 48, Width: 160, Height: 112},
			Layer:   9, Scope: gfx.Item(31), DismissID: gfx.Item(7), Viewport: gfx.Rect{Width: 800, Height: 600},
		})
		selectControl.AddItem(SelectItem{ID: gfx.Item(2), Label: "Comfortable"})
		selectControl.AddItem(SelectItem{ID: gfx.Item(3), Label: "Compact"})
	}
}

func TestSelectIconsShiftLabelsAndStayAllocationFree(t *testing.T) {
	icon := gfx.Icon{ViewBox: 24, Layers: []gfx.IconLayer{{
		Fill: gfx.Color{Red: 1, Alpha: 1}, Commands: []gfx.PathCommand{gfx.MoveTo(2, 2), gfx.LineTo(22, 2), gfx.LineTo(12, 22), gfx.ClosePath()},
	}}}
	var plain, withIcon gfx.Canvas
	paint := func(canvas *gfx.Canvas, icon gfx.Icon) {
		canvas.Reset()
		selectControl := BeginSelect(canvas, gfx.DarkTheme(), gfx.Rect{Width: 200}, SelectOptions{
			TriggerID: gfx.Item(1), Label: "Provider", Value: "Claude", Icon: icon, Open: true,
			Content: gfx.Rect{Y: 36, Width: 200, Height: 80}, Layer: 6, Scope: gfx.Item(10),
		})
		selectControl.AddItem(SelectItem{ID: gfx.Item(2), Label: "Claude", Selected: true, Icon: icon})
	}
	paint(&plain, gfx.Icon{})
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
	var canvas gfx.Canvas
	paint := func() {
		canvas.Reset()
		control := BeginSelect(&canvas, gfx.LightTheme(), gfx.Rect{Width: 200}, SelectOptions{
			TriggerID: gfx.Item(1), Label: "Language", Value: "繁體中文", IconText: "繁", Open: true,
			Content: gfx.Rect{Y: 36, Width: 200, Height: 80}, Layer: 6, Scope: gfx.Item(10),
		})
		control.AddItem(SelectItem{ID: gfx.Item(2), Label: "繁體中文", IconText: "繁"})
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
	bitmap := gfx.Bitmap{Pix: make([]byte, 4*3*4), Width: 4, Height: 3, Stride: 16, ID: 100}
	var canvas gfx.Canvas
	paint := func() {
		canvas.Reset()
		control := BeginSelect(&canvas, gfx.LightTheme(), gfx.Rect{Width: 200}, SelectOptions{
			TriggerID: gfx.Item(1), Label: "Language", Value: "English", IconImage: bitmap, Open: true,
			Content: gfx.Rect{Y: 36, Width: 200, Height: 80}, Layer: 6, Scope: gfx.Item(10),
		})
		control.AddItem(SelectItem{ID: gfx.Item(2), Label: "English", IconImage: bitmap})
		control.End()
	}
	paint()
	images := canvas.Images()
	if len(images) != 2 || images[0].Bounds != (gfx.Rect{X: testInset, Y: 10, Width: 16, Height: 12}) || images[1].Bounds != (gfx.Rect{X: 12, Y: 50, Width: 16, Height: 12}) {
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
	var canvas gfx.Canvas
	BeginSelect(&canvas, gfx.LightTheme(), gfx.Rect{Width: 200}, SelectOptions{
		TriggerID: gfx.Item(1), Value: "Traditional Chinese", IconText: "繁",
		IconImage: gfx.Bitmap{Pix: []byte{0, 0, 0, 0}, Width: 2, Height: 2, Stride: 8},
	})
	if len(canvas.Images()) != 0 || len(canvas.Texts()) != 2 || canvas.Texts()[0].Value != "繁" {
		t.Fatal("an image with short pixel storage hid the text icon")
	}
}

// The arrow of a select must sit on the vertical middle of its trigger. It was a
// text glyph, which sits low in its line box, so it hung below the middle.
func TestSelectChevronIsVerticallyCenteredInItsTrigger(t *testing.T) {
	for _, size := range []SelectSize{SelectSizeDefault, SelectSizeSmall} {
		var canvas gfx.Canvas
		selectControl := BeginSelect(&canvas, gfx.DarkTheme(), gfx.Rect{X: 10, Y: 20, Width: 200}, SelectOptions{TriggerID: gfx.Item(1), Label: "Provider", Value: "Claude", Size: size})
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
	paint := func() (*gfx.Canvas, []gfx.Rect) {
		canvas := &gfx.Canvas{}
		control := BeginSelect(canvas, gfx.DarkTheme(), gfx.Rect{X: 10, Y: 10, Width: 160}, SelectOptions{
			TriggerID: gfx.Item(1), Open: true, Layer: 7, Scope: gfx.Item(2),
			Content: gfx.Rect{X: 10, Y: 46, Width: 160, Height: 104}, Scroll: &scroll,
		})
		var rows []gfx.Rect
		for index := range 8 {
			rows = append(rows, control.AddItem(SelectItem{ID: gfx.Item(100 + index), Label: "Item"}))
		}
		control.End()
		return canvas, rows
	}
	if _, rows := paint(); rows[2] == (gfx.Rect{}) || rows[3] != (gfx.Rect{}) || scroll.Max() <= 0 {
		t.Fatalf("at the top rows 2 and 3 are %+v and %+v, scroll range %v", rows[2], rows[3], scroll.Max())
	}
	scroll.Wheel(gfx.ScrollEvent{DeltaY: 10000})
	canvas, rows := paint()
	if rows[0] != (gfx.Rect{}) || rows[7] == (gfx.Rect{}) {
		t.Fatalf("scrolled to the end the first and last rows are %+v and %+v", rows[0], rows[7])
	}
	reached := map[gfx.ID]bool{}
	for _, hit := range canvas.Hits() {
		reached[hit.ID] = true
	}
	if !reached[gfx.Item(107)] || reached[gfx.Item(100)] {
		t.Fatalf("hits after scrolling: last %v, first %v", reached[gfx.Item(107)], reached[gfx.Item(100)])
	}
}

// A select open inside a dialog is the deeper overlay: its own trap and Escape
// target survive the dialog finishing its paint, and the dialog takes them back
// on the frame after the list closes.
func TestSelectInsideADialogKeepsItsTrapAndEscape(t *testing.T) {
	paint := func(withSelect bool) *gfx.Canvas {
		var canvas gfx.Canvas
		dialog := BeginDialog(&canvas, gfx.LightTheme(), gfx.Rect{Width: 800, Height: 600}, DialogOptions{
			Layer: 7, Scope: gfx.Item(91), DismissID: gfx.Item(11), CloseID: gfx.Item(12), ContentSize: gfx.Size{Width: 400, Height: 220},
		})
		layout := dialog.Begin("Preset", "Choose a preset.")
		if withSelect {
			selectControl := BeginSelect(&canvas, gfx.LightTheme(), gfx.Rect{X: layout.Body.X, Y: layout.Body.Y, Width: 200}, SelectOptions{
				TriggerID: gfx.Item(20), Label: "Preset", Open: true,
				Content: gfx.Rect{X: layout.Body.X, Y: layout.Body.Y + 36, Width: 200, Height: 80},
				Layer:   8, Scope: gfx.Item(31), DismissID: gfx.Item(7),
			})
			selectControl.AddItem(SelectItem{ID: gfx.Item(21), Label: "Fast"})
			selectControl.End()
		}
		dialog.End()
		return &canvas
	}
	open := paint(true)
	if open.TrappedFocusScope() != gfx.Item(31) || open.EscapeTarget() != gfx.Item(7) {
		t.Fatalf("open select: trap=%d escape=%d, want the list's", open.TrappedFocusScope(), open.EscapeTarget())
	}
	closed := paint(false)
	if closed.TrappedFocusScope() != gfx.Item(91) || closed.EscapeTarget() != gfx.Item(11) {
		t.Fatalf("closed select: trap=%d escape=%d, want the dialog's", closed.TrappedFocusScope(), closed.EscapeTarget())
	}
}

// An open list declared inside a scroll area is not cut by it: its surface, rows
// and scrollbar are painted and hit-tested against the viewport, and the area
// still owns its clip when the select is done.
func TestSelectPopupEscapesAncestorScrollClip(t *testing.T) {
	var scroll MenuScroll
	viewport := gfx.Rect{Width: 400, Height: 300}
	body := gfx.Rect{X: 10, Y: 10, Width: 200, Height: 80}
	paint := func(withViewport bool) *gfx.Canvas {
		var canvas gfx.Canvas
		options := SelectOptions{
			TriggerID: gfx.Item(1), Label: "Density", Open: true, Layer: 6, Scope: gfx.Item(10), DismissID: gfx.Item(9),
			Content: gfx.Rect{X: 10, Y: 70, Width: 200, Height: 8 + 32*6}, Scroll: &scroll,
		}
		if withViewport {
			options.Viewport = viewport
		}
		canvas.Clip(body)
		control := BeginSelect(&canvas, gfx.LightTheme(), gfx.Rect{X: 10, Y: 40, Width: 200}, options)
		for index := range 6 {
			control.AddItem(SelectItem{ID: gfx.Item(100 + index), Label: "Option"})
		}
		control.End()
		canvas.Unclip()
		return &canvas
	}
	// The viewport reaches below the scroll area: the last row is painted whole and
	// its hit is not cut at the area's bottom edge.
	canvas := paint(true)
	popup := gfx.Rect{X: 10, Y: 70, Width: 200, Height: 8 + 32*6}
	foundSurface := false
	for _, r := range canvas.Rectangles() {
		if r.Bounds == popup {
			foundSurface = true
			if !r.HasClip || r.ClipOf != viewport {
				t.Fatalf("popup surface inherited the scroll clip: %+v", r)
			}
		}
	}
	if !foundSurface {
		t.Fatal("the popup surface was not painted")
	}
	var last gfx.Hit
	for _, hit := range canvas.Hits() {
		if hit.ID == gfx.Item(105) {
			last = hit
		}
	}
	if last.Bounds.Height != 32 || last.Bounds.Y+last.Bounds.Height > viewport.Y+viewport.Height {
		t.Fatalf("the last row's hit is %+v, want a whole row inside the viewport", last.Bounds)
	}
	// Without a viewport the select keeps the ancestor clip, as it always has.
	clipped := paint(false)
	for _, r := range clipped.Rectangles() {
		if r.Bounds == popup && r.ClipOf != body {
			t.Fatalf("a select without a viewport left the scroll clip: %+v", r)
		}
	}
}

// The popup's scrollbar floats with the rows: a list cut by a scroll area shows the
// bar inside the viewport too.
func TestSelectPopupScrollbarEscapesAncestorScrollClip(t *testing.T) {
	var scroll MenuScroll
	viewport := gfx.Rect{Width: 400, Height: 300}
	var canvas gfx.Canvas
	canvas.Clip(gfx.Rect{X: 10, Y: 10, Width: 200, Height: 80})
	control := BeginSelect(&canvas, gfx.LightTheme(), gfx.Rect{X: 10, Y: 40, Width: 200}, SelectOptions{
		TriggerID: gfx.Item(1), Open: true, Layer: 6, Viewport: viewport,
		Content: gfx.Rect{X: 10, Y: 70, Width: 200, Height: 8 + 32*2}, Scroll: &scroll,
	})
	for index := range 6 {
		control.AddItem(SelectItem{ID: gfx.Item(100 + index), Label: "Option"})
	}
	control.End()
	canvas.Unclip()
	if scroll.Max() <= 0 {
		t.Fatal("the fixture did not overflow its popup")
	}
	bar := false
	for _, r := range canvas.Rectangles() {
		if r.Bounds.Width == 4 && r.Bounds.Height >= 18 && r.Bounds.X == 10+200-6 {
			bar = true
			if r.ClipOf != viewport {
				t.Fatalf("the scrollbar is clipped to the scroll area: %+v", r)
			}
		}
	}
	if !bar {
		t.Fatal("the scrollbar was not painted")
	}
}
