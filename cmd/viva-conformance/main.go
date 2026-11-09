//go:build darwin || windows || linux

// Command viva-conformance runs one set of lifecycle checks against the native
// backend of the platform it is built for, through the public app package only.
// The same scenarios run on macOS, Windows and Linux, so a behavior that one
// platform does differently shows up as a FAIL there and nowhere else.
//
// It opens real windows and needs a desktop session. Each scenario prints one
// line, PASS, FAIL or SKIP with a reason, and the exit status is 1 if any failed.
// The checks are about events and lifecycle (what is called, how often, and when
// the loop ends), not pixels, and none needs a person at the keyboard. Checks
// that do need input, a click on the status icon, losing focus, are made by hand
// or by the platform scripts and are listed in docs/conformance.md.
package main

import (
	"errors"
	"fmt"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
)

// step is how long a scenario waits for something that should happen at once, and
// quiet how long it watches for something that should not happen at all.
const (
	step  = 3 * time.Second
	quiet = 700 * time.Millisecond
)

func main() {
	results := &report{}
	watchdog := time.AfterFunc(90*time.Second, func() {
		results.add("whole run", "FAIL", "did not finish in 90 seconds")
		results.print()
		os.Exit(2)
	})
	defer watchdog.Stop()

	loopEnded := make(chan struct{})
	finished := make(chan struct{})
	err := app.RunApplication(func(application *app.Application) error {
		runner := &runner{application: application, loopEnded: loopEnded}
		control, err := application.OpenWindow(app.Config{
			Title: "viva-conformance", Width: 320, Height: 120,
			Invalidator: &runner.inv, OnWake: runner.drain,
			Paint: func(canvas *gfx.Canvas, size gfx.Size) {
				canvas.Label("running checks", gfx.Rect{X: 12, Y: 12, Width: 280, Height: 24}, gfx.TextStyle{Size: 14})
			},
		}, nil)
		if err != nil {
			return err
		}
		runner.control = control
		go func() {
			runner.scenarios(results)
			close(finished)
		}()
		return nil
	})
	close(loopEnded)
	if err != nil {
		results.add("RunApplication", "FAIL", err.Error())
	}
	// The last scenario ends the loop itself, so it finishes after RunApplication
	// has returned.
	select {
	case <-finished:
	case <-time.After(step):
		results.add("the scenarios", "FAIL", "did not finish after the loop ended")
	}
	results.print()
	if results.failed() {
		os.Exit(1)
	}
}

type report struct {
	mu    sync.Mutex
	lines []string
	bad   bool
}

func (r *report) add(name, verdict, detail string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	line := verdict + " " + name
	if detail != "" {
		line += ": " + detail
	}
	r.lines = append(r.lines, line)
	if verdict == "FAIL" {
		r.bad = true
	}
}

func (r *report) failed() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.bad
}

func (r *report) print() {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, line := range r.lines {
		fmt.Println(line)
	}
}

// runner drives the scenarios from a goroutine. Everything that must run on the
// event-loop thread goes through onLoop, which hands it to the control window.
type runner struct {
	application *app.Application
	control     *app.Window
	loopEnded   chan struct{} // closed when RunApplication has returned
	inv         app.Invalidator
	mu          sync.Mutex
	queue       []func()
}

// drain runs what onLoop queued; it is the control window's OnWake.
func (r *runner) drain() {
	r.mu.Lock()
	work := r.queue
	r.queue = nil
	r.mu.Unlock()
	for _, f := range work {
		f()
	}
}

// onLoop runs f on the event-loop thread and waits for it.
func (r *runner) onLoop(f func()) {
	done := make(chan struct{})
	r.mu.Lock()
	r.queue = append(r.queue, func() { f(); close(done) })
	r.mu.Unlock()
	r.inv.Invalidate()
	select {
	case <-done:
	case <-time.After(step):
	}
}

// probe is a window the scenarios watch: how many times it was woken and painted,
// and whether it has closed.
type probe struct {
	window  *app.Window
	inv     app.Invalidator
	wakes   atomic.Int64
	paints  atomic.Int64
	closed  chan struct{}
	refuse  bool
	request atomic.Int64
}

func (r *runner) open(title string, mode app.WindowMode, refuseClose bool) (*probe, error) {
	p := &probe{closed: make(chan struct{}), refuse: refuseClose}
	var err error
	r.onLoop(func() {
		config := app.Config{
			Title: title, Width: 300, Height: 160, WindowMode: mode,
			Invalidator: &p.inv, OnWake: func() { p.wakes.Add(1) },
			Paint:    func(*gfx.Canvas, gfx.Size) { p.paints.Add(1) },
			OnClosed: func() { close(p.closed) },
		}
		if refuseClose {
			config.OnCloseRequest = func(allow func()) { p.request.Add(1) } // never allows
		}
		p.window, err = r.application.OpenWindow(config, nil)
	})
	if err == nil && p.window == nil {
		err = errors.New("the window was not opened on the event loop in time")
	}
	return p, err
}

func (p *probe) waitFor(what string, condition func() bool) error {
	deadline := time.Now().Add(step)
	for time.Now().Before(deadline) {
		if condition() {
			return nil
		}
		time.Sleep(10 * time.Millisecond)
	}
	return fmt.Errorf("%s did not happen within %v", what, step)
}

func (p *probe) isClosed() bool {
	select {
	case <-p.closed:
		return true
	default:
		return false
	}
}

func (r *runner) scenarios(results *report) {
	check := func(name string, run func() error) {
		switch err := run(); {
		case err == nil:
			results.add(name, "PASS", "")
		case errors.Is(err, errSkip):
			results.add(name, "SKIP", err.Error())
		default:
			results.add(name, "FAIL", err.Error())
		}
	}

	check("a window paints, and closes from another goroutine", func() error {
		p, err := r.open("lifecycle", app.WindowTitled, false)
		if err != nil {
			return err
		}
		if err := p.waitFor("the first paint", func() bool { return p.paints.Load() >= 1 }); err != nil {
			return err
		}
		p.window.Close()
		if err := p.waitFor("OnClosed", p.isClosed); err != nil {
			return err
		}
		if !p.window.Closed() {
			return errors.New("OnClosed ran but Window.Closed is false")
		}
		p.window.Close() // closing again must be harmless
		return nil
	})

	check("an idle window does no work", func() error {
		p, err := r.open("idle", app.WindowTitled, false)
		if err != nil {
			return err
		}
		defer p.window.Close()
		if err := p.waitFor("the first paint", func() bool { return p.paints.Load() >= 1 }); err != nil {
			return err
		}
		time.Sleep(300 * time.Millisecond) // let start-up settle
		paints, wakes := p.paints.Load(), p.wakes.Load()
		time.Sleep(quiet)
		if p.paints.Load() != paints || p.wakes.Load() != wakes {
			return fmt.Errorf("paints %d to %d and wakes %d to %d with nothing asked for", paints, p.paints.Load(), wakes, p.wakes.Load())
		}
		return nil
	})

	check("watching the system appearance costs an idle window nothing", func() error {
		p, err := r.open("appearance", app.WindowTitled, false)
		if err != nil {
			return err
		}
		defer p.window.Close()
		if err := p.waitFor("the first paint", func() bool { return p.paints.Load() >= 1 }); err != nil {
			return err
		}
		time.Sleep(300 * time.Millisecond)
		goroutines := runtime.NumGoroutine()
		var changes atomic.Int32
		stop := app.WatchSystemAppearance(func() { changes.Add(1) })
		defer stop()
		time.Sleep(500 * time.Millisecond) // whatever the adapter opens to hear the system
		paints, wakes := p.paints.Load(), p.wakes.Load()
		time.Sleep(quiet)
		if p.paints.Load() != paints || p.wakes.Load() != wakes {
			return fmt.Errorf("paints %d to %d and wakes %d to %d with nothing asked for", paints, p.paints.Load(), wakes, p.wakes.Load())
		}
		if now := runtime.NumGoroutine(); now > goroutines {
			return fmt.Errorf("watching started %d goroutines", now-goroutines)
		}
		if got := changes.Load(); got != 0 {
			return fmt.Errorf("a change was reported %d times though the appearance did not change", got)
		}
		return nil
	})

	check("Invalidate from another goroutine wakes the window once", func() error {
		p, err := r.open("wake", app.WindowTitled, false)
		if err != nil {
			return err
		}
		defer p.window.Close()
		if err := p.waitFor("the first paint", func() bool { return p.paints.Load() >= 1 }); err != nil {
			return err
		}
		time.Sleep(300 * time.Millisecond)
		before := p.wakes.Load()
		p.inv.Invalidate()
		if err := p.waitFor("OnWake", func() bool { return p.wakes.Load() > before }); err != nil {
			return err
		}
		time.Sleep(quiet)
		if got := p.wakes.Load() - before; got != 1 {
			return fmt.Errorf("one Invalidate caused %d wakes", got)
		}
		return nil
	})

	check("a flood of Invalidate calls costs about one wake per frame", func() error {
		p, err := r.open("flood", app.WindowTitled, false)
		if err != nil {
			return err
		}
		defer p.window.Close()
		if err := p.waitFor("the first paint", func() bool { return p.paints.Load() >= 1 }); err != nil {
			return err
		}
		time.Sleep(300 * time.Millisecond)
		before, floodFor := p.wakes.Load(), 600*time.Millisecond
		for end := time.Now().Add(floodFor); time.Now().Before(end); {
			p.inv.Invalidate()
		}
		time.Sleep(quiet)
		got := p.wakes.Load() - before
		// The frame grid is 120 Hz; a few more allow for the first wake, which goes out at
		// once, and a window thread that runs late.
		if limit := int64(floodFor/(time.Second/120)) + 8; got < 1 || got > limit {
			return fmt.Errorf("%d wakes in %v of constant Invalidate, want between 1 and %d", got, floodFor, limit)
		}
		return nil
	})

	check("a hidden window stays open and still receives wakes", func() error {
		p, err := r.open("hidden", app.WindowTitled, false)
		if err != nil {
			return err
		}
		defer p.window.Close()
		if err := p.waitFor("the first paint", func() bool { return p.paints.Load() >= 1 }); err != nil {
			return err
		}
		p.window.Hide()
		if !p.window.Hidden() {
			return errors.New("Hidden is false right after Hide")
		}
		time.Sleep(400 * time.Millisecond)
		if p.isClosed() || p.window.Closed() {
			return errors.New("hiding closed the window")
		}
		// A tray tooltip is refreshed from the wake of a window nobody can see.
		before := p.wakes.Load()
		p.inv.Invalidate()
		if err := p.waitFor("OnWake while hidden", func() bool { return p.wakes.Load() > before }); err != nil {
			return err
		}
		p.window.Show()
		if p.window.Hidden() {
			return errors.New("Hidden is still true after Show")
		}
		before = p.wakes.Load()
		p.inv.Invalidate()
		if err := p.waitFor("OnWake after Show", func() bool { return p.wakes.Load() > before }); err != nil {
			return err
		}
		return nil
	})

	check("a hidden window runs OnWake but paints nothing until it is shown again", func() error {
		p, err := r.open("occluded", app.WindowTitled, false)
		if err != nil {
			return err
		}
		defer p.window.Close()
		if err := p.waitFor("the first paint", func() bool { return p.paints.Load() >= 1 }); err != nil {
			return err
		}
		p.window.Hide()
		time.Sleep(500 * time.Millisecond) // the platform's visibility change arrives
		paints, wakes := p.paints.Load(), p.wakes.Load()
		for range 3 {
			p.inv.Invalidate()
			time.Sleep(50 * time.Millisecond)
		}
		if err := p.waitFor("OnWake while hidden", func() bool { return p.wakes.Load() > wakes }); err != nil {
			return err
		}
		time.Sleep(quiet)
		if got := p.paints.Load() - paints; got != 0 {
			return fmt.Errorf("a hidden window painted %d times", got)
		}
		p.window.Show()
		return p.waitFor("the frame the hidden window missed", func() bool { return p.paints.Load() > paints })
	})

	check("a panel opens, paints, and closes from another goroutine", func() error {
		p, err := r.open("panel", app.WindowPanel, false)
		if err != nil {
			return err
		}
		if err := p.waitFor("the panel's first paint", func() bool { return p.paints.Load() >= 1 }); err != nil {
			return err
		}
		p.window.Close()
		return p.waitFor("the panel's OnClosed", p.isClosed)
	})

	check("a status item can be added, changed and removed", func() error {
		var item *app.StatusItem
		var addErr, iconErr error
		r.onLoop(func() {
			icon := app.StatusIcon{Size: 16, RGBA: make([]byte, 16*16*4)}
			for i := 3; i < len(icon.RGBA); i += 4 {
				icon.RGBA[i] = 255
			}
			item, addErr = r.application.AddStatusItem(app.StatusItemConfig{
				Tooltip: "viva-conformance", Icon: icon,
				// Never opened here: a click needs a person. It makes the menu
				// path (NSMenu, a popup menu, D-Bus) part of what is added.
				Menu: func() []app.MenuItem {
					return []app.MenuItem{{Title: "conformance", Disabled: true}, {Separator: true}, {Title: "Quit", OnSelect: func() {}}}
				},
			})
			if addErr == nil {
				item.SetTooltip("changed")
				iconErr = item.SetIcon(icon)
			}
		})
		if errors.Is(addErr, app.ErrNoStatusArea) {
			return fmt.Errorf("%w: %v", errSkip, addErr)
		}
		if addErr != nil {
			return addErr
		}
		if iconErr != nil {
			return iconErr
		}
		item.Remove() // from this goroutine, which is allowed
		item.Remove()
		time.Sleep(quiet)
		return nil
	})

	check("Quit ends the loop though windows refuse to close, even as the last-used window closes", func() error {
		// The panel is opened last, so it is the window a platform would take for
		// the one in use.
		hidden, err := r.open("hidden", app.WindowTitled, false)
		if err != nil {
			return err
		}
		refuser, err := r.open("refuses", app.WindowTitled, true)
		if err != nil {
			return err
		}
		panel, err := r.open("panel", app.WindowPanel, false)
		if err != nil {
			return err
		}
		hidden.window.Hide()
		time.Sleep(300 * time.Millisecond)
		// The panel is the window that handled an event last, as when a user has
		// just clicked in it, and it is closing as Quit is asked for: a Quit posted
		// to just that window would be lost with it. This is what the tray's Quit
		// row does.
		before := panel.wakes.Load()
		panel.inv.Invalidate()
		if err := panel.waitFor("the panel's wake", func() bool { return panel.wakes.Load() > before }); err != nil {
			return err
		}
		panel.window.Close()
		r.application.Quit()
		for index, p := range []*probe{refuser, panel, hidden} {
			if err := p.waitFor("OnClosed after Quit", p.isClosed); err != nil {
				return fmt.Errorf("window %d: %w", index+1, err)
			}
		}
		if refuser.request.Load() != 0 {
			return errors.New("Quit asked a window's OnCloseRequest; it must not")
		}
		select {
		case <-r.loopEnded:
			return nil
		case <-time.After(step):
			return errors.New("RunApplication did not return after Quit closed every window")
		}
	})

	// If Quit failed, RunApplication never returns to report it: say so and stop.
	select {
	case <-r.loopEnded:
	case <-time.After(step):
		results.print()
		os.Exit(1)
	}
}

var errSkip = errors.New("skipped")
