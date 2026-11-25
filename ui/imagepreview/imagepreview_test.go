package imagepreview_test

import (
	"strconv"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/imagepreview"
)

func bitmap() gfx.Bitmap {
	const w, h = 400, 300
	return gfx.Bitmap{Pix: make([]byte, w*h*4), Width: w, Height: h, Stride: w * 4, ID: 77, Version: 1}
}

type model struct {
	p   *imagepreview.Preview
	bmp gfx.Bitmap
}

func (m *model) page(c ui.C) {
	ui.Column(c).Fill().Pad(16).Children(func() {
		ui.Text(c, "page")
		m.p.View(c, imagepreview.Spec{Bitmap: m.bmp, Title: "Shot", Width: 400, Height: 240})
	})
}

type driver struct {
	t *testing.T
	*app.Driver
	wake *app.Invalidator
}

// repaint paints the page again, as the window does when the model changed.
func (d driver) repaint() {
	d.t.Helper()
	d.wake.Invalidate()
	if err := d.Pump(); err != nil {
		d.t.Fatal(err)
	}
}

func open(t *testing.T, m *model) driver {
	t.Helper()
	m.p, m.bmp = imagepreview.New(), bitmap()
	page := ui.NewPage(ui.PageConfig{View: m.page})
	wake := new(app.Invalidator)
	d, err := app.NewDriver(ui.Configure(page, app.Config{Width: 640, Height: 560, Invalidator: wake}), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(d.Close)
	return driver{t, d, wake}
}

func (d driver) has(text string) bool {
	for _, v := range d.Snapshot().Texts {
		if v.Value == text {
			return true
		}
	}
	return false
}

func (d driver) click(label string) {
	d.t.Helper()
	hits := d.Snapshot().Hits
	for i := len(hits) - 1; i >= 0; i-- {
		if h := hits[i]; h.Label == label && !h.IsLabel && !h.ID.IsZero() {
			if err := d.Click(h.Bounds.X+h.Bounds.Width/2, h.Bounds.Y+h.Bounds.Height/2); err != nil {
				d.t.Fatal(err)
			}
			return
		}
	}
	d.t.Fatalf("no control labeled %q", label)
}

func TestClosedPreviewDeclaresNothing(t *testing.T) {
	d := open(t, &model{})
	for _, h := range d.Snapshot().Hits {
		if h.Label == "Zoom in" {
			t.Fatal("a closed preview shows its buttons")
		}
	}
}

// percent is the zoom the dialog shows, as a number.
func (d driver) percent() int {
	d.t.Helper()
	for _, v := range d.Snapshot().Texts {
		if n := len(v.Value); n > 1 && v.Value[n-1] == '%' {
			p, err := strconv.Atoi(v.Value[:n-1])
			if err == nil {
				return p
			}
		}
	}
	d.t.Fatal("the dialog shows no zoom")
	return 0
}

func TestZoomButtonsChangeAndRestoreTheZoom(t *testing.T) {
	m := &model{}
	d := open(t, m)
	m.p.Open()
	d.repaint()
	// The fitted zoom depends on the display's scale (a percentage of device
	// pixels), so the test compares the zoom with itself.
	fitted := d.percent()
	d.click("Zoom in")
	in := d.percent()
	if in <= fitted {
		t.Fatalf("zooming in went from %d%% to %d%%", fitted, in)
	}
	d.click("Zoom out")
	if got := d.percent(); got != fitted {
		t.Fatalf("zooming in and out ended at %d%%, want %d%%", got, fitted)
	}
	d.click("Zoom in")
	d.click("Actual size")
	if got := d.percent(); got != 100 {
		t.Fatalf("Actual size is %d%%, want 100%%", got)
	}
	d.click("Fit")
	if got := d.percent(); got != fitted {
		t.Fatalf("Fit is %d%%, want %d%%", got, fitted)
	}
}

func TestEscapeClosesThePreview(t *testing.T) {
	m := &model{}
	d := open(t, m)
	m.p.Open()
	d.repaint()
	if err := d.Key(gfx.KeyEscape, 0); err != nil {
		t.Fatal(err)
	}
	if m.p.IsOpen() {
		t.Fatal("Escape left the preview open")
	}
}

func TestWarmPaintDoesNotAllocate(t *testing.T) {
	m := &model{p: imagepreview.New(), bmp: bitmap()}
	m.p.Open()
	theme := gfx.LightTheme()
	page := ui.NewPage(ui.PageConfig{View: m.page, Theme: &theme})
	canvas := &gfx.Canvas{}
	canvas.SetReduceMotion(true)
	draw := func() {
		canvas.BeginFrame()
		canvas.Reset()
		page.Draw(canvas, gfx.Rect{Width: 640, Height: 560}, m.page)
		canvas.EndFrame()
	}
	draw()
	draw()
	if n := testing.AllocsPerRun(100, draw); n != 0 {
		t.Fatalf("a warm paint allocates %v times", n)
	}
}
