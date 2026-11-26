package autocomplete_test

import (
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/autocomplete"
)

var hosts = []string{"build-01", "build-02", "db-primary", "staging"}

type model struct {
	a      *autocomplete.Autocomplete
	field  ui.Field
	chosen []string
	enters int
}

func (m *model) page(c ui.C) {
	ui.Column(c).Fill().Pad(16).Children(func() {
		m.a.View(c, &m.field, "Host", "", autocomplete.Spec{Options: hosts}).FillWidth()
		if v, ok := m.a.Chosen(); ok {
			m.chosen = append(m.chosen, v)
		}
	})
}

type driver struct {
	t *testing.T
	*app.Driver
}

func open(t *testing.T) (*model, driver) {
	t.Helper()
	m := &model{a: autocomplete.New()}
	page := ui.NewPage(ui.PageConfig{View: m.page})
	d, err := app.NewDriver(ui.Configure(page, app.Config{Width: 400, Height: 320, Invalidator: new(app.Invalidator)}), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(d.Close)
	dr := driver{t, d}
	dr.click("Host")
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

func TestTypingListsTheOptionsThatContainIt(t *testing.T) {
	_, d := open(t)
	d.typ("BUILD")
	if !d.listed("build-01") || !d.listed("build-02") || d.listed("staging") {
		t.Fatal("the list does not follow what was typed")
	}
}

func TestAnOptionEqualToTheTextIsNotOffered(t *testing.T) {
	_, d := open(t)
	d.typ("staging")
	if d.listed("staging") {
		t.Fatal("the option the text already is was offered")
	}
}

func TestEnterWithoutAHighlightIsLeftToThePage(t *testing.T) {
	m, d := open(t)
	d.typ("bui")
	d.key(gfx.KeyEnter)
	if len(m.chosen) != 0 || m.field.GetEditor().String() != "bui" {
		t.Fatalf("Enter chose something nobody highlighted: %v %q", m.chosen, m.field.GetEditor().String())
	}
}

func TestArrowsAndEnterChoose(t *testing.T) {
	m, d := open(t)
	d.typ("build")
	d.key(gfx.KeyDown)
	d.key(gfx.KeyDown)
	d.key(gfx.KeyUp)
	d.key(gfx.KeyEnter)
	if got := m.field.GetEditor().String(); got != "build-01" {
		t.Fatalf("field = %q, want build-01", got)
	}
	if len(m.chosen) != 1 || m.chosen[0] != "build-01" {
		t.Fatalf("chosen %v", m.chosen)
	}
}

func TestUpFromNothingGoesToTheLastOption(t *testing.T) {
	m, d := open(t)
	d.typ("build")
	d.key(gfx.KeyUp)
	d.key(gfx.KeyEnter)
	if got := m.field.GetEditor().String(); got != "build-02" {
		t.Fatalf("field = %q, want build-02", got)
	}
}

func TestPressingAnOptionChoosesIt(t *testing.T) {
	m, d := open(t)
	d.typ("db")
	d.click("db-primary")
	if got := m.field.GetEditor().String(); got != "db-primary" {
		t.Fatalf("field = %q", got)
	}
}

func TestFreeTextIsKept(t *testing.T) {
	m, d := open(t)
	d.typ("elsewhere")
	d.key(gfx.KeyEnter)
	if got := m.field.GetEditor().String(); got != "elsewhere" {
		t.Fatalf("field = %q", got)
	}
}

func TestEscapeClosesUntilTheTextChanges(t *testing.T) {
	_, d := open(t)
	d.typ("bu")
	d.key(gfx.KeyEscape)
	if d.listed("build-01") {
		t.Fatal("Escape left the list open")
	}
	d.typ("i")
	if !d.listed("build-01") {
		t.Fatal("typing did not bring the list back")
	}
}
