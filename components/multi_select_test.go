package components

import (
	"slices"
	"strings"
	"testing"

	"github.com/vivaui/viva/text"
	"github.com/vivaui/viva/ui"
)

func multiSelectFixture() *MultiSelectState {
	s := &MultiSelectState{AllowCreate: true, MatchCaseInsensitive: true}
	s.SetOptions([]MultiSelectOption{{Value: "开发"}, {Value: "测试"}, {Value: "生产"}, {Value: "disabled", Disabled: true}})
	return s
}

func TestMultiSelectFiltersCreatesAndPreservesSelection(t *testing.T) {
	s := multiSelectFixture()
	s.Open = true
	if !s.Choose(0) || !s.Choose(1) || !slices.Equal(s.Values(), []string{"开发", "测试"}) || !s.Open {
		t.Fatal("multi-selection closed or lost values", s.Values())
	}
	s.Query.Insert("数据库")
	choices := s.Choices()
	if len(choices) != 1 || !choices[0].Create || choices[0].Value != "数据库" {
		t.Fatal("missing create row", choices)
	}
	s.Key(ui.KeyEvent{Key: ui.KeyEnter})
	if !slices.Equal(s.Values(), []string{"开发", "测试", "数据库"}) || s.Query.String() != "" {
		t.Fatal("create did not commit and clear query", s.Values())
	}
	if len(s.Choices()) != 5 {
		t.Fatal("created value was not available for later deselection")
	}
	s.Query.Insert("DISABLED")
	if c := s.Choices(); len(c) != 1 || c[0].Create || !c[0].Disabled || s.Choose(0) {
		t.Fatal("disabled or case-insensitive matching failed", c)
	}
	s.Query = text.Editor{}
	s.Query.Insert("数据库")
	s.Key(ui.KeyEvent{Key: ui.KeyEnter})
	if len(s.Values()) != 3 {
		t.Fatal("duplicate query removed or duplicated selection")
	}
	if !s.Remove(1) || !slices.Equal(s.Values(), []string{"开发", "数据库"}) {
		t.Fatal("removing one chip damaged other selections")
	}
}

func TestMultiSelectKeyboardLimitAndIME(t *testing.T) {
	s := multiSelectFixture()
	s.Limit = 1
	s.SetValidateCreate(func(value string) bool { return len([]rune(value)) <= 4 })
	s.Key(ui.KeyEvent{Key: ui.KeyEnter})
	if len(s.Values()) != 0 || !s.Open {
		t.Fatal("opening an empty select chose a value")
	}
	s.Key(ui.KeyEvent{Key: ui.KeyDown})
	s.Key(ui.KeyEvent{Key: ui.KeyEnter, Repeat: true})
	if len(s.Values()) != 0 {
		t.Fatal("repeat Enter committed a selection")
	}
	s.Key(ui.KeyEvent{Key: ui.KeyEnter})
	if !slices.Equal(s.Values(), []string{"测试"}) {
		t.Fatal("arrow selection failed", s.Values())
	}
	s.Query.Insert("新标签")
	if !s.Choices()[0].Disabled || s.Choose(0) {
		t.Fatal("selection limit was bypassed")
	}
	s.Query = text.Editor{}
	s.Key(ui.KeyEvent{Key: ui.KeyBackspace})
	if len(s.Values()) != 0 {
		t.Fatal("empty query backspace did not remove last chip")
	}
	s.Query.SetMarkedText("中文", text.Range{End: 2})
	s.Key(ui.KeyEvent{Key: ui.KeyEnter})
	if len(s.Values()) != 0 {
		t.Fatal("IME Enter created a value")
	}
	s.Query.Unmark()
	s.Key(ui.KeyEvent{Key: ui.KeyEnter})
	if !slices.Equal(s.Values(), []string{"中文"}) {
		t.Fatal("committed Chinese query was not created")
	}
	result := s.Key(ui.KeyEvent{Key: ui.KeyTab})
	if s.Open || !result.ContinueTraversal {
		t.Fatal("Tab trapped focus")
	}
	s.Open = true
	result = s.Key(ui.KeyEvent{Key: ui.KeyTab, Modifiers: ui.ModifierShift})
	if s.Open || !result.ContinueTraversal {
		t.Fatal("Shift+Tab trapped focus")
	}
	s.Query = text.Editor{}
	s.Query.Insert(strings.Repeat("长", 5))
	if !s.Choices()[0].Disabled {
		t.Fatal("invalid create query was enabled")
	}
}

func TestMultiSelectCreateValidatorReachesTheCreateRow(t *testing.T) {
	s := multiSelectFixture()
	s.Query.Insert("自定义")
	if choices := s.Choices(); len(choices) == 0 || !choices[0].Create || choices[0].Disabled {
		t.Fatal("create row did not start enabled", choices)
	}
	s.SetValidateCreate(func(string) bool { return false })
	if choices := s.Choices(); !choices[0].Create || !choices[0].Disabled {
		t.Fatal("a new validator did not reach the create row", choices)
	}
	s.SetValidateCreate(nil)
	if choices := s.Choices(); choices[0].Disabled {
		t.Fatal("clearing the validator did not re-enable the create row", choices)
	}
}

func TestMultiSelectInputOwnsNativeTextBoundsInsideThePopupScope(t *testing.T) {
	var tree ui.Tree
	f := tree.Feature("multi-select-test")
	o := MultiSelectOptions{InputID: tree.ID(f, "input"), TriggerID: tree.ID(f, "trigger"), DismissID: tree.ID(f, "dismiss"), Scope: tree.ID(f, "scope"), OptionIDs: tree.Sub(f, tree.Name("option")), RemoveIDs: tree.Sub(f, tree.Name("remove")), Label: "Tags", Placeholder: "Select tags", Viewport: ui.Rect{Width: 800, Height: 600}, Layer: 4}
	s := multiSelectFixture()
	s.SetValues([]string{"开发", "测试", "生产", "a very long custom tag"})
	s.Open = true
	for _, theme := range []ui.Theme{ui.LightTheme(), ui.DarkTheme()} {
		for _, width := range []float32{180, 320, 600} {
			var canvas ui.Canvas
			layout := PaintMultiSelect(&canvas, theme, ui.Rect{X: 20, Y: 540, Width: width}, s, o)
			if layout.Input.Width <= 0 || layout.Input.X < layout.Bounds.X || layout.Input.X+layout.Input.Width > layout.Bounds.X+layout.Bounds.Width {
				t.Fatal("query escaped the select", layout)
			}
			if layout.Popup.Y+layout.Popup.Height > 540 || layout.Popup.Y < 0 {
				t.Fatal("popup did not flip above the field", layout)
			}
			if canvas.TrappedFocusScope() != o.Scope || canvas.InitialFocus() != o.InputID {
				t.Fatal("popup excludes its native query input")
			}
			inputHits := 0
			for _, hit := range canvas.Hits() {
				if hit.ID == o.InputID {
					inputHits++
					if hit.TextBounds != layout.Input || hit.Role != ui.RoleCombobox || !hit.Expanded {
						t.Fatal("native input geometry or semantics lost", hit)
					}
				}
			}
			if inputHits != 1 {
				t.Fatal("query has duplicate native text hits", inputHits)
			}
		}
	}
}

func TestMultiSelectPopupEscapesScrollClipWithoutExposingInput(t *testing.T) {
	var tree ui.Tree
	feature := tree.Feature("clipped-multi-select")
	o := MultiSelectOptions{InputID: tree.ID(feature, "input"), TriggerID: tree.ID(feature, "trigger"), DismissID: tree.ID(feature, "dismiss"), Scope: tree.ID(feature, "scope"), OptionIDs: tree.Sub(feature, tree.Name("option")), RemoveIDs: tree.Sub(feature, tree.Name("remove")), Label: "Tags", Viewport: ui.Rect{Width: 400, Height: 300}, Layer: 4}
	s := multiSelectFixture()
	s.SetOptions([]MultiSelectOption{{Value: "one"}, {Value: "two"}, {Value: "three"}, {Value: "four"}, {Value: "five"}})
	s.Open = true
	var canvas ui.Canvas
	body := ui.Rect{X: 10, Y: 20, Width: 360, Height: 65}
	canvas.Clip(body)
	layout := PaintMultiSelect(&canvas, ui.LightTheme(), ui.Rect{X: 30, Y: 60, Width: 250, Height: 32}, s, o)
	canvas.Unclip()
	if layout.Popup.Y+layout.Popup.Height <= body.Y+body.Height {
		t.Fatal("fixture did not place the menu outside the scroll area", layout)
	}
	var input, last, dismiss ui.Hit
	for _, hit := range canvas.Hits() {
		switch hit.ID {
		case o.InputID:
			input = hit
		case o.OptionIDs.At(4):
			last = hit
		case o.DismissID:
			dismiss = hit
		}
	}
	if input.Bounds.X != layout.Input.X || input.Bounds.Y != layout.Input.Y ||
		input.Bounds.Height >= layout.Input.Height || input.Bounds.Y+input.Bounds.Height != body.Y+body.Height ||
		input.Scope != o.Scope || input.Layer != o.Layer ||
		last.Bounds.Height != 32 || last.Bounds.Y <= body.Y+body.Height || last.Scope != o.Scope ||
		dismiss.Bounds != o.Viewport || !dismiss.Backdrop {
		t.Fatalf("incomplete or unreachable popup: input=%+v last=%+v backdrop=%+v layout=%+v", input, last, dismiss, layout)
	}
	foundSurface := false
	for _, r := range canvas.Rectangles() {
		if r.Bounds == layout.Popup {
			foundSurface = true
			if !r.HasClip || r.ClipOf != o.Viewport {
				t.Fatalf("popup surface still clipped to the form: %+v", r)
			}
		}
	}
	if !foundSurface {
		t.Fatal("popup surface was not painted")
	}
	// The parent still owns its scroll clip after the component returns.
	canvas.Clip(body)
	canvas.Hit(ui.Item(99), ui.Rect{X: 30, Y: 120, Width: 20, Height: 20})
	if len(canvas.Hits()) != 0 && canvas.Hits()[len(canvas.Hits())-1].ID == ui.Item(99) {
		t.Fatal("the scroll area no longer clips its own content")
	}
	canvas.Unclip()
}

func TestMultiSelectClampsPopupToTinyViewport(t *testing.T) {
	var canvas ui.Canvas
	s := multiSelectFixture()
	s.Open = true
	viewport := ui.Rect{Width: 140, Height: 50}
	layout := PaintMultiSelect(&canvas, ui.LightTheme(), ui.Rect{Y: 2, Width: 120, Height: 32}, s, MultiSelectOptions{Viewport: viewport})
	if layout.Popup.Y < viewport.Y || layout.Popup.Y+layout.Popup.Height > viewport.Y+viewport.Height {
		t.Fatalf("popup escaped the viewport: %+v", layout)
	}
}

func BenchmarkMultiSelectPaint(b *testing.B) {
	var tree ui.Tree
	f := tree.Feature("benchmark")
	o := MultiSelectOptions{InputID: tree.ID(f, "input"), TriggerID: tree.ID(f, "trigger"), DismissID: tree.ID(f, "dismiss"), Scope: tree.ID(f, "scope"), OptionIDs: tree.Sub(f, tree.Name("option")), RemoveIDs: tree.Sub(f, tree.Name("remove")), Label: "Tags", CreatePrefix: "Create: ", Viewport: ui.Rect{Width: 800, Height: 600}, Layer: 4}
	for _, name := range []string{"closed", "open", "create", "overflow"} {
		b.Run(name, func(b *testing.B) {
			s := multiSelectFixture()
			s.SetValues([]string{"开发", "生产"})
			s.Open = name != "closed"
			if name == "create" {
				s.Query.Insert("custom")
			}
			if name == "overflow" {
				s.SetValues([]string{"开发", "测试", "生产", strings.Repeat("long", 8)})
			}
			var canvas ui.Canvas
			paint := func() {
				canvas.Reset()
				PaintMultiSelect(&canvas, ui.LightTheme(), ui.Rect{Width: 320, Height: 32}, s, o)
			}
			paint()
			b.ReportAllocs()
			for b.Loop() {
				paint()
			}
		})
	}
}

func BenchmarkMultiSelectPaintInScrollArea(b *testing.B) {
	var tree ui.Tree
	feature := tree.Feature("nested-popup-bench")
	o := MultiSelectOptions{InputID: tree.ID(feature, "input"), TriggerID: tree.ID(feature, "trigger"), DismissID: tree.ID(feature, "dismiss"), Scope: tree.ID(feature, "scope"), OptionIDs: tree.Sub(feature, tree.Name("option")), RemoveIDs: tree.Sub(feature, tree.Name("remove")), Viewport: ui.Rect{Width: 800, Height: 600}, Layer: 4}
	for _, open := range []bool{false, true} {
		name := "closed"
		if open {
			name = "open"
		}
		b.Run(name, func(b *testing.B) {
			s := multiSelectFixture()
			s.Open = open
			var canvas ui.Canvas
			paint := func() {
				canvas.Reset()
				canvas.Clip(ui.Rect{X: 10, Y: 10, Width: 500, Height: 100})
				PaintMultiSelect(&canvas, ui.LightTheme(), ui.Rect{X: 20, Y: 70, Width: 320, Height: 32}, s, o)
				canvas.Unclip()
			}
			paint()
			b.ReportAllocs()
			for b.Loop() {
				paint()
			}
		})
	}
}
