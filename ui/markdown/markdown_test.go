package markdown_test

import (
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/markdown"
	"github.com/vivaui/viva/window"
)

const source = "## Restarting nginx\n\nCheck the **config** with `nginx -t`, then read the [docs](https://nginx.org/en/docs/).\n\n1. run `systemctl status nginx`\n2. restart it\n"

type model struct {
	view  markdown.State
	input window.TextInput

	target         gfx.ID
	keys, pointers int
	scrolls, texts int
	link           int
	linkOK         bool
	control        *ui.Page
	canvas         *gfx.Canvas
}

func (m *model) wire() {
	m.view.Pointer = func(event gfx.PointerEvent) gfx.EventResult {
		m.pointers++
		m.target = event.Target
		m.link, m.linkOK = m.view.LinkIndex(event.Target)
		return gfx.EventResult{Handled: true}
	}
	m.view.Key = func(event gfx.KeyEvent) gfx.EventResult {
		m.keys++
		m.target = event.Target
		return gfx.EventResult{Handled: true}
	}
	m.view.Scroll = func(event gfx.ScrollEvent) gfx.EventResult {
		m.scrolls++
		m.target = event.Target
		return gfx.EventResult{Handled: true}
	}
	m.view.Text = func(window.TextInputContext) *window.TextInput {
		m.texts++
		return &m.input
	}
}

func (m *model) viewPage(c ui.C) {
	markdown.View(c, "Answer", &m.view).FillWidth().Height(320)
}

func drawPage(page *ui.Page, canvas *gfx.Canvas, view func(ui.C)) {
	canvas.Reset()
	canvas.BeginFrame()
	page.Draw(canvas, gfx.Rect{Width: 480, Height: 360}, view)
	canvas.EndFrame()
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

func open(t *testing.T, m *model) {
	t.Helper()
	m.wire()
	m.view.Source = source
	m.view.Style = markdown.Style{Size: 13, LineHeight: 20}
	theme := gfx.LightTheme()
	m.control = ui.NewPage(ui.PageConfig{View: m.viewPage, Theme: &theme})
	m.canvas = new(gfx.Canvas)
	drawPage(m.control, m.canvas, m.viewPage)
}

// The page declares the view, the wrapper parses and lays the source out, and
// the renderer paints its text and the control's link regions.
func TestMarkdownViewPaintsTextAndControl(t *testing.T) {
	m := new(model)
	open(t, m)

	hit, ok := hitLabelled(m.canvas, "Answer")
	if !ok {
		t.Fatal("the markdown view painted no control named Answer")
	}
	if hit.Role != gfx.RoleGroup {
		t.Fatalf("control role = %v, want group", hit.Role)
	}
	if hit.Bounds.Width != 480 || hit.Bounds.Height != 320 {
		t.Fatalf("control bounds = %+v, want a 480x320 box", hit.Bounds)
	}
	if m.view.TextLen() == 0 || m.view.Height() == 0 {
		t.Fatalf("document = %d text bytes, height %g; want a laid-out document", m.view.TextLen(), m.view.Height())
	}
	if got := m.view.SelectedText(); got != "" {
		t.Fatalf("empty selection copied %q", got)
	}
	if m.view.LinkCount() != 1 {
		t.Fatalf("link count = %d, want 1", m.view.LinkCount())
	}
	link, ok := hitLabelled(m.canvas, "docs")
	if !ok {
		t.Fatal("the renderer painted no hit region labelled docs")
	}
	if link.Role != gfx.RoleLink {
		t.Fatalf("link role = %v, want link", link.Role)
	}
	if index, ok := m.view.LinkIndex(link.ID); !ok || index != 0 {
		t.Fatalf("LinkIndex(%v) = %d, %v; want 0, true", link.ID, index, ok)
	}
	if got := m.view.Link(0); got != "https://nginx.org/en/docs/" {
		t.Fatalf("Link(0) = %q", got)
	}
	if _, ok := m.view.OffsetAt(gfx.Point{X: hit.Bounds.X + 8, Y: hit.Bounds.Y + 8}); !ok {
		t.Fatal("OffsetAt found no text after paint")
	}
}

// Pointer, keys, the wheel and the text service reach the callbacks, and a link
// target is still routed to the control that owns the link.
func TestMarkdownViewRoutesInput(t *testing.T) {
	m := new(model)
	open(t, m)

	hit, ok := hitLabelled(m.canvas, "Answer")
	if !ok {
		t.Fatal("the markdown view painted no control named Answer")
	}
	if result := m.control.Pointer(gfx.PointerEvent{
		Kind: gfx.PointerDown, Button: gfx.PointerButtonPrimary, Target: hit.ID,
		X: hit.Bounds.X + 8, Y: hit.Bounds.Y + 8,
	}); !result.Handled || m.pointers != 1 || m.target != hit.ID {
		t.Fatalf("pointer result = %+v after %d callbacks targeting %v", result, m.pointers, m.target)
	}
	link, _ := hitLabelled(m.canvas, "docs")
	if result := m.control.Pointer(gfx.PointerEvent{
		Kind: gfx.PointerDown, Button: gfx.PointerButtonPrimary, Target: link.ID,
		X: link.Bounds.X + 1, Y: link.Bounds.Y + 1,
	}); !result.Handled || m.pointers != 2 {
		t.Fatalf("link pointer result = %+v after %d callbacks", result, m.pointers)
	}
	if !m.linkOK || m.link != 0 {
		t.Fatalf("link target = %d, %v; want link 0", m.link, m.linkOK)
	}
	if result := m.control.Key(gfx.KeyEvent{Key: gfx.KeyDown, Target: hit.ID}); !result.Handled || m.keys != 1 {
		t.Fatalf("key result = %+v after %d callbacks", result, m.keys)
	}
	if result := m.control.Wheel(gfx.ScrollEvent{Target: hit.ID, Y: 12}); !result.Handled || m.scrolls != 1 {
		t.Fatalf("wheel result = %+v after %d callbacks", result, m.scrolls)
	}
	if m.control.TextInput(window.TextInputContext{ID: hit.ID}) == nil || m.texts != 1 {
		t.Fatalf("text service got no input; callbacks = %d", m.texts)
	}
}

// A warm repaint parses nothing and lays nothing out: the wrapper caches both
// by source, style and width.
func TestMarkdownViewWarmPaintAllocatesNothing(t *testing.T) {
	m := new(model)
	open(t, m)
	if allocations := testing.AllocsPerRun(100, func() { drawPage(m.control, m.canvas, m.viewPage) }); allocations != 0 {
		t.Fatalf("warm paint allocated %v times, want 0", allocations)
	}
}

func BenchmarkMarkdownViewWarmPaint(b *testing.B) {
	m := new(model)
	m.wire()
	m.view.Source = source
	m.view.Style = markdown.Style{Size: 13, LineHeight: 20}
	theme := gfx.LightTheme()
	m.control = ui.NewPage(ui.PageConfig{View: m.viewPage, Theme: &theme})
	m.canvas = new(gfx.Canvas)
	drawPage(m.control, m.canvas, m.viewPage)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		drawPage(m.control, m.canvas, m.viewPage)
	}
}
