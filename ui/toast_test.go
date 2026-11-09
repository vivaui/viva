package ui_test

import (
	"testing"
	"time"

	"github.com/vivaui/viva/ui"
)

type toastModel struct {
	toaster ui.Toaster
	saves   int
}

func (m *toastModel) page(c ui.C) {
	ui.Column(c).Pad(16).Children(func() {
		if ui.Button(c, "Save").Clicked() {
			m.saves++
			m.toaster.Show("Saved", "", ui.ToastSuccess)
		}
		ui.Button(c, "Other")
	})
	ui.Toasts(c, &m.toaster)
}

// A toast is up once shown and goes away when its time is up, and the window does
// no work once it is gone.
func TestToastExpires(t *testing.T) {
	m := &toastModel{}
	d := openPage(t, m.page)
	d.click("Save")
	if !tooltipShown(d, "Saved") {
		t.Fatal("the toast is not shown")
	}
	if err := d.driver.Advance(4 * time.Second); err != nil {
		t.Fatal(err)
	}
	if !tooltipShown(d, "Saved") {
		t.Fatal("the toast went before its time")
	}
	if err := d.driver.Advance(2 * time.Second); err != nil {
		t.Fatal(err)
	}
	if err := d.driver.Advance(time.Second); err != nil { // its exit
		t.Fatal(err)
	}
	if tooltipShown(d, "Saved") || m.toaster.Len() != 0 {
		t.Fatalf("the toast is still up after its time: %d", m.toaster.Len())
	}
	if d.driver.Transitioning() {
		t.Fatal("frames are still scheduled with no toast up")
	}
}

// The close button takes a toast down; the pointer on it holds its time.
func TestToastCloseAndHold(t *testing.T) {
	m := &toastModel{}
	d := openPage(t, m.page)
	d.click("Save")
	if err := d.driver.Move(center(d.hit("Close toast").Bounds)); err != nil {
		t.Fatal(err)
	}
	if err := d.driver.Advance(8 * time.Second); err != nil {
		t.Fatal(err)
	}
	if m.toaster.Len() != 1 {
		t.Fatal("the toast went while the pointer rested on it")
	}
	d.click("Close toast")
	if m.toaster.Len() != 0 {
		t.Fatal("the close button did not take the toast down")
	}
}

// Showing the same toast again restarts its time instead of stacking a copy, and
// no more than three are up.
func TestToastDedupAndLimit(t *testing.T) {
	m := &toastModel{}
	d := openPage(t, m.page)
	d.click("Save")
	d.click("Save")
	if m.toaster.Len() != 1 {
		t.Fatalf("the same toast twice is %d toasts", m.toaster.Len())
	}
	for _, title := range []string{"a", "b", "c"} {
		m.toaster.Show(title, "", ui.ToastInfo)
		d.repaint()
	}
	if m.toaster.Len() != 3 || tooltipShown(d, "Saved") {
		t.Fatalf("%d toasts, the oldest shown %v", m.toaster.Len(), tooltipShown(d, "Saved"))
	}
}

func BenchmarkToasts(b *testing.B) {
	m := &toastModel{}
	m.toaster.Show("Saved", "The host list was written", ui.ToastSuccess)
	page, canvas := ui.NewPage(ui.PageConfig{Theme: &pageTheme}), benchCanvas()
	canvas.SetClock(func() time.Duration { return time.Second }) // inside its five seconds
	b.ReportAllocs()
	for b.Loop() {
		drawPage(page, canvas, m.page)
	}
}
