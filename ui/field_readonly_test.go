package ui_test

import (
	"errors"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
	"github.com/vivaui/viva/ui"
)

// A read-only TextField keeps its value and takes the focus, but the platform is
// given no editor: typing and Enter do nothing. Both the chained option and the
// Field state reach the hit the platform reads.
func TestPageTextFieldReadOnly(t *testing.T) {
	var chained, state ui.Field
	chained.SetText("fixed")
	state.SetText("locked")
	state.ReadOnly = true
	d := openPage(t, func(c ui.C) {
		ui.Column(c).Pad(16).Gap(8).Children(func() {
			ui.TextField(c, &chained, "Chained", "").ReadOnly(true)
			ui.TextField(c, &state, "State", "")
		})
	})
	for _, label := range []string{"Chained", "State"} {
		hit := d.hit(label)
		if !hit.ReadOnly || !hit.Focusable {
			t.Fatalf("%s hit = %+v, want a focusable read-only field", label, hit)
		}
		d.click(label)
		if focused := d.Focused(); focused != label {
			t.Fatalf("focus = %q, want %q", focused, label)
		}
		if err := d.driver.Type("x"); !errors.Is(err, app.ErrNoTextInput) {
			t.Fatalf("%s accepted typing: %v", label, err)
		}
	}
	if chained.Text() != "fixed" || state.Text() != "locked" {
		t.Fatalf("read-only texts = %q and %q", chained.Text(), state.Text())
	}
}

// A read-only TextArea is the same: Enter, typing and the row keys leave its
// editor alone.
func TestPageTextAreaReadOnly(t *testing.T) {
	var notes ui.Field
	notes.SetText("one\ntwo")
	d := openPage(t, func(c ui.C) {
		ui.Column(c).Pad(16).Children(func() {
			ui.TextArea(c, &notes, "Notes", "").Height(120).ReadOnly(true)
		})
	})
	hit := d.hit("Notes")
	if !hit.ReadOnly || !hit.Focusable {
		t.Fatalf("hit = %+v, want a focusable read-only textarea", hit)
	}
	d.click("Notes")
	if err := d.driver.Type("x"); !errors.Is(err, app.ErrNoTextInput) {
		t.Fatalf("read-only textarea accepted typing: %v", err)
	}
	d.key(gfx.KeyEnter)
	if notes.Text() != "one\ntwo" {
		t.Fatalf("read-only textarea text = %q", notes.Text())
	}
}

// Field.ReadOnly reaches the textarea snapshot too, and the field's invalid and
// disabled state is painted as components.PaintTextarea paints it.
func TestPageTextAreaStateAndReadOnly(t *testing.T) {
	var invalid, disabled ui.Field
	invalid.SetText("too short")
	invalid.Invalid = true
	invalid.ReadOnly = true
	disabled.SetText("unavailable")
	disabled.Disabled = true
	view := func(c ui.C) {
		ui.Column(c).Pad(16).Gap(8).Children(func() {
			ui.TextAreaID(c, gfx.Item(1), &invalid, "Invalid", "").Height(104)
			ui.TextAreaID(c, gfx.Item(2), &disabled, "Disabled", "").Height(104)
		})
	}
	page := ui.NewPage(ui.PageConfig{View: view})
	canvas := benchCanvas()
	drawPage(page, canvas, view)
	hits := canvas.Hits()
	if len(hits) != 2 {
		t.Fatalf("hits = %d, want 2", len(hits))
	}
	if !hits[0].ReadOnly || hits[0].Disabled {
		t.Fatalf("invalid read-only hit = %+v", hits[0])
	}
	if !hits[1].Disabled || hits[1].ReadOnly {
		t.Fatalf("disabled hit = %+v", hits[1])
	}
}

// Field.Select gives a caller-owned editor's selection to the page: a read-only
// field then paints it without offering editing.
func TestFieldSelect(t *testing.T) {
	var field ui.Field
	field.SetText("abcdef")
	field.Select(1, 4)
	editor := field.GetEditor()
	if editor == nil {
		t.Fatal("Select did not ensure the field's editor")
	}
	if got := editor.Selection(); got != (text.Range{Start: 1, End: 4}) {
		t.Fatalf("selection = %+v", got)
	}
}

// The read-only page path allocates nothing once warm: it is the same snapshot
// and the same components paint as an editable field.
func TestPageReadOnlyAllocatesNothing(t *testing.T) {
	var name, notes ui.Field
	name.SetText("value")
	name.ReadOnly = true
	notes.SetText("rows")
	notes.ReadOnly = true
	view := func(c ui.C) {
		ui.Column(c).Pad(16).Gap(8).Children(func() {
			ui.TextField(c, &name, "Name", "").ReadOnly(true)
			ui.TextArea(c, &notes, "Notes", "").Height(80)
		})
	}
	page := ui.NewPage(ui.PageConfig{View: view})
	canvas := benchCanvas()
	drawPage(page, canvas, view)
	if allocs := testing.AllocsPerRun(100, func() { drawPage(page, canvas, view) }); allocs != 0 {
		t.Fatalf("a warm read-only page allocates %v times a frame, want 0", allocs)
	}
}

type fieldStateModel struct {
	state   ui.FormFieldState
	field   ui.Box
	name    ui.Field
	message string
	error   string
}

func (m *fieldStateModel) View(c ui.C) {
	m.state.Description = "Used for release notices."
	m.state.Required = m.message != "" || m.error != ""
	m.state.Message = m.message
	m.state.Error = m.error
	m.field = ui.FormField(c, &m.state, "Username").Width(300).Children(func() {
		ui.TextField(c, &m.name, "Username", "")
	})
}

// Required paints the title marker, and Message paints a success line in the row
// the field already reserves. An error takes that row instead.
func TestFormFieldRequiredAndMessage(t *testing.T) {
	m := &fieldStateModel{message: "Username is available."}
	m.name.SetText("gopher")
	page := ui.NewPage(ui.PageConfig{View: m.View})
	canvas := benchCanvas()
	drawPage(page, canvas, m.View)

	if !canvasHasText(canvas, "*") {
		t.Fatal("Required did not paint the title marker")
	}
	message := canvasText(canvas, "Username is available.")
	if message == nil {
		t.Fatal("Message was not painted")
	}
	if message.Style.Color != pageTheme.Success {
		t.Fatalf("message color = %+v, want success %+v", message.Style.Color, pageTheme.Success)
	}
	if bounds := message.Bounds; bounds.Y < m.field.Rect().Y+m.field.Rect().Height/2 {
		t.Fatalf("message at %+v is not under the control in field %+v", bounds, m.field.Rect())
	}

	m.error = "Username is taken."
	m.message = ""
	drawPage(page, canvas, m.View)
	if canvasText(canvas, "Username is available.") != nil {
		t.Fatal("an old message survived an error")
	}
	if canvasText(canvas, "Username is taken.") == nil {
		t.Fatal("Error was not painted")
	}
}

type orientationModel struct {
	state   ui.FormFieldState
	field   ui.Box
	control ui.Box
	name    ui.Field
}

func (m *orientationModel) View(c ui.C) {
	m.field = ui.FormField(c, &m.state, "Email").Width(400).Children(func() {
		m.control = ui.TextField(c, &m.name, "Email", "")
	})
}

// A FormField takes the shared field layout's orientation: horizontal puts the
// control beside the title, vertical under it, and responsive picks by width.
func TestFormFieldOrientation(t *testing.T) {
	m := &orientationModel{state: ui.FormFieldState{Orientation: ui.FieldVertical}}
	m.name.SetText("team@viva.dev")
	page := ui.NewPage(ui.PageConfig{View: m.View})
	canvas := benchCanvas()
	drawPage(page, canvas, m.View)
	verticalControl, verticalTitle := m.control.Rect(), m.field.Rect()
	if verticalControl.Y <= verticalTitle.Y+4 {
		t.Fatalf("vertical control at %+v is not under the title", verticalControl)
	}

	m.state.Orientation = ui.FieldHorizontal
	drawPage(page, canvas, m.View)
	if m.control.Rect().X <= m.field.Rect().X+4 {
		t.Fatalf("horizontal control at %+v is not beside the title", m.control.Rect())
	}

	m.state.Orientation = ui.FieldResponsive
	drawPage(page, canvas, m.View)
	if m.control.Rect().X <= m.field.Rect().X+4 {
		t.Fatalf("responsive control at 400 wide is not horizontal: %+v", m.control.Rect())
	}
}

// A warm FormField with the required marker, a message and a responsive
// orientation allocates nothing: the message is painted into the row the shared
// layout already reserved.
func TestFormFieldStateAllocatesNothing(t *testing.T) {
	m := &fieldStateModel{message: "Username is available."}
	m.name.SetText("gopher")
	m.state.Orientation = ui.FieldResponsive
	page := ui.NewPage(ui.PageConfig{View: m.View})
	canvas := benchCanvas()
	drawPage(page, canvas, m.View)
	if allocs := testing.AllocsPerRun(100, func() { drawPage(page, canvas, m.View) }); allocs != 0 {
		t.Fatalf("a warm FormField with Required, Message and Orientation allocates %v times a frame, want 0", allocs)
	}
}

func canvasText(canvas *gfx.Canvas, value string) *gfx.Text {
	for index := range canvas.Texts() {
		if canvas.Texts()[index].Value == value {
			return &canvas.Texts()[index]
		}
	}
	return nil
}

func canvasHasText(canvas *gfx.Canvas, value string) bool { return canvasText(canvas, value) != nil }
