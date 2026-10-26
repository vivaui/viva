//go:build darwin || windows || linux

package app

import (
	"errors"
	"runtime"
	"testing"

	"github.com/vivaui/viva/ui"
)

// fakeHost is an event loop with no windows: it records what the application
// asked of it and lets a test play the platform's part.
type fakeHost struct {
	windows []*fakeWindow
	items   []*fakeStatusItem
	quits   int
	reject  string
}

type fakeWindow struct {
	config       Config
	closed       func()
	hides, shows int
	done         bool
}

func (window *fakeWindow) Close() {
	if !window.done {
		window.done = true
		window.closed()
	}
}
func (window *fakeWindow) Hide() { window.hides++ }
func (window *fakeWindow) Show() { window.shows++ }

// Frame is where the window's Config.Origin put it.
func (window *fakeWindow) Frame() (ui.Rect, bool) {
	origin := window.config.originPoint()
	return ui.Rect{X: origin.X, Y: origin.Y, Width: float32(window.config.Width), Height: float32(window.config.Height)}, true
}

type fakeStatusItem struct {
	config  StatusItemConfig
	icon    StatusIcon
	tooltip string
	removed int
}

func (item *fakeStatusItem) SetIcon(icon StatusIcon) error { item.icon = icon; return nil }
func (item *fakeStatusItem) SetTooltip(tooltip string)     { item.tooltip = tooltip }
func (item *fakeStatusItem) Remove()                       { item.removed++ }

func (host *fakeHost) OpenWindow(config Config, canvas *ui.Canvas, closed func()) (WindowControl, error) {
	if host.reject != "" && config.Title == host.reject {
		return nil, errors.New("rejected")
	}
	window := &fakeWindow{config: config, closed: closed}
	host.windows = append(host.windows, window)
	return window, nil
}

func (host *fakeHost) AddStatusItem(config StatusItemConfig) (StatusItemControl, error) {
	item := &fakeStatusItem{config: config, icon: config.Icon, tooltip: config.Tooltip}
	host.items = append(host.items, item)
	return item, nil
}

func (host *fakeHost) Quit() { host.quits++ }

func useHost(t *testing.T, host *fakeHost) {
	t.Helper()
	previous := applicationRunner
	t.Cleanup(func() {
		applicationRunner = previous
		// A window the test left open is the host's to close when the test ends.
		for _, window := range host.windows {
			window.Close()
		}
	})
	applicationRunner = func(start func(Host) error) error { return start(host) }
}

func TestRunApplicationOwnsIndependentWindowsUntilEachCloses(t *testing.T) {
	host := &fakeHost{reject: "rejected"}
	useHost(t, host)

	var first, second *Window
	err := RunApplication(func(application *Application) error {
		var err error
		first, err = application.OpenWindow(Config{Title: "first"}, nil)
		if err != nil {
			return err
		}
		second, err = application.OpenWindow(Config{Title: "second"}, nil)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if first == second || first.Closed() || second.Closed() {
		t.Fatalf("initial windows first=%p closed=%t second=%p closed=%t", first, first.Closed(), second, second.Closed())
	}
	host.windows[1].Close()
	if first.Closed() || !second.Closed() {
		t.Fatalf("closing second leaked across windows: first=%t second=%t", first.Closed(), second.Closed())
	}
	first.Close()
	first.Close()
	if !first.Closed() {
		t.Fatal("first window did not finish closing")
	}
}

func TestRunApplicationRequiresAWindowAndDoesNotRetainRejectedOpen(t *testing.T) {
	useHost(t, &fakeHost{reject: "rejected"})
	if err := RunApplication(func(*Application) error { return nil }); !errors.Is(err, ErrNoWindows) {
		t.Fatalf("empty application error = %v", err)
	}
	if err := RunApplication(func(application *Application) error {
		_, err := application.OpenWindow(Config{Title: "rejected"}, nil)
		return err
	}); err == nil || err.Error() != "rejected" {
		t.Fatalf("rejected open error = %v", err)
	}
}

func TestWindowTracksTheMinimizeThePlatformReports(t *testing.T) {
	host := &fakeHost{}
	useHost(t, host)
	var window *Window
	var reported []bool
	if err := RunApplication(func(application *Application) error {
		var err error
		window, err = application.OpenWindow(Config{Title: "main", OnMinimize: func(minimized bool) {
			reported = append(reported, minimized)
		}}, nil)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if window.Minimized() {
		t.Fatal("a new window is minimized")
	}
	// The platform reports a minimize through the config it was opened with.
	host.windows[0].config.OnMinimize(true)
	if !window.Minimized() {
		t.Fatal("Minimized did not follow the platform")
	}
	host.windows[0].config.OnMinimize(false)
	if window.Minimized() {
		t.Fatal("Minimized did not follow the restore")
	}
	if len(reported) != 2 || !reported[0] || reported[1] {
		t.Fatalf("OnMinimize calls = %v", reported)
	}
}

func TestHiddenWindowStaysOpenAndShowBringsItBack(t *testing.T) {
	host := &fakeHost{}
	useHost(t, host)
	var window *Window
	if err := RunApplication(func(application *Application) error {
		var err error
		window, err = application.OpenWindow(Config{Title: "main"}, nil)
		return err
	}); err != nil {
		t.Fatal(err)
	}

	window.Hide()
	window.Hide()
	if !window.Hidden() || window.Closed() || host.windows[0].hides != 1 {
		t.Fatalf("after Hide: hidden=%t closed=%t hides=%d", window.Hidden(), window.Closed(), host.windows[0].hides)
	}
	window.Show()
	if window.Hidden() || host.windows[0].shows != 1 {
		t.Fatalf("after Show: hidden=%t shows=%d", window.Hidden(), host.windows[0].shows)
	}
	// Showing a window that is on screen raises it, and hiding does not touch a
	// window that has already closed.
	window.Show()
	window.Close()
	window.Hide()
	window.Show()
	if host.windows[0].shows != 2 || host.windows[0].hides != 1 {
		t.Fatalf("closed window was driven: shows=%d hides=%d", host.windows[0].shows, host.windows[0].hides)
	}
}

func TestDockReopenThenHideReachesNativeWindow(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("Dock reopen is macOS-specific")
	}
	host := &fakeHost{}
	useHost(t, host)
	var window, other *Window
	if err := RunApplication(func(application *Application) error {
		var err error
		window, err = application.OpenWindow(Config{
			Title: "Dock reopen regression",
			OnCloseRequest: func(allow func()) {
				window.Hide()
			},
		}, nil)
		if err != nil {
			return err
		}
		other, err = application.OpenWindow(Config{Title: "other"}, nil)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	native := host.windows[0]
	if native.config.onReopen == nil {
		t.Fatal("the platform was given no reopen handler")
	}
	other.Hide()
	for cycle := 1; cycle <= 3; cycle++ {
		native.config.OnCloseRequest(func() { t.Fatal("close-to-tray request closed the window") })
		window.Hide()
		if !window.Hidden() || window.Closed() || native.hides != cycle {
			t.Fatalf("cycle %d after close: hidden=%t closed=%t native hides=%d", cycle, window.Hidden(), window.Closed(), native.hides)
		}
		// A Dock click asks the platform to reopen, without a caller using Show.
		native.config.onReopen()
		if window.Hidden() || window.Closed() || native.shows != cycle {
			t.Fatalf("cycle %d after Dock reopen: hidden=%t closed=%t native shows=%d", cycle, window.Hidden(), window.Closed(), native.shows)
		}
		if !other.Hidden() || host.windows[1].shows != 0 {
			t.Fatal("reopening one window changed another window's state")
		}
	}
	window.Close()
	native.config.onReopen()
	if native.shows != 3 {
		t.Fatal("a Dock reopen showed a closed window")
	}
}

func TestStatusItemReportsClicksAndRemovesOnce(t *testing.T) {
	host := &fakeHost{}
	useHost(t, host)
	var clicks []StatusClick
	var item *StatusItem
	if err := RunApplication(func(application *Application) error {
		var err error
		item, err = application.AddStatusItem(StatusItemConfig{
			Tooltip: "Sessions",
			Icon:    StatusIcon{Size: 2, RGBA: make([]byte, 16), Template: true},
			OnClick: func(click StatusClick) { clicks = append(clicks, click) },
		})
		if err != nil {
			return err
		}
		_, err = application.OpenWindow(Config{}, nil)
		return err
	}); err != nil {
		t.Fatal(err)
	}

	host.items[0].config.OnClick(StatusClick{Button: ui.PointerButtonSecondary, PanelDismissed: true})
	if len(clicks) != 1 || clicks[0].Button != ui.PointerButtonSecondary || !clicks[0].PanelDismissed {
		t.Fatalf("clicks = %+v", clicks)
	}
	item.SetTooltip("Two sessions")
	if host.items[0].tooltip != "Two sessions" {
		t.Fatalf("tooltip = %q", host.items[0].tooltip)
	}
	if err := item.SetIcon(StatusIcon{Size: 1, RGBA: make([]byte, 4)}); err != nil || host.items[0].icon.Size != 1 {
		t.Fatalf("SetIcon: err=%v icon=%+v", err, host.items[0].icon)
	}
	item.Remove()
	item.Remove()
	if host.items[0].removed != 1 {
		t.Fatalf("Remove reached the host %d times", host.items[0].removed)
	}
	if err := item.SetIcon(StatusIcon{Size: 1, RGBA: make([]byte, 4)}); err == nil {
		t.Fatal("a removed item accepted an icon")
	}
}

func TestStatusItemRejectsIconsWhosePixelsDoNotMatchTheirSize(t *testing.T) {
	host := &fakeHost{}
	useHost(t, host)
	for _, icon := range []StatusIcon{
		{},
		{Size: 2, RGBA: make([]byte, 15)},
		{Size: 2000, RGBA: nil},
		{Size: -1, RGBA: make([]byte, 4)},
	} {
		err := RunApplication(func(application *Application) error {
			_, err := application.AddStatusItem(StatusItemConfig{Icon: icon})
			return err
		})
		if err == nil {
			t.Fatalf("icon %d with %d bytes was accepted", icon.Size, len(icon.RGBA))
		}
	}
	if len(host.items) != 0 {
		t.Fatalf("%d invalid items reached the host", len(host.items))
	}
}

func TestApplicationQuitReachesTheHost(t *testing.T) {
	host := &fakeHost{}
	useHost(t, host)
	var application *Application
	if err := RunApplication(func(started *Application) error {
		application = started
		_, err := started.OpenWindow(Config{}, nil)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	application.Quit()
	if host.quits != 1 {
		t.Fatalf("quits = %d", host.quits)
	}
	(*Application)(nil).Quit()
}

func BenchmarkApplicationOpenClosePair(b *testing.B) {
	previous := applicationRunner
	b.Cleanup(func() { applicationRunner = previous })
	host := &fakeHost{}
	applicationRunner = func(start func(Host) error) error { return start(host) }
	b.ReportAllocs()
	for b.Loop() {
		host.windows = host.windows[:0]
		if err := RunApplication(func(application *Application) error {
			first, err := application.OpenWindow(Config{}, nil)
			if err != nil {
				return err
			}
			second, err := application.OpenWindow(Config{}, nil)
			if err != nil {
				return err
			}
			second.Close()
			first.Close()
			return nil
		}); err != nil {
			b.Fatal(err)
		}
	}
}

func TestPlatformMenuCopiesEveryLevelAndItsSelectHandler(t *testing.T) {
	var chosen []string
	items := []MenuItem{
		{Title: "Open", OnSelect: func() { chosen = append(chosen, "open") }},
		{Separator: true},
		{Title: "Recent", Submenu: []MenuItem{
			{Title: "One", Checked: true, OnSelect: func() { chosen = append(chosen, "one") }},
			{Title: "Two", Disabled: true},
		}},
	}
	menu := platformMenu(items)
	if len(menu) != 3 || menu[0].Title != "Open" || !menu[1].Separator || len(menu[2].Submenu) != 2 {
		t.Fatalf("menu = %+v", menu)
	}
	if !menu[2].Submenu[0].Checked || !menu[2].Submenu[1].Disabled {
		t.Fatalf("submenu flags lost: %+v", menu[2].Submenu)
	}
	menu[0].OnSelect()
	menu[2].Submenu[0].OnSelect()
	if len(chosen) != 2 || chosen[0] != "open" || chosen[1] != "one" {
		t.Fatalf("chosen = %v", chosen)
	}
	if platformMenu(nil) != nil {
		t.Fatal("an empty menu allocated")
	}
}

func TestWindowFrameIsWhereTheOriginPutIt(t *testing.T) {
	host := &fakeHost{}
	application := &Application{host: host}
	window, err := application.OpenWindow(Config{Width: 300, Height: 200, Origin: &ui.Point{X: 40, Y: 50}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if frame, ok := window.Frame(); !ok || frame != (ui.Rect{X: 40, Y: 50, Width: 300, Height: 200}) {
		t.Fatalf("frame %+v, %v", frame, ok)
	}
	window.Close()
	if _, ok := window.Frame(); ok {
		t.Fatal("a closed window reports a frame")
	}
	var none *Window
	if _, ok := none.Frame(); ok {
		t.Fatal("a nil window reports a frame")
	}
}
