package alert_test

import (
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/alert"
)

var shown int // the paints in which ActionClicked was true

func open(t *testing.T, a *alert.Alert, spec alert.Spec) *headless.Session {
	t.Helper()
	shown = 0
	page := ui.NewPage(ui.PageConfig{View: func(c ui.C) {
		ui.Column(c).Pad(16).Children(func() {
			a.View(c, spec).FillWidth()
			if a.ActionClicked() {
				shown++
			}
			ui.Button(c, "After")
		})
	}})
	session, err := headless.Open(ui.Configure(page, app.Config{Title: "Alert", Width: 480, Height: 240}), nil, headless.Options{Scale: 1})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(session.Close)
	return session
}

// The notice announces its title as a status, with the words it was given.
func TestAlertIsAStatusNamedByItsTitle(t *testing.T) {
	session := open(t, alert.New(), alert.Spec{Status: ui.StatusWarning, Title: "Host key changed", Description: "Review it."})
	node, ok := session.AccessByName("Host key changed")
	if !ok || node.Role != gfx.RoleStatus {
		t.Fatalf("access node = %+v, %v; want a status named by the title", node, ok)
	}
	if err := session.Expect("text", "Review it."); err != nil {
		t.Fatal(err)
	}
}

// A press on the action is reported once, in the paint that delivered it.
func TestActionClickIsReported(t *testing.T) {
	session := open(t, alert.New(), alert.Spec{Title: "Update", Action: "Review"})
	if err := session.ClickLabel("Review"); err != nil {
		t.Fatal(err)
	}
	if shown != 1 {
		t.Fatalf("ActionClicked was true in %d paints after one press, want 1", shown)
	}
}

// The close button closes the notice for good, until it is reopened.
func TestCloseButtonClosesTheNotice(t *testing.T) {
	a := alert.New()
	session := open(t, a, alert.Spec{Title: "Update", Closable: true})
	if err := session.ClickLabel("Close"); err != nil {
		t.Fatal(err)
	}
	if !a.Closed() {
		t.Fatal("Closed was false after the close button was pressed")
	}
	if _, ok := session.AccessByName("Update"); ok {
		t.Fatal("a closed notice is still in the accessibility tree")
	}
	a.Reopen()
	if err := session.Frame(); err != nil {
		t.Fatal(err)
	}
	if _, ok := session.AccessByName("Update"); !ok {
		t.Fatal("a reopened notice is not shown again")
	}
}

// A notice with no close button or action offers neither.
func TestPlainNoticeHasNoButtons(t *testing.T) {
	session := open(t, alert.New(), alert.Spec{Title: "Saved", Status: ui.StatusSuccess})
	for _, name := range []string{"Close", "Review"} {
		if _, err := session.Locate(name); err == nil {
			t.Fatalf("a plain notice offers %q", name)
		}
	}
}

func TestWarmPaintAllocatesNothing(t *testing.T) {
	for name, spec := range map[string]alert.Spec{
		"plain":  {Title: "Saved", Description: "All changes were written.", Status: ui.StatusSuccess},
		"full":   {Title: "Update", Description: "A new version is ready.", Action: "Review", Closable: true, Status: ui.StatusWarning},
		"banner": {Title: "Offline", Banner: true, Status: ui.StatusDanger},
	} {
		t.Run(name, func(t *testing.T) {
			a := alert.New()
			view := func(c ui.C) { a.View(c, spec).FillWidth() }
			theme := gfx.LightTheme()
			page := ui.NewPage(ui.PageConfig{View: view, Theme: &theme})
			canvas := &gfx.Canvas{}
			canvas.SetReduceMotion(true)
			draw := func() {
				canvas.BeginFrame()
				canvas.Reset()
				page.Draw(canvas, gfx.Rect{Width: 480, Height: 240}, view)
				canvas.EndFrame()
			}
			draw()
			if allocations := testing.AllocsPerRun(100, draw); allocations != 0 {
				t.Fatalf("warm paint allocated %v times, want 0", allocations)
			}
		})
	}
}

func BenchmarkViewPaint(b *testing.B) {
	a := alert.New()
	spec := alert.Spec{Title: "Update", Description: "A new version is ready.", Action: "Review", Closable: true, Status: ui.StatusWarning}
	view := func(c ui.C) { a.View(c, spec).FillWidth() }
	theme := gfx.LightTheme()
	page := ui.NewPage(ui.PageConfig{View: view, Theme: &theme})
	canvas := &gfx.Canvas{}
	canvas.SetReduceMotion(true)
	b.ReportAllocs()
	for range b.N {
		canvas.BeginFrame()
		canvas.Reset()
		page.Draw(canvas, gfx.Rect{Width: 480, Height: 240}, view)
		canvas.EndFrame()
	}
}
