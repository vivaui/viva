package hovercard_test

import (
	"testing"
	"time"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/hovercard"
)

const (
	testOpenDelay  = 120 * time.Millisecond
	testCloseDelay = 80 * time.Millisecond
)

// pageDriver is a headless window that paints a page.
type pageDriver struct {
	t      *testing.T
	driver *app.Driver
}

func openPage(t *testing.T, view func(ui.C)) *pageDriver {
	t.Helper()
	page := ui.NewPage(ui.PageConfig{View: view})
	driver, err := app.NewDriver(ui.Configure(page, app.Config{Width: 480, Height: 480}), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(driver.Close)
	return &pageDriver{t: t, driver: driver}
}

func (d *pageDriver) hit(label string) gfx.Hit {
	d.t.Helper()
	hits := d.driver.Snapshot().Hits
	for i := len(hits) - 1; i >= 0; i-- {
		if hit := hits[i]; hit.Label == label && !hit.IsLabel && !hit.Backdrop && !hit.ID.IsZero() {
			return hit
		}
	}
	d.t.Fatalf("no hit labeled %q", label)
	return gfx.Hit{}
}

func (d *pageDriver) hasText(value string) bool {
	for _, text := range d.driver.Snapshot().Texts {
		if text.Value == value {
			return true
		}
	}
	return false
}

func (d *pageDriver) move(x, y float32) {
	d.t.Helper()
	if err := d.driver.Move(x, y); err != nil {
		d.t.Fatal(err)
	}
}

func (d *pageDriver) advance(duration time.Duration) {
	d.t.Helper()
	if err := d.driver.Advance(duration); err != nil {
		d.t.Fatal(err)
	}
}

func center(r gfx.Rect) (float32, float32) { return r.X + r.Width/2, r.Y + r.Height/2 }

// viewWithCard declares one anchored card and returns its hit's label.
func viewWithCard(delays bool, extra func(*hovercard.HoverCard)) (func(ui.C), string) {
	const bodyLabel = "Engine"
	view := func(c ui.C) {
		ui.Column(c).Fill().Center().Children(func() {
			anchor := ui.Button(c, "Profile").Outline().Width(160)
			card := &hovercard.HoverCard{
				Anchor:      anchor,
				Title:       "Ada Lovelace",
				Description: "First programmer.",
				Body: func() {
					ui.Button(c, bodyLabel)
				},
			}
			if delays {
				card.Delays(testOpenDelay, testCloseDelay)
			}
			if extra != nil {
				extra(card)
			}
			hovercard.View(c, "Profile", card)
		})
	}
	return view, bodyLabel
}

func TestCardOpensAfterTheOpenDelayAndClosesAfterLeaving(t *testing.T) {
	view, bodyLabel := viewWithCard(true, nil)
	d := openPage(t, view)
	anchor := d.hit("Profile")
	x, y := center(anchor.Bounds)

	if d.hasText("Ada Lovelace") && d.hasText("First programmer.") {
		t.Fatal("the card painted before the pointer rested on the anchor")
	}
	d.move(x, y)
	if d.hasText("First programmer.") {
		t.Fatal("the card opened on the frame the pointer arrived")
	}
	d.advance(testOpenDelay - time.Millisecond)
	if d.hasText("First programmer.") {
		t.Fatal("the card opened before the open delay elapsed")
	}
	d.advance(time.Millisecond)
	if !d.hasText("Ada Lovelace") || !d.hasText("First programmer.") || !d.hasText(bodyLabel) {
		t.Fatal("the card did not paint its page children when its delay elapsed")
	}

	d.move(470, 470)
	if !d.hasText("First programmer.") {
		t.Fatal("the card closed before the close delay elapsed")
	}
	d.advance(testCloseDelay - time.Millisecond)
	if !d.hasText("First programmer.") {
		t.Fatal("the card closed before the close delay elapsed")
	}
	d.advance(time.Millisecond)
	if d.hasText("First programmer.") {
		t.Fatal("the card stayed open after the close delay")
	}
}

func TestCardStaysOpenWhileThePointerIsOnItsContent(t *testing.T) {
	view, bodyLabel := viewWithCard(true, nil)
	d := openPage(t, view)
	x, y := center(d.hit("Profile").Bounds)
	d.move(x, y)
	d.advance(testOpenDelay)
	if !d.hasText("First programmer.") {
		t.Fatal("the card did not open")
	}

	body := d.hit(bodyLabel)
	d.move(center(body.Bounds))
	d.advance(testCloseDelay * 4)
	if !d.hasText("First programmer.") {
		t.Fatal("the card closed while the pointer was on its content")
	}

	d.move(470, 470)
	d.advance(testCloseDelay + time.Millisecond)
	if d.hasText("First programmer.") {
		t.Fatal("the card stayed open after the pointer left both the anchor and the card")
	}
}

func TestCardUsesTheCallersAnchorGeometry(t *testing.T) {
	view := func(c ui.C) {
		ui.Row(c).Fill().Pad(20).Items(ui.Center).Children(func() {
			anchor := ui.Button(c, "Profile").Outline().Width(80)
			card := &hovercard.HoverCard{
				Anchor:      anchor,
				Title:       "Ada Lovelace",
				Description: "First programmer.",
			}
			card.Side(hovercard.Right).Align(hovercard.Center).Delays(testOpenDelay, testCloseDelay)
			hovercard.View(c, "Profile", card)
		})
	}
	d := openPage(t, view)
	anchor := d.hit("Profile")
	d.move(center(anchor.Bounds))
	d.advance(testOpenDelay)

	var content gfx.Hit
	for _, hit := range d.driver.Snapshot().Hits {
		if hit.Role == gfx.RoleGroup && hit.Label == "Ada Lovelace" && !hit.ID.IsZero() {
			content = hit
		}
	}
	if content.ID.IsZero() {
		t.Fatal("the open card declared no content hit")
	}
	if content.Bounds.X < anchor.Bounds.X+anchor.Bounds.Width {
		t.Fatalf("card at %+v is not to the right of the anchor %+v", content.Bounds, anchor.Bounds)
	}
	if got, want := content.Bounds.Y+content.Bounds.Height/2, anchor.Bounds.Y+anchor.Bounds.Height/2; abs(got-want) > 1 {
		t.Fatalf("card center Y = %v, anchor center Y = %v; Align(Center) did not center it", got, want)
	}
	if !d.hasText("Ada Lovelace") {
		t.Fatal("the card did not paint its title")
	}
}

func TestOpenStateIsWrittenToTheCallersValue(t *testing.T) {
	open := false
	view, _ := viewWithCard(true, func(card *hovercard.HoverCard) {
		card.Open = &open
	})
	d := openPage(t, view)
	x, y := center(d.hit("Profile").Bounds)
	d.move(x, y)
	d.advance(testOpenDelay)
	if !open {
		t.Fatal("the card opened without writing the caller's open state")
	}
	d.move(470, 470)
	d.advance(testCloseDelay + time.Millisecond)
	if open {
		t.Fatal("the card closed without clearing the caller's open state")
	}
}

func TestOnOpenChangeReportsHoverTransitions(t *testing.T) {
	var changes []bool
	change := func(open bool) { changes = append(changes, open) }
	view, _ := viewWithCard(true, func(card *hovercard.HoverCard) {
		card.OnOpenChange = change
	})
	d := openPage(t, view)
	x, y := center(d.hit("Profile").Bounds)
	d.move(x, y)
	d.advance(testOpenDelay)
	d.move(470, 470)
	d.advance(testCloseDelay + time.Millisecond)
	if len(changes) != 2 || !changes[0] || changes[1] {
		t.Fatalf("OnOpenChange calls = %v, want [true false]", changes)
	}
}

func TestDisabledCardPaintsNothing(t *testing.T) {
	view, _ := viewWithCard(true, func(card *hovercard.HoverCard) {
		card.Disabled()
	})
	d := openPage(t, view)
	x, y := center(d.hit("Profile").Bounds)
	d.move(x, y)
	d.advance(testOpenDelay * 4)
	if d.hasText("First programmer.") {
		t.Fatal("a disabled card painted")
	}
}

func BenchmarkViewWarmPaint(b *testing.B) {
	open := true
	card := &hovercard.HoverCard{
		Title:       "Ada Lovelace",
		Description: "First programmer.",
		Open:        &open,
		Body:        func() {},
	}
	card.Delays(testOpenDelay, testCloseDelay)
	view := func(c ui.C) {
		card.Anchor = ui.Button(c, "Ada Lovelace").Outline().Width(160)
		hovercard.View(c, "Ada Lovelace", card)
	}
	page := ui.NewPage(ui.PageConfig{View: view})
	canvas := new(gfx.Canvas)
	canvas.SetMeasurer(func(value string, size, weight float32) float32 { return float32(len(value)) * size / 2 })
	paint(page, canvas, view)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		paint(page, canvas, view)
	}
}

func paint(page *ui.Page, canvas *gfx.Canvas, view func(ui.C)) {
	canvas.Reset()
	canvas.BeginFrame()
	page.Draw(canvas, gfx.Rect{Width: 480, Height: 480}, view)
	canvas.EndFrame()
}

func abs(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}
