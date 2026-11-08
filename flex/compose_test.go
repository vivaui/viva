//go:build darwin || windows || linux

package flex

import (
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/ui"
)

var composeScope = tree.Sub(page, tree.Name("compose-strip"))

// A strip made of item Bases is a complete tab strip or segmented control with a look of
// its own: a click selects, the arrow keys move the roving focus through the enabled tabs,
// and the state the application keeps is the only state. This is why Tabs and Segmented have
// no whole-control Base: the keyboard functions work on identities, so the items are enough.
type composedStrip struct {
	view     View
	labels   []string
	selected int
	segments bool
	options  []components.TabOption
	choices  []components.SegmentOption
}

func newComposedStrip(segments bool, labels ...string) *composedStrip {
	s := &composedStrip{labels: labels, segments: segments}
	for i, label := range labels {
		s.options = append(s.options, components.TabOption{ID: composeScope.At(i), Label: label})
		s.choices = append(s.choices, components.SegmentOption{ID: composeScope.At(i), Label: label})
	}
	return s
}

func (s *composedStrip) paint(canvas *ui.Canvas, size ui.Size) {
	s.view.Draw(canvas, ui.LightTheme(), ui.Rect{Width: size.Width, Height: size.Height}, func(f *Frame) {
		f.Row().Pad(8).Gap(4).Items(Start).Children(func() {
			for i, label := range s.labels {
				var item Box
				if s.segments {
					item = f.RadioBase(composeScope.At(i), label, i == s.selected)
				} else {
					item = f.TabBase(composeScope.At(i), label, i == s.selected)
				}
				if item.Pad(8).Children(func() { f.Text(label) }).Clicked() {
					s.selected = i
				}
			}
		})
	})
}

func (s *composedStrip) key(event ui.KeyEvent) ui.EventResult {
	var (
		next ui.ID
		ok   bool
	)
	if s.segments {
		next, ok = components.SegmentedControlKeyDown(event.Target, s.choices, components.DirectionLTR, event.Key)
	} else {
		next, ok = components.TabsKeyDown(event.Target, s.options, components.TabsHorizontal, components.DirectionLTR, event.Key)
	}
	if !ok {
		return ui.EventResult{}
	}
	return ui.EventResult{Handled: true, Repaint: true, Focus: next, FocusVisible: true}
}

func TestStripsMadeOfItemBasesSelectByClickAndMoveFocusByArrowKeys(t *testing.T) {
	for _, segments := range []bool{false, true} {
		name := "tabs"
		if segments {
			name = "segmented"
		}
		t.Run(name, func(t *testing.T) {
			strip := newComposedStrip(segments, "General", "Keys", "Advanced")
			var canvas ui.Canvas
			driver, err := app.NewDriver(app.Config{Tree: &tree, Title: "strip", Width: 360, Height: 80, Paint: strip.paint, OnKey: strip.key}, &canvas)
			if err != nil {
				t.Fatal(err)
			}
			defer driver.Close()
			centerOf := func(i int) (float32, float32) {
				for _, hit := range driver.Snapshot().Hits {
					if hit.ID == composeScope.At(i) {
						return hit.Bounds.X + hit.Bounds.Width/2, hit.Bounds.Y + hit.Bounds.Height/2
					}
				}
				t.Fatalf("no item %d on screen", i)
				return 0, 0
			}
			x, y := centerOf(1)
			if err := driver.Click(x, y); err != nil {
				t.Fatal(err)
			}
			if strip.selected != 1 {
				t.Fatalf("a click on the second item selected %d", strip.selected)
			}
			if got := driver.Snapshot().Focused; got != composeScope.At(1) {
				t.Fatalf("focus after the click = %v, want the clicked item", got)
			}
			if err := driver.Key(ui.KeyRight, 0); err != nil {
				t.Fatal(err)
			}
			if got := driver.Snapshot().Focused; got != composeScope.At(2) {
				t.Errorf("Right moved the focus to %v, want the third item", got)
			}
			if err := driver.Key(ui.KeyRight, 0); err != nil {
				t.Fatal(err)
			}
			if got := driver.Snapshot().Focused; got != composeScope.At(0) {
				t.Errorf("Right at the end did not wrap to the first item: focus %v", got)
			}
			if err := driver.Key(ui.KeyLeft, 0); err != nil {
				t.Fatal(err)
			}
			if got := driver.Snapshot().Focused; got != composeScope.At(2) {
				t.Errorf("Left at the start did not wrap to the last item: focus %v", got)
			}
			role := ui.RoleTab
			if segments {
				role = ui.RoleRadio
			}
			chosen := 0
			for _, hit := range driver.Snapshot().Hits {
				if hit.Role != role {
					t.Errorf("item %v is a %v, want %v", hit.Label, hit.Role, role)
				}
				if hit.Selected {
					chosen++
				}
			}
			if chosen != 1 {
				t.Errorf("%d items report themselves selected, want exactly one", chosen)
			}
		})
	}
}
