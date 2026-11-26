package mentions_test

import (
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/mentions"
)

var people = []mentions.Option{{Label: "Ada", Detail: "Engineer"}, {Label: "Grace", Detail: "Admiral"}, {Label: "Alan"}}

type model struct {
	m       *mentions.Mentions
	field   ui.Field
	chosen  []string
	options []mentions.Option
}

func (m *model) page(c ui.C) {
	ui.Column(c).Fill().Pad(16).Children(func() {
		m.m.View(c, &m.field, "Message", "", mentions.Spec{Options: m.options, Empty: "No one"}).FillWidth()
		if o, ok := m.m.Chosen(); ok {
			m.chosen = append(m.chosen, o.Label)
		}
	})
}

type driver struct {
	t *testing.T
	*app.Driver
}

func open(t *testing.T) (*model, driver) {
	t.Helper()
	m := &model{m: mentions.New(), options: people}
	page := ui.NewPage(ui.PageConfig{View: m.page})
	d, err := app.NewDriver(ui.Configure(page, app.Config{Width: 400, Height: 320, Invalidator: new(app.Invalidator)}), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(d.Close)
	dr := driver{t, d}
	dr.click("Message")
	return m, dr
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

func (d driver) listed(label string) bool {
	for _, h := range d.Snapshot().Hits {
		if h.Label == label && h.Role == gfx.RoleOption {
			return true
		}
	}
	return false
}

func (d driver) typeText(s string) {
	d.t.Helper()
	if err := d.Type(s); err != nil {
		d.t.Fatal(err)
	}
}

func TestTypingTheTriggerOpensTheMatchingOptions(t *testing.T) {
	_, d := open(t)
	d.typeText("hi @gr")
	if !d.listed("Grace") || d.listed("Ada") || d.listed("Alan") {
		t.Fatalf("list after @gr: Grace %v Ada %v Alan %v", d.listed("Grace"), d.listed("Ada"), d.listed("Alan"))
	}
}

func TestATriggerInsideAWordOpensNothing(t *testing.T) {
	_, d := open(t)
	d.typeText("mail me at x@gr")
	if d.listed("Grace") {
		t.Fatal("an @ inside a word opened the list")
	}
}

func TestEnterChoosesTheHighlightedOption(t *testing.T) {
	m, d := open(t)
	d.typeText("hi @a")
	if err := d.Key(gfx.KeyDown, 0); err != nil {
		t.Fatal(err)
	}
	if err := d.Key(gfx.KeyEnter, 0); err != nil {
		t.Fatal(err)
	}
	if got := m.field.GetEditor().String(); got != "hi @Grace " {
		t.Fatalf("field = %q, want %q", got, "hi @Grace ")
	}
	if len(m.chosen) != 1 || m.chosen[0] != "Grace" {
		t.Fatalf("chosen %v", m.chosen)
	}
	if d.listed("Ada") {
		t.Fatal("the list stayed open after a choice")
	}
}

func TestPressingAnOptionChoosesItAndKeepsTheField(t *testing.T) {
	m, d := open(t)
	d.typeText("@")
	d.click("Alan")
	if got := m.field.GetEditor().String(); got != "@Alan " {
		t.Fatalf("field = %q", got)
	}
}

func TestEscapeClosesTheListUntilTheWordChanges(t *testing.T) {
	_, d := open(t)
	d.typeText("@a")
	if err := d.Key(gfx.KeyEscape, 0); err != nil {
		t.Fatal(err)
	}
	if d.listed("Ada") {
		t.Fatal("Escape left the list open")
	}
	d.typeText("d")
	if d.listed("Ada") {
		t.Fatal("the list came back while the same word was being typed")
	}
	d.typeText(" @a")
	if !d.listed("Ada") {
		t.Fatal("a new word did not open the list")
	}
}

func TestClosedFieldPaintsWithoutAllocating(t *testing.T) {
	m := &model{m: mentions.New(), options: people}
	theme := gfx.LightTheme()
	page := ui.NewPage(ui.PageConfig{View: m.page, Theme: &theme})
	canvas := &gfx.Canvas{}
	canvas.SetReduceMotion(true)
	draw := func() {
		canvas.BeginFrame()
		canvas.Reset()
		page.Draw(canvas, gfx.Rect{Width: 400, Height: 320}, m.page)
		canvas.EndFrame()
	}
	draw()
	draw()
	if n := testing.AllocsPerRun(100, draw); n != 0 {
		t.Fatalf("a warm paint allocates %v times", n)
	}
}
