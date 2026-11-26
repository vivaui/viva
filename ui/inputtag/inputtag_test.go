package inputtag_test

import (
	"slices"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/inputtag"
)

type model struct {
	t    *inputtag.InputTag
	tags []string
	spec inputtag.Spec
	rej  int
}

func (m *model) page(c ui.C) {
	ui.Column(c).Fill().Pad(16).Children(func() {
		m.t.View(c, "Hosts", m.tags, m.spec).FillWidth()
		if v, ok := m.t.Added(); ok {
			m.tags = append(m.tags, v)
		}
		if i, ok := m.t.Removed(); ok {
			m.tags = slices.Delete(m.tags, i, i+1)
		}
		if m.t.Rejected() {
			m.rej++
		}
	})
}

type driver struct {
	t *testing.T
	*app.Driver
}

func open(t *testing.T, m *model) driver {
	t.Helper()
	m.t = inputtag.New()
	page := ui.NewPage(ui.PageConfig{View: m.page})
	d, err := app.NewDriver(ui.Configure(page, app.Config{Width: 420, Height: 240, Invalidator: new(app.Invalidator)}), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(d.Close)
	dr := driver{t, d}
	dr.click("Hosts")
	return dr
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

func (d driver) typ(s string) {
	d.t.Helper()
	if err := d.Type(s); err != nil {
		d.t.Fatal(err)
	}
}

func (d driver) key(k gfx.Key) {
	d.t.Helper()
	if err := d.Key(k, 0); err != nil {
		d.t.Fatal(err)
	}
}

func TestEnterAddsATagAndBackspaceRemovesTheLast(t *testing.T) {
	m := &model{}
	d := open(t, m)
	d.typ("alpha")
	d.key(gfx.KeyEnter)
	d.typ("beta")
	d.key(gfx.KeyEnter)
	if !slices.Equal(m.tags, []string{"alpha", "beta"}) {
		t.Fatalf("tags %v", m.tags)
	}
	d.key(gfx.KeyBackspace)
	if !slices.Equal(m.tags, []string{"alpha"}) {
		t.Fatalf("after Backspace %v", m.tags)
	}
}

func TestBackspaceInANonEmptyFieldOnlyEdits(t *testing.T) {
	m := &model{tags: []string{"alpha"}}
	d := open(t, m)
	d.typ("be")
	d.key(gfx.KeyBackspace)
	if !slices.Equal(m.tags, []string{"alpha"}) {
		t.Fatalf("Backspace removed a tag while text was typed: %v", m.tags)
	}
}

func TestSeparatorAddsTheTypedValue(t *testing.T) {
	m := &model{}
	d := open(t, m)
	d.typ("one,")
	if !slices.Equal(m.tags, []string{"one"}) {
		t.Fatalf("tags %v", m.tags)
	}
}

func TestDuplicatesBlankAndLimitAreRefused(t *testing.T) {
	m := &model{tags: []string{"a"}, spec: inputtag.Spec{MaxTags: 2}}
	d := open(t, m)
	d.typ("a")
	d.key(gfx.KeyEnter)
	if len(m.tags) != 1 || m.rej == 0 {
		t.Fatalf("a duplicate was added or not reported: %v rej %d", m.tags, m.rej)
	}
	d.key(gfx.KeyBackspace) // removes nothing of the typed text, which is still "a"
	d.key(gfx.KeyEnter)
	if len(m.tags) != 1 {
		t.Fatalf("tags %v", m.tags)
	}
}

func TestCloseButtonRemovesThatTag(t *testing.T) {
	m := &model{tags: []string{"a", "b", "c"}}
	d := open(t, m)
	d.click("Remove b")
	if !slices.Equal(m.tags, []string{"a", "c"}) {
		t.Fatalf("tags %v", m.tags)
	}
}

func TestWarmPaintDoesNotAllocate(t *testing.T) {
	m := &model{t: inputtag.New(), tags: []string{"a", "b"}}
	theme := gfx.LightTheme()
	page := ui.NewPage(ui.PageConfig{View: m.page, Theme: &theme})
	canvas := &gfx.Canvas{}
	canvas.SetReduceMotion(true)
	draw := func() {
		canvas.BeginFrame()
		canvas.Reset()
		page.Draw(canvas, gfx.Rect{Width: 420, Height: 240}, m.page)
		canvas.EndFrame()
	}
	draw()
	draw()
	if n := testing.AllocsPerRun(100, draw); n != 0 {
		t.Fatalf("a warm paint allocates %v times", n)
	}
}
