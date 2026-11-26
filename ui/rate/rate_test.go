//go:build !js

package rate_test

import (
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/rate"
)

// changes counts the paints that reported a change, as a page reads Changed.
var changes int

// open runs a page holding one rating over *score, headlessly.
func open(t *testing.T, r *rate.Rate, score *float32) *headless.Session {
	t.Helper()
	page := ui.NewPage(ui.PageConfig{View: func(c ui.C) {
		ui.Column(c).Pad(16).Children(func() {
			r.View(c, "Rating", score)
			if r.Changed() {
				changes++
			}
		})
	}})
	session, err := headless.Open(ui.Configure(page, app.Config{Title: "Rate", Width: 320, Height: 120}), nil, headless.Options{Scale: 1})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(session.Close)
	return session
}

// star is the center of the left or right half of star n, as the page drew it.
func star(t *testing.T, session *headless.Session, n string, right bool) (x, y float32) {
	t.Helper()
	hit, err := session.Locate(n)
	if err != nil {
		t.Fatal(err)
	}
	b := hit.Bounds
	x = b.X + b.Width/4
	if right {
		x = b.X + b.Width*3/4
	}
	return x, b.Y + b.Height/2
}

// A click on a star chooses it, a click on the chosen one clears it, and the
// page learns of each change once.
func TestClickChoosesAndClears(t *testing.T) {
	var score float32
	changes = 0
	session := open(t, rate.New(), &score)
	x, y := star(t, session, "4", true)
	if err := session.Click(x, y); err != nil {
		t.Fatal(err)
	}
	if score != 4 {
		t.Fatalf("score = %v after a click on star 4, want 4", score)
	}
	if changes != 1 {
		t.Fatalf("Changed was reported in %d paints after one choice, want 1", changes)
	}
	if err := session.Click(x, y); err != nil {
		t.Fatal(err)
	}
	if score != 0 {
		t.Fatalf("score = %v after a second click on the chosen star, want it cleared", score)
	}
}

func TestKeepScoreKeepsTheChosenOne(t *testing.T) {
	score := float32(4)
	session := open(t, rate.New().KeepScore(), &score)
	x, y := star(t, session, "4", true)
	if err := session.Click(x, y); err != nil {
		t.Fatal(err)
	}
	if score != 4 {
		t.Fatalf("score = %v, want the chosen score kept", score)
	}
}

// With AllowHalf the left half of a star chooses the half, the right half the whole.
func TestAllowHalfChoosesHalfStars(t *testing.T) {
	var score float32
	session := open(t, rate.New().AllowHalf(), &score)
	x, y := star(t, session, "2", false)
	if err := session.Click(x, y); err != nil {
		t.Fatal(err)
	}
	if score != 1.5 {
		t.Fatalf("score = %v after a click on the left half of star 2, want 1.5", score)
	}
	x, y = star(t, session, "2", true)
	if err := session.Click(x, y); err != nil {
		t.Fatal(err)
	}
	if score != 2 {
		t.Fatalf("score = %v after a click on the right half of star 2, want 2", score)
	}
}

// The stars are one tab stop and the arrow keys change the score from there.
func TestKeysChangeTheScore(t *testing.T) {
	score := float32(2)
	session := open(t, rate.New(), &score)
	for _, step := range []struct {
		key  string
		want float32
	}{{"tab", 2}, {"right", 3}, {"up", 4}, {"left", 3}, {"down", 2}, {"end", 5}, {"right", 5}, {"home", 0}, {"left", 0}} {
		if err := session.Keys(step.key); err != nil {
			t.Fatal(err)
		}
		if score != step.want {
			t.Fatalf("score = %v after %q, want %v", score, step.key, step.want)
		}
	}
}

// A read only rating shows the score and answers to nothing.
func TestReadOnlyTakesNoInput(t *testing.T) {
	score := float32(3)
	session := open(t, rate.New().ReadOnly(), &score)
	if _, err := session.Locate("5"); err == nil {
		t.Fatal("a read only rating declares stars to click")
	}
	if err := session.Keys("tab"); err != nil {
		t.Fatal(err)
	}
	if err := session.Keys("right"); err != nil {
		t.Fatal(err)
	}
	if score != 3 {
		t.Fatalf("score = %v after a key on a read only rating, want 3", score)
	}
}

// A score the rating cannot show is brought into range: a whole number of
// stars, or of halves, from none to all.
func TestScoreIsKeptInRange(t *testing.T) {
	for _, test := range []struct {
		rate  *rate.Rate
		score float32
		want  float32
	}{
		{rate.New(), 9, 5},
		{rate.New(), -2, 0},
		{rate.New(), 2.6, 3},
		{rate.New().AllowHalf(), 2.6, 2.5},
		{rate.New().Count(3), 4, 3},
	} {
		score := test.score
		open(t, test.rate, &score)
		if score != test.want {
			t.Fatalf("score %v became %v, want %v", test.score, score, test.want)
		}
	}
}

func TestWarmPaintAllocatesNothing(t *testing.T) {
	score := float32(3.5)
	r := rate.New().AllowHalf()
	view := func(c ui.C) { r.View(c, "Rating", &score) }
	theme := gfx.LightTheme()
	page := ui.NewPage(ui.PageConfig{View: view, Theme: &theme})
	canvas := &gfx.Canvas{}
	canvas.SetReduceMotion(true)
	draw := func() {
		canvas.BeginFrame()
		canvas.Reset()
		page.Draw(canvas, gfx.Rect{Width: 320, Height: 120}, view)
		canvas.EndFrame()
	}
	draw()
	if allocations := testing.AllocsPerRun(100, draw); allocations != 0 {
		t.Fatalf("warm paint allocated %v times, want 0", allocations)
	}
}

func BenchmarkViewPaint(b *testing.B) {
	score := float32(3.5)
	r := rate.New().AllowHalf()
	view := func(c ui.C) { r.View(c, "Rating", &score) }
	theme := gfx.LightTheme()
	page := ui.NewPage(ui.PageConfig{View: view, Theme: &theme})
	canvas := &gfx.Canvas{}
	canvas.SetReduceMotion(true)
	b.ReportAllocs()
	for range b.N {
		canvas.BeginFrame()
		canvas.Reset()
		page.Draw(canvas, gfx.Rect{Width: 320, Height: 120}, view)
		canvas.EndFrame()
	}
}
