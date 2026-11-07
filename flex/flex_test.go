package flex

import (
	"testing"

	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/ui"
)

var (
	tree   = ui.Tree{}
	page   = tree.Feature("flex")
	save   = tree.ID(page, "save")
	omit   = tree.ID(page, "cancel")
	rowID  = tree.ID(page, "row")
	listID = tree.ID(page, "list")
)

func testCanvas() *ui.Canvas {
	canvas := &ui.Canvas{}
	canvas.SetTree(&tree)
	canvas.SetMeasurer(func(value string, size, weight float32) float32 { return float32(len(value)) * size / 2 })
	return canvas
}

func near(t *testing.T, name string, got, want ui.Rect) {
	t.Helper()
	d := func(a, b float32) bool { return a-b > 0.01 || b-a > 0.01 }
	if d(got.X, want.X) || d(got.Y, want.Y) || d(got.Width, want.Width) || d(got.Height, want.Height) {
		t.Errorf("%s = %+v, want %+v", name, got, want)
	}
}

func TestColumnPadsGapsAndStretchesText(t *testing.T) {
	var view View
	var a, b, c Box
	f := view.Begin(testCanvas(), ui.LightTheme(), ui.Rect{Width: 200, Height: 100})
	f.Column().Pad(10).Gap(4).Children(func() {
		a = f.Text("abcd").FontSize(10)
		b = f.Text("xy").FontSize(10)
		c = f.Slot().Height(20)
	})
	f.End()
	near(t, "first", a.Rect(), ui.Rect{X: 10, Y: 10, Width: 180, Height: 17})
	near(t, "second", b.Rect(), ui.Rect{X: 10, Y: 31, Width: 180, Height: 17})
	near(t, "slot", c.Rect(), ui.Rect{X: 10, Y: 52, Width: 180, Height: 20})
}

func TestRowSpacerPushesTheLastChildToTheEdge(t *testing.T) {
	var view View
	var left, right Box
	f := view.Begin(testCanvas(), ui.LightTheme(), ui.Rect{Width: 300, Height: 60})
	f.Row().Height(60).Pad(8).Gap(6).Items(Center).Children(func() {
		left = f.Button(save, "Save")
		f.Spacer()
		right = f.Button(omit, "Cancel").Outline()
	})
	f.End()
	if got := right.Rect().X + right.Rect().Width; got < 291.9 || got > 292.1 {
		t.Errorf("the last button ends at %v, want 292 (the edge less padding)", got)
	}
	if left.Rect().X != 8 {
		t.Errorf("the first button starts at %v, want 8", left.Rect().X)
	}
	if mid := left.Rect().Y + left.Rect().Height/2; mid < 29.9 || mid > 30.1 {
		t.Errorf("a centered button's middle is at %v, want 30", mid)
	}
}

func TestGrowSharesFreeRoomInProportion(t *testing.T) {
	var view View
	var a, b, fixed Box
	f := view.Begin(testCanvas(), ui.LightTheme(), ui.Rect{Width: 310, Height: 40})
	f.Row().Height(40).Gap(5).Children(func() {
		fixed = f.Slot().Width(100)
		a = f.Slot().Grow(1)
		b = f.Slot().Grow(2)
	})
	f.End()
	near(t, "fixed", fixed.Rect(), ui.Rect{X: 0, Y: 0, Width: 100, Height: 40})
	near(t, "a", a.Rect(), ui.Rect{X: 105, Y: 0, Width: 66.667, Height: 40})
	near(t, "b", b.Rect(), ui.Rect{X: 176.667, Y: 0, Width: 133.333, Height: 40})
}

func TestCenterAndJustifyPlaceWhatIsLeft(t *testing.T) {
	var view View
	var box, first, last Box
	f := view.Begin(testCanvas(), ui.LightTheme(), ui.Rect{Width: 100, Height: 100})
	f.Stack().Fill().Children(func() {
		box = f.Column().Size(40, 20).Self(Center)
	})
	f.End()
	near(t, "centered", box.Rect(), ui.Rect{X: 30, Y: 40, Width: 40, Height: 20})

	f = view.Begin(testCanvas(), ui.LightTheme(), ui.Rect{Width: 100, Height: 30})
	f.Row().Height(30).Justify(Between).Children(func() {
		first = f.Slot().Width(10)
		f.Slot().Width(10)
		last = f.Slot().Width(10)
	})
	f.End()
	if first.Rect().X != 0 || last.Rect().X != 90 {
		t.Errorf("Between put the ends at %v and %v, want 0 and 90", first.Rect().X, last.Rect().X)
	}
}

func TestTextShrinksBeforeAFixedSiblingDoes(t *testing.T) {
	var view View
	var text, fixed Box
	f := view.Begin(testCanvas(), ui.LightTheme(), ui.Rect{Width: 100, Height: 30})
	f.Row().Height(30).Children(func() {
		text = f.Text("a very long label indeed").FontSize(10)
		fixed = f.Slot().Width(40)
	})
	f.End()
	if fixed.Rect().Width != 40 || text.Rect().Width != 60 {
		t.Errorf("fixed %v and text %v wide, want 40 and 60", fixed.Rect().Width, text.Rect().Width)
	}
}

func TestClickedIsReadWhereTheControlIsDeclared(t *testing.T) {
	var view View
	draw := func(canvas *ui.Canvas) (saved, cancelled, row bool) {
		view.Draw(canvas, ui.LightTheme(), ui.Rect{Width: 300, Height: 100}, func(f *Frame) {
			f.Column().Children(func() {
				f.Row().Hit(rowID, "row").Children(func() {
					saved = f.Button(save, "Save").Clicked()
					cancelled = f.Button(omit, "Cancel").Clicked()
				})
				row = false
			})
		})
		return
	}
	canvas := testCanvas()
	if saved, cancelled, _ := draw(canvas); saved || cancelled {
		t.Fatal("a paint that is not an event pass reports a click")
	}
	canvas.Reset()
	canvas.BeginEventPass(save)
	saved, cancelled, _ := draw(canvas)
	canvas.EndEventPass()
	if !saved || cancelled {
		t.Errorf("click on Save reported saved=%v cancelled=%v", saved, cancelled)
	}
}

func TestButtonsAreTheComponentsButtons(t *testing.T) {
	theme := ui.LightTheme()
	var view View
	canvas := testCanvas()
	view.Draw(canvas, theme, ui.Rect{Width: 300, Height: 100}, func(f *Frame) {
		f.Row().Children(func() { f.Button(save, "Save") })
	})
	flexed := append([]ui.Rectangle(nil), canvas.Rectangles()...)
	canvas.Reset()
	components.BeginButton(canvas, theme, save, ui.Rect{Width: components.BeginButton(canvas, theme, ui.ID{}, ui.Rect{}).SizeToFit("Save").BoundsWidth()}).Default("Save")
	direct := canvas.Rectangles()
	if len(flexed) != len(direct) || len(flexed) == 0 {
		t.Fatalf("flex painted %d rectangles, the component %d", len(flexed), len(direct))
	}
	for i := range direct {
		if flexed[i].Bounds != direct[i].Bounds || flexed[i].Color != direct[i].Color {
			t.Errorf("rectangle %d differs: %+v vs %+v", i, flexed[i], direct[i])
		}
	}
}

// page declares a screen with every kind of element, to count its allocations.
func declare(view *View, canvas *ui.Canvas, theme ui.Theme, title, detail string, checked bool) {
	view.Draw(canvas, theme, ui.Rect{Width: 800, Height: 600}, func(f *Frame) {
		f.Column().Pad(24).Gap(12).Children(func() {
			f.Row().Gap(10).Items(Center).Children(func() {
				f.Icon(ui.Icon{ViewBox: 24}).Size(20, 20)
				f.Text(title).FontSize(18).Bold()
				f.Spacer()
				f.Switch(save, checked).Small()
			})
			f.Divider()
			for range 12 {
				f.Row().Gap(8).Pad(6).Radius(6).Hit(rowID, "row").Hover(Look{}.Background(theme.Hover)).Children(func() {
					f.Checkbox(omit, checked)
					f.Text(detail).Grow(1).Muted()
					if f.Button(save, "Open").Ghost().Small().Clicked() {
						checked = !checked
					}
				})
			}
			f.Row().Gap(8).Justify(End).Children(func() {
				f.Button(omit, "Cancel").Outline()
				f.Button(save, "Save")
			})
		})
	})
}

func TestADeclaredPageAllocatesNothingOnceWarm(t *testing.T) {
	var view View
	canvas := testCanvas()
	theme := ui.LightTheme()
	run := func() { canvas.Reset(); declare(&view, canvas, theme, "Hosts", "a row of the list", true) }
	run()
	if allocs := testing.AllocsPerRun(200, run); allocs != 0 {
		t.Fatalf("a warm page allocates %v times a frame, want 0", allocs)
	}
}

func BenchmarkDeclaredPage(b *testing.B) {
	var view View
	canvas := testCanvas()
	theme := ui.LightTheme()
	b.ReportAllocs()
	for b.Loop() {
		canvas.Reset()
		declare(&view, canvas, theme, "Hosts", "a row of the list", true)
	}
}

func TestParagraphIsAsTallAsItsWidthMakesIt(t *testing.T) {
	var view View
	var wide, narrow, after Box
	text := "one two three four five six seven eight nine ten"
	draw := func(width float32) {
		f := view.Begin(testCanvas(), ui.LightTheme(), ui.Rect{Width: width, Height: 400})
		f.Column().Children(func() {
			if width > 200 {
				wide = f.Paragraph(text, 5).FontSize(10)
				after = f.Slot().Height(10)
			} else {
				narrow = f.Paragraph(text, 5).FontSize(10)
				after = f.Slot().Height(10)
			}
		})
		f.End()
	}
	draw(300) // 5 pt per character: the 49 characters fit one 300 wide line
	oneLine := wide.Rect().Height
	if oneLine != 17 || after.Rect().Y != 17 {
		t.Fatalf("a paragraph that fits is %v tall and the next element starts at %v, want 17 and 17", oneLine, after.Rect().Y)
	}
	draw(100)
	if narrow.Rect().Height <= oneLine*2 || after.Rect().Y != narrow.Rect().Height {
		t.Errorf("at 100 wide the paragraph is %v tall and the next element starts at %v: it did not push it down", narrow.Rect().Height, after.Rect().Y)
	}
	if narrow.Rect().Height > 5*17 {
		t.Errorf("a paragraph limited to 5 lines is %v tall", narrow.Rect().Height)
	}
}

func TestScrollClipsShiftsAndReportsItsContent(t *testing.T) {
	var view View
	var area, first, last Box
	f := view.Begin(testCanvas(), ui.LightTheme(), ui.Rect{Width: 100, Height: 100})
	f.Column().Fill().Children(func() {
		area = f.Scroll(30).Grow(1).Gap(5).Children(func() {
			first = f.Slot().Height(40)
			f.Slot().Height(40)
			last = f.Slot().Height(40)
		})
	})
	f.End()
	if area.Content() != 130 || area.Rect().Height != 100 {
		t.Errorf("content %v in a %v tall area, want 130 in 100", area.Content(), area.Rect().Height)
	}
	if first.Rect().Y != -30 || last.Rect().Y != 60 || first.Rect().Height != 40 {
		t.Errorf("children at %v and %v (first %v tall), want -30 and 60, none squeezed", first.Rect().Y, last.Rect().Y, first.Rect().Height)
	}
}

func TestGridFitsAsManyColumnsAsWidthAllows(t *testing.T) {
	var view View
	var cells [5]Box
	draw := func(width float32) float32 {
		f := view.Begin(testCanvas(), ui.LightTheme(), ui.Rect{Width: width, Height: 400})
		var grid Box
		f.Column().Children(func() {
			grid = f.Grid(100).Gap(10).Children(func() {
				for i := range cells {
					cells[i] = f.Slot().Height(20 + float32(i%2)*10)
				}
			})
		})
		f.End()
		return grid.Rect().Height
	}
	height := draw(330) // 3 columns of 103.33: two rows
	if cells[0].Rect().Width < 103 || cells[0].Rect().Width > 104 || cells[3].Rect().Y != cells[0].Rect().Y+30+10 {
		t.Errorf("3 columns: cell width %v, second row at %v, want 103.33 and 40 below the first", cells[0].Rect().Width, cells[3].Rect().Y)
	}
	if cells[2].Rect().X < 226 || height != 30+10+30 {
		t.Errorf("third cell at x=%v, grid %v tall, want 226.7 and 70", cells[2].Rect().X, height)
	}
	if narrow := draw(150); cells[1].Rect().Y <= cells[0].Rect().Y || narrow <= height {
		t.Errorf("one column at 150 wide: second cell at y=%v, grid %v tall, want it below the first and taller", cells[1].Rect().Y, narrow)
	}
}

func TestParagraphScrollAndGridAllocateNothingOnceWarm(t *testing.T) {
	var view View
	canvas := testCanvas()
	theme := ui.LightTheme()
	run := func() {
		canvas.Reset()
		view.Draw(canvas, theme, ui.Rect{Width: 500, Height: 400}, func(f *Frame) {
			f.Column().Fill().Pad(8).Gap(8).Children(func() {
				f.Paragraph("a longer explanation that needs more than one line in a column this narrow, truly", 4).Muted()
				f.Grid(90).Gap(6).Children(func() {
					for range 9 {
						f.Column().Pad(6).Border(1, theme.Border).Children(func() { f.Text("cell").FontSize(11) })
					}
				})
				f.Scroll(20).Grow(1).Gap(4).Children(func() {
					for range 30 {
						f.Row().Gap(6).Children(func() { f.Text("row").Grow(1); f.Button(save, "Go").Small() })
					}
				})
			})
		})
	}
	run()
	if allocs := testing.AllocsPerRun(200, run); allocs != 0 {
		t.Fatalf("a warm page with a paragraph, a grid and a scroll area allocates %v times a frame, want 0", allocs)
	}
}

func TestPercentBasisAndCappedGrowth(t *testing.T) {
	var view View
	var half, rest, capped, spacer, tail Box
	f := view.Begin(testCanvas(), ui.LightTheme(), ui.Rect{Width: 400, Height: 40})
	f.Column().Children(func() {
		f.Row().Height(40).Children(func() {
			half = f.Slot().WidthPercent(50)
			rest = f.Slot().Grow(1)
		})
	})
	f.End()
	if half.Rect().Width != 200 || rest.Rect().Width != 200 {
		t.Errorf("50%% and the rest are %v and %v wide, want 200 and 200", half.Rect().Width, rest.Rect().Width)
	}

	// A child that hits its maximum gives what it cannot take to the others, so
	// a capped box beside a Spacer pushes the trailing element to the far edge.
	f = view.Begin(testCanvas(), ui.LightTheme(), ui.Rect{Width: 800, Height: 40})
	f.Row().Height(40).Gap(10).Children(func() {
		capped = f.Slot().Grow(1).MaxWidth(300).Height(40)
		spacer = f.Spacer()
		tail = f.Slot().Width(100)
	})
	f.End()
	if capped.Rect().Width != 300 || tail.Rect().X+tail.Rect().Width != 800 || spacer.Rect().Width != 380 {
		t.Errorf("capped %v wide, spacer %v, tail ends at %v; want 300, 380 and 800", capped.Rect().Width, spacer.Rect().Width, tail.Rect().X+tail.Rect().Width)
	}

	// Basis sets the proportions Grow shares around.
	var a, b Box
	f = view.Begin(testCanvas(), ui.LightTheme(), ui.Rect{Width: 300, Height: 40})
	f.Row().Height(40).Children(func() {
		a = f.Slot().Basis(100).Grow(1)
		b = f.Slot().Basis(50).Grow(1)
	})
	f.End()
	if a.Rect().Width != 175 || b.Rect().Width != 125 {
		t.Errorf("basis 100 and 50 growing equally are %v and %v, want 175 and 125", a.Rect().Width, b.Rect().Width)
	}
}

func TestBaselineAlignmentLinesUpDifferentSizes(t *testing.T) {
	var view View
	var big, small Box
	baseline := func(b Box, size float32) float32 {
		return b.Rect().Y + (b.Rect().Height-size-7)/2 + (size+7-size)/2 + size*0.8
	}
	f := view.Begin(testCanvas(), ui.LightTheme(), ui.Rect{Width: 300, Height: 60})
	f.Row().Height(60).Items(Baseline).Children(func() {
		big = f.Text("Big").FontSize(24)
		small = f.Text("small").FontSize(11)
	})
	f.End()
	if d := baseline(big, 24) - baseline(small, 11); d > 0.01 || d < -0.01 {
		t.Errorf("baselines differ by %v: big at %v, small at %v", d, big.Rect().Y, small.Rect().Y)
	}
	if small.Rect().Y <= big.Rect().Y {
		t.Errorf("the small label (y=%v) should sit lower than the top of the big one (y=%v)", small.Rect().Y, big.Rect().Y)
	}
}

func TestMinimumsApplyAfterSharingNotBefore(t *testing.T) {
	var view View
	var a, b Box
	draw := func(height float32) {
		f := view.Begin(testCanvas(), ui.LightTheme(), ui.Rect{Width: 100, Height: height})
		f.Column().Fill().Children(func() {
			a = f.Spacer().Grow(0.42).MinHeight(16)
			b = f.Spacer().Grow(0.58)
		})
		f.End()
	}
	draw(100)
	if a.Rect().Height != 42 || b.Rect().Height != 58 {
		t.Errorf("in 100: %v and %v, want 42 and 58", a.Rect().Height, b.Rect().Height)
	}
	draw(20) // 42% of 20 is below the minimum: it holds at 16 and the other takes what is left
	if a.Rect().Height != 16 || b.Rect().Height != 4 {
		t.Errorf("in 20: %v and %v, want 16 and 4", a.Rect().Height, b.Rect().Height)
	}
}

func TestANegativeMarginPullsAChildOut(t *testing.T) {
	var view View
	var first, second Box
	f := view.Begin(testCanvas(), ui.LightTheme(), ui.Rect{Width: 100, Height: 100})
	f.Column().Children(func() {
		first = f.Slot().Height(20)
		second = f.Slot().Height(20).MarginSides(-6, -12, 0, 0)
	})
	f.End()
	if second.Rect().Y != 14 || second.Rect().Width != 112 {
		t.Errorf("second at y=%v, %v wide, want 14 and 112", second.Rect().Y, second.Rect().Width)
	}
	_ = first
}

func TestEllipsisKeepsOneLineAndEndsItWithAnEllipsis(t *testing.T) {
	var view View
	var name, after Box
	f := view.Begin(testCanvas(), ui.LightTheme(), ui.Rect{Width: 60, Height: 100})
	f.Column().Children(func() {
		name = f.Text("a file name that is far too long to fit").FontSize(10).Ellipsis()
		after = f.Slot().Height(5)
	})
	f.End()
	if name.Rect().Height != 17 || after.Rect().Y != 17 {
		t.Errorf("an ellipsized text is %v tall and the next element starts at %v, want one line, 17 and 17", name.Rect().Height, after.Rect().Y)
	}
	if got := f.lines[f.nodes[name.i].lineFirst]; got == "a file name that is far too long to fit" || got == "" {
		t.Errorf("the line is %q, want it shortened", got)
	}
}

func TestPopupTakesNoRoomIsPlacedAndKeptInsideTheFrame(t *testing.T) {
	var view View
	var after, menu Box
	anchor := ui.Rect{X: 300, Y: 20, Width: 80, Height: 30}
	draw := func(height float32, place func(Box) Box) {
		f := view.Begin(testCanvas(), ui.LightTheme(), ui.Rect{Width: 400, Height: height})
		f.Column().Fill().Gap(10).Children(func() {
			f.Slot().Height(20)
			menu = place(f.Popup().Width(120).Children(func() { f.Slot().Height(60) }))
			after = f.Slot().Height(20)
		})
		f.End()
	}
	draw(300, func(b Box) Box { return b.Below(anchor, 4, End) })
	if after.Rect().Y != 30 {
		t.Errorf("the popup took room: the next element is at y=%v, want 30", after.Rect().Y)
	}
	if r := menu.Rect(); r.X != 260 || r.Y != 54 || r.Width != 120 || r.Height != 60 {
		t.Errorf("below, right-aligned: %+v, want x=260 y=54 120x60", r)
	}
	draw(80, func(b Box) Box { return b.Below(anchor, 4, End) }) // no room under the anchor: opens over it, kept in the frame
	if r := menu.Rect(); r.Y < 0 || r.Y+r.Height > 80 {
		t.Errorf("in an 80 high frame the popup spans %v to %v", r.Y, r.Y+r.Height)
	}
	draw(300, func(b Box) Box { return b.At(390, 290) }) // at the pointer, pushed back inside
	if r := menu.Rect(); r.X != 280 || r.Y != 240 {
		t.Errorf("at the corner: %+v, want x=280 y=240", r)
	}
}

func TestEndJustificationOverflowsAtTheStart(t *testing.T) {
	var view View
	var first, last Box
	f := view.Begin(testCanvas(), ui.LightTheme(), ui.Rect{Width: 100, Height: 30})
	f.Row().Height(30).Justify(End).Children(func() {
		first = f.Slot().Width(70).Shrink(0)
		last = f.Slot().Width(70).Shrink(0)
	})
	f.End()
	if last.Rect().X+last.Rect().Width != 100 || first.Rect().X != -40 {
		t.Errorf("the last ends at %v and the first starts at %v, want 100 and -40", last.Rect().X+last.Rect().Width, first.Rect().X)
	}
}

func TestPopupsAllocateNothingOnceWarm(t *testing.T) {
	var view View
	canvas := testCanvas()
	theme := ui.LightTheme()
	run := func() {
		canvas.Reset()
		view.Draw(canvas, theme, ui.Rect{Width: 400, Height: 300}, func(f *Frame) {
			f.Column().Fill().Children(func() { f.Text("page") })
			f.Popup().Below(ui.Rect{X: 10, Y: 10, Width: 50, Height: 20}, 4, Start).Pad(4).Children(func() {
				f.Text("one")
				f.Text("two")
			})
		})
	}
	run()
	if allocs := testing.AllocsPerRun(200, run); allocs != 0 {
		t.Fatalf("a warm page with a popup allocates %v times, want 0", allocs)
	}
}

func TestFieldsAreLaidOutAndPaintedInTreeOrder(t *testing.T) {
	var view View
	canvas := testCanvas()
	theme := ui.LightTheme()
	options := []components.SegmentOption{{ID: save, Label: "A"}, {ID: omit, Label: "B"}}
	var input, segment, area Box
	f := view.Begin(canvas, theme, ui.Rect{Width: 300, Height: 300})
	f.Column().Fill().Gap(8).Children(func() {
		input = f.Input(components.InputSnapshot{ID: rowID, Label: "Name", Value: "x"})
		segment = f.Segmented(options, save)
		area = f.Textarea(components.TextareaSnapshot{Label: "Notes"})
	})
	f.End()
	h := theme.Metrics.ControlHeight
	if input.Rect().Width != 300 || input.Rect().Height != h || segment.Rect().Y != h+8 || area.Rect().Height != 64 {
		t.Errorf("input %+v segment %+v textarea %+v", input.Rect(), segment.Rect(), area.Rect())
	}
	if len(canvas.Hits()) < 3 {
		t.Errorf("the fields declared %d hit regions, want the input's and one for each segment", len(canvas.Hits()))
	}
	run := func() {
		canvas.Reset()
		view.Draw(canvas, theme, ui.Rect{Width: 300, Height: 300}, func(f *Frame) {
			f.Column().Fill().Gap(8).Children(func() {
				f.Input(components.InputSnapshot{ID: rowID, Value: "x"})
				f.Segmented(options, save)
				f.Textarea(components.TextareaSnapshot{})
				f.InputGroup(components.InputGroupSnapshot{})
			})
		})
	}
	run()
	if allocs := testing.AllocsPerRun(100, run); allocs != 0 {
		t.Errorf("fields allocate %v times a frame, want 0", allocs)
	}
}

func TestStrokeTakesNoRoomAndHitOutsetWidensTheClickRegion(t *testing.T) {
	var view View
	canvas := testCanvas()
	var bordered, stroked Box
	f := view.Begin(canvas, ui.LightTheme(), ui.Rect{Width: 100, Height: 100})
	f.Column().Children(func() {
		bordered = f.Column().Border(2, ui.Color{Alpha: 1}).Children(func() { f.Slot().Height(10) })
		stroked = f.Column().Stroke(2, ui.Color{Alpha: 1}).Children(func() { f.Slot().Height(10) })
		f.Column().Height(10).Hit(rowID, "row").HitOutset(0, 6, 0, 4)
	})
	f.End()
	if bordered.Rect().Height != 14 || stroked.Rect().Height != 10 {
		t.Errorf("a border-box is %v tall and a stroked one %v, want 14 and 10", bordered.Rect().Height, stroked.Rect().Height)
	}
	hits := canvas.Hits()
	if len(hits) != 1 || hits[0].Bounds.X != -4 || hits[0].Bounds.Width != 110 {
		t.Errorf("hit region %+v, want it to start at -4 and be 110 wide", hits)
	}
}

func TestBesideOpensOnTheOtherSideWhenItDoesNotFit(t *testing.T) {
	var view View
	var sub Box
	draw := func(anchorX float32) ui.Rect {
		f := view.Begin(testCanvas(), ui.LightTheme(), ui.Rect{Width: 400, Height: 200})
		f.Column().Fill()
		sub = f.Popup().Width(100).Height(50).Beside(ui.Rect{X: anchorX, Y: 20, Width: 80, Height: 24}, 4, Start)
		f.End()
		return sub.Rect()
	}
	if r := draw(100); r.X != 184 || r.Y != 20 {
		t.Errorf("room on the right: %+v, want x=184 y=20", r)
	}
	if r := draw(310); r.X != 206 {
		t.Errorf("no room on the right: x=%v, want 206 (its right edge 4 from the anchor's left)", r.X)
	}
}

func TestScrollAreaKeepsItsOffsetInRangeAndHandlesTheWheel(t *testing.T) {
	var view View
	draw := func(height float32) {
		f := view.Begin(testCanvas(), ui.LightTheme(), ui.Rect{Width: 100, Height: height})
		f.Column().Fill().Children(func() {
			f.ScrollArea(listID).Grow(1).Children(func() {
				for range 10 {
					f.Slot().Height(30)
				}
			})
		})
		f.End()
	}
	draw(100)
	state := view.ScrollState(listID)
	if state.Content != 300 || state.Viewport != 100 || state.Max() != 200 {
		t.Fatalf("content %v viewport %v max %v, want 300, 100, 200", state.Content, state.Viewport, state.Max())
	}
	if r := state.Wheel(ui.ScrollEvent{DeltaY: 5}); !r.Repaint || state.Offset != 24 {
		t.Errorf("a wheel notch of 5 moved to %v (repaint %v), want one line, 24", state.Offset, r.Repaint)
	}
	state.Wheel(ui.ScrollEvent{DeltaY: 5000, Precise: true})
	if state.Offset != 200 {
		t.Errorf("scrolled past the end: %v, want 200", state.Offset)
	}
	draw(250) // the window grew: less to scroll, and the old offset is out of range
	if state.Offset != 50 {
		t.Errorf("after the window grew the offset is %v, want it clamped to 50", state.Offset)
	}
}

func TestChosenReportsTheClickedSegment(t *testing.T) {
	var view View
	options := []components.SegmentOption{{ID: save, Label: "A"}, {ID: omit, Label: "B"}}
	var got ui.ID
	canvas := testCanvas()
	draw := func() {
		view.Draw(canvas, ui.LightTheme(), ui.Rect{Width: 200, Height: 60}, func(f *Frame) {
			if id, ok := f.Segmented(options, save).Chosen(); ok {
				got = id
			}
		})
	}
	draw()
	if !got.IsZero() {
		t.Fatal("a plain paint reports a choice")
	}
	canvas.Reset()
	canvas.BeginEventPass(omit)
	draw()
	canvas.EndEventPass()
	if got != omit {
		t.Errorf("chosen %v, want the second option", got)
	}
}

func TestAColumnDoesNotSquashContentThatDoesNotFit(t *testing.T) {
	var view View
	var grid, tall, scroll Box
	f := view.Begin(testCanvas(), ui.LightTheme(), ui.Rect{Width: 200, Height: 100})
	f.Column().Fill().Children(func() {
		grid = f.Column().Children(func() { f.Slot().Height(40) })
		tall = f.Column().Children(func() { f.Slot().Height(80) })
		scroll = f.Scroll(0).Children(func() { f.Slot().Height(200) })
	})
	f.End()
	if grid.Rect().Height != 40 || tall.Rect().Height != 80 {
		t.Errorf("content-sized columns were squashed to %v and %v, want 40 and 80", grid.Rect().Height, tall.Rect().Height)
	}
	if scroll.Rect().Height != 0 {
		t.Errorf("the scroll area took %v of a full column, want it squeezed out", scroll.Rect().Height)
	}
}

func TestPopupCanHangOffAnElementLaidOutLater(t *testing.T) {
	var view View
	var button, menu Box
	f := view.Begin(testCanvas(), ui.LightTheme(), ui.Rect{Width: 400, Height: 200})
	f.Column().Fill().Pad(20).Children(func() {
		f.Row().Children(func() {
			f.Spacer()
			button = f.Slot().Size(80, 30)
		})
		menu = f.Popup().Width(120).Height(40).BelowBox(button, 4, End)
	})
	f.End()
	b, m := button.Rect(), menu.Rect()
	if m.Y != b.Y+b.Height+4 || m.X+m.Width != b.X+b.Width {
		t.Errorf("menu %+v under button %+v, want 4 below it with right edges aligned", m, b)
	}
}

func TestASegmentedControlInARowHasItsOwnWidth(t *testing.T) {
	var view View
	var seg Box
	f := view.Begin(testCanvas(), ui.LightTheme(), ui.Rect{Width: 600, Height: 60})
	f.Row().Height(40).Children(func() {
		f.Text("Theme").Grow(1)
		seg = f.Segmented([]components.SegmentOption{{ID: save, Label: "System"}, {ID: omit, Label: "Dark"}}, save)
	})
	f.End()
	if seg.Rect().Width < 80 || seg.Rect().Width > 300 {
		t.Errorf("a segmented control in a row is %v wide, want about the width of its labels", seg.Rect().Width)
	}
}

func TestChoiceNeedsNoIdentitiesOfItsOwn(t *testing.T) {
	var view View
	scope := tree.Sub(page, tree.Name("theme"))
	labels := []string{"System", "Light", "Dark"}
	got := -1
	canvas := testCanvas()
	draw := func() {
		view.Draw(canvas, ui.LightTheme(), ui.Rect{Width: 400, Height: 60}, func(f *Frame) {
			if i, ok := f.Choice(scope, labels, 0).ChosenIndex(); ok {
				got = i
			}
		})
	}
	draw()
	if got != -1 {
		t.Fatal("a plain paint reports a choice")
	}
	canvas.Reset()
	canvas.BeginEventPass(scope.At(2))
	draw()
	canvas.EndEventPass()
	if got != 2 {
		t.Errorf("chosen %d, want 2", got)
	}
}

// A Text, an Icon or a Slot given Hit is a pointer target too, as the Hit
// documentation promises for every element that is not a control.
func TestHitMakesATextAnIconAndASlotPointerTargets(t *testing.T) {
	var view View
	canvas := testCanvas()
	view.Draw(canvas, ui.LightTheme(), ui.Rect{Width: 300, Height: 100}, func(f *Frame) {
		f.Column().Children(func() {
			f.Text("item").Hit(rowID, "item")
			f.Icon(ui.Icon{ViewBox: 24}).Size(20, 20).Hit(save, "icon")
			f.Slot().Height(10).Hit(omit, "slot")
		})
	})
	labels := map[string]bool{}
	for _, hit := range canvas.Hits() {
		labels[hit.Label] = true
	}
	if !labels["item"] || !labels["icon"] || !labels["slot"] {
		t.Fatalf("hits %+v: a text, icon or slot with Hit registered no target", canvas.Hits())
	}
}

func TestShadowIsCastByTheElementsRoundedRectangleUnderItsFill(t *testing.T) {
	var view View
	canvas := testCanvas()
	f := view.Begin(canvas, ui.LightTheme(), ui.Rect{Width: 100, Height: 100})
	f.Column().Size(40, 20).Radius(6).Background(ui.Color{Alpha: 1}).Shadow(ui.ShadowMD).Children(func() {})
	f.End()
	shadows := canvas.Shadows()
	if len(shadows) != 2 {
		t.Fatalf("shadows = %d, want the two layers of shadow-md", len(shadows))
	}
	if shadows[0].Bounds != (ui.Rect{Width: 40, Height: 20}) || shadows[0].Radius != 6 {
		t.Fatalf("shadow cast by %+v radius %v, want the element's rectangle", shadows[0].Bounds, shadows[0].Radius)
	}
	if len(canvas.Rectangles()) != 1 || shadows[1].RectsBefore != 0 {
		t.Fatalf("the shadow must be painted before the fill: %d rectangles, shadow after %d", len(canvas.Rectangles()), shadows[1].RectsBefore)
	}
}

func TestAMovesWindowRowDeclaresAMoveRegionBeneathItsControls(t *testing.T) {
	var view View
	canvas := testCanvas()
	var bar Box
	f := view.Begin(canvas, ui.LightTheme(), ui.Rect{Width: 200, Height: 100})
	f.Column().Children(func() {
		bar = f.Row().Height(30).MovesWindow().Children(func() {
			f.Button(save, "Save")
		})
		f.Row().Height(30)
	})
	f.End()
	var order []string
	for _, hit := range canvas.Hits() {
		switch {
		case hit.MovesWindow:
			near(t, "move region", hit.Bounds, bar.Rect())
			order = append(order, "move")
		case hit.ID == save:
			order = append(order, "save")
		}
	}
	if len(order) != 2 || order[0] != "move" || order[1] != "save" {
		t.Fatalf("hits in paint order = %v, want the move region first, so the button lies over it", order)
	}
	moves := 0
	for _, hit := range canvas.Hits() {
		if hit.MovesWindow {
			moves++
		}
	}
	if moves != 1 {
		t.Fatalf("%d move regions, want only the marked row's", moves)
	}
}

// A checkbox or switch declares an enlarged hit region. The layout has to
// reserve that region, not just the visible square, or the hit targets of a
// compact column overlap and a press near the boundary reaches the wrong one.
func TestCompactCheckboxAndSwitchHitRegionsDoNotOverlap(t *testing.T) {
	var view View
	canvas := testCanvas()
	const width, height = 200, 200
	view.Draw(canvas, ui.LightTheme(), ui.Rect{Width: width, Height: height}, func(f *Frame) {
		f.Column().Children(func() {
			f.Checkbox(save, false).Label("one")
			f.Checkbox(omit, false).Label("two")
			f.Switch(rowID, false).Label("three")
			f.Switch(listID, false).Label("four")
		})
	})
	hits := canvas.Hits()
	if len(hits) != 4 {
		t.Fatalf("declared %d hit regions, want four", len(hits))
	}
	for index, hit := range hits {
		if hit.Bounds.X < 0 || hit.Bounds.Y < 0 || hit.Bounds.X+hit.Bounds.Width > width+0.01 || hit.Bounds.Y+hit.Bounds.Height > height+0.01 {
			t.Errorf("hit %d (%v) leaves the page", index, hit.Bounds)
		}
		for previous := 0; previous < index; previous++ {
			a, b := hits[previous].Bounds, hit.Bounds
			if a.X < b.X+b.Width && b.X < a.X+a.Width && a.Y < b.Y+b.Height && b.Y < a.Y+a.Height {
				t.Errorf("hits %d and %d overlap: %v and %v", previous, index, a, b)
			}
		}
	}
}

func TestViewTrimReleasesLargePageNodes(t *testing.T) {
	var view View
	canvas := testCanvas()
	theme := ui.LightTheme()
	view.Draw(canvas, theme, ui.Rect{Width: 200, Height: 200}, func(f *Frame) {
		f.Column().Children(func() {
			for i := 0; i < 10000; i++ {
				f.Text("row")
			}
		})
	})
	view.Draw(canvas, theme, ui.Rect{Width: 200, Height: 200}, func(f *Frame) {
		f.Text("one")
	})
	if got := cap(view.frame.nodes); got > 4096 {
		t.Fatalf("nodes capacity after a large page = %d, want the spike released", got)
	}
}
