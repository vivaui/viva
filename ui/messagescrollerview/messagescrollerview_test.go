package messagescrollerview_test

import (
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/messagescrollerview"
)

var (
	testEarlier   = gfx.Item(71)
	testLater     = gfx.Item(72)
	testScrollbar = gfx.Item(73)

	testLabels = [...]string{
		"Project started", "Build complete", "Review requested",
		"Changes approved", "Release ready",
	}
)

type model struct {
	view    messagescrollerview.Scroller
	page    *ui.Page
	canvas  *gfx.Canvas
	geom    messagescrollerview.Geometry
	offsets []float32
	jumps   []messagescrollerview.Direction
	targets []float32
}

func (m *model) content(c ui.C) {
	for _, label := range testLabels {
		ui.Row(c).Height(62).Children(func() {
			ui.Text(c, label)
		})
	}
}

func (m *model) wire() {
	m.view = messagescrollerview.Scroller{
		EarlierLabel:   "Earlier messages",
		LaterLabel:     "Later messages",
		EarlierID:      testEarlier,
		LaterID:        testLater,
		ScrollbarID:    testScrollbar,
		ScrollbarLabel: "Message position",
		ScrollbarValue: "Conversation",
		Content:        m.content,
	}
	m.view.OnScroll = func(offset float32) {
		m.offsets = append(m.offsets, offset)
		m.view.Offset = offset
	}
	m.view.OnJump = func(direction messagescrollerview.Direction, offset float32) {
		m.jumps = append(m.jumps, direction)
		m.targets = append(m.targets, offset)
		m.view.Offset = offset
	}
}

func (m *model) viewPage(c ui.C) {
	messagescrollerview.View(c, "Message stream", &m.view).FillWidth().Height(200)
}

func (m *model) draw() {
	m.canvas.Reset()
	m.canvas.BeginFrame()
	m.page.Draw(m.canvas, gfx.Rect{Width: 400, Height: 300}, m.viewPage)
	m.canvas.EndFrame()
	m.geom = m.view.Geometry()
}

func (m *model) fresh(t testing.TB) *model {
	t.Helper()
	m.wire()
	theme := gfx.LightTheme()
	m.page = ui.NewPage(ui.PageConfig{View: m.viewPage, Theme: &theme})
	m.canvas = new(gfx.Canvas)
	m.draw()
	m.draw() // the first paint measures the content and enables the controls
	return m
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

func hit(t *testing.T, canvas *gfx.Canvas, label string) gfx.Hit {
	t.Helper()
	got, ok := hitLabelled(canvas, label)
	if !ok {
		t.Fatalf("no hit labelled %q was painted", label)
	}
	return got
}

// The view declares the page's own scroll area and the caller's identities:
// the stream region, both jump controls and the named scrollbar. The content
// extent and viewport come from the page's layout.
func TestViewDeclaresControlledStream(t *testing.T) {
	m := new(model).fresh(t)

	stream := hit(t, m.canvas, "Message stream")
	if stream.Role != gfx.RoleGroup {
		t.Fatalf("stream role = %v, want group", stream.Role)
	}
	earlier := hit(t, m.canvas, "Earlier messages")
	later := hit(t, m.canvas, "Later messages")
	if earlier.ID != testEarlier || later.ID != testLater {
		t.Fatalf("jump hits = %v %v, want %v %v", earlier.ID, later.ID, testEarlier, testLater)
	}
	if !earlier.Disabled || later.Disabled {
		t.Fatalf("jump hits = %+v %+v, want earlier disabled and later enabled", earlier, later)
	}
	bar := hit(t, m.canvas, "Message position")
	if bar.ID != testScrollbar || bar.Role != gfx.RoleSeparator || bar.Value != "Conversation" {
		t.Fatalf("scrollbar = %+v, want the caller's named bar with its value", bar)
	}

	if m.geom.ContentExtent != 310 || m.geom.ViewportExtent <= 0 {
		t.Fatalf("geometry = %+v, want 310 content in a bounded viewport", m.geom)
	}
	if m.geom.Offset != 0 || m.geom.MaxOffset != 310-m.geom.ViewportExtent {
		t.Fatalf("geometry offset = %g/%g, want 0 of %g", m.geom.Offset, m.geom.MaxOffset, 310-m.geom.ViewportExtent)
	}
	if m.geom.CanEarlier || !m.geom.CanLater {
		t.Fatalf("geometry directions = %v/%v, want only later", m.geom.CanEarlier, m.geom.CanLater)
	}
	center := gfx.Point{X: m.geom.Viewport.X + m.geom.Viewport.Width/2, Y: m.geom.Viewport.Y + m.geom.Viewport.Height/2}
	if !m.view.Contains(center.X, center.Y) || m.view.Contains(-1, -1) {
		t.Fatalf("Contains = %v/%v, want the viewport but not the page corner",
			m.view.Contains(center.X, center.Y), m.view.Contains(-1, -1))
	}
	if m.view.SettledOffset() != 0 || m.view.ContentHeight() != 310 || m.view.ViewportHeight() != m.geom.ViewportExtent {
		t.Fatalf("settled = %g content %g viewport %g, want 0/310/%g",
			m.view.SettledOffset(), m.view.ContentHeight(), m.view.ViewportHeight(), m.geom.ViewportExtent)
	}
	for _, label := range testLabels {
		if _, ok := m.viewText(label); !ok {
			t.Fatalf("the content row %q was not painted", label)
		}
	}
}

func (m *model) viewText(value string) (gfx.Text, bool) {
	for _, text := range m.canvas.Texts() {
		if text.Value == value {
			return text, true
		}
	}
	return gfx.Text{}, false
}

// The wheel reports the clamped offset the caller stores; the next paint shows
// it and the earlier control becomes available.
func TestWheelReportsOffset(t *testing.T) {
	m := new(model).fresh(t)

	result := m.view.Wheel(gfx.ScrollEvent{DeltaY: 40, Precise: true})
	if !result.Handled || len(m.offsets) != 1 || m.offsets[0] != 40 {
		t.Fatalf("wheel = %+v, offsets %v; want a handled 40", result, m.offsets)
	}
	m.draw()
	if m.geom.Offset != 40 || !m.geom.CanEarlier || !m.geom.CanLater {
		t.Fatalf("geometry after wheel = %+v, want offset 40 between the ends", m.geom)
	}
}

// A click on a jump control reports the target offset and direction, and an
// ordinary paint reports nothing.
func TestJumpReportsTarget(t *testing.T) {
	m := new(model).fresh(t)
	target := m.geom.MaxOffset

	m.canvas.BeginEventPass(testLater, gfx.ClickInfo{})
	m.canvas.Reset()
	m.canvas.BeginFrame()
	m.page.Draw(m.canvas, gfx.Rect{Width: 400, Height: 300}, m.viewPage)
	m.canvas.EndFrame()
	m.canvas.EndEventPass()
	if len(m.jumps) != 1 || m.jumps[0] != messagescrollerview.Later ||
		len(m.targets) != 1 || m.targets[0] != target {
		t.Fatalf("jump = %v targets %v, want one later target %g", m.jumps, m.targets, target)
	}

	m.draw()
	if len(m.jumps) != 1 {
		t.Fatalf("an ordinary paint reported another jump: %v", m.jumps)
	}
	if m.geom.Offset != target || !m.geom.CanEarlier || m.geom.CanLater {
		t.Fatalf("geometry after jump = %+v, want the end", m.geom)
	}
}

// A warm repaint declares the same stream and allocates nothing.
func TestWarmPaintAllocatesNothing(t *testing.T) {
	m := new(model).fresh(t)
	if allocations := testing.AllocsPerRun(100, m.draw); allocations != 0 {
		t.Fatalf("warm paint allocated %v times, want 0", allocations)
	}
}

func BenchmarkViewPaint(b *testing.B) {
	m := new(model).fresh(b)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		m.draw()
	}
}
