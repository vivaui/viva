package gallery

import (
	"slices"
	"testing"
	"time"

	"github.com/vivaui/viva/gfx"
)

// tallSize returns a window tall enough to show the whole open page without
// scrolling, so nothing is culled and every card or stage is laid out.
func tallSize(state *State, width float32) gfx.Size {
	pageWidth := max(width-pageInset*2, 0)
	return gfx.Size{Width: width, Height: toolbarHeight + pageTopGap + state.contentHeight(pageWidth) + pageBottomGap}
}

func paintTall(state *State, width float32) *gfx.Canvas {
	canvas := &gfx.Canvas{}
	state.Paint(canvas, tallSize(state, width))
	return canvas
}

// click delivers a click on id the way the runtime does: one paint of the whole
// page, the event pass, in which the control that declared id reads Clicked.
func click(state *State, id gfx.ID) {
	canvas := &gfx.Canvas{}
	canvas.BeginEventPass(id, gfx.ClickInfo{})
	state.Paint(canvas, tallSize(state, DefaultWidth))
	canvas.EndEventPass()
}

func hasHit(canvas *gfx.Canvas, id gfx.ID) bool {
	return slices.ContainsFunc(canvas.Hits(), func(hit gfx.Hit) bool { return hit.ID == id })
}

func hasText(canvas *gfx.Canvas, value string) bool {
	return slices.ContainsFunc(canvas.Texts(), func(text gfx.Text) bool { return text.Value == value })
}

func findHit(canvas *gfx.Canvas, id gfx.ID) (gfx.Hit, bool) {
	for _, hit := range canvas.Hits() {
		if hit.ID == id {
			return hit, true
		}
	}
	return gfx.Hit{}, false
}

func TestCatalogListsEveryComponentOnce(t *testing.T) {
	state := New()
	canvas := paintTall(state, DefaultWidth)
	seen := map[gfx.ID]bool{}
	for _, hit := range canvas.Hits() {
		if card, ok := cardOf(hit.ID); ok {
			if seen[hit.ID] {
				t.Fatalf("card %d is declared twice", card)
			}
			seen[hit.ID] = true
			if want := catalog[card].Title; hit.Label != want {
				t.Fatalf("card %d is labelled %q, want %q", card, hit.Label, want)
			}
		}
	}
	if len(seen) != len(catalog) {
		t.Fatalf("%d of %d components have a card", len(seen), len(catalog))
	}
	for _, section := range categories {
		if !hasText(canvas, section.name) {
			t.Errorf("category %q has no heading", section.name)
		}
	}
	if len(catalog) < 60 {
		t.Fatalf("the catalog lists only %d components", len(catalog))
	}
}

func TestCatalogGridFollowsTheWindowWidth(t *testing.T) {
	for _, width := range []float32{420, 680, 1180, 1600, 2400} {
		state := New()
		canvas := paintTall(state, width)
		pageWidth := width - pageInset*2
		columns := catalogGrid.Columns(pageWidth)
		wantWidth := (pageWidth - catalogGrid.Gap*float32(columns-1)) / float32(columns)

		cards := 0
		for _, hit := range canvas.Hits() {
			if _, ok := cardOf(hit.ID); !ok {
				continue
			}
			cards++
			bounds := hit.Bounds
			if bounds.X < pageInset-0.01 || bounds.X+bounds.Width > width-pageInset+0.01 {
				t.Fatalf("width %v: card %q spans [%v, %v], outside the page", width, hit.Label, bounds.X, bounds.X+bounds.Width)
			}
			if d := bounds.Width - wantWidth; d < -0.01 || d > 0.01 {
				t.Fatalf("width %v: card %q is %v wide, want %v", width, hit.Label, bounds.Width, wantWidth)
			}
			if bounds.Width < min(catalogGrid.MinCellWidth, pageWidth)-0.01 {
				t.Fatalf("width %v: card %q is narrower than the minimum", width, hit.Label)
			}
		}
		if cards != len(catalog) {
			t.Fatalf("width %v: %d of %d cards laid out", width, cards, len(catalog))
		}
		// The widest section reaches both page edges: the grid stretches to the
		// window instead of sitting at a fixed size in one corner of it.
		left, right := float32(1e9), float32(0)
		for _, hit := range canvas.Hits() {
			if _, ok := cardOf(hit.ID); ok {
				left, right = min(left, hit.Bounds.X), max(right, hit.Bounds.X+hit.Bounds.Width)
			}
		}
		widest := 0
		for _, section := range categories {
			widest = max(widest, section.count)
		}
		if left != pageInset {
			t.Fatalf("width %v: cards start at %v, want %v", width, left, pageInset)
		}
		if widest >= columns && (right < width-pageInset-0.01 || right > width-pageInset+0.01) {
			t.Fatalf("width %v: a full row ends at %v, want %v", width, right, width-pageInset)
		}
	}
}

func TestHoveredCardKeepsItsSurface(t *testing.T) {
	state := New()
	var canvas gfx.Canvas
	canvas.SetInteraction(cardScope.ID(gfx.Index(0)), gfx.ID{}, gfx.ID{}, false)
	state.Paint(&canvas, gfx.Size{Width: DefaultWidth, Height: DefaultHeight})
	theme := state.Theme()
	card, ok := findHit(&canvas, cardScope.ID(gfx.Index(0)))
	if !ok {
		t.Fatal("the first card was not painted")
	}

	// Probe a point in the card's empty top-right interior, clear of the ring
	// and of the text. What the user sees there is the last visible rectangle
	// painted over it, and it must be the card surface: a solid ring color
	// covering the card reads as a blue background.
	probeX, probeY := card.Bounds.X+card.Bounds.Width-24, card.Bounds.Y+8
	var visible gfx.Color
	for _, rectangle := range canvas.Rectangles() {
		bounds := rectangle.Bounds
		if rectangle.Color.Alpha > 0 && probeX >= bounds.X && probeX < bounds.X+bounds.Width && probeY >= bounds.Y && probeY < bounds.Y+bounds.Height {
			visible = rectangle.Color
		}
	}
	if visible == theme.Primary {
		t.Fatal("hovering a card floods its surface with the primary color")
	}
	if visible != theme.Card {
		t.Fatalf("the visible surface of a hovered card is %+v, want the card color %+v", visible, theme.Card)
	}
	ring := false
	for _, rectangle := range canvas.Rectangles() {
		ring = ring || rectangle.Bounds == card.Bounds && rectangle.Color == theme.Primary
	}
	if !ring {
		t.Fatal("hovering a card paints no ring")
	}
}

func TestComponentPageShowsEveryExampleLive(t *testing.T) {
	for index := range catalog {
		component := &catalog[index]
		state := New()
		click(state, cardScope.ID(gfx.Index(index)))
		if state.page != index+1 || len(state.live) != len(component.Examples) {
			t.Fatalf("%s: opened page %d with %d live examples, want %d", component.ID, state.page, len(state.live), len(component.Examples))
		}
		for _, width := range []float32{420, DefaultWidth, 2400} {
			canvas := paintTall(state, width)
			for _, example := range component.Examples {
				if !hasText(canvas, example.Title) {
					t.Errorf("%s at width %v: example %q has no heading", component.ID, width, example.Title)
				}
			}
			for _, hit := range canvas.Hits() {
				slot, local, ok := state.decodeExample(hit.ID)
				if !ok {
					continue
				}
				if slot >= len(state.live) || local.IsZero() {
					t.Fatalf("%s: hit %s decodes to slot %d, local %v", component.ID, tree.Path(hit.ID.Scope()), slot, local)
				}
				// A control can only be reached inside the stage it belongs to.
				view := state.live[slot]
				stage := gfx.Rect{X: view.origin.X, Y: view.origin.Y, Width: view.size.Width, Height: view.size.Height}
				if hit.Bounds.X < stage.X-0.01 || hit.Bounds.Y < stage.Y-0.01 ||
					hit.Bounds.X+hit.Bounds.Width > stage.X+stage.Width+0.01 || hit.Bounds.Y+hit.Bounds.Height > stage.Y+stage.Height+0.01 {
					t.Fatalf("%s: hit %s (%q) at %+v leaves its stage %+v", component.ID, tree.Path(hit.ID.Scope()), hit.Label, hit.Bounds, stage)
				}
			}
		}
	}
}

func TestExamplesOfOnePageNeverShareHitIDs(t *testing.T) {
	// Every demo numbers its controls from its own constants, so two examples
	// of one component use the very same local IDs. Their windows must not meet.
	state := New()
	state.openPage(1)
	canvas := paintTall(state, 2400)
	owner := map[gfx.ID]int{}
	for _, hit := range canvas.Hits() {
		slot, _, ok := state.decodeExample(hit.ID)
		if !ok {
			continue
		}
		if previous, seen := owner[hit.ID]; seen && previous != slot {
			t.Fatalf("hit %s is claimed by examples %d and %d", tree.Path(hit.ID.Scope()), previous, slot)
		}
		owner[hit.ID] = slot
	}
	if len(owner) == 0 {
		t.Fatal("the button page declared no example controls")
	}
	locals := map[gfx.ID][]int{}
	for id, slot := range owner {
		_, local, _ := state.decodeExample(id)
		locals[local] = append(locals[local], slot)
	}
	shared := false
	for _, slots := range locals {
		shared = shared || len(slots) > 1
	}
	if !shared {
		t.Fatal("expected examples to reuse local IDs; the isolation is not being exercised")
	}
}

func TestExamplesNeverPaintAboveTheToolbar(t *testing.T) {
	for index := range catalog {
		state := New()
		state.openPage(index + 1)
		for _, density := range []gfx.Density{gfx.DensityRegular, gfx.DensityLarge, gfx.DensityCompact} {
			for _, light := range []bool{true, false} {
				state.density, state.lightMode = density, light
				canvas := paintTall(state, DefaultWidth)
				for _, rectangle := range canvas.Rectangles() {
					if rectangle.Layer >= toolbarLayer && rectangle.Bounds.Y >= toolbarHeight {
						t.Fatalf("%s: a rectangle on layer %d at %+v would cover the toolbar", catalog[index].ID, rectangle.Layer, rectangle.Bounds)
					}
				}
				for _, text := range canvas.Texts() {
					if text.Layer >= toolbarLayer && text.Bounds.Y >= toolbarHeight {
						t.Fatalf("%s: text %q on layer %d would cover the toolbar", catalog[index].ID, text.Value, text.Layer)
					}
				}
			}
		}
	}
}

func TestToolbarCoversContentScrolledUnderIt(t *testing.T) {
	size := gfx.Size{Width: DefaultWidth, Height: DefaultHeight}
	state := New()
	var canvas gfx.Canvas
	state.Paint(&canvas, size)

	reachedUnderToolbar := false
	for offset := float32(0); offset <= state.scrollMax; offset += 7 {
		state.viewScroll = offset
		canvas.Reset()
		state.Paint(&canvas, size)

		var surface gfx.Rectangle
		for _, rectangle := range canvas.Rectangles() {
			if rectangle.Bounds == (gfx.Rect{Width: size.Width, Height: toolbarHeight}) {
				surface = rectangle
			}
		}
		if surface.Color.Alpha != 1 {
			t.Fatalf("offset %v: the toolbar surface is not opaque: %+v", offset, surface)
		}
		// A label that is only partly inside the scroll area is drawn in full,
		// so everything that can reach up under the toolbar has to be on a lower
		// layer than its surface.
		if surface.Layer != toolbarLayer {
			t.Fatalf("offset %v: toolbar layer %d, want %d", offset, surface.Layer, toolbarLayer)
		}
		for _, text := range canvas.Texts() {
			if text.Layer < surface.Layer && text.Bounds.Y < toolbarHeight-1 {
				reachedUnderToolbar = true
			}
		}
	}
	if !reachedUnderToolbar {
		t.Fatal("no content line ever reached under the toolbar; the scan proves nothing")
	}
}

func TestGalleryHasNoSidebar(t *testing.T) {
	for _, page := range []int{0, 1, 33, 63} {
		state := New()
		state.openPage(page)
		canvas := paintTall(state, DefaultWidth)
		for _, rectangle := range canvas.Rectangles() {
			if rectangle.Bounds.X == 0 && rectangle.Bounds.Width < DefaultWidth/2 && rectangle.Bounds.Height >= DefaultHeight {
				t.Fatalf("page %d paints a full-height left panel: %+v", page, rectangle.Bounds)
			}
		}
	}
}

func TestEveryComponentIsReachableFromTheToolbarAlone(t *testing.T) {
	state := New()
	visited := make([]bool, len(catalog)+1)
	for step := 0; step <= len(catalog); step++ {
		visited[state.page] = true
		click(state, hitPageNext)
	}
	for page, ok := range visited {
		if !ok {
			t.Errorf("Next never reached page %d", page)
		}
	}
	if state.page != 0 {
		t.Fatalf("a full lap of Next ended on page %d, want the catalog", state.page)
	}
	click(state, hitPagePrevious)
	if state.page != len(catalog) {
		t.Fatalf("Previous from the catalog went to page %d, want the last component %d", state.page, len(catalog))
	}
}

func TestToolbarComponentsButtonReturnsToTheCatalog(t *testing.T) {
	state := New()
	if canvas := paintTall(state, DefaultWidth); hasHit(canvas, hitPageCatalog) {
		t.Fatal("the catalog needs no way back to itself")
	}
	state.openPage(3)
	if canvas := paintTall(state, DefaultWidth); !hasHit(canvas, hitPageCatalog) {
		t.Fatal("a component page has no way back to the catalog")
	}
	click(state, hitPageCatalog)
	if state.page != 0 || len(state.live) != 0 {
		t.Fatalf("back left the gallery on page %d with %d live examples", state.page, len(state.live))
	}
}

func TestNavigationStartsPagesAtTheTopWithFreshExamples(t *testing.T) {
	state := New()
	var canvas gfx.Canvas
	state.Paint(&canvas, gfx.Size{Width: DefaultWidth, Height: DefaultHeight})
	state.Scroll(gfx.ScrollEvent{DeltaY: 700})
	click(state, cardScope.ID(gfx.Index(0)))
	if state.page != 1 || state.viewScroll != 0 {
		t.Fatalf("opening a component left it at page %d, offset %v", state.page, state.viewScroll)
	}

	state.viewScroll = 90
	first := state.live[0].demo
	state.openPage(1) // the same component again
	if state.viewScroll != 90 || state.live[0].demo != first {
		t.Fatal("re-opening the current page must keep its scroll position and example state")
	}
	click(state, hitPageNext)
	if state.viewScroll != 0 {
		t.Fatal("the toolbar next button must start the new page at the top")
	}
}

func TestSettingsReachEveryExample(t *testing.T) {
	state := New()
	state.openPage(1)
	canvas := paintTall(state, DefaultWidth)
	regular, ok := findLabelHit(canvas, "Click me")
	if !ok {
		t.Fatal("the button example is missing its control")
	}
	if want := state.Theme().Metrics.ControlHeight; regular.Bounds.Height != want {
		t.Fatalf("regular button is %v high, want %v", regular.Bounds.Height, want)
	}
	click(state, hitDensityCycle)
	canvas = paintTall(state, DefaultWidth)
	changed, _ := findLabelHit(canvas, "Click me")
	if want := state.Theme().Metrics.ControlHeight; changed.Bounds.Height != want || changed.Bounds.Height == regular.Bounds.Height {
		t.Fatalf("the density did not reach the example: %v then %v, want %v", regular.Bounds.Height, changed.Bounds.Height, want)
	}

	// The example paints its own background first; it must follow the toggle.
	stageBackground := func() gfx.Color { return state.live[0].canvas.Rectangles()[0].Color }
	light := stageBackground()
	click(state, hitThemeToggle)
	paintTall(state, DefaultWidth)
	if dark := stageBackground(); dark == light {
		t.Fatalf("the appearance toggle did not reach the example: still %+v", light)
	}
}

// findLabelHit returns the first hit with the accessibility label: the button
// example's primary control is the one called "Click me".
func findLabelHit(canvas *gfx.Canvas, label string) (gfx.Hit, bool) {
	for _, hit := range canvas.Hits() {
		if hit.Label == label {
			return hit, true
		}
	}
	return gfx.Hit{}, false
}

func TestWheelScrollClampsAtBothEnds(t *testing.T) {
	state := New()
	var canvas gfx.Canvas
	state.Paint(&canvas, gfx.Size{Width: DefaultWidth, Height: DefaultHeight})
	if state.scrollMax <= 0 {
		t.Fatal("the catalog should be taller than the window")
	}

	// Trackpad momentum keeps arriving after the end is reached. It must not
	// bank an offset the next gesture in the other direction has to unwind.
	for range 80 {
		state.Scroll(gfx.ScrollEvent{DeltaY: 400, Precise: true})
	}
	if state.viewScroll != state.scrollMax {
		t.Fatalf("offset %v after momentum past the end, want the end %v", state.viewScroll, state.scrollMax)
	}
	result := state.Scroll(gfx.ScrollEvent{DeltaY: -30, Precise: true})
	if !result.Handled || !result.Repaint || state.viewScroll != state.scrollMax-30 {
		t.Fatalf("scrolling back moved to %v (%+v), want %v at once", state.viewScroll, result, state.scrollMax-30)
	}

	for range 80 {
		state.Scroll(gfx.ScrollEvent{DeltaY: -400, Precise: true})
	}
	if state.viewScroll != 0 {
		t.Fatalf("offset %v after momentum past the start, want 0", state.viewScroll)
	}
	result = state.Scroll(gfx.ScrollEvent{DeltaY: -10})
	if !result.Handled || result.Repaint {
		t.Fatalf("a gesture that cannot move the page must be handled without a repaint: %+v", result)
	}
}

func TestWheelDeltaSignRevealsContentBelow(t *testing.T) {
	state := New()
	size := gfx.Size{Width: DefaultWidth, Height: DefaultHeight}
	var canvas gfx.Canvas
	state.Paint(&canvas, size)
	state.Scroll(gfx.ScrollEvent{DeltaY: 120})
	if state.viewScroll != 120 {
		t.Fatalf("a positive delta moved the offset to %v, want 120", state.viewScroll)
	}
	canvas.Reset()
	state.Paint(&canvas, size)
	for _, text := range canvas.Texts() {
		if text.Value == "Components" && text.Style.Size > 20 {
			if text.Bounds.Y >= toolbarHeight {
				t.Fatalf("the page title is still at y=%v after scrolling down", text.Bounds.Y)
			}
			return
		}
	}
	t.Fatal("page title was not painted")
}

func TestPageScrollbarFollowsOverflowAndDrags(t *testing.T) {
	size := gfx.Size{Width: DefaultWidth, Height: DefaultHeight}
	state := New()
	var canvas gfx.Canvas
	state.Paint(&canvas, size)
	track, ok := findHit(&canvas, hitPageScrollbar)
	if !ok || track.Bounds.X+track.Bounds.Width != size.Width {
		t.Fatalf("scrollbar track %+v is not on the right window edge", track.Bounds)
	}

	thumb := state.scrollGeometry(size).VerticalThumb
	grab := gfx.PointerEvent{
		Kind: gfx.PointerDown, Button: gfx.PointerButtonPrimary, X: thumb.X + 1, Y: thumb.Y + thumb.Height/2,
		Viewport: size, Target: hitPageScrollbar,
	}
	if result := state.Pointer(grab); !result.Handled || !state.thumbDragging {
		t.Fatalf("pressing the thumb did not start a drag: %+v", result)
	}
	travel := track.Bounds.Height - thumb.Height
	drag := grab
	drag.Kind, drag.Captured = gfx.PointerMove, true
	drag.Y += travel / 2
	if result := state.Pointer(drag); !result.Handled || !result.Repaint {
		t.Fatalf("dragging did not repaint: %+v", result)
	}
	if want := state.scrollMax / 2; state.viewScroll < want-want*0.02 || state.viewScroll > want+want*0.02 {
		t.Fatalf("dragging half the track left the page at %v, want about %v", state.viewScroll, want)
	}
	drag.Y += 10000
	state.Pointer(drag)
	if state.viewScroll != state.scrollMax {
		t.Fatalf("dragging past the track end left the page at %v, want %v", state.viewScroll, state.scrollMax)
	}
	up := drag
	up.Kind = gfx.PointerUp
	if result := state.Pointer(up); !result.Handled || state.thumbDragging {
		t.Fatalf("releasing did not end the drag: %+v", result)
	}
	before := state.viewScroll
	drag.Y = 0
	if result := state.Pointer(drag); result.Handled || state.viewScroll != before {
		t.Fatal("pointer moves after a drag ended must not scroll the page")
	}
}

func TestPageScrollbarTrackPressCentersThumb(t *testing.T) {
	size := gfx.Size{Width: DefaultWidth, Height: DefaultHeight}
	state := New()
	var canvas gfx.Canvas
	state.Paint(&canvas, size)
	geometry := state.scrollGeometry(size)
	state.Pointer(gfx.PointerEvent{
		Kind: gfx.PointerDown, Button: gfx.PointerButtonPrimary, X: geometry.VerticalTrack.X + 4,
		Y: geometry.VerticalTrack.Y + geometry.VerticalTrack.Height - 4, Viewport: size, Target: hitPageScrollbar,
	})
	if state.viewScroll < state.scrollMax*0.95 {
		t.Fatalf("pressing the bottom of the track left the page at %v of %v", state.viewScroll, state.scrollMax)
	}
}

func TestShortPageHasNoScrollbar(t *testing.T) {
	state := New()
	for index := range catalog {
		if catalog[index].ID == "separator" {
			state.openPage(index + 1)
		}
	}
	var canvas gfx.Canvas
	state.Paint(&canvas, gfx.Size{Width: DefaultWidth, Height: DefaultHeight})
	if hasHit(&canvas, hitPageScrollbar) || state.scrollMax != 0 {
		t.Fatalf("a page that fits must not scroll (max %v)", state.scrollMax)
	}
	state.Scroll(gfx.ScrollEvent{DeltaY: 500})
	if state.viewScroll != 0 {
		t.Fatalf("a page that fits scrolled to %v", state.viewScroll)
	}
}

func TestReloadRoundTripKeepsWhereYouWere(t *testing.T) {
	state := New()
	click(state, cardScope.ID(gfx.Index(32))) // input
	click(state, hitDensityCycle)
	click(state, hitThemeToggle)
	state.viewScroll = 77.5
	data := state.SnapshotReloadState()

	var restored State
	if err := restored.RestoreReloadState(data); err != nil {
		t.Fatalf("restore: %v", err)
	}
	if restored.page != state.page || restored.density != state.density || restored.lightMode != state.lightMode || restored.viewScroll != 77.5 {
		t.Fatalf("restored page %d density %v light %v scroll %v, want %d %v %v 77.5",
			restored.page, restored.density, restored.lightMode, restored.viewScroll, state.page, state.density, state.lightMode)
	}
	if len(restored.live) != len(catalog[state.page-1].Examples) {
		t.Fatalf("the restored page has %d live examples", len(restored.live))
	}
}

func TestReloadStateSurvivesACatalogThatChanged(t *testing.T) {
	data := appendString(nil, "a-component-that-was-removed")
	data = append(data, 1, 0)
	data = appendUint32(data, 0)
	var state State
	if err := state.RestoreReloadState(data); err != nil || state.page != 0 {
		t.Fatalf("an unknown component should restore to the catalog: page %d, err %v", state.page, err)
	}
}

func TestReloadStateRejectsDamagedData(t *testing.T) {
	good := New().SnapshotReloadState()
	for name, data := range map[string][]byte{
		"empty":     nil,
		"truncated": good[:len(good)-1],
		"trailing":  append(append([]byte(nil), good...), 0),
	} {
		var state State
		if err := state.RestoreReloadState(data); err == nil {
			t.Errorf("%s data was accepted", name)
		}
	}
	bad := appendString(nil, "")
	bad = append(bad, 0xFE, 0)
	bad = appendUint32(bad, 0)
	var state State
	if err := state.RestoreReloadState(bad); err == nil {
		t.Error("unknown flag bits were accepted")
	}
}

func TestPaintAllocatesNothingOnceWarm(t *testing.T) {
	size := gfx.Size{Width: DefaultWidth, Height: DefaultHeight}
	state := New()
	var canvas gfx.Canvas
	for page := 0; page <= len(catalog); page++ {
		state.openPage(page)
		canvas.Reset()
		state.Paint(&canvas, size)
		allocations := testing.AllocsPerRun(10, func() {
			canvas.Reset()
			state.Paint(&canvas, size)
		})
		if allocations != 0 {
			name := "catalog"
			if page > 0 {
				name = catalog[page-1].ID
			}
			t.Errorf("page %q allocated %v times per paint", name, allocations)
		}
	}
}

func BenchmarkGalleryPaint(b *testing.B) {
	for _, page := range []struct {
		name  string
		index int
	}{{"catalog", 0}, {"button", 1}, {"dialog", 63}, {"table", 8}} {
		b.Run(page.name, func(b *testing.B) {
			state := New()
			state.openPage(page.index)
			size := gfx.Size{Width: DefaultWidth, Height: DefaultHeight}
			var canvas gfx.Canvas
			state.Paint(&canvas, size)
			b.ReportAllocs()
			b.ResetTimer()
			for iteration := 0; iteration < b.N; iteration++ {
				canvas.Reset()
				state.Paint(&canvas, size)
			}
		})
	}
}

func TestNoPageRequestsFramesAtRest(t *testing.T) {
	size := gfx.Size{Width: DefaultWidth, Height: DefaultHeight}
	for page := 0; page <= len(catalog); page++ {
		state := New()
		state.openPage(page)
		var canvas gfx.Canvas
		now := time.Duration(0)
		canvas.SetClock(func() time.Duration { return now })
		frame := func() bool {
			canvas.Reset()
			canvas.BeginFrame()
			state.Paint(&canvas, size)
			canvas.EndFrame()
			return canvas.Animating()
		}
		frame()
		animating := false
		for step := 0; step < 8; step++ {
			now += 500 * time.Millisecond
			animating = frame()
		}
		if animating {
			name := "catalog"
			if page > 0 {
				name = catalog[page-1].ID
			}
			t.Errorf("page %q still asks for frames after four idle seconds", name)
		}
	}
}

func TestAnExampleTransitionKeepsThePageAnimatingUntilItSettles(t *testing.T) {
	size := gfx.Size{Width: DefaultWidth, Height: DefaultHeight}
	state := New()
	for index := range catalog {
		if catalog[index].ID == "switch" {
			state.openPage(index + 1)
		}
	}
	var canvas gfx.Canvas
	now := time.Duration(0)
	canvas.SetClock(func() time.Duration { return now })
	frame := func() bool {
		canvas.Reset()
		canvas.BeginFrame()
		state.Paint(&canvas, size)
		canvas.EndFrame()
		return canvas.Animating()
	}
	frame()
	var toggle gfx.ID
	for _, hit := range canvas.Hits() {
		if hit.Role == gfx.RoleSwitch {
			toggle = hit.ID
			break
		}
	}
	if toggle.IsZero() {
		t.Fatal("the switch page has no switch")
	}
	canvas.BeginEventPass(toggle, gfx.ClickInfo{}) // the click, read by the demo while it paints
	frame()
	canvas.EndEventPass()
	now += 16 * time.Millisecond
	if !frame() {
		t.Fatal("a switch changed state but the page did not ask for the frames of its transition")
	}
	now += time.Second
	frame()
	if frame() {
		t.Fatal("the page kept asking for frames after the transition ended")
	}
}

// cardOf returns the catalog index a card's identity opens.
func cardOf(id gfx.ID) (int, bool) {
	card, ok := id.Name().Index()
	return card, ok && id.Scope() == cardScope && card < len(catalog)
}

// The identity tree only grows, so painting must name its places once. Visiting
// every page and painting again, in either theme, must not add a scope.
func TestPaintingAgainDoesNotGrowTheIdentityTree(t *testing.T) {
	state := New()
	visit := func() {
		for page := 0; page <= len(catalog); page++ {
			state.openPage(page)
			paintTall(state, DefaultWidth)
		}
	}
	visit()
	grown := tree.Len()
	state.lightMode = !state.lightMode
	visit()
	visit()
	if tree.Len() != grown {
		t.Fatalf("the identity tree grew from %d to %d scopes while repainting pages it had already painted", grown, tree.Len())
	}
}
