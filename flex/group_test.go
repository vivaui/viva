package flex

import (
	"testing"

	"github.com/vivaui/viva/ui"
)

var (
	groupField = tree.ID(page, "group-field")
	groupOther = tree.ID(page, "group-other")
)

// groupBorder paints an input group (a Row with an InputBase and addons in it) with the focus
// where focus says and returns the color of the frame's line.
func groupBorder(t *testing.T, focus ui.ID, visible bool) ui.Color {
	t.Helper()
	theme := ui.LightTheme()
	var field Field
	var view View
	canvas := testCanvas()
	canvas.SetInteraction(ui.ID{}, ui.ID{}, focus, visible)
	var frame Box
	view.Draw(canvas, theme, ui.Rect{Width: 360, Height: 100}, func(f *Frame) {
		f.Column().Pad(12).Children(func() {
			var input Box
			frame = f.Row().Items(Center).Height(34).Radius(theme.Metrics.RadiusControl).Border(1, theme.Input).
				Focused(Look{}.Border(theme.Primary)).Children(func() {
				f.Text("https://").Muted().PadX(10)
				input = f.FieldBase(&field, groupField, "Address", "example.com").Grow(1).Height(32)
				f.Text(".com").Muted().PadX(10)
			})
			frame.StateFrom(input)
		})
	})
	// The frame is the one element drawn with a border over its own rectangle: its outline is
	// painted as strokes, so the color is read from the strokes that lie on the frame's edge.
	rect := frame.Rect()
	for _, triangle := range canvas.Triangles() {
		a := triangle.Points[0]
		if triangle.Color.Alpha > 0 && a.X >= rect.X-1 && a.X <= rect.X+rect.Width+1 && a.Y >= rect.Y-1 && a.Y <= rect.Y+rect.Height+1 {
			return triangle.Color
		}
	}
	t.Fatalf("no outline painted on the frame %+v", rect)
	return ui.Color{}
}

// An input group is a Row around an InputBase that takes the field's focus as its own: a click
// into the field focuses it without making the focus visible, and the frame still follows it
// with Focused, as shadcn's input group does. No mode of the control is needed.
func TestAnInputGroupIsARowAroundAnInputBaseThatFollowsItsFocus(t *testing.T) {
	theme := ui.LightTheme()
	rest := groupBorder(t, groupOther, false)
	clicked := groupBorder(t, groupField, false)
	keyboard := groupBorder(t, groupField, true)
	if rest == clicked {
		t.Fatalf("a click into the field did not change the frame: still %+v", rest)
	}
	if clicked != keyboard {
		t.Errorf("the frame differs between a click (%+v) and the keyboard (%+v) on the same field", clicked, keyboard)
	}
	if clicked.Red == rest.Red && clicked.Green == rest.Green && clicked.Blue == rest.Blue {
		t.Errorf("the focused frame %+v is not another color than at rest %+v", clicked, rest)
	}
	_ = theme
}

// Focus is Focus (the keyboard made it visible) and Focused is any focus: a look on Focus
// alone does not show after a click, which is why the group needs Focused.
func TestFocusLookIsKeyboardOnlyAndFocusedLookIsAnyFocus(t *testing.T) {
	theme := ui.LightTheme()
	on := ui.Color{Red: .9, Alpha: 1}
	run := func(focused, focusVisible Look, visible bool) ui.Color {
		var view View
		canvas := testCanvas()
		canvas.SetInteraction(ui.ID{}, ui.ID{}, groupField, visible)
		var box Box
		view.Draw(canvas, theme, ui.Rect{Width: 100, Height: 60}, func(f *Frame) {
			box = f.Row().Size(60, 30).Hit(groupField, "x").Focused(focused).Focus(focusVisible).Children(func() { f.Text("x") }).Background(ui.Color{Alpha: 1})
		})
		for _, r := range canvas.Rectangles() {
			if r.Bounds == box.Rect() {
				return r.Color
			}
		}
		t.Fatal("no fill")
		return ui.Color{}
	}
	rest := ui.Color{Alpha: 1}
	if got := run(Look{}.Background(on), Look{}, false); got != on {
		t.Errorf("Focused after a click = %+v, want %+v", got, on)
	}
	if got := run(Look{}, Look{}.Background(on), false); got != rest {
		t.Errorf("Focus after a click = %+v, want the resting fill: a click does not make the focus visible", got)
	}
	if got := run(Look{}, Look{}.Background(on), true); got != on {
		t.Errorf("Focus by the keyboard = %+v, want %+v", got, on)
	}
	both := ui.Color{Blue: .9, Alpha: 1}
	if got := run(Look{}.Background(both), Look{}.Background(on), true); got != on {
		t.Errorf("Focus and Focused both set = %+v, want Focus to win: %+v", got, on)
	}
}
