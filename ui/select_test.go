package ui

import (
	"fmt"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
)

var (
	selScope  = tree.Sub(page, tree.Name("select"))
	selLabels = []string{"Small", "Medium", "Large"}
)

// selIcons are the icons of the three labels, a stroke at each one's own height:
// an icon painted on another option's row shows in the triangles.
var selIcons = []gfx.Icon{
	{ViewBox: 24, Stroke: 2, Cap: gfx.CapRound, Join: gfx.JoinRound,
		Commands: []gfx.PathCommand{gfx.MoveTo(4, 6), gfx.LineTo(20, 6)}},
	{ViewBox: 24, Stroke: 2, Cap: gfx.CapRound, Join: gfx.JoinRound,
		Commands: []gfx.PathCommand{gfx.MoveTo(4, 10), gfx.LineTo(20, 10)}},
	{ViewBox: 24, Stroke: 2, Cap: gfx.CapRound, Join: gfx.JoinRound,
		Commands: []gfx.PathCommand{gfx.MoveTo(4, 14), gfx.LineTo(20, 14)}},
}

// A Select looks exactly like the components.Select it is painted by, closed and
// open, with a value and with a placeholder.
func TestSelectIsPaintedLikeTheComponent(t *testing.T) {
	theme := gfx.LightTheme()
	for _, test := range []struct {
		open     bool
		selected int
		hovered  int
	}{{false, 1, -1}, {false, -1, 3}, {true, 1, 2}, {true, -1, -1}} {
		name := fmt.Sprintf("open=%v selected=%d", test.open, test.selected)
		flexed, direct := testCanvas(), testCanvas()
		if test.hovered >= 0 {
			flexed.SetInteraction(selScope.At(test.hovered), gfx.ID{}, gfx.ID{}, false)
			direct.SetInteraction(selScope.At(test.hovered), gfx.ID{}, gfx.ID{}, false)
		}
		state := SelectState{Open: test.open}
		viewport := gfx.Rect{Width: 400, Height: 300}
		var view View
		view.Draw(flexed, theme, viewport, func(f *Frame) {
			f.Column().Pad(10).Children(func() {
				f.Select(selScope, &state, selLabels, test.selected).Placeholder("Size").Width(200)
			})
		})
		options := components.SelectOptions{
			TriggerID: selScope.At(3), Label: "Size", Placeholder: "Size", Open: test.open,
			Layer: 1, Scope: selScope.At(5), DismissID: selScope.At(4), Viewport: viewport,
		}
		if test.selected >= 0 {
			options.Value = selLabels[test.selected]
		}
		if test.open {
			options.Content = gfx.Rect{X: 10, Y: 10 + 32 + 4, Width: 200, Height: 8 + 32*3}
		}
		control := components.BeginSelect(direct, theme, gfx.Rect{X: 10, Y: 10, Width: 200}, options)
		if test.open {
			for i, label := range selLabels {
				control.AddItem(components.SelectItem{ID: selScope.At(i), Label: label, Selected: i == test.selected})
			}
		}
		sameCanvas(t, name, flexed, direct)
	}
}

// A Select with icons is painted exactly like the components.Select an application
// would call with the same icons: the chosen option's before the value on the
// trigger, each option's own before its label on its row — a select that was given
// no icon function paints what TestSelectIsPaintedLikeTheComponent pins above.
func TestSelectWithIconsIsPaintedLikeTheComponent(t *testing.T) {
	theme := gfx.LightTheme()
	for _, test := range []struct {
		open     bool
		selected int
	}{{false, 1}, {false, -1}, {true, 1}, {true, -1}, {true, 2}} {
		name := fmt.Sprintf("open=%v selected=%d", test.open, test.selected)
		flexed, direct := testCanvas(), testCanvas()
		state := SelectState{Open: test.open}
		viewport := gfx.Rect{Width: 400, Height: 300}
		var view View
		view.Draw(flexed, theme, viewport, func(f *Frame) {
			f.Column().Pad(10).Children(func() {
				f.Select(selScope, &state, selLabels, test.selected).Placeholder("Size").Width(200).Icons(selIcons)
			})
		})
		options := components.SelectOptions{
			TriggerID: selScope.At(3), Label: "Size", Placeholder: "Size", Open: test.open,
			Layer: 1, Scope: selScope.At(5), DismissID: selScope.At(4), Viewport: viewport,
		}
		if test.selected >= 0 {
			options.Value, options.Icon = selLabels[test.selected], selIcons[test.selected]
		}
		if test.open {
			options.Content = gfx.Rect{X: 10, Y: 10 + 32 + 4, Width: 200, Height: 8 + 32*3}
		}
		control := components.BeginSelect(direct, theme, gfx.Rect{X: 10, Y: 10, Width: 200}, options)
		if test.open {
			for i, label := range selLabels {
				control.AddItem(components.SelectItem{ID: selScope.At(i), Label: label, Selected: i == test.selected, Icon: selIcons[i]})
			}
		}
		sameCanvas(t, name, flexed, direct)
	}
}

// Two options that share a label keep their own icon: the rows of a list are not
// interchangeable behind one label.
func TestSelectIconsFollowTheirOwnRows(t *testing.T) {
	theme := gfx.LightTheme()
	labels := []string{"Same", "Same"}
	state := SelectState{Open: true}
	flexed, direct := testCanvas(), testCanvas()
	viewport := gfx.Rect{Width: 300, Height: 200}
	var view View
	view.Draw(flexed, theme, viewport, func(f *Frame) {
		f.Column().Pad(10).Children(func() {
			f.Select(selScope, &state, labels, 0).Placeholder("Pick").Width(200).Icons(selIcons)
		})
	})
	options := components.SelectOptions{
		TriggerID: selScope.At(2), Label: "Pick", Placeholder: "Pick", Open: true,
		Layer: 1, Scope: selScope.At(4), DismissID: selScope.At(3), Viewport: viewport,
		Content: gfx.Rect{X: 10, Y: 10 + 32 + 4, Width: 200, Height: 8 + 32*2},
		Value:   "Same", Icon: selIcons[0],
	}
	control := components.BeginSelect(direct, theme, gfx.Rect{X: 10, Y: 10, Width: 200}, options)
	for i, label := range labels {
		control.AddItem(components.SelectItem{ID: selScope.At(i), Label: label, Selected: i == 0, Icon: selIcons[i]})
	}
	sameCanvas(t, "same-label options", flexed, direct)
}

func TestSelectOpensChoosesAndClosesThroughItsClicks(t *testing.T) {
	theme := gfx.LightTheme()
	var state SelectState
	selected := 0
	draw := func(canvas *gfx.Canvas) {
		var view View
		view.Draw(canvas, theme, gfx.Rect{Width: 300, Height: 300}, func(f *Frame) {
			f.Column().Pad(10).Children(func() {
				if i, ok := f.Select(selScope, &state, selLabels, selected).Picked(); ok {
					selected = i
				}
			})
		})
	}
	click := func(index int) {
		canvas := testCanvas()
		canvas.BeginEventPass(selScope.At(index), gfx.ClickInfo{})
		draw(canvas)
		canvas.EndEventPass()
	}
	click(3) // the trigger
	if !state.Open {
		t.Fatal("a click on the trigger did not open the list")
	}
	canvas := testCanvas()
	draw(canvas)
	var items, dismiss int
	for _, hit := range canvas.Hits() {
		if i, ok := hit.ID.In(selScope); ok {
			if i < 3 {
				items++
			}
			if i == 4 {
				dismiss++
			}
		}
	}
	if items != 3 || dismiss != 1 {
		t.Errorf("an open list declares %d options and %d dismiss targets, want 3 and 1", items, dismiss)
	}
	click(2)
	if state.Open || selected != 2 {
		t.Errorf("choosing the third option left open=%v selected=%d", state.Open, selected)
	}
	click(3)
	click(4) // outside, or Escape
	if state.Open || selected != 2 {
		t.Errorf("dismissing left open=%v selected=%d", state.Open, selected)
	}
}

// A list too long for the frame shows a window of options, with arrows that move it.
func TestSelectTooLongForTheFrameScrollsItsWindow(t *testing.T) {
	theme := gfx.LightTheme()
	labels := make([]string, 40)
	for i := range labels {
		labels[i] = fmt.Sprintf("option %d", i)
	}
	state := SelectState{Open: true}
	draw := func(canvas *gfx.Canvas) (first, last, arrows int) {
		first, last = -1, -1
		var view View
		view.Draw(canvas, theme, gfx.Rect{Width: 300, Height: 300}, func(f *Frame) {
			f.Column().Pad(10).Children(func() {
				f.Select(selScope, &state, labels, 20).Picked()
			})
		})
		for _, hit := range canvas.Hits() {
			if i, ok := hit.ID.In(selScope); ok {
				switch {
				case i < 40:
					if first < 0 {
						first = i
					}
					last = i
				case i == 43 || i == 44:
					arrows++
				}
			}
		}
		return
	}
	first, last, arrows := draw(testCanvas())
	if arrows != 2 || last-first+1 > 8 || last-first+1 < 4 {
		t.Fatalf("a window of options %d..%d with %d arrows", first, last, arrows)
	}
	canvas := testCanvas()
	canvas.BeginEventPass(selScope.At(44), gfx.ClickInfo{}) // scroll down
	draw(canvas)
	canvas.EndEventPass()
	if state.First <= first {
		t.Errorf("the down arrow left the window at %d", state.First)
	}
}

func TestSelectAllocatesNothingOnceWarm(t *testing.T) {
	var view View
	canvas := testCanvas()
	theme := gfx.LightTheme()
	state := SelectState{Open: true}
	closed := SelectState{}
	run := func() {
		canvas.Reset()
		view.Draw(canvas, theme, gfx.Rect{Width: 400, Height: 300}, func(f *Frame) {
			f.Column().Pad(10).Gap(8).Children(func() {
				f.Select(selScope, &state, selLabels, 1)
				f.Select(selScope, &state, selLabels, 1).Icons(selIcons)
				f.Select(selScope, &closed, selLabels, -1).Placeholder("Pick").Small()
			})
		})
	}
	run()
	if allocs := testing.AllocsPerRun(200, run); allocs != 0 {
		t.Fatalf("warm selects allocate %v times a frame, want 0", allocs)
	}
}

// The widest option is measured once and then kept, but a frame that declares
// different options, or the same options at another size, measures again: the
// select's width follows its content, not its cache.
func TestSelectWidestOptionFollowsItsLabels(t *testing.T) {
	canvas := testCanvas()
	state := SelectState{}
	narrow := state.widestOption(canvas, []string{"short"}, 10)
	if again := state.widestOption(canvas, []string{"short"}, 10); again != narrow {
		t.Fatalf("the same labels measured %v and then %v", narrow, again)
	}
	wide := state.widestOption(canvas, []string{"a much longer option"}, 10)
	if wide <= narrow {
		t.Fatalf("longer labels measured %v, not wider than %v", wide, narrow)
	}
	if grown := state.widestOption(canvas, []string{"a much longer option"}, 20); grown <= wide {
		t.Fatalf("the same labels at a larger size measured %v, not wider than %v", grown, wide)
	}
}

func BenchmarkOpenSelectAndTabs(b *testing.B) {
	var view View
	canvas := testCanvas()
	theme := gfx.LightTheme()
	state := SelectState{Open: true}
	b.ReportAllocs()
	for b.Loop() {
		canvas.Reset()
		view.Draw(canvas, theme, gfx.Rect{Width: 400, Height: 300}, func(f *Frame) {
			f.Column().Pad(10).Gap(8).Children(func() {
				f.Tabs(tabsScope, tabLabels, 1).Line()
				f.Select(selScope, &state, selLabels, 1)
			})
		})
	}
}

// A closed select with many options still measures its widest option once, not
// per frame; with the test measurer the per-label cost is trivial, so this
// guards the steady-state shape (the win is the native engines' per-label
// text measurement this no longer runs).
func BenchmarkSelectClosedManyOptions(b *testing.B) {
	var view View
	canvas := testCanvas()
	theme := gfx.LightTheme()
	state := SelectState{}
	labels := make([]string, 1000)
	for index := range labels {
		labels[index] = "Option " + strconv.Itoa(index)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		canvas.Reset()
		view.Draw(canvas, theme, gfx.Rect{Width: 400, Height: 300}, func(f *Frame) {
			f.Column().Pad(10).Children(func() {
				f.Select(selScope, &state, labels, 0).Placeholder("Size")
			})
		})
	}
}

var fruit = []string{"Apple", "Avocado", "Banana", "Blueberry", "Cherry", "Date"}

func drawFruit(state *SelectState, labels []string, height float32) {
	var view View
	view.Draw(testCanvas(), gfx.LightTheme(), gfx.Rect{Width: 300, Height: height}, func(f *Frame) {
		f.Column().Pad(10).Children(func() { f.Select(selScope, state, labels, 0).Picked() })
	})
}

// Typing jumps to the option that starts with the letters: in an open list the match
// takes the focus, on the focused trigger of a closed select the list opens on it; a
// repeated letter cycles, a pause starts a new word, and other focus is left alone.
func TestSelectTypeAheadJumpsToTheMatchingOption(t *testing.T) {
	now := time.Unix(1000, 0)
	clock = func() time.Time { return now }
	defer func() { clock = time.Now }()
	var state SelectState
	drawFruit(&state, fruit, 400)
	trigger := selScope.At(len(fruit))
	typed := func(target gfx.ID, text string) gfx.EventResult {
		return state.Key(gfx.KeyEvent{Text: text, Target: target})
	}

	if got := typed(omit, "b"); got.Handled || state.Open {
		t.Fatal("typing with another control focused opened the select")
	}
	got := typed(trigger, "b")
	if !state.Open || got.Focus != selScope.At(2) {
		t.Fatalf("typing b on the closed select: open=%v focus=%v, want it open on Banana", state.Open, got.Focus)
	}
	now = now.Add(200 * time.Millisecond)
	if got := typed(selScope.At(2), "l"); got.Focus != selScope.At(3) {
		t.Errorf("b then l focuses %v, want Blueberry", got.Focus)
	}
	now = now.Add(2 * time.Second)
	if got := typed(selScope.At(3), "a"); got.Focus != selScope.At(0) {
		t.Errorf("after a pause, a focuses %v, want Apple", got.Focus)
	}
	now = now.Add(100 * time.Millisecond)
	if got := typed(selScope.At(0), "a"); got.Focus != selScope.At(1) {
		t.Errorf("a pressed twice focuses %v, want to cycle on to Avocado", got.Focus)
	}
	now = now.Add(2 * time.Second)
	if got := typed(selScope.At(1), "z"); got.Focus.IsZero() == false || !got.Handled {
		t.Errorf("a letter nothing starts with gave %+v, want it swallowed without moving focus", got)
	}
	if got := state.Key(gfx.KeyEvent{Text: "c", Modifiers: gfx.ModifierControl, Target: selScope.At(1)}); got.Handled {
		t.Error("a Control chord was taken as type-ahead")
	}
}

func TestSelectTypeAheadScrollsALongListToTheMatch(t *testing.T) {
	labels := make([]string, 60)
	for i := range labels {
		labels[i] = fmt.Sprintf("item %02d", i)
	}
	labels[50] = "zebra"
	state := SelectState{Open: true}
	drawFruit(&state, labels, 300)
	if state.visible >= len(labels) || state.visible < 2 {
		t.Fatalf("the window shows %d of %d options", state.visible, len(labels))
	}
	got := state.Key(gfx.KeyEvent{Text: "z", Target: selScope.At(0)})
	if got.Focus != selScope.At(50) || state.First > 50 || state.First+state.visible <= 50 {
		t.Errorf("z: focus %v, window starts at %d showing %d, want option 50 inside it", got.Focus, state.First, state.visible)
	}
}

// The wheel scrolls an open list a row at a time and stays in range; a closed select
// leaves the event to the page.
func TestSelectWheelScrollsTheOpenList(t *testing.T) {
	labels := make([]string, 40)
	for i := range labels {
		labels[i] = fmt.Sprintf("option %d", i)
	}
	state := SelectState{}
	drawFruit(&state, labels, 300)
	if got := state.Wheel(gfx.ScrollEvent{DeltaY: 10}); got.Handled {
		t.Error("a closed select took the wheel")
	}
	state.Open = true
	drawFruit(&state, labels, 300)
	start := state.First
	if got := state.Wheel(gfx.ScrollEvent{DeltaY: 10}); !got.Handled || !got.Repaint || state.First != start+1 {
		t.Errorf("a notch moved the window from %d to %d, want one row", start, state.First)
	}
	state.Wheel(gfx.ScrollEvent{DeltaY: 64, Precise: true})
	if state.First != start+3 {
		t.Errorf("64 pixels of trackpad moved the window to %d, want two more rows (%d)", state.First, start+3)
	}
	state.Wheel(gfx.ScrollEvent{DeltaY: 1e6})
	if want := len(labels) - state.visible; state.First != want {
		t.Errorf("scrolled far down the window starts at %d, want the last full one %d", state.First, want)
	}
	state.Wheel(gfx.ScrollEvent{DeltaY: -1e6})
	if state.First != 0 {
		t.Errorf("scrolled far up the window starts at %d, want 0", state.First)
	}
}

var baseSizes = tree.Sub(page, tree.Name("base-sizes"))

// A SelectBase is the select to the pointer, the keyboard and assistive technology and paints
// nothing of its own: the trigger and the open list are the caller's, drawn where the select
// says they are, with the rows' rectangles from Part.
func TestSelectBaseDeclaresTheSelectAndLeavesThePaintingToTheCaller(t *testing.T) {
	labels := []string{"Small", "Medium", "Large"}
	theme := gfx.LightTheme()
	run := func(base, open bool) (*gfx.Canvas, Box, gfx.Rect, [3]gfx.Rect) {
		var view View
		canvas := testCanvas()
		state := SelectState{Open: open}
		var box Box
		var panel gfx.Rect
		var rows [3]gfx.Rect
		view.Draw(canvas, theme, gfx.Rect{Width: 300, Height: 300}, func(f *Frame) {
			f.Column().Pad(8).Children(func() {
				if base {
					box = f.SelectBase(baseSizes, &state, labels, 1).Width(160)
				} else {
					box = f.Select(baseSizes, &state, labels, 1).Width(160)
				}
				if popup, ok := box.Popup(); ok {
					popup.Draw(func(d *DrawContext) {
						panel = d.Rect
						for i := range rows {
							rows[i] = box.Part(i)
						}
					})
				}
			})
		})
		return canvas, box, panel, rows
	}
	painted, _, _, _ := run(false, true)
	bare, box, panel, rows := run(true, true)
	if len(painted.Rectangles()) == 0 {
		t.Fatal("the painted select painted nothing: the comparison means nothing")
	}
	if len(bare.Rectangles()) != 0 || len(bare.Texts()) != 0 || len(bare.Shadows()) != 0 {
		t.Errorf("a SelectBase painted: %d rectangles, %d texts, %d shadows", len(bare.Rectangles()), len(bare.Texts()), len(bare.Shadows()))
	}
	if !reflect.DeepEqual(painted.Hits(), bare.Hits()) {
		t.Errorf("hits differ:\npainted %+v\nbare    %+v", painted.Hits(), bare.Hits())
	}
	if painted.TrappedFocusScope() != bare.TrappedFocusScope() || painted.EscapeTarget() != bare.EscapeTarget() {
		t.Errorf("the focus scope or the escape target differ")
	}
	if panel.Width <= 0 || panel.Height <= 0 {
		t.Fatalf("the open list has no panel: %+v", panel)
	}
	for i, row := range rows {
		if row.Width <= 0 || row.Height <= 0 {
			t.Errorf("option %d has no rectangle: %+v", i, row)
		}
		if row.X < panel.X || row.Y < panel.Y || row.X+row.Width > panel.X+panel.Width || row.Y+row.Height > panel.Y+panel.Height {
			t.Errorf("option %d at %+v is outside the panel %+v", i, row, panel)
		}
		if i > 0 && row.Y <= rows[i-1].Y {
			t.Errorf("option %d (%+v) is not below option %d (%+v)", i, row, i-1, rows[i-1])
		}
	}
	// The row's rectangle is where its hit is: the caller paints where the pointer lands.
	for i, row := range rows {
		found := false
		for _, hit := range bare.Hits() {
			if hit.ID == baseSizes.At(i) && hit.Bounds.Y == row.Y && hit.Bounds.Height == row.Height {
				found = true
			}
		}
		if !found {
			t.Errorf("no hit for option %d over its rectangle %+v", i, row)
		}
	}
	if _, ok := box.Popup(); !ok {
		t.Error("an open select has no popup")
	}
	closed, closedBox, _, closedRows := run(true, false)
	if _, ok := closedBox.Popup(); ok || closedRows[0].Width != 0 || len(closed.Rectangles()) != 0 {
		t.Errorf("a closed select reports a list: rows %+v", closedRows)
	}
}

// A trigger narrower than the list's minimum still gets a list whose element is the panel the
// rows are laid in: what the caller draws on it is not narrower than its options.
func TestSelectBasePanelIsAsWideAsItsRowsForANarrowTrigger(t *testing.T) {
	var view View
	canvas := testCanvas()
	state := SelectState{Open: true}
	var panel gfx.Rect
	var row gfx.Rect
	view.Draw(canvas, gfx.LightTheme(), gfx.Rect{Width: 300, Height: 300}, func(f *Frame) {
		f.Column().Pad(8).Children(func() {
			box := f.SelectBase(baseSizes, &state, []string{"A", "B"}, 0).Width(60)
			if popup, ok := box.Popup(); ok {
				popup.Draw(func(d *DrawContext) { panel, row = d.Rect, box.Part(0) })
			}
		})
	})
	if panel.Width <= 60 {
		t.Fatalf("the panel is %v wide, no wider than its 60 px trigger", panel.Width)
	}
	if row.X < panel.X || row.X+row.Width > panel.X+panel.Width {
		t.Errorf("row %+v is outside the panel %+v", row, panel)
	}
}
