//go:build darwin || windows || linux

package headless

import (
	"bytes"
	"strings"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
)

// trayApplication is a main window, a status item that toggles a panel, and a
// panel that closes when it loses focus: the whole menu-bar-app lifecycle on the
// headless host.
type trayApplication struct {
	application *applicationSession
	main        *app.Window
	panel       *app.Window
	clicks      []gfx.PointerButton
}

func newTrayApplication(t *testing.T) *trayApplication {
	t.Helper()
	host := newApplicationSession(Options{Scale: 1})
	t.Cleanup(host.closeAll)
	tray := &trayApplication{application: host}
	app.UseApplicationRunner(func(start func(app.Host) error) error { return start(host) })
	t.Cleanup(func() { app.UseApplicationRunner(nil) })
	err := app.RunApplication(tray.start(t))
	if err != nil {
		t.Fatal(err)
	}
	return tray
}

func (tray *trayApplication) start(t *testing.T) func(*app.Application) error {
	return func(application *app.Application) error {
		var err error
		tray.main, err = application.OpenWindow(app.Config{Title: "main", Width: 200, Height: 100}, nil)
		if err != nil {
			return err
		}
		_, err = application.AddStatusItem(app.StatusItemConfig{
			Tooltip: "Sessions",
			Icon:    app.StatusIcon{Size: 2, RGBA: make([]byte, 16), Template: true},
			OnClick: func(click app.StatusClick) {
				tray.clicks = append(tray.clicks, click.Button)
				if click.Button == gfx.PointerButtonSecondary {
					application.Quit()
					return
				}
				if tray.panel != nil && !tray.panel.Closed() {
					tray.panel.Close()
					return
				}
				var err error
				tray.panel, err = application.OpenWindow(app.Config{
					Title: "panel", Width: 100, Height: 80, WindowMode: app.WindowPanel, Anchor: click.Anchor,
				}, nil)
				if err != nil {
					t.Error(err)
				}
			},
		})
		return err
	}
}

func run(t *testing.T, application *applicationSession, script string) string {
	t.Helper()
	var output bytes.Buffer
	if err := ServeApplication(application, strings.NewReader(script), &output); err != nil {
		t.Fatal(err)
	}
	return output.String()
}

func TestStatusItemTogglesAPanelThatClosesWhenItLosesFocus(t *testing.T) {
	tray := newTrayApplication(t)
	text := run(t, tray.application, strings.Join([]string{
		"statusitems",
		"statusclick 1",
		"windows",
		"blur 2",
		"windows",
		"statusclick 1",
		"statusclick 1",
		"windows",
	}, "\n")+"\n")
	for _, fragment := range []string{
		`id=1 tooltip="Sessions" icon=2x2 template=true`,
		"ok statusitems=1",
		`id=2 active=true panel=true hidden=false title="panel"`,
		"ok windows=2 active=2",
		"ok windows=1 active=1",
	} {
		if !strings.Contains(text, fragment) {
			t.Fatalf("output missing %q:\n%s", fragment, text)
		}
	}
	// Open, dismissed by losing focus, open, closed by the second press: the
	// panel is gone and the main window never was.
	if tray.main.Closed() || tray.panel == nil || !tray.panel.Closed() {
		t.Fatalf("main closed=%t panel=%v", tray.main.Closed(), tray.panel)
	}
	if len(tray.clicks) != 3 {
		t.Fatalf("clicks = %v", tray.clicks)
	}
}

func TestHiddenWindowIsListedAndSurvivesTheLoop(t *testing.T) {
	tray := newTrayApplication(t)
	tray.main.Hide()
	text := run(t, tray.application, "windows\n")
	if !strings.Contains(text, `id=1 active=true panel=false hidden=true title="main"`) {
		t.Fatalf("hidden window not reported:\n%s", text)
	}
	tray.main.Show()
	text = run(t, tray.application, "windows\n")
	if !strings.Contains(text, "hidden=false") {
		t.Fatalf("shown window still hidden:\n%s", text)
	}
}

func TestQuitFromAStatusClickClosesEveryWindowAndEndsTheRun(t *testing.T) {
	tray := newTrayApplication(t)
	var output bytes.Buffer
	err := ServeApplication(tray.application, strings.NewReader("statusclick 1 secondary\nwindows\n"), &output)
	if err != nil {
		t.Fatal(err)
	}
	text := output.String()
	if !tray.main.Closed() {
		t.Fatal("Quit left the main window open")
	}
	if !strings.Contains(text, "ok windows=0") {
		t.Fatalf("the reply to the quitting click does not say the windows are gone:\n%s", text)
	}
	if strings.Contains(text, "ok windows=0 active=0") {
		t.Fatalf("the run continued after Quit:\n%s", text)
	}
}

func TestStatusClickReportsUnknownItemsAndButtons(t *testing.T) {
	tray := newTrayApplication(t)
	text := run(t, tray.application, "statusclick 9\nstatusclick 1 middle\nstatusclick\nblur 7\n")
	for _, fragment := range []string{"error headless: no status item 9", "error statusclick button is primary or secondary", "error statusclick needs an item ID", "error headless: no window 7"} {
		if !strings.Contains(text, fragment) {
			t.Fatalf("output missing %q:\n%s", fragment, text)
		}
	}
}

func TestRequestCloseLetsTheApplicationHideInsteadOfClosing(t *testing.T) {
	host := newApplicationSession(Options{Scale: 1})
	t.Cleanup(host.closeAll)
	app.UseApplicationRunner(func(start func(app.Host) error) error { return start(host) })
	t.Cleanup(func() { app.UseApplicationRunner(nil) })
	var hiding, plain *app.Window
	err := app.RunApplication(func(application *app.Application) error {
		var err error
		hiding, err = application.OpenWindow(app.Config{Title: "hides", OnCloseRequest: func(allow func()) { hiding.Hide() }}, nil)
		if err != nil {
			return err
		}
		plain, err = application.OpenWindow(app.Config{Title: "plain"}, nil)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	text := run(t, host, "requestclose 1\nwindows\nrequestclose 2\nwindows\nrequestclose 7\n")
	for _, fragment := range []string{
		"ok requested=1 windows=2",
		`id=1 active=false panel=false hidden=true title="hides"`,
		"ok requested=2 windows=1",
		"error headless: no such window",
	} {
		if !strings.Contains(text, fragment) {
			t.Fatalf("output missing %q:\n%s", fragment, text)
		}
	}
	if hiding.Closed() || !hiding.Hidden() || !plain.Closed() {
		t.Fatalf("hiding closed=%t hidden=%t, plain closed=%t", hiding.Closed(), hiding.Hidden(), plain.Closed())
	}
}

func TestStatusItemMenuIsListedAndEntriesAreChosen(t *testing.T) {
	host := newApplicationSession(Options{Scale: 1})
	t.Cleanup(host.closeAll)
	app.UseApplicationRunner(func(start func(app.Host) error) error { return start(host) })
	t.Cleanup(func() { app.UseApplicationRunner(nil) })
	var chosen []string
	opened, activated := 0, 0
	err := app.RunApplication(func(application *app.Application) error {
		if _, err := application.OpenWindow(app.Config{Title: "main", Width: 200, Height: 100}, nil); err != nil {
			return err
		}
		_, err := application.AddStatusItem(app.StatusItemConfig{
			Tooltip: "Menu",
			Icon:    app.StatusIcon{Size: 2, RGBA: make([]byte, 16)},
			OnClick: func(app.StatusClick) { activated++ },
			Menu: func() []app.MenuItem {
				opened++
				return []app.MenuItem{
					{Title: "Status", Disabled: true},
					{Title: "Sessions", Submenu: []app.MenuItem{
						{Title: "alpha", OnSelect: func() { chosen = append(chosen, "alpha") }},
					}},
					{Separator: true},
					{Title: "Quit", Checked: true, OnSelect: func() { chosen = append(chosen, "quit") }},
				}
			},
		})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	text := run(t, host, strings.Join([]string{
		"statusitems",
		"statusmenu 1",
		"statusselect 1 2.1",
		"statusselect 1 4",
		"statusselect 1 1",
		"statusselect 1 2",
		"statusselect 1 3",
		"statusselect 1 9",
		"statusclick 1",
		"statusclick 1 secondary",
	}, "\n")+"\n")
	for _, fragment := range []string{
		`tooltip="Menu" icon=2x2 template=false menu=true`,
		`path=1 title="Status" disabled=true checked=false submenu=false`,
		`path=2 title="Sessions" disabled=false checked=false submenu=true`,
		`  path=2.1 title="alpha" disabled=false checked=false submenu=false`,
		`path=3 separator`,
		`path=4 title="Quit" disabled=false checked=true submenu=false`,
		"ok entries=4",
		`error headless: menu entry "1" cannot be chosen`,
		`error headless: menu entry "2" cannot be chosen`,
		`error headless: menu entry "3" cannot be chosen`,
		`error headless: no menu entry "9"`,
		"status item 1 opens a menu",
	} {
		if !strings.Contains(text, fragment) {
			t.Fatalf("output missing %q:\n%s", fragment, text)
		}
	}
	if len(chosen) != 2 || chosen[0] != "alpha" || chosen[1] != "quit" {
		t.Fatalf("chosen = %v", chosen)
	}
	if activated != 1 {
		t.Fatalf("the application's handler ran %d times: only a primary press is its own", activated)
	}
	if opened == 0 {
		t.Fatal("the menu was never built")
	}
}
