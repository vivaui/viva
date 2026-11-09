package ui_test

import (
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

type basesModel struct {
	clicks         int
	on, toggled, s bool
	radio, tab     string
	slider         ui.SliderState
	size           string
	text           string
	changed        int
	paintFn        func(*ui.DrawContext)
}

func (m *basesModel) page(c ui.C) {
	ui.Column(c).Pad(16).Gap(8).Children(func() {
		if ui.ButtonBase(c, "Base button").Pad(8).Children(func() { ui.Text(c, "Press") }).Clicked() {
			m.clicks++
		}
		if ui.CheckboxBase(c, &m.on, "Base check").Changed() {
			m.changed++
		}
		ui.SwitchBase(c, &m.s, "Base switch")
		ui.ToggleBase(c, &m.toggled, "Base toggle").Pad(4).Children(func() { ui.Text(c, "Bold") })
		ui.Row(c).Gap(4).FocusGroup(ui.FocusHorizontal).Children(func() {
			for _, name := range []string{"one", "two", "three"} {
				ui.RadioBase(c.Key(name), &m.radio, name, radioLabels[name]).Pad(6).Children(func() { ui.Text(c, name) })
			}
		})
		ui.Row(c).Gap(4).Children(func() {
			for _, name := range []string{"general", "about"} {
				ui.TabBase(c.Key(name), &m.tab, name, tabLabels[name]).Pad(6).Children(func() { ui.Text(c, name) })
			}
		})
		ui.SliderBase(c, &m.slider).Height(16).Width(120).Label("Base slider")
		ui.TextFieldBase(c, &m.text, "Base field", "").Width(120)
	})
}

// the labels, made once: a string joined in each paint allocates
var (
	radioLabels = map[string]string{"one": "radio one", "two": "radio two", "three": "radio three"}
	tabLabels   = map[string]string{"general": "tab general", "about": "tab about"}
)

func newBasesModel() *basesModel {
	m := &basesModel{radio: "one", tab: "general"}
	m.slider.Value = 50
	return m
}

// Each Base is the control without its look: it is on the page, named for assistive
// technology, and does what the dressed control does.
func TestPageBasesDoWhatTheControlsDo(t *testing.T) {
	m := newBasesModel()
	d := openPage(t, m.page)

	d.click("Base button")
	if m.clicks != 1 {
		t.Fatalf("ButtonBase clicked %d times", m.clicks)
	}
	d.click("Base check")
	if !m.on || m.changed != 1 {
		t.Fatalf("CheckboxBase: on %v, Changed reported %d", m.on, m.changed)
	}
	d.click("Base switch")
	d.click("Base toggle")
	if !m.s || !m.toggled {
		t.Fatalf("SwitchBase %v, ToggleBase %v", m.s, m.toggled)
	}
	d.click("radio two")
	if m.radio != "two" {
		t.Fatalf("RadioBase chose %q", m.radio)
	}
	d.repaint()
	if !d.hit("radio two").Checked || d.hit("radio one").Checked {
		t.Fatal("assistive technology is not told which radio button is chosen")
	}
	d.click("tab about")
	if m.tab != "about" {
		t.Fatalf("TabBase chose %q", m.tab)
	}
	d.repaint()
	if !d.hit("tab about").Selected || d.hit("tab general").Selected {
		t.Fatal("assistive technology is not told which tab is selected")
	}
	d.click("Base field")
	d.typeText("hi")
	if m.text != "hi" {
		t.Fatalf("TextFieldBase holds %q", m.text)
	}
	d.click("Base slider")
	before := m.slider.Value
	d.key(gfx.KeyRight)
	if m.slider.Value <= before {
		t.Fatalf("SliderBase: Right left the value at %v (from %v)", m.slider.Value, before)
	}
}

// Roles reach assistive technology as the control's.
func TestPageBasesHaveTheirRoles(t *testing.T) {
	m := newBasesModel()
	d := openPage(t, m.page)
	for label, role := range map[string]gfx.Role{
		"Base button": gfx.RoleButton, "Base check": gfx.RoleCheckbox, "Base switch": gfx.RoleSwitch,
		"radio one": gfx.RoleRadio, "tab general": gfx.RoleTab, "Base field": gfx.RoleTextField,
	} {
		if got := d.hit(label).Role; got != role {
			t.Errorf("%s has role %v, want %v", label, got, role)
		}
	}
}

// A restyled control keeps its behavior with the keys of a group around it: the arrows move
// between the radio options, which are one stop of Tab.
func TestPageBasesComposeWithAFocusGroup(t *testing.T) {
	m := newBasesModel()
	d := openPage(t, m.page)
	d.click("radio one")
	d.key(gfx.KeyRight)
	if d.focused() != "radio two" {
		t.Fatalf("Right in the group focused %q", d.focused())
	}
	d.key(gfx.KeySpace) // a radio button is chosen with Space
	if m.radio != "two" {
		t.Fatalf("Space on a RadioBase chose %q", m.radio)
	}
}

// The Bases repaint without allocating.
func TestPageBasesRepaintAllocates(t *testing.T) {
	m := newBasesModel()
	d := openPage(t, m.page)
	d.click("radio two")
	if allocs := testing.AllocsPerRun(100, d.repaint); allocs != 0 {
		t.Fatalf("%v allocations a repaint", allocs)
	}
}

// The dressed Slider takes the keys that move its thumb, as its Base does.
func TestPageSliderTakesItsKeys(t *testing.T) {
	var s ui.SliderState
	s.Value = 50
	d := openPage(t, func(c ui.C) {
		ui.Column(c).Pad(8).Children(func() { ui.Slider(c, &s).Width(160).Label("Volume") })
	})
	d.click("Volume")
	before := s.Value
	d.key(gfx.KeyRight)
	if s.Value <= before {
		t.Fatalf("Right moved the value from %v to %v", before, s.Value)
	}
	d.key(gfx.KeyHome)
	if s.Value != 0 {
		t.Fatalf("Home left the value at %v", s.Value)
	}
	d.key(gfx.KeyEnd)
	if s.Value != 100 {
		t.Fatalf("End left the value at %v", s.Value)
	}
}

// SelectBase opens a list of options a click chooses from, with nothing painted by
// the framework: the trigger and the options are the caller's.
func TestPageSelectBaseOpensAndPicks(t *testing.T) {
	size := "M"
	sizes := []string{"S", "M", "L"}
	var trigger, panel func(*ui.DrawContext)
	trigger = func(d *ui.DrawContext) { d.Canvas.Fill(d.Rect, d.Theme.Muted) }
	panel = func(d *ui.DrawContext) { d.Canvas.Fill(d.Rect, d.Theme.Popover) }
	d := openPage(t, func(c ui.C) {
		ui.Column(c).Pad(8).Children(func() {
			sel := ui.SelectBase(c, &size, sizes, nil).Label("Size").Size(100, 32).Draw(trigger)
			if list, ok := sel.Popup(); ok {
				list.Draw(panel)
			}
		})
	})
	d.click("Size")
	if !d.has("L") || !d.has("S") {
		t.Fatal("the list did not open with its options")
	}
	d.click("L")
	if size != "L" {
		t.Fatalf("picking L left %q", size)
	}
}
