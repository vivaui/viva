//go:build darwin || windows || linux

package headless

import (
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
)

// multiSelectApp wires the component's state to a driver the way an application
// does, so the focus a popup takes and gives back is observable.
type multiSelectApp struct {
	state   components.MultiSelectState
	options components.MultiSelectOptions
	native  app.TextInput
}

func newMultiSelectApp() *multiSelectApp {
	app := &multiSelectApp{options: components.MultiSelectOptions{
		InputID: gfx.Item(11), TriggerID: gfx.Item(12), DismissID: gfx.Item(13), Scope: gfx.Item(14),
		Label: "Tags", Placeholder: "Select tags", EmptyLabel: "None", CreatePrefix: "Create: ",
		Viewport: gfx.Rect{Width: 640, Height: 300}, Layer: 6,
	}}
	app.state.AllowCreate = true
	app.state.Open = true
	app.state.SetOptions([]components.MultiSelectOption{{Value: "开发"}, {Value: "测试"}})
	return app
}

func (a *multiSelectApp) config() app.Config {
	return app.Config{
		Title: "multi-select focus", Width: 640, Height: 300,
		Paint: func(canvas *gfx.Canvas, size gfx.Size) {
			theme := gfx.LightTheme()
			canvas.Fill(gfx.Rect{Width: size.Width, Height: size.Height}, theme.Background)
			components.PaintMultiSelect(canvas, theme, gfx.Rect{X: 40, Y: 60, Width: 360}, &a.state, a.options)
			if canvas.Clicked(a.options.DismissID) {
				a.state.Open = false
			}
		},
		OnKey: func(event gfx.KeyEvent) gfx.EventResult {
			if event.Target != a.options.InputID && !a.state.Open {
				return gfx.EventResult{}
			}
			return a.state.Key(event)
		},
		TextInput: func(context app.TextInputContext) *app.TextInput {
			if context.ID != a.options.InputID {
				return nil
			}
			a.native = app.TextInput{Editor: &a.state.Query}
			return &a.native
		},
	}
}

// A list opened without a press on the query input (the programmatic open the
// demo and a restored draft use) must not drop keyboard focus when it closes:
// the user keeps typing into the query.
func TestMultiSelectKeepsQueryFocusWhenThePopupCloses(t *testing.T) {
	subject := newMultiSelectApp()
	session, err := Open(subject.config(), nil, Options{Scale: 1})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(session.Close)

	if err := session.Click(200, 76); err != nil { // focus the query input
		t.Fatal(err)
	}
	if err := session.Keys("escape"); err != nil {
		t.Fatal(err)
	}
	if focused := session.Snapshot().Focused; focused != subject.options.InputID {
		t.Fatalf("focus after close = %v, want the query input %v", focused, subject.options.InputID)
	}
	if err := session.Type("x"); err != nil {
		t.Fatalf("typing after close: %v", err)
	}
	if got := subject.state.Query.String(); got != "x" {
		t.Fatalf("query after close = %q, want %q", got, "x")
	}
}

// Closing with a press outside the popup also hands focus back: the dismiss
// backdrop is not focusable, so without it the press would leave nothing
// focused and the query could not be typed into afterwards.
func TestMultiSelectCloseOnTheBackdropKeepsQueryFocus(t *testing.T) {
	subject := newMultiSelectApp()
	session, err := Open(subject.config(), nil, Options{Scale: 1})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(session.Close)

	if err := session.Click(200, 76); err != nil { // focus the query input
		t.Fatal(err)
	}
	if err := session.Click(500, 260); err != nil { // press the dismiss backdrop
		t.Fatal(err)
	}
	if subject.state.Open {
		t.Fatal("the press outside did not close the list")
	}
	if focused := session.Snapshot().Focused; focused != subject.options.InputID {
		t.Fatalf("focus after a backdrop close = %v, want the query input %v", focused, subject.options.InputID)
	}
}
