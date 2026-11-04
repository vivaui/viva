//go:build darwin || windows || linux

package app

import (
	"errors"
	"testing"

	"github.com/vivaui/viva/ui"
)

// typeAheadProbe is a list that declares it takes typed text next to a plain
// button that does not.
type typeAheadProbe struct {
	query string
	keys  []ui.KeyEvent
	asked []TextInputContext
}

func (probe *typeAheadProbe) config(provide bool) Config {
	config := Config{Title: "type-ahead", Width: 400, Height: 200}
	config.Paint = func(canvas *ui.Canvas, size ui.Size) {
		canvas.HitWithOptions(ui.Item(1), ui.Rect{Width: 400, Height: 100}, "Files", ui.HitOptions{Role: ui.RoleGroup, TypeAhead: true})
		canvas.HitWithOptions(ui.Item(2), ui.Rect{Y: 100, Width: 400, Height: 100}, "Plain", ui.HitOptions{Role: ui.RoleButton})
	}
	config.OnKey = func(event ui.KeyEvent) ui.EventResult {
		probe.keys = append(probe.keys, event)
		return ui.EventResult{Handled: true}
	}
	if provide {
		config.TextInput = func(context TextInputContext) *TextInput {
			probe.asked = append(probe.asked, context)
			if !context.TypeAhead {
				return nil
			}
			return &TextInput{
				Bounds: ui.Rect{X: 8, Y: 8, Width: 8, Height: 16}, FontSize: 13,
				OnEvent: func(event TextEvent) (TextState, error) {
					if event.Kind == TextInsert {
						probe.query += event.Text
					}
					return TextState{}, nil
				},
			}
		}
	}
	return config
}

func TestTypeAheadHitTakesTypedTextAndRawNavigationKeys(t *testing.T) {
	probe := &typeAheadProbe{}
	driver, err := NewDriver(probe.config(true), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	if err := driver.Click(100, 50); err != nil {
		t.Fatal(err)
	}

	for _, character := range []string{"F", "o", "o"} {
		if err := driver.Chord(0, character); err != nil {
			t.Fatal(err)
		}
	}
	if probe.query != "Foo" {
		t.Fatalf("query = %q, want the typed characters with their case", probe.query)
	}
	if err := driver.Key(ui.KeyDown, 0); err != nil {
		t.Fatal(err)
	}
	if err := driver.Key(ui.KeyBackspace, 0); err != nil {
		t.Fatal(err)
	}
	if len(probe.keys) != 2 || probe.keys[0].Key != ui.KeyDown || probe.keys[1].Key != ui.KeyBackspace || probe.keys[0].Target != ui.Item(1) {
		t.Fatalf("raw keys = %+v", probe.keys)
	}
	if probe.query != "Foo" {
		t.Fatalf("a named key changed the query: %q", probe.query)
	}
}

func TestTypeAheadAcceptsInputMethodComposition(t *testing.T) {
	probe := &typeAheadProbe{}
	driver, err := NewDriver(probe.config(true), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	if err := driver.Click(100, 50); err != nil {
		t.Fatal(err)
	}
	if err := driver.Type("文件"); err != nil {
		t.Fatal(err)
	}
	if probe.query != "文件" {
		t.Fatalf("query = %q", probe.query)
	}
}

func TestTypeAheadNeedsADeclarationAndAProvider(t *testing.T) {
	probe := &typeAheadProbe{}
	driver, err := NewDriver(probe.config(true), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	if err := driver.Click(100, 150); err != nil {
		t.Fatal(err)
	}
	if err := driver.Type("f"); !errors.Is(err, ErrNoTextInput) {
		t.Fatalf("typing into an undeclared hit: %v", err)
	}

	bare := &typeAheadProbe{}
	driver, err = NewDriver(bare.config(false), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	if err := driver.Click(100, 50); err != nil {
		t.Fatal(err)
	}
	if err := driver.Type("f"); !errors.Is(err, ErrNoTextInput) {
		t.Fatalf("typing without a TextInput provider: %v", err)
	}
}
