package messageview_test

import (
	"strings"
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/messageview"
)

type model struct {
	view   messageview.Message
	page   *ui.Page
	canvas *gfx.Canvas
	body   int
	action int
}

func (m *model) viewPage(c ui.C) {
	messageview.View(c, "Incoming message", m.view).FillWidth()
}

func draw(page *ui.Page, canvas *gfx.Canvas, view func(ui.C)) {
	canvas.Reset()
	canvas.BeginFrame()
	page.Draw(canvas, gfx.Rect{Width: 400, Height: 300}, view)
	canvas.EndFrame()
}

func open(t testing.TB, m *model) {
	t.Helper()
	theme := gfx.LightTheme()
	m.page = ui.NewPage(ui.PageConfig{View: m.viewPage, Theme: &theme})
	m.canvas = new(gfx.Canvas)
	draw(m.page, m.canvas, m.viewPage)
}

func hitLabelled(canvas *gfx.Canvas, label string) (gfx.Hit, bool) {
	hits := canvas.Hits()
	for i := len(hits) - 1; i >= 0; i-- {
		if hits[i].Label == label && !hits[i].ID.IsZero() {
			return hits[i], true
		}
	}
	return gfx.Hit{}, false
}

// textNamed returns the painted text with the value.
func textNamed(canvas *gfx.Canvas, value string) (gfx.Text, bool) {
	for _, text := range canvas.Texts() {
		if text.Value == value {
			return text, true
		}
	}
	return gfx.Text{}, false
}

func requireText(t *testing.T, canvas *gfx.Canvas, value string) gfx.Text {
	t.Helper()
	text, ok := textNamed(canvas, value)
	if !ok {
		var painted []string
		for _, text := range canvas.Texts() {
			painted = append(painted, text.Value)
		}
		t.Fatalf("%q was not painted; painted %q", value, strings.Join(painted, " | "))
	}
	return text
}

// The page declares one row; the view paints the avatar, name, time and body and
// names the row for assistive technology.
func TestViewPaintsVisibleData(t *testing.T) {
	m := &model{view: messageview.New().
		Name("Ari · Design").Time("09:42").Avatar("AR").
		Body("Native preview ready.")}
	open(t, m)

	row, ok := hitLabelled(m.canvas, "Incoming message")
	if !ok {
		t.Fatal("the message row painted no labelled region")
	}
	if row.Role != gfx.RoleGroup {
		t.Fatalf("row role = %v, want group", row.Role)
	}
	initials := requireText(t, m.canvas, "AR")
	name := requireText(t, m.canvas, "Ari · Design")
	time := requireText(t, m.canvas, "09:42")
	body := requireText(t, m.canvas, "Native preview ready.")
	if initials.Bounds.X >= body.Bounds.X {
		t.Fatalf("start avatar x = %g, body x = %g; want the avatar first", initials.Bounds.X, body.Bounds.X)
	}
	if name.Bounds.Y >= body.Bounds.Y || time.Bounds.Y >= body.Bounds.Y {
		t.Fatalf("metadata y = %g/%g, body y = %g; want metadata above", name.Bounds.Y, time.Bounds.Y, body.Bounds.Y)
	}
	if name.Bounds.X >= time.Bounds.X {
		t.Fatalf("name x = %g, time x = %g; want the name first", name.Bounds.X, time.Bounds.X)
	}
}

// End mirrors the row: the avatar moves to the end and the bubble hugs the end
// edge.
func TestEndMirrorsTheRow(t *testing.T) {
	start := &model{view: messageview.New().Avatar("AR").Body("Native preview ready.")}
	open(t, start)
	startInitials := requireText(t, start.canvas, "AR")
	startBody := requireText(t, start.canvas, "Native preview ready.")
	if startInitials.Bounds.X >= startBody.Bounds.X {
		t.Fatalf("start avatar x = %g, body x = %g; want the avatar first", startInitials.Bounds.X, startBody.Bounds.X)
	}

	end := &model{view: messageview.New().End().Avatar("YO").Body("Ship the compact variant.")}
	open(t, end)
	endInitials := requireText(t, end.canvas, "YO")
	endBody := requireText(t, end.canvas, "Ship the compact variant.")
	if endInitials.Bounds.X <= endBody.Bounds.X {
		t.Fatalf("end avatar x = %g, body x = %g; want the avatar last", endInitials.Bounds.X, endBody.Bounds.X)
	}
	if endBody.Bounds.X <= startBody.Bounds.X {
		t.Fatalf("end body x = %g, start body x = %g; want the end body farther along", endBody.Bounds.X, startBody.Bounds.X)
	}
}

// Content replaces the bubble body and Actions add a row below it.
func TestContentAndActions(t *testing.T) {
	m := &model{}
	m.view = messageview.New().Avatar("AR").Body("this text is replaced").
		Content(func(c ui.C) {
			m.body++
			ui.Text(c, "Rich body")
		}).
		Actions(func(c ui.C) {
			m.action++
			ui.Text(c, "Reply")
		})
	open(t, m)

	requireText(t, m.canvas, "Rich body")
	requireText(t, m.canvas, "Reply")
	if _, ok := textNamed(m.canvas, "this text is replaced"); ok {
		t.Fatal("Content did not replace Body")
	}
	if m.body != 1 || m.action != 1 {
		t.Fatalf("callbacks ran %d/%d times, want once each", m.body, m.action)
	}
}

// A reserved avatar keeps the column without painting initials, and Group
// paints every row it is given.
func TestReservedAvatarAndGroup(t *testing.T) {
	theme := gfx.LightTheme()
	var canvas gfx.Canvas
	view := func(c ui.C) {
		messageview.Group(c, "Ari",
			messageview.New().ReserveAvatar().Body("First line."),
			messageview.New().Avatar("AR").Body("Second line."),
		).FillWidth()
	}
	page := ui.NewPage(ui.PageConfig{Theme: &theme, View: view})
	draw(page, &canvas, view)

	requireText(t, &canvas, "First line.")
	requireText(t, &canvas, "Second line.")
	if _, ok := textNamed(&canvas, "AR"); !ok {
		t.Fatal("the grouped row with an avatar did not paint its initials")
	}
	if _, ok := hitLabelled(&canvas, "Ari"); !ok {
		t.Fatal("the group painted no labelled region")
	}
}

// A warm repaint declares the same values and allocates nothing.
func TestWarmPaintAllocatesNothing(t *testing.T) {
	m := &model{view: messageview.New().
		Name("Ari · Design").Time("09:42").Avatar("AR").
		Body("Native preview ready.")}
	open(t, m)
	if allocations := testing.AllocsPerRun(100, func() { draw(m.page, m.canvas, m.viewPage) }); allocations != 0 {
		t.Fatalf("warm paint allocated %v times, want 0", allocations)
	}
}

func BenchmarkViewPaint(b *testing.B) {
	m := &model{view: messageview.New().
		Name("Ari · Design").Time("09:42").Avatar("AR").
		Body("Native preview ready.")}
	open(b, m)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		draw(m.page, m.canvas, m.viewPage)
	}
}
