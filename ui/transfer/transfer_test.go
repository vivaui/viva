package transfer_test

import (
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/transfer"
)

// changes counts the paints that reported a change, as a page reads Changed.
var changes int

func open(t *testing.T, tr *transfer.Transfer, items []transfer.Item) *headless.Session {
	t.Helper()
	changes = 0
	page := ui.NewPage(ui.PageConfig{View: func(c ui.C) {
		ui.Column(c).Pad(16).Children(func() {
			tr.View(c, "Groups", transfer.Spec{Items: items, Titles: [2]string{"Available", "Selected"}}).FillWidth()
			if tr.Changed() {
				changes++
			}
		})
	}})
	session, err := headless.Open(ui.Configure(page, app.Config{Title: "Transfer", Width: 640, Height: 420}), nil, headless.Options{Scale: 1})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(session.Close)
	return session
}

func sides(items []transfer.Item) (left, right []string) {
	for _, item := range items {
		if item.Target {
			right = append(right, item.Title)
		} else {
			left = append(left, item.Title)
		}
	}
	return left, right
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// The button moves the ticked items of a list to the other and unticks them; an
// item nobody ticked stays where it is.
func TestButtonMovesTheTickedItems(t *testing.T) {
	items := []transfer.Item{{Title: "alpha"}, {Title: "beta"}, {Title: "gamma"}, {Title: "delta", Target: true}}
	session := open(t, transfer.New(), items)
	if err := session.ClickLabel("alpha"); err != nil {
		t.Fatal(err)
	}
	if err := session.ClickLabel("gamma"); err != nil {
		t.Fatal(err)
	}
	if err := session.ClickLabel("Move right"); err != nil {
		t.Fatal(err)
	}
	left, right := sides(items)
	if !equal(left, []string{"beta"}) || !equal(right, []string{"alpha", "gamma", "delta"}) {
		t.Fatalf("lists = %v | %v after moving alpha and gamma right, want [beta] | [alpha gamma delta]", left, right)
	}
	if items[0].Checked || items[2].Checked {
		t.Fatal("a moved item stayed ticked")
	}
	if changes != 1 {
		t.Fatalf("Changed was reported in %d paints, want 1", changes)
	}
	if err := session.ClickLabel("Move left"); err == nil {
		t.Fatal("Move left could be pressed with nothing ticked on the right")
	}
}

// A disabled item is neither ticked by the tick for all nor moved.
func TestTickForAllSkipsDisabledItems(t *testing.T) {
	items := []transfer.Item{{Title: "alpha"}, {Title: "beta", Disabled: true}, {Title: "gamma"}}
	session := open(t, transfer.New(), items)
	if err := session.ClickLabel("Available"); err != nil {
		t.Fatal(err)
	}
	if !items[0].Checked || items[1].Checked || !items[2].Checked {
		t.Fatalf("ticks = %v %v %v after the tick for all, want true false true", items[0].Checked, items[1].Checked, items[2].Checked)
	}
	if err := session.ClickLabel("Move right"); err != nil {
		t.Fatal(err)
	}
	left, right := sides(items)
	if !equal(left, []string{"beta"}) || !equal(right, []string{"alpha", "gamma"}) {
		t.Fatalf("lists = %v | %v, want the disabled item left behind", left, right)
	}
}

// The tick for all follows the items: it is ticked once every tickable item is,
// and clears them when it is cleared.
func TestTickForAllFollowsTheItems(t *testing.T) {
	items := []transfer.Item{{Title: "alpha", Checked: true}, {Title: "beta", Checked: true}}
	session := open(t, transfer.New(), items)
	if err := session.ClickLabel("Available"); err != nil {
		t.Fatal(err)
	}
	if items[0].Checked || items[1].Checked {
		t.Fatalf("items stayed ticked after the tick for all was cleared: %v %v", items[0].Checked, items[1].Checked)
	}
}

func TestWarmPaintAllocatesNothing(t *testing.T) {
	items := []transfer.Item{{Title: "alpha"}, {Title: "beta", Checked: true}, {Title: "gamma", Target: true}}
	tr := transfer.New()
	view := func(c ui.C) { tr.View(c, "Groups", transfer.Spec{Items: items}).FillWidth() }
	theme := gfx.LightTheme()
	page := ui.NewPage(ui.PageConfig{View: view, Theme: &theme})
	canvas := &gfx.Canvas{}
	canvas.SetReduceMotion(true)
	draw := func() {
		canvas.BeginFrame()
		canvas.Reset()
		page.Draw(canvas, gfx.Rect{Width: 640, Height: 420}, view)
		canvas.EndFrame()
	}
	draw()
	if allocations := testing.AllocsPerRun(100, draw); allocations != 0 {
		t.Fatalf("warm paint allocated %v times, want 0", allocations)
	}
}

func BenchmarkViewPaint(b *testing.B) {
	items := make([]transfer.Item, 200)
	for i := range items {
		items[i] = transfer.Item{Title: "host", Target: i%3 == 0, Checked: i%5 == 0}
	}
	tr := transfer.New()
	view := func(c ui.C) { tr.View(c, "Groups", transfer.Spec{Items: items}).FillWidth() }
	theme := gfx.LightTheme()
	page := ui.NewPage(ui.PageConfig{View: view, Theme: &theme})
	canvas := &gfx.Canvas{}
	canvas.SetReduceMotion(true)
	b.ReportAllocs()
	for range b.N {
		canvas.BeginFrame()
		canvas.Reset()
		page.Draw(canvas, gfx.Rect{Width: 640, Height: 420}, view)
		canvas.EndFrame()
	}
}
