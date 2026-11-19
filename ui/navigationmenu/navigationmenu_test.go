package navigationmenu_test

import (
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/navigationmenu"
)

var (
	productsID = gfx.Item(1)
	pricingID  = gfx.Item(2)
	docsID     = gfx.Item(3)
	guideID    = gfx.Item(4)
	apiID      = gfx.Item(5)
	dismissID  = gfx.Item(6)
)

type model struct {
	menu navigationmenu.NavigationMenu

	open     gfx.ID
	direct   bool
	page     string
	selected gfx.ID

	pointerTarget gfx.ID
	keyTarget     gfx.ID
	keys          int
}

func (m *model) wire() {
	m.menu.Scope = gfx.Item(80)
	m.menu.DismissID = dismissID
	m.menu.Pointer = func(event gfx.PointerEvent) gfx.EventResult {
		m.pointerTarget = event.Target
		return gfx.EventResult{Handled: true, Repaint: true}
	}
	m.menu.Key = func(event gfx.KeyEvent) gfx.EventResult {
		m.keys++
		m.keyTarget = event.Target
		return gfx.EventResult{Handled: true}
	}
}

func (m *model) view(c ui.C) {
	m.menu.DirectContent = m.direct
	if m.menu.Selected == guideID {
		m.page = "guide"
	} else if m.menu.Selected == apiID {
		m.page = "components"
	} else if m.menu.Selected == docsID {
		m.page = "docs"
	}
	m.menu.Reset()
	m.menu.Trigger(productsID, "Products").Hint("Open product navigation")
	m.menu.Link(docsID, "Docs").Value("/docs").Active(m.page == "docs")
	m.menu.ContentLink(guideID, "Getting started").
		Description("Build a native window and retained scene.").
		Value("/guide").Active(m.page == "guide")
	m.menu.ContentLink(apiID, "Component API").
		Description("Read the source-owned recipes.").
		Value("/components").Active(m.page == "components")
	m.menu.Trigger(pricingID, "Pricing")
	m.menu.View(c, "Navigation menu").Width(520).Height(36)
}

func openPage(t *testing.T, m *model) (*ui.Page, *gfx.Canvas) {
	t.Helper()
	m.wire()
	theme := gfx.LightTheme()
	page := ui.NewPage(ui.PageConfig{View: m.view, Theme: &theme})
	canvas := benchCanvas()
	draw(page, canvas, m.view)
	return page, canvas
}

func benchCanvas() *gfx.Canvas {
	canvas := &gfx.Canvas{}
	canvas.SetMeasurer(func(value string, size, weight float32) float32 { return float32(len(value)) * size / 2 })
	return canvas
}

func draw(page *ui.Page, canvas *gfx.Canvas, view func(ui.C)) {
	canvas.Reset()
	canvas.BeginFrame()
	page.Draw(canvas, gfx.Rect{Width: 600, Height: 400}, view)
	canvas.EndFrame()
}

func drawClick(page *ui.Page, canvas *gfx.Canvas, view func(ui.C), id gfx.ID) {
	canvas.BeginEventPass(id, gfx.ClickInfo{})
	draw(page, canvas, view)
	canvas.EndEventPass()
}

func hitLabel(canvas *gfx.Canvas, label string) (gfx.Hit, bool) {
	hits := canvas.Hits()
	for index := len(hits) - 1; index >= 0; index-- {
		hit := hits[index]
		if hit.Label == label && !hit.ID.IsZero() && !hit.Backdrop {
			return hit, true
		}
	}
	return gfx.Hit{}, false
}

func mustHit(t *testing.T, canvas *gfx.Canvas, label string) gfx.Hit {
	t.Helper()
	hit, ok := hitLabel(canvas, label)
	if !ok {
		t.Fatalf("no hit labelled %q", label)
	}
	return hit
}

func TestNavigationMenuOpensAndChoosesALink(t *testing.T) {
	m := &model{}
	page, canvas := openPage(t, m)
	mustHit(t, canvas, "Products")
	mustHit(t, canvas, "Docs")
	if _, ok := hitLabel(canvas, "Getting started"); ok {
		t.Fatal("a closed navigation menu painted its panel links")
	}

	drawClick(page, canvas, m.view, productsID)
	if m.menu.Open != productsID {
		t.Fatalf("open = %v, want Products", m.menu.Open)
	}
	draw(page, canvas, m.view)
	for _, label := range []string{"Getting started", "Component API"} {
		mustHit(t, canvas, label)
	}

	row := mustHit(t, canvas, "Getting started")
	drawClick(page, canvas, m.view, row.ID)
	if m.menu.Selected != guideID {
		t.Fatalf("selected = %v, want guide", m.menu.Selected)
	}
}

func TestNavigationMenuDirectContent(t *testing.T) {
	m := &model{direct: true}
	m.menu.Open = productsID
	_, canvas := openPage(t, m)
	if _, ok := hitLabel(canvas, "Getting started"); !ok {
		t.Fatal("direct content did not paint the panel links")
	}
}

func TestNavigationMenuKeysMoveOpenAndChoose(t *testing.T) {
	m := &model{}
	m.menu.Open = productsID
	page, canvas := openPage(t, m)
	control := mustHit(t, canvas, "Navigation menu")
	products := mustHit(t, canvas, "Products")
	guide := mustHit(t, canvas, "Getting started")

	result, ok := page.RouteKey(gfx.KeyEvent{Key: gfx.KeyDown, Target: control.ID})
	if !ok || !result.Handled || m.menu.Open != productsID {
		t.Fatalf("Down = %+v, %v; open %v", result, ok, m.menu.Open)
	}
	if result.Focus != guide.ID {
		t.Fatalf("Down focused %v, want the first panel link", result.Focus)
	}
	if m.keyTarget != control.ID {
		t.Fatalf("the key callback saw %v, want the control", m.keyTarget)
	}

	pricing := mustHit(t, canvas, "Pricing")
	m.menu.Focus = productsID
	result, _ = page.RouteKey(gfx.KeyEvent{Key: gfx.KeyRight, Target: control.ID})
	if m.menu.Open != pricingID || result.Focus != pricing.ID {
		t.Fatalf("Right opened %v and focused %v, want Pricing", m.menu.Open, result.Focus)
	}

	m.menu.Open = productsID
	m.menu.Focus = guideID
	draw(page, canvas, m.view)
	guide = mustHit(t, canvas, "Getting started")
	result, _ = page.RouteKey(gfx.KeyEvent{Key: gfx.KeyEnter, Target: control.ID})
	if result.Activate != guide.ID || m.menu.Selected != guideID {
		t.Fatalf("Enter activated %v and selected %v, want Getting started", result.Activate, m.menu.Selected)
	}

	result, _ = page.RouteKey(gfx.KeyEvent{Key: gfx.KeyEscape, Target: control.ID})
	if !m.menu.Open.IsZero() || result.Focus != products.ID {
		t.Fatalf("Escape left %v open and focused %v, want Products", m.menu.Open, result.Focus)
	}
}

func TestNavigationMenuTopLevelLinkActivates(t *testing.T) {
	m := &model{}
	page, canvas := openPage(t, m)
	control := mustHit(t, canvas, "Navigation menu")
	docs := mustHit(t, canvas, "Docs")
	m.menu.Focus = docsID
	result, _ := page.RouteKey(gfx.KeyEvent{Key: gfx.KeyEnter, Target: control.ID})
	if result.Activate != docs.ID || m.menu.Selected != docsID {
		t.Fatalf("Enter activated %v and selected %v, want Docs", result.Activate, m.menu.Selected)
	}
}

func TestNavigationMenuPointerTranslation(t *testing.T) {
	m := &model{}
	m.menu.Open = productsID
	page, canvas := openPage(t, m)
	control := mustHit(t, canvas, "Navigation menu")
	page.Pointer(gfx.PointerEvent{Kind: gfx.PointerDown, Target: control.ID, X: control.Bounds.X + 4, Y: control.Bounds.Y + 4})
	if m.pointerTarget != control.ID {
		t.Fatalf("the pointer callback saw %v, want the control", m.pointerTarget)
	}
}

func BenchmarkNavigationMenuPaint(b *testing.B) {
	m := &model{page: "guide"}
	m.menu.Open = productsID
	theme := gfx.LightTheme()
	page := ui.NewPage(ui.PageConfig{View: m.view, Theme: &theme})
	canvas := benchCanvas()
	view := m.view
	draw(page, canvas, view)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		draw(page, canvas, view)
	}
}
