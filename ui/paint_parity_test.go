package ui_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// This file compares the paint a page control produces with the paint its
// components.* declaration produces for the same state. Both paths are painted
// into a page of the same theme and size, one through the control and one
// through a Slot.Draw callback, so the comparison covers the whole page path:
// the control's geometry, its state forwarding and its use of the component.
//
// A paintSnapshot is what survives a frame: rectangles and labels. Identity is
// deliberately not part of it. Layer, Order, Text.Identity and the text run
// table offsets only name a primitive across frames or index retained storage;
// two paints of the same picture through different page plumbing may number
// them differently. Bounds, colors, radii and label text are the picture, and
// those are compared exactly.

// paintSnapshot is the observable picture of one frame.
type paintSnapshot struct {
	Rects []gfx.Rectangle
	Texts []gfx.Text
}

func (snapshot paintSnapshot) dump() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%d rectangles:\n", len(snapshot.Rects))
	for i, rect := range snapshot.Rects {
		fmt.Fprintf(&b, "  [%d] %+v\n", i, rect)
	}
	fmt.Fprintf(&b, "%d texts:\n", len(snapshot.Texts))
	for i, text := range snapshot.Texts {
		fmt.Fprintf(&b, "  [%d] %+v\n", i, text)
	}
	return b.String()
}

// takePaint copies the frame's paint lists and removes the identity-only fields:
// the canvas layer, the paint order index, the retained-text identity and the
// run-table indices. Everything else is the primitive itself.
func takePaint(canvas *gfx.Canvas) paintSnapshot {
	rects := append([]gfx.Rectangle(nil), canvas.Rectangles()...)
	for i := range rects {
		rects[i].Layer, rects[i].Order = 0, 0
	}
	texts := append([]gfx.Text(nil), canvas.Texts()...)
	for i := range texts {
		texts[i].Layer, texts[i].Order = 0, 0
		texts[i].Identity = gfx.TextIdentity{}
		texts[i].RunFirst, texts[i].RunCount = 0, 0
	}
	return paintSnapshot{Rects: rects, Texts: texts}
}

// firstPaintDifference reports the first field that differs between two
// pictures, or "" when they are the same picture. The message names the list,
// the index and the field, so a failure points at the primitive and the value
// that diverged rather than at a whole-struct dump.
func firstPaintDifference(page, component paintSnapshot) string {
	if len(page.Rects) != len(component.Rects) {
		return fmt.Sprintf("the page painted %d rectangles, the component %d", len(page.Rects), len(component.Rects))
	}
	for i := range page.Rects {
		a, b := page.Rects[i], component.Rects[i]
		switch {
		case a.Bounds != b.Bounds:
			return fmt.Sprintf("rectangle %d bounds: page %+v, component %+v", i, a.Bounds, b.Bounds)
		case a.Color != b.Color:
			return fmt.Sprintf("rectangle %d color: page %+v, component %+v", i, a.Color, b.Color)
		case a.Radius != b.Radius:
			return fmt.Sprintf("rectangle %d radius: page %v, component %v", i, a.Radius, b.Radius)
		case a.HasClip != b.HasClip || a.ClipOf != b.ClipOf:
			return fmt.Sprintf("rectangle %d clip: page %+v/%v, component %+v/%v", i, a.ClipOf, a.HasClip, b.ClipOf, b.HasClip)
		}
	}
	if len(page.Texts) != len(component.Texts) {
		return fmt.Sprintf("the page painted %d texts, the component %d", len(page.Texts), len(component.Texts))
	}
	for i := range page.Texts {
		a, b := page.Texts[i], component.Texts[i]
		switch {
		case a.Value != b.Value:
			return fmt.Sprintf("text %d value: page %q, component %q", i, a.Value, b.Value)
		case a.Bounds != b.Bounds:
			return fmt.Sprintf("text %d bounds: page %+v, component %+v", i, a.Bounds, b.Bounds)
		case a.Style != b.Style:
			return fmt.Sprintf("text %d style: page %+v, component %+v", i, a.Style, b.Style)
		case a.CellWidth != b.CellWidth:
			return fmt.Sprintf("text %d cell width: page %v, component %v", i, a.CellWidth, b.CellWidth)
		case a.Baseline != b.Baseline:
			return fmt.Sprintf("text %d baseline: page %v, component %v", i, a.Baseline, b.Baseline)
		case a.Quad != b.Quad || a.Tint != b.Tint || a.HasQuad != b.HasQuad:
			return fmt.Sprintf("text %d quad: page %+v/%+v/%v, component %+v/%+v/%v", i, a.Quad, a.Tint, a.HasQuad, b.Quad, b.Tint, b.HasQuad)
		case a.HasClip != b.HasClip || a.ClipOf != b.ClipOf:
			return fmt.Sprintf("text %d clip: page %+v/%v, component %+v/%v", i, a.ClipOf, a.HasClip, b.ClipOf, b.HasClip)
		}
	}
	return ""
}

// requirePaintParity fails with the first difference and both pictures when the
// page path and the components path do not paint the same frame.
func requirePaintParity(t *testing.T, name string, page, component paintSnapshot) {
	t.Helper()
	if difference := firstPaintDifference(page, component); difference != "" {
		t.Fatalf("%s: %s\npage:\n%s\ncomponent:\n%s", name, difference, page.dump(), component.dump())
	}
}

// requirePainted fails when a parity case painted fewer rectangles or labels
// than the control promises. It keeps a harness bug (a slot that never runs, a
// picture that was emptied by normalization) from passing as parity.
func requirePainted(t *testing.T, name string, snapshot paintSnapshot, rects, texts int) {
	t.Helper()
	if len(snapshot.Rects) < rects || len(snapshot.Texts) < texts {
		t.Fatalf("%s painted %d rectangles and %d texts, want at least %d and %d", name, len(snapshot.Rects), len(snapshot.Texts), rects, texts)
	}
}

// paintOnPage paints view on a fresh page of the given theme and size and
// returns the frame's picture.
func paintOnPage(theme gfx.Theme, viewport gfx.Rect, view func(ui.C)) paintSnapshot {
	canvas := benchCanvas()
	page := ui.NewPage(ui.PageConfig{Theme: &theme, View: view})
	canvas.Reset()
	canvas.BeginFrame()
	page.Draw(canvas, viewport, view)
	canvas.EndFrame()
	return takePaint(canvas)
}

// slotView is a page whose only element is a slot at rect that paints fn. It is
// how the components path is put on a page: the same page plumbing, theme and
// text measurer as the control, without the control's declaration.
func slotView(rect gfx.Rect, paint func(*ui.DrawContext)) func(ui.C) {
	return func(c ui.C) {
		ui.Slot(c).Absolute().Left(rect.X).Top(rect.Y).Size(rect.Width, rect.Height).Draw(paint)
	}
}

// pageViewport is the page the control and component pictures are compared on.
var pageViewport = gfx.Rect{Width: 480, Height: 480}

// paintCaption reproduces the caption the page paints beside a Checkbox or
// Switch (see base/caption.go: the line is the font size plus seven, CaptionGap
// from the control's edge, vertically centered in the element). The caption is
// page composition, not part of components.Checkbox or components.Switch: the
// components declare a label for assistive technology but paint no text. The
// comparison therefore gives the component path the same caption explicitly.
func paintCaption(d *ui.DrawContext, control gfx.Rect, text string) {
	if text == "" {
		return
	}
	size := d.Theme.Metrics.FontSizeM
	line := size + 7
	x := control.X + control.Width + ui.CaptionGap
	bounds := gfx.Rect{
		X: x, Y: d.Rect.Y + (d.Rect.Height-line)/2,
		Width: max(d.Rect.X+d.Rect.Width-x, 0), Height: line,
	}
	d.Canvas.Label(text, bounds, gfx.TextStyle{Size: size, Weight: gfx.WeightMedium, Color: d.Theme.Foreground})
}

// A Checkbox and components.BeginCheckbox paint the same box, mark, ring and
// caption for the same state. The page centers the box inside the element's
// enlarged hit region, as CheckboxHitExtent documents.
func TestCheckboxPaintParity(t *testing.T) {
	theme := gfx.LightTheme()
	for _, test := range []struct {
		name              string
		checked, mixed    bool
		disabled, invalid bool
		caption           string
	}{
		{name: "unchecked"},
		{name: "checked", checked: true},
		{name: "mixed", mixed: true},
		{name: "disabled", disabled: true},
		{name: "invalid", invalid: true},
		{name: "captioned", checked: true, caption: "Accept terms"},
	} {
		t.Run(test.name, func(t *testing.T) {
			checked := test.checked
			var element ui.Box
			pageView := func(c ui.C) {
				ui.Column(c).Pad(16).Children(func() {
					element = ui.Checkbox(c, &checked, test.caption).
						Mixed(test.mixed).Disabled(test.disabled).Invalid(test.invalid)
				})
			}
			page := paintOnPage(theme, pageViewport, pageView)
			rect := element.Rect()
			wantTexts := 0
			if test.caption != "" {
				wantTexts = 1
			}
			requirePainted(t, test.name, page, 1, wantTexts)

			componentView := slotView(rect, func(d *ui.DrawContext) {
				side := components.CheckboxExtent(*d.Theme)
				bounds := gfx.Rect{
					X: rect.X + (rect.Width-side)/2, Y: rect.Y + (rect.Height-side)/2,
					Width: side, Height: side,
				}
				if test.caption != "" {
					bounds.X = rect.X
				}
				control := components.BeginCheckbox(d.Canvas, *d.Theme, gfx.ID{}, bounds, test.checked)
				if test.mixed {
					control = control.Indeterminate()
				}
				if test.disabled {
					control = control.Disabled()
				}
				if test.invalid {
					control = control.Invalid()
				}
				control.Paint()
				paintCaption(d, bounds, test.caption)
			})
			requirePaintParity(t, test.name, page, paintOnPage(theme, pageViewport, componentView))
		})
	}
}

// A Switch and components.BeginSwitch paint the same track and thumb for the
// same state, small or default, with the same caption.
func TestSwitchPaintParity(t *testing.T) {
	theme := gfx.LightTheme()
	for _, test := range []struct {
		name     string
		on       bool
		small    bool
		disabled bool
		caption  string
	}{
		{name: "off"},
		{name: "on", on: true},
		{name: "small", small: true},
		{name: "disabled", disabled: true},
		{name: "captioned", on: true, caption: "Enabled"},
	} {
		t.Run(test.name, func(t *testing.T) {
			on := test.on
			var element ui.Box
			pageView := func(c ui.C) {
				ui.Column(c).Pad(16).Children(func() {
					element = ui.Switch(c, &on, test.caption)
					if test.small {
						element = element.Small()
					}
					element = element.Disabled(test.disabled)
				})
			}
			page := paintOnPage(theme, pageViewport, pageView)
			rect := element.Rect()
			wantTexts := 0
			if test.caption != "" {
				wantTexts = 1
			}
			requirePainted(t, test.name, page, 1, wantTexts)

			componentView := slotView(rect, func(d *ui.DrawContext) {
				width, height := components.SwitchExtent(*d.Theme, test.small)
				bounds := gfx.Rect{
					X: rect.X + (rect.Width-width)/2, Y: rect.Y + (rect.Height-height)/2,
					Width: width, Height: height,
				}
				if test.caption != "" {
					bounds.X = rect.X
				}
				control := components.BeginSwitch(d.Canvas, *d.Theme, gfx.ID{}, bounds, test.on)
				if test.small {
					control = control.Small()
				}
				if test.disabled {
					control = control.Disabled()
				}
				control.Paint()
				paintCaption(d, bounds, test.caption)
			})
			requirePaintParity(t, test.name, page, paintOnPage(theme, pageViewport, componentView))
		})
	}
}

// A closed Select and components.BeginSelect paint the same trigger: surface,
// border, placeholder or value, and chevron. The open list is left out of this
// comparison: the page adds a popup element, a dismiss backdrop and an overlay
// layer that are page structure, and its painting is covered by the Select tests
// that drive the page.
func TestSelectPaintParity(t *testing.T) {
	theme := gfx.LightTheme()
	choices := []string{"Small", "Large"}
	for _, test := range []struct {
		name              string
		value             string
		disabled, invalid bool
	}{
		{name: "placeholder"},
		{name: "chosen", value: "Large"},
		{name: "disabled", disabled: true},
		{name: "invalid", invalid: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			value := test.value
			var element ui.Box
			pageView := func(c ui.C) {
				ui.Column(c).Pad(16).Children(func() {
					element = ui.Select(c, &value, choices, nil).Placeholder("Size").
						Disabled(test.disabled).Invalid(test.invalid).Width(220)
				})
			}
			page := paintOnPage(theme, pageViewport, pageView)
			rect := element.Rect()
			requirePainted(t, test.name, page, 1, 1) // the trigger surface and its value

			componentView := slotView(rect, func(d *ui.DrawContext) {
				components.BeginSelect(d.Canvas, *d.Theme, rect, components.SelectOptions{
					Value: test.value, Placeholder: "Size",
					Disabled: test.disabled, Invalid: test.invalid,
				})
			})
			requirePaintParity(t, test.name, page, paintOnPage(theme, pageViewport, componentView))
		})
	}
}

// A Table and the components.BeginTable calls the page makes paint the same
// static grid: header row, rows, cell text and rules. The comparison uses a
// table that fits its element exactly, so the page adds no scroll area, clip,
// hover overlay or scrollbar: those are page structure that surrounds the grid
// and are exercised by the Table scrollbar tests. A selected row is included
// because its fill is part of components.Table's row state.
func TestTablePaintParity(t *testing.T) {
	theme := gfx.LightTheme()
	rows := ui.TableRows{
		{"alpha", "1"},
		{"beta", "2"},
		{"gamma", "3"},
	}
	headers := []string{"Name", "Count"}
	columns := []components.TableColumn{{Width: 120}, {Width: 0}}
	var state ui.TableState
	state.Select(1)
	var element ui.Box
	pageView := func(c ui.C) {
		ui.Column(c).Pad(16).Children(func() {
			element = ui.Table(c, &state, ui.TableSpec{
				Columns: columns, Headers: headers, Source: &rows,
			}).Width(240)
		})
	}
	page := paintOnPage(theme, pageViewport, pageView)
	rect := element.Rect()
	requirePainted(t, "table", page, 1, 1+len(rows)) // the header and row labels

	componentView := slotView(rect, func(d *ui.DrawContext) {
		th := *d.Theme
		rowHeight := components.TableRowHeight(th)
		headerHeight := components.TableHeaderHeight(th)
		headerRow := gfx.Rect{X: rect.X, Y: rect.Y, Width: rect.Width, Height: headerHeight}
		if rect.Height > headerHeight {
			headerRow.Height = headerHeight + 1 // the rule below the header
		}
		header := components.BeginTable(d.Canvas, th, headerRow, columns).
			RowHeight(rowHeight).HeaderHeight(headerHeight)
		header.Header(headers)

		body := gfx.Rect{X: rect.X, Y: rect.Y + headerHeight, Width: rect.Width, Height: rect.Height - headerHeight}
		bodyTable := components.BeginTable(d.Canvas, th, body, columns).RowHeight(rowHeight)
		for row := range rows {
			bodyTable.RowState(gfx.ID{}, rows[row], components.TableRowState{Selected: row == state.Selected()})
		}
	})
	requirePaintParity(t, "table", page, paintOnPage(theme, pageViewport, componentView))
}

// A Toaster and components.PaintSonner paint the same stack: the cards, their
// icons, titles, descriptions, action button and close buttons. The page owns
// the queue, the timeout and the throttled frames, which paint nothing; the
// stack it hands the painter is exactly the SonnerToast slice the caller can
// build. Only the identities differ, and they are not part of the picture.
func TestToastPaintParity(t *testing.T) {
	theme := gfx.LightTheme()
	var toaster ui.Toaster
	toaster.Show("Saved", "The host list was written", ui.ToastSuccess)
	toaster.ShowAction("Connection lost", "Retrying in 5 seconds", ui.ToastError, "Retry")
	pageView := func(c ui.C) { ui.Toasts(c, &toaster) }
	page := paintOnPage(theme, pageViewport, pageView)
	requirePainted(t, "toasts", page, 1, 2) // two titles, at least

	// The page hands its stack to PaintSonner newest first; the scene items are
	// the same snapshots the Toaster was given, in that order.
	stack := []components.SonnerToast{
		{
			Snapshot: components.ToastSnapshot{Title: "Connection lost", Description: "Retrying in 5 seconds", Type: components.ToastError},
			Scope:    gfx.Item(21), DismissID: gfx.Item(22), ActionID: gfx.Item(23), ActionLabel: "Retry",
		},
		{
			Snapshot: components.ToastSnapshot{Title: "Saved", Description: "The host list was written", Type: components.ToastSuccess},
			Scope:    gfx.Item(11), DismissID: gfx.Item(12),
		},
	}
	componentView := slotView(pageViewport, func(d *ui.DrawContext) {
		components.PaintSonner(d.Canvas, *d.Theme, pageViewport, stack, components.SonnerOptions{Layer: ui.ToastLayer})
	})
	requirePaintParity(t, "toasts", page, paintOnPage(theme, pageViewport, componentView))
}

// A Drawer and components.BeginDrawer paint the same scrim, panel, rule, title,
// description and close button at rest. The page adds its body and footer as
// elements; they paint nothing here (no body or footer is declared). The open
// and close transitions are compared frame by frame by
// TestPageDrawerAnimatesLikeTheComponentsDrawer.
func TestDrawerPaintParity(t *testing.T) {
	theme := gfx.LightTheme()
	const size = 300
	open := true
	pageView := func(c ui.C) {
		ui.Drawer(c, &open, "Host details", ui.DrawerSpec{
			Description: "The selected host and its services", Side: ui.DrawerRight, Size: size,
		}, nil, nil)
	}
	page := paintOnPage(theme, pageViewport, pageView)
	requirePainted(t, "drawer", page, 1, 2) // title and description

	componentView := slotView(pageViewport, func(d *ui.DrawContext) {
		drawer := components.BeginDrawer(d.Canvas, *d.Theme, pageViewport, components.DrawerOptions{
			DismissID: gfx.Item(1), CloseID: gfx.Item(2), Side: components.DrawerRight, ContentExtent: size,
		})
		drawer.Begin("Host details", "The selected host and its services")
		drawer.End()
	})
	requirePaintParity(t, "drawer", page, paintOnPage(theme, pageViewport, componentView))
}

// The harness is not vacuous: a moved rectangle and a changed label are both
// reported, with the field named. Without this, a normalization mistake that
// erased the lists would make every parity test pass.
func TestPaintParityHarnessDetectsAMismatch(t *testing.T) {
	base := paintSnapshot{
		Rects: []gfx.Rectangle{{Bounds: gfx.Rect{X: 1, Y: 2, Width: 3, Height: 4}, Radius: 2}},
		Texts: []gfx.Text{{Value: "ok", Bounds: gfx.Rect{X: 5, Y: 6, Width: 7, Height: 8}}},
	}
	moved := base
	moved.Rects = append([]gfx.Rectangle(nil), base.Rects...)
	moved.Rects[0].Bounds.X++
	if difference := firstPaintDifference(base, moved); !strings.Contains(difference, "rectangle 0 bounds") {
		t.Fatalf("a moved rectangle is reported as %q", difference)
	}
	relabeled := base
	relabeled.Texts = append([]gfx.Text(nil), base.Texts...)
	relabeled.Texts[0].Value = "changed"
	if difference := firstPaintDifference(base, relabeled); !strings.Contains(difference, "text 0 value") {
		t.Fatalf("a changed label is reported as %q", difference)
	}
	if difference := firstPaintDifference(base, base); difference != "" {
		t.Fatalf("the same picture differs: %s", difference)
	}
}
