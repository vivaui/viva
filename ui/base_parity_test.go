package ui_test

import (
	"fmt"
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// semantics is what a control tells the pointer, the keyboard and assistive
// technology it is. A control and its Base must declare the same, because a
// restyled control is the same control: the look is the only thing that differs.
type semantics struct {
	Role                     gfx.Role
	Label, Value             string
	Checked, Mixed, Selected bool
	Expanded, Expandable     bool
	Focusable, Disabled      bool
	Multiline, ReadOnly      bool
	Secure                   bool
	Range                    gfx.ValueRange
}

func semanticsOf(h gfx.Hit) semantics {
	return semantics{
		Role: h.Role, Label: h.Label, Value: h.Value,
		Checked: h.Checked, Mixed: h.Mixed, Selected: h.Selected,
		Expanded: h.Expanded, Expandable: h.Expandable,
		Focusable: h.Focusable, Disabled: h.Disabled,
		Multiline: h.Multiline, ReadOnly: h.ReadOnly, Secure: h.Secure,
		Range: h.Range,
	}
}

// declared is the semantics of what a page declares, in paint order: the controls
// themselves, not the labels painted beside them nor the backdrops under overlays.
func declared(t *testing.T, view func(ui.C)) []semantics {
	t.Helper()
	d := openPage(t, view)
	var out []semantics
	for _, h := range d.driver.Snapshot().Hits {
		if h.IsLabel || h.Backdrop || h.Block || h.ID.IsZero() {
			continue
		}
		out = append(out, semanticsOf(h))
	}
	return out
}

func page(body func(c ui.C)) func(ui.C) {
	return func(c ui.C) { ui.Column(c).Pad(16).Gap(8).Children(func() { body(c) }) }
}

// Every styled control that has a Base declares what the Base declares, in each
// state a caller can put it in. A new Base, or a new state, belongs in this table:
// base_scan_test.go fails for a Base that is in neither this table nor its list of
// Bases without a styled twin.
func TestStyledControlsDeclareWhatTheirBasesDeclare(t *testing.T) {
	choices := []string{"Small", "Large"}
	for _, state := range []struct {
		name     string
		on       bool
		disabled bool
	}{{"off", false, false}, {"on", true, false}, {"disabled off", false, true}, {"disabled on", true, true}} {
		on, disabled := state.on, state.disabled
		// The label Checkbox and Switch put beside the box is the name the box
		// gets; CheckboxBase and SwitchBase are given the same name.
		pairs := []struct {
			name         string
			styled, base func(ui.C)
		}{
			{"button",
				func(c ui.C) { ui.Button(c, "Go").Disabled(disabled) },
				func(c ui.C) { ui.ButtonBase(c, "Go").Size(40, 30).Disabled(disabled) }},
			{"checkbox",
				func(c ui.C) { ui.Checkbox(c, &on, "Go").Disabled(disabled) },
				func(c ui.C) { ui.CheckboxBase(c, &on, "Go").Disabled(disabled) }},
			{"switch",
				func(c ui.C) { ui.Switch(c, &on, "Go").Disabled(disabled) },
				func(c ui.C) { ui.SwitchBase(c, &on, "Go").Disabled(disabled) }},
		}
		for _, p := range pairs {
			t.Run(p.name+"/"+state.name, func(t *testing.T) {
				styled, base := declared(t, page(p.styled)), declared(t, page(p.base))
				if fmt.Sprint(styled) != fmt.Sprint(base) {
					t.Fatalf("the control and its Base declare different semantics\n  styled: %+v\n  base:   %+v", styled, base)
				}
			})
		}
	}

	for _, selected := range []string{"", "Large"} {
		value := selected
		t.Run("select/"+fmt.Sprintf("%q", selected), func(t *testing.T) {
			styled := declared(t, page(func(c ui.C) { ui.Select(c, &value, choices, nil).Placeholder("Size") }))
			base := declared(t, page(func(c ui.C) { ui.SelectBase(c, &value, choices, nil).Label("Size") }))
			if fmt.Sprint(styled) != fmt.Sprint(base) {
				t.Fatalf("the control and its Base declare different semantics\n  styled: %+v\n  base:   %+v", styled, base)
			}
		})
	}

	t.Run("slider", func(t *testing.T) {
		var a, b ui.SliderState
		styled := declared(t, page(func(c ui.C) { ui.Slider(c, &a).Range(0, 10).Step(1).Label("Volume") }))
		base := declared(t, page(func(c ui.C) { ui.SliderBase(c, &b).Range(0, 10).Step(1).Label("Volume") }))
		if fmt.Sprint(styled) != fmt.Sprint(base) {
			t.Fatalf("the control and its Base declare different semantics\n  styled: %+v\n  base:   %+v", styled, base)
		}
	})

	t.Run("text field", func(t *testing.T) {
		var a, b string
		styled := declared(t, page(func(c ui.C) { ui.TextField(c, &a, "Name", "Ada") }))
		base := declared(t, page(func(c ui.C) { ui.TextFieldBase(c, &b, "Name", "Ada") }))
		if fmt.Sprint(styled) != fmt.Sprint(base) {
			t.Fatalf("the control and its Base declare different semantics\n  styled: %+v\n  base:   %+v", styled, base)
		}
	})

	// A strip of tabs and a segmented control are rows of Bases: each option says what
	// the Base for that option says.
	for _, chosen := range []string{"General", "About"} {
		tab := chosen
		t.Run("tabs/"+chosen, func(t *testing.T) {
			names := []string{"General", "About"}
			styled := declared(t, page(func(c ui.C) { ui.Tabs(c, &tab, names, nil) }))
			base := declared(t, page(func(c ui.C) {
				ui.Row(c).Children(func() {
					for _, name := range names {
						ui.TabBase(c.Key(name), &tab, name, name).Size(60, 28)
					}
				})
			}))
			// Only what a Base is responsible for is compared: the strip's own
			// roving focus (which tab is a Tab stop) is the group's, not the tab's.
			if fmt.Sprint(styled) != fmt.Sprint(base) {
				t.Fatalf("the strip and its Bases declare different semantics\n  styled: %+v\n  base:   %+v", styled, base)
			}
		})
		choice := chosen
		t.Run("choice/"+chosen, func(t *testing.T) {
			names := []string{"General", "About"}
			styled := declared(t, page(func(c ui.C) { ui.Choice(c, &choice, names, nil) }))
			base := declared(t, page(func(c ui.C) {
				ui.Row(c).Children(func() {
					for _, name := range names {
						ui.RadioBase(c.Key(name), &choice, name, name).Size(60, 28)
					}
				})
			}))
			if fmt.Sprint(styled) != fmt.Sprint(base) {
				t.Fatalf("the segmented control and its Bases declare different semantics\n  styled: %+v\n  base:   %+v", styled, base)
			}
		})
	}
}
