package comment_test

import (
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/avatar"
	"github.com/vivaui/viva/ui/comment"
)

type model struct {
	avatars [2]*avatar.State
	replied int
	view    func(ui.C)
	actions [2]func()
	reply   func()
	top     func(ui.C)
}

func newModel() *model {
	m := &model{}
	for i := range m.avatars {
		m.avatars[i] = avatar.New().Initials("AB")
	}
	var c ui.C
	m.actions[0] = func() {
		if ui.Button(c, "Reply").Ghost().Small().Clicked() {
			m.replied++
		}
	}
	m.reply = func() {
		comment.View(c, comment.Spec{Avatar: m.avatars[1], Author: "Grace", Time: "1h", Content: "Agreed.", Actions: m.actions[1]}).FillWidth()
	}
	m.actions[1] = func() {}
	m.view = func(cc ui.C) {
		c = cc
		ui.Column(c).Fill().Pad(16).Children(func() {
			comment.View(c, comment.Spec{Avatar: m.avatars[0], Author: "Ada", Time: "2h ago", Content: "A longer comment that wraps over more than one line when the column is narrow enough.", Actions: m.actions[0], Replies: m.reply}).FillWidth()
		})
	}
	return m
}

func TestActionsAndRepliesAreDeclared(t *testing.T) {
	m := newModel()
	page := ui.NewPage(ui.PageConfig{View: m.view})
	d, err := app.NewDriver(ui.Configure(page, app.Config{Width: 320, Height: 320, Invalidator: new(app.Invalidator)}), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	seen := map[string]bool{}
	for _, text := range d.Snapshot().Texts {
		seen[text.Value] = true
	}
	for _, want := range []string{"Ada", "2h ago", "Grace", "Agreed."} {
		if !seen[want] {
			t.Errorf("%q is not painted", want)
		}
	}
	for _, h := range d.Snapshot().Hits {
		if h.Label == "Reply" && !h.IsLabel {
			if err := d.Click(h.Bounds.X+h.Bounds.Width/2, h.Bounds.Y+h.Bounds.Height/2); err != nil {
				t.Fatal(err)
			}
			break
		}
	}
	if m.replied != 1 {
		t.Fatalf("the action was pressed %d times, want 1", m.replied)
	}
}

func TestWarmPaintDoesNotAllocate(t *testing.T) {
	m := newModel()
	theme := gfx.LightTheme()
	page := ui.NewPage(ui.PageConfig{View: m.view, Theme: &theme})
	canvas := &gfx.Canvas{}
	canvas.SetReduceMotion(true)
	draw := func() {
		canvas.BeginFrame()
		canvas.Reset()
		page.Draw(canvas, gfx.Rect{Width: 320, Height: 320}, m.view)
		canvas.EndFrame()
	}
	draw()
	if n := testing.AllocsPerRun(100, draw); n != 0 {
		t.Fatalf("a warm paint allocates %v times", n)
	}
}
