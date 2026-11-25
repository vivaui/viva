package ui_test

import (
	"testing"
	"time"

	"github.com/vivaui/viva/ui"
)

type toastModel struct {
	toaster ui.Toaster
	saves   int
	undone  int
	undo    ui.ToastID
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
	if id, ok := m.toaster.Acted(); ok && id == m.undo {
		m.undone++
	}
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

// A toast's action is reported once and takes the toast down; one in progress stays
// until Done replaces it, whose time then runs.
func TestToastActionAndLoading(t *testing.T) {
	m := &toastModel{}
	d := openPage(t, m.page)
	m.undo = m.toaster.ShowAction("Host deleted", "", ui.ToastInfo, "Undo")
	d.repaint()
	d.click("Undo")
	if m.undone != 1 || m.toaster.Len() != 0 {
		t.Fatalf("undone %d times, %d toasts up", m.undone, m.toaster.Len())
	}
	d.repaint()
	if m.undone != 1 {
		t.Fatal("the action is reported again on the next paint")
	}

	if err := d.driver.Move(1, 1); err != nil { // off the stack, which holds its time
		t.Fatal(err)
	}
	id := m.toaster.Show("Uploading", "", ui.ToastLoading)
	d.repaint()
	if err := d.driver.Advance(20 * time.Second); err != nil {
		t.Fatal(err)
	}
	if !tooltipShown(d, "Uploading") {
		t.Fatal("a toast in progress went by itself")
	}
	m.toaster.Done(id, "Uploaded", "", ui.ToastSuccess)
	d.repaint()
	if !tooltipShown(d, "Uploaded") || tooltipShown(d, "Uploading") {
		t.Fatal("Done did not replace the toast")
	}
	for range 8 {
		if err := d.driver.Advance(time.Second); err != nil {
			t.Fatal(err)
		}
	}
	if m.toaster.Len() != 0 {
		t.Fatal("the finished toast did not go by itself")
	}
}

// A toast can hide its close button and set its own lifetime; the close button
// is there by default and the Toaster's Timeout is the default lifetime.
func TestToastConfigHidesItsCloseButtonAndSetsItsOwnTime(t *testing.T) {
	m := &toastModel{}
	d := openPage(t, m.page)
	m.toaster.ShowConfig("Quiet", "", ui.ToastInfo, ui.ToastConfig{HideClose: true, Timeout: 2 * time.Second})
	d.repaint()
	if d.has("Close toast") {
		t.Fatal("a toast that hides its close button has one")
	}
	if !tooltipShown(d, "Quiet") {
		t.Fatal("the toast is not shown")
	}
	if err := d.driver.Advance(1500 * time.Millisecond); err != nil {
		t.Fatal(err)
	}
	if !tooltipShown(d, "Quiet") {
		t.Fatal("the toast went before its own time")
	}
	if err := d.driver.Advance(600 * time.Millisecond); err != nil {
		t.Fatal(err)
	}
	if err := d.driver.Advance(time.Second); err != nil { // its exit
		t.Fatal(err)
	}
	if tooltipShown(d, "Quiet") || m.toaster.Len() != 0 {
		t.Fatal("the toast is still up after its own time")
	}
}

// The Toaster's Timeout is the lifetime of a toast that does not name one.
func TestToasterTimeoutSetsTheDefaultLifetime(t *testing.T) {
	m := &toastModel{}
	m.toaster.Timeout = 2 * time.Second
	d := openPage(t, m.page)
	m.toaster.Show("Quick", "", ui.ToastInfo)
	d.repaint()
	if err := d.driver.Advance(1500 * time.Millisecond); err != nil {
		t.Fatal(err)
	}
	if !tooltipShown(d, "Quick") {
		t.Fatal("the toast went before the Toaster's time")
	}
	if err := d.driver.Advance(600 * time.Millisecond); err != nil {
		t.Fatal(err)
	}
	if err := d.driver.Advance(time.Second); err != nil {
		t.Fatal(err)
	}
	if tooltipShown(d, "Quick") || m.toaster.Len() != 0 {
		t.Fatal("the toast is still up after the Toaster's time")
	}
}

// The Toaster's Limit is how many toasts it keeps up.
func TestToasterLimitSetsTheVisibleCount(t *testing.T) {
	m := &toastModel{}
	m.toaster.Limit = 1
	d := openPage(t, m.page)
	m.toaster.Show("a", "", ui.ToastInfo)
	d.repaint()
	m.toaster.Show("b", "", ui.ToastInfo)
	d.repaint()
	if m.toaster.Len() != 1 || !tooltipShown(d, "b") || tooltipShown(d, "a") {
		t.Fatalf("%d toasts, a shown %v, b shown %v", m.toaster.Len(), tooltipShown(d, "a"), tooltipShown(d, "b"))
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
