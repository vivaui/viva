package ui_test

import (
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

type overlayModel struct {
	overlay ui.Box
	Content ui.Box
}

func (m *overlayModel) View(c ui.C) {
	ui.Column(c).Fill().Pad(10).Children(func() {
		m.Content = ui.Button(c, "Content")
	})
	m.overlay = ui.Overlay(c, ui.OverlaySpec{
		Band: ui.LayerPaneChrome, Bounds: gfx.Rect{X: 40, Y: 40, Width: 160, Height: 120},
	}).Children(func() {
		ui.Button(c, "Overlay")
	})
}

// An overlay is painted on its band, after the page, and its controls take
// hits and Tab on that band: the page's content is hit before it, and the
// overlay's button is reached after the content's by Tab.
func TestOverlayPaintsAndTabsOnItsBand(t *testing.T) {
	m := &overlayModel{}
	page := ui.NewPage(ui.PageConfig{View: m.View})
	canvas := benchCanvas()
	drawPage(page, canvas, m.View)

	if got := m.overlay.Rect(); got != (gfx.Rect{X: 40, Y: 40, Width: 160, Height: 120}) {
		t.Fatalf("overlay rect = %+v", got)
	}
	var contentLayer, overlayLayer uint16
	var contentIndex, overlayIndex = -1, -1
	for i, hit := range canvas.Hits() {
		switch hit.Label {
		case "Content":
			contentLayer, contentIndex = hit.Layer, i
		case "Overlay":
			overlayLayer, overlayIndex = hit.Layer, i
		}
	}
	if contentLayer != uint16(ui.LayerContent) || overlayLayer != uint16(ui.LayerPaneChrome) {
		t.Fatalf("content layer %d, overlay layer %d", contentLayer, overlayLayer)
	}
	if contentIndex < 0 || overlayIndex <= contentIndex {
		t.Fatalf("hits: content at %d, overlay at %d; the overlay must be painted after the page", contentIndex, overlayIndex)
	}

	d := openPage(t, m.View)
	d.key(gfx.KeyTab)
	if got := d.Focused(); got != "Content" {
		t.Fatalf("first Tab focuses %q, want the page's Content", got)
	}
	d.key(gfx.KeyTab)
	if got := d.Focused(); got != "Overlay" {
		t.Fatalf("second Tab focuses %q, want the overlay's Overlay", got)
	}
}

type dialogOverlayModel struct {
	overlay ui.Box
	open    bool
}

func (m *dialogOverlayModel) View(c ui.C) {
	m.overlay = ui.Overlay(c, ui.OverlaySpec{
		Band: ui.LayerPaneChrome, Bounds: gfx.Rect{X: 0, Y: 0, Width: 120, Height: 120},
	}).Children(func() {
		ui.Button(c, "Overlay")
	})
	ui.Modal(c, &m.open, "Settings", ui.ModalSpec{}, func() {
		ui.Button(c, "Inside")
	}, nil)
}

// Viva's own widgets stay above an application band: a modal opened over a
// sidebar paints its contents and takes hits above the overlay.
func TestWidgetsStayAboveAnApplicationOverlay(t *testing.T) {
	m := &dialogOverlayModel{open: true}
	page := ui.NewPage(ui.PageConfig{View: m.View})
	canvas := benchCanvas()
	drawPage(page, canvas, m.View)

	var overlayLayer, insideLayer uint16
	for _, hit := range canvas.Hits() {
		switch hit.Label {
		case "Overlay":
			overlayLayer = hit.Layer
		case "Inside":
			insideLayer = hit.Layer
		}
	}
	if overlayLayer == 0 || insideLayer <= overlayLayer {
		t.Fatalf("overlay layer %d, modal layer %d; the modal must be above the overlay", overlayLayer, insideLayer)
	}
}

// A warm page with an overlay allocates nothing: the overlay is a Popup in the
// frame's reused float list, and the band is a node field.
func TestOverlayAllocatesNothing(t *testing.T) {
	m := &overlayModel{}
	page := ui.NewPage(ui.PageConfig{View: m.View})
	canvas := benchCanvas()
	drawPage(page, canvas, m.View)
	run := func() { drawPage(page, canvas, m.View) }
	if allocs := testing.AllocsPerRun(100, run); allocs != 0 {
		t.Fatalf("a warm page with an overlay allocates %v times a frame, want 0", allocs)
	}
}

func BenchmarkOverlay(b *testing.B) {
	m := &overlayModel{}
	page := ui.NewPage(ui.PageConfig{View: m.View})
	canvas := benchCanvas()
	b.ReportAllocs()
	for b.Loop() {
		drawPage(page, canvas, m.View)
	}
}

type blockOverlayModel struct {
	underClicks int
}

func (m *blockOverlayModel) View(c ui.C) {
	ui.Column(c).Fill().Children(func() {
		if ui.Button(c, "Under").Size(180, 180).Clicked() {
			m.underClicks++
		}
	})
	ui.Overlay(c, ui.OverlaySpec{
		Band: ui.LayerPaneChrome, Bounds: gfx.Rect{Width: 200, Height: 200},
	}).Children(func() {
		ui.Slot(c).Fill().Draw(func(d *ui.DrawContext) { d.Canvas.Block(d.Rect) })
	})
}

// An overlay that declares Block over its surface keeps presses from reaching
// the page: the button under its empty area is not clicked.
func TestOverlayBlockKeepsPressesFromThePage(t *testing.T) {
	m := &blockOverlayModel{}
	d := openPage(t, m.View)
	if err := d.driver.Click(90, 90); err != nil {
		t.Fatal(err)
	}
	if m.underClicks != 0 {
		t.Fatalf("a press on the overlay's blocked surface reached the page button")
	}
}
