//go:build dev || vivadev

package base

import (
	"strings"
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/style"
)

var (
	attrTabsScope   = tree.Sub(page, tree.Name("attr-tabs"))
	attrSelectScope = tree.Sub(page, tree.Name("attr-select"))
)

type devTableRows struct{ n int }

func (r *devTableRows) Len() int                    { return r.n }
func (r *devTableRows) Cell(row, column int) string { return "r" }

var devTableSpec = TableSpec{
	Columns: []style.TableColumn{{Width: 80}},
	Headers: []string{"Name"},
	Source:  &devTableRows{n: 1},
}

// drawBase declares one page through the engine under the dev attribute rules.
func drawBase(declare func(f *Frame)) {
	var view View
	view.Draw(styleCanvas(gfx.ID{}, gfx.ID{}, gfx.ID{}, false), gfx.LightTheme(), gfx.Rect{Width: 400, Height: 300}, declare)
}

// expectAttributePanic runs a declaration that sets an attribute the element
// cannot use, and checks the dev build reports the element and the attribute.
func expectAttributePanic(t *testing.T, run func(f *Frame), want ...string) {
	t.Helper()
	defer func() {
		recovered := recover()
		if recovered == nil {
			t.Fatalf("declaration did not report; want a message containing %q", want)
		}
		message, _ := recovered.(string)
		for _, part := range want {
			if !strings.Contains(message, part) {
				t.Fatalf("diagnostic %q does not name %q", message, part)
			}
		}
	}()
	drawBase(run)
}

func TestBareConstructorsDoNotTripTheAttributeRules(t *testing.T) {
	var state SelectState
	drawBase(func(f *Frame) {
		f.Column().Children(func() {
			f.Button(gfx.Item(1), "Save")
			f.Badge("New")
			f.Toggle(gfx.Item(3), "Bold", true)
			f.Tabs(attrTabsScope, []string{"A", "B"}, 0).PadSides(2, 8, 4, 12)
			f.Select(attrSelectScope, &state, []string{"A", "B"}, 0).Label("Size")
			f.Input(InputSnapshot{ID: gfx.Item(4), Label: "Name", Value: "x"})
			f.Textarea(TextareaSnapshot{ID: gfx.Item(5), Label: "Notes", Value: "x"})
			f.Checkbox(gfx.Item(6), true).Caption("Sync")
			f.Progress(gfx.ID{}, 0.5).Label("Loading")
			f.Image(gfx.Bitmap{}).Radius(4).Opacity(0.5)
			f.Switch(gfx.Item(7), false).Caption("Run")
			f.Slot()
		})
	})
}

func TestContainerAttributesAreChecked(t *testing.T) {
	drawBase(func(f *Frame) {
		f.Row().Gap(4).Items(Center).Justify(End).Children(func() { f.Text("x") })
		f.Column().Gap(4).Items(Start).Children(func() { f.Text("x") })
		f.Flow().Gap(4).LineGap(2).Children(func() { f.Text("x") })
		f.Stack()
		f.Slot().Children(func() { f.Text("x") })
	})
}

// StateFrom follows an element that reports no state, and a span outside a Grid,
// and a placement outside a Popup are declaration errors.
func TestStructuralAttributesAreChecked(t *testing.T) {
	expectAttributePanic(t, func(f *Frame) {
		row := f.Row()
		f.Text("x").StateFrom(row)
	}, "Text", "StateFrom")
	expectAttributePanic(t, func(f *Frame) {
		f.Column().Children(func() { f.Text("x").ColSpan(2) })
	}, "Text", "ColSpan/RowSpan")
	expectAttributePanic(t, func(f *Frame) {
		f.Text("x").At(4, 4)
	}, "Text", "At/Below/Above/Beside/Within")
	expectAttributePanic(t, func(f *Frame) {
		f.Splitter(gfx.Item(1), &SplitterState{}).LabelFor(gfx.Item(2))
	}, "Splitter", "LabelFor")
}

// The type attributes of a Text are its own: they are the element-level type.
func TestTextTypeAttributesAreItsOwn(t *testing.T) {
	drawBase(func(f *Frame) {
		f.Text("x").Mono().TextCenter().LineHeight(20).MaxLines(3).FontSize(18).Bold()
	})
}

func TestAttributesThatDoNothingReport(t *testing.T) {
	cases := []struct {
		name string
		want []string
		run  func(f *Frame)
	}{
		{"Stack Gap", []string{"Stack", "Gap"}, func(f *Frame) { f.Stack().Gap(4) }},
		{"Grid Items", []string{"Grid", "Items"}, func(f *Frame) { f.Grid(40).Items(Center) }},
		{"Stack Justify", []string{"Stack", "Justify"}, func(f *Frame) { f.Stack().Justify(End) }},
		{"Column LineGap", []string{"Column", "LineGap"}, func(f *Frame) { f.Column().LineGap(2) }},
		{"Input Children", []string{"Input", "Children"}, func(f *Frame) {
			f.Input(InputSnapshot{ID: gfx.Item(1), Label: "Name"}).Children(func() { f.Text("x") })
		}},
		{"Textarea Children", []string{"Textarea", "Children"}, func(f *Frame) {
			f.Textarea(TextareaSnapshot{ID: gfx.Item(1), Label: "Notes"}).Children(func() { f.Text("x") })
		}},
		{"Tabs Children", []string{"Tabs", "Children"}, func(f *Frame) {
			f.Tabs(attrTabsScope, []string{"A"}, 0).Children(func() { f.Text("x") })
		}},
		{"Checkbox Pad", []string{"Checkbox", "Pad"}, func(f *Frame) { f.Checkbox(gfx.Item(1), true).Pad(4) }},
		{"Checkbox Children", []string{"Checkbox", "Children"}, func(f *Frame) {
			f.Checkbox(gfx.Item(1), true).Children(func() { f.Text("x") })
		}},
		{"Switch Gap", []string{"Switch", "Gap"}, func(f *Frame) { f.Switch(gfx.Item(1), false).Gap(4) }},
		{"Switch Children", []string{"Switch", "Children"}, func(f *Frame) {
			f.Switch(gfx.Item(1), false).Children(func() { f.Text("x") })
		}},
		{"Slider Pad", []string{"Slider", "Pad"}, func(f *Frame) {
			f.SliderBase(gfx.Item(1), &SliderState{}).Pad(4)
		}},
		{"Slider HitWith", []string{"Slider", "HitWith"}, func(f *Frame) {
			f.SliderBase(gfx.Item(1), &SliderState{}).HitWith(gfx.Item(1), "Volume", gfx.HitOptions{Role: gfx.RoleButton})
		}},
		{"Segmented Background", []string{"Segmented", "Background"}, func(f *Frame) {
			f.Segmented([]style.SegmentOption{{ID: gfx.Item(1), Label: "A"}}, gfx.Item(1)).Background(gfx.Color{Red: 1, Alpha: 1})
		}},
		{"Segmented Children", []string{"Segmented", "Children"}, func(f *Frame) {
			f.Segmented(nil, gfx.ID{}).Children(func() { f.Text("x") })
		}},
		{"Progress Pad", []string{"Progress", "Pad"}, func(f *Frame) { f.Progress(gfx.ID{}, 0.5).Pad(4) }},
		{"Progress FontSize", []string{"Progress", "FontSize"}, func(f *Frame) { f.Progress(gfx.ID{}, 0.5).FontSize(18) }},
		{"Pagination Color", []string{"Pagination", "Color"}, func(f *Frame) {
			f.Pagination(nil).Color(gfx.Color{Red: 1, Alpha: 1})
		}},
		{"Progress Disabled", []string{"Progress", "Disabled"}, func(f *Frame) { f.Progress(gfx.ID{}, 0.5).Disabled(true) }},
		{"Spinner Disabled", []string{"Spinner", "Disabled"}, func(f *Frame) { f.Spinner().Disabled(true) }},
		{"Pagination Disabled", []string{"Pagination", "Disabled"}, func(f *Frame) {
			f.Pagination(nil).Disabled(true)
		}},
		{"Splitter LabelFor", []string{"Splitter", "LabelFor"}, func(f *Frame) {
			f.Splitter(gfx.Item(1), &SplitterState{}).LabelFor(gfx.Item(2))
		}},
		{"Table Weight", []string{"Table", "Weight"}, func(f *Frame) {
			f.Table(&TableState{}, devTableSpec).Bold()
		}},
		{"Select Hit", []string{"Select", "Hit"}, func(f *Frame) {
			f.Select(attrSelectScope, &SelectState{}, []string{"A"}, 0).Hit(gfx.Item(9), "Size")
		}},
		{"Input Hit", []string{"Input", "Hit"}, func(f *Frame) {
			f.Input(InputSnapshot{ID: gfx.Item(1), Label: "Name"}).Hit(gfx.Item(9), "Name")
		}},
		{"Scroll on a Text", []string{"Text", "Scroll*"}, func(f *Frame) {
			f.Text("x").Scrollbars()
		}},
		{"HitExact on an Input", []string{"Input", "HitExact"}, func(f *Frame) {
			f.Input(InputSnapshot{ID: gfx.Item(1), Label: "Name"}).HitExact()
		}},
		{"HitOutset on an Input", []string{"Input", "HitOutset"}, func(f *Frame) {
			f.Input(InputSnapshot{ID: gfx.Item(1), Label: "Name"}).HitOutset(2, 2, 2, 2)
		}},
		{"HitWith on a Slider", []string{"Slider", "HitWith"}, func(f *Frame) {
			f.SliderBase(gfx.Item(1), &SliderState{}).HitWith(gfx.Item(1), "Volume", gfx.HitOptions{Role: gfx.RoleButton})
		}},
		{"Type without caption", []string{"Checkbox", "FontSize/Weight/Color"}, func(f *Frame) {
			f.Checkbox(gfx.Item(1), true).Color(gfx.Color{Red: 1, Alpha: 1})
		}},
		{"Badge size method", []string{"Badge", "Size"}, func(f *Frame) { f.Badge("New").Small() }},
		{"Checkbox type without caption", []string{"Checkbox", "FontSize/Weight/Color"}, func(f *Frame) {
			f.Checkbox(gfx.Item(1), true).FontSize(18)
		}},
		{"Select Variant", []string{"Select", "Variant"}, func(f *Frame) {
			f.Select(attrSelectScope, &SelectState{}, []string{"A"}, 0).Secondary()
		}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			expectAttributePanic(t, test.run, test.want...)
		})
	}
}
