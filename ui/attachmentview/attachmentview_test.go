package attachmentview_test

import (
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/attachmentview"
)

var (
	cardID   = gfx.Item(71)
	actionID = gfx.Item(72)
	secondID = gfx.Item(73)
	linkID   = gfx.Item(74)
	thirdID  = gfx.Item(75)
)

var testIcon = gfx.Icon{ViewBox: 24, Stroke: 2, Commands: []gfx.PathCommand{
	gfx.MoveTo(4, 4), gfx.LineTo(20, 20),
}}

func draw(page *ui.Page, canvas *gfx.Canvas, view func(ui.C)) {
	canvas.Reset()
	canvas.BeginFrame()
	page.Draw(canvas, gfx.Rect{Width: 640, Height: 320}, view)
	canvas.EndFrame()
}

func drawClick(page *ui.Page, canvas *gfx.Canvas, view func(ui.C), id gfx.ID) {
	canvas.BeginEventPass(id, gfx.ClickInfo{})
	draw(page, canvas, view)
	canvas.EndEventPass()
}

func hit(t *testing.T, canvas *gfx.Canvas, label string) gfx.Hit {
	t.Helper()
	hits := canvas.Hits()
	for index := len(hits) - 1; index >= 0; index-- {
		if hits[index].Label == label {
			return hits[index]
		}
	}
	t.Fatalf("no hit labelled %q was painted", label)
	return gfx.Hit{}
}

func open(t testing.TB, view func(ui.C)) (*ui.Page, *gfx.Canvas) {
	t.Helper()
	theme := gfx.LightTheme()
	page := ui.NewPage(ui.PageConfig{View: view, Theme: &theme})
	canvas := new(gfx.Canvas)
	draw(page, canvas, view)
	return page, canvas
}

type model struct {
	card       attachmentview.Attachment
	group      attachmentview.Group
	cardClicks int
	actionHits int
	cardHit    int
	onCard     func()
	onAction   func()
}

func (m *model) init() {
	m.onCard = func() { m.cardClicks++ }
	m.onAction = func() { m.actionHits++ }
}

func (m *model) configureCard() {
	m.card.Reset().ID(cardID).Title("product-demo.mov").Meta("2.4 MB · Complete").File().
		OnClick(m.onCard)
}

func (m *model) viewCard(c ui.C) {
	m.configureCard()
	m.card.View(c, "product-demo.mov")
}

// The page declares a card, and the component paints its title, meta and
// trigger with the caller's identity inside the control.
func TestViewPaintsCardAndReportsClick(t *testing.T) {
	m := new(model)
	m.init()
	page, canvas := open(t, m.viewCard)

	trigger := hit(t, canvas, "product-demo.mov")
	if trigger.ID != cardID || trigger.Role != gfx.RoleButton {
		t.Fatalf("trigger = %+v, want the caller's button identity", trigger)
	}
	if !hasText(canvas, "2.4 MB · Complete") {
		t.Fatal("the card's meta line was not painted")
	}

	drawClick(page, canvas, m.viewCard, cardID)
	if !m.card.Clicked() {
		t.Fatal("Clicked did not report the clicked card")
	}
	if m.cardClicks != 1 {
		t.Fatalf("OnClick ran %d times, want 1", m.cardClicks)
	}
	draw(page, canvas, m.viewCard)
	if m.card.Clicked() {
		t.Fatal("Clicked reported a click in an ordinary paint")
	}
}

// An action is a button above the card trigger, and its callback and
// ClickedAction both report the caller's action identity.
func TestActionPaintsAndCallsBack(t *testing.T) {
	m := new(model)
	m.init()
	view := func(c ui.C) {
		m.configureCard()
		m.card.Action(actionID, "Remove report.pdf", testIcon, m.onAction)
		m.card.View(c, "product-demo.mov")
	}
	page, canvas := open(t, view)

	action := hit(t, canvas, "Remove report.pdf")
	if action.ID != actionID || action.Role != gfx.RoleButton {
		t.Fatalf("action = %+v, want the caller's action button", action)
	}

	drawClick(page, canvas, view, actionID)
	if m.actionHits != 1 {
		t.Fatalf("action callback ran %d times, want 1", m.actionHits)
	}
	if !m.card.ClickedAction(actionID) || m.card.Clicked() {
		t.Fatal("ClickedAction did not isolate the fired action from the card")
	}
}

// A group paints a row of cards and routes each card and action identity.
func TestGroupPaintsCardsAndCallsBack(t *testing.T) {
	m := new(model)
	m.init()
	view := func(c ui.C) {
		m.group.Reset().Gap(12)
		m.group.Add().ID(cardID).ExtraSmall().Title("notes.md").Meta("18 KB").File().
			Action(actionID, "Remove notes.md", testIcon, m.onAction)
		m.group.Add().ID(secondID).ExtraSmall().Title("cover.png").Meta("18 KB").Image().
			Action(linkID, "Remove cover.png", testIcon, m.onAction)
		m.group.View(c, "Attachments")
	}
	page, canvas := open(t, view)

	if len(hit(t, canvas, "Attachments").Label) == 0 {
		t.Fatal("group control has no label")
	}
	first := hit(t, canvas, "notes.md")
	second := hit(t, canvas, "cover.png")
	if first.ID != cardID || second.ID != secondID {
		t.Fatalf("group cards = %v, %v, want the caller's identities", first.ID, second.ID)
	}

	drawClick(page, canvas, view, linkID)
	if m.actionHits != 1 {
		t.Fatalf("group action callback ran %d times, want 1", m.actionHits)
	}
	if !m.group.Clicked(linkID) || m.group.Clicked(cardID) {
		t.Fatal("Clicked did not isolate the fired group action")
	}
}

func hasText(canvas *gfx.Canvas, value string) bool {
	for _, text := range canvas.Texts() {
		if text.Value == value {
			return true
		}
	}
	return false
}

func TestWarmPaintAllocatesNothing(t *testing.T) {
	m := new(model)
	m.init()
	var group attachmentview.Group
	view := func(c ui.C) {
		m.configureCard()
		m.card.Action(actionID, "Remove product-demo.mov", testIcon, m.onAction)
		m.card.View(c, "product-demo.mov")
		group.Reset().Gap(12)
		group.Add().ID(secondID).ExtraSmall().Title("notes.md").Meta("18 KB").File().
			Action(linkID, "Remove notes.md", testIcon, m.onAction)
		group.Add().ID(thirdID).ExtraSmall().Title("cover.png").Meta("18 KB").Image()
		group.View(c, "Attachments")
	}
	page, canvas := open(t, view)
	if allocations := testing.AllocsPerRun(100, func() { draw(page, canvas, view) }); allocations != 0 {
		t.Fatalf("warm paint allocated %v times, want 0", allocations)
	}
}

func BenchmarkViewPaint(b *testing.B) {
	m := new(model)
	m.init()
	var group attachmentview.Group
	view := func(c ui.C) {
		m.configureCard()
		m.card.Action(actionID, "Remove product-demo.mov", testIcon, m.onAction)
		m.card.View(c, "product-demo.mov")
		group.Reset().Gap(12)
		group.Add().ID(secondID).ExtraSmall().Title("notes.md").Meta("18 KB").File().
			Action(linkID, "Remove notes.md", testIcon, m.onAction)
		group.Add().ID(thirdID).ExtraSmall().Title("cover.png").Meta("18 KB").Image()
		group.View(c, "Attachments")
	}
	page, canvas := open(b, view)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		draw(page, canvas, view)
	}
}
