---
title: Test without a window
summary: Drive an application from a test, script it over a line protocol, and check that the platforms still agree.
order: 13
group: distribute
---

Verifying a desktop application usually means opening a window, which interrupts you, depends on which display it lands on and cannot run unattended. Viva gives you three layers that avoid that, each usable without the next.

## app.Driver

`app.NewDriver` runs your application's real painter, input router and focus rules from explicit calls: `Move`, `Click`, `Key`, `Type`, `Shortcut`, `Scroll`, `Resize` and `Advance`. It has no platform code, and its animation clock moves only when you tell it to. `Snapshot` returns what was painted: primitives, labels, hit regions and focus. This test drives the program from [Get started](/en/docs/getting-started/):

<!-- example: hello/main_test.go -->
```go
package main

import (
	"strings"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/ui"
)

// open runs the page in a headless window: the real painter, router and focus
// rules, driven by explicit calls.
func open(t *testing.T, m *model) *app.Driver {
	t.Helper()
	page := ui.NewPage(ui.PageConfig{View: m.page})
	driver, err := app.NewDriver(page.Config(app.Config{Width: 480, Height: 240}), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(driver.Close)
	return driver
}

// click presses the middle of the control a user would find by its name.
func click(t *testing.T, driver *app.Driver, name string) {
	t.Helper()
	for _, hit := range driver.Snapshot().Hits {
		if strings.HasPrefix(hit.Label, name) {
			b := hit.Bounds
			if err := driver.Click(b.X+b.Width/2, b.Y+b.Height/2); err != nil {
				t.Fatal(err)
			}
			return
		}
	}
	t.Fatalf("no control named %q on screen", name)
}

func shows(driver *app.Driver, want string) bool {
	for _, label := range driver.Snapshot().Labels {
		if strings.Contains(label.Value, want) {
			return true
		}
	}
	return false
}

func TestClickCounts(t *testing.T) {
	driver := open(t, &model{})
	if !shows(driver, "Clicked 0 times") {
		t.Fatal("the button should start at zero")
	}
	click(t, driver, "Clicked")
	if !shows(driver, "Clicked 1 times") {
		t.Fatal("the click did not reach the button")
	}
}
```

The test finds controls by the name a user and a screen reader know them by (`Hit.Label`), not by coordinates. A page names its elements itself, as paths such as `page/~k1/#0` in its tree; `c.Part("word")` gives an element a name of yours, which a test or the headless driver can address by path.

## headless.Session

`headless.Open` wraps that driver in a session that finds controls by their
accessible name and owns the clock. `Now`, `Advance` and `Frame` move time and
paint, `Transitioning` says whether anything is still moving, `Access` returns
the accessibility tree of the last frame, and `Notifications` returns what the
application sent:

```go
session, err := headless.Open(config, nil, headless.Options{})
if err != nil {
	t.Fatal(err)
}
defer session.Close()

node, ok := session.AccessByRole(gfx.RoleCheckbox, "Auto update")
if !ok || !node.Checked {
	t.Fatalf("checkbox = %+v", node)
}
session.ClickLabel("Install")
session.Advance(200 * time.Millisecond)
if session.Transitioning() {
	t.Error("the progress bar is still moving")
}
if got := session.Notifications(); len(got) != 1 || got[0].Title != "Installed" {
	t.Errorf("notifications = %+v", got)
}
```

`Access` reports what assistive technology is told: the role, bounds, name,
value, description, states, range and live region of every control, in reading
order. It is the same tree the platform adapters send to the system, without the
platform.

## Mistakes that compile

A page can compile and still do the wrong thing: a `Button` with the same label in every row of a loop whose rows have no Key can take a click meant for another row when a row is added above. `viva vet` finds it without running anything:

```sh
viva vet            # or go tool viva vet; packages default to ./...
```

It reports each one as `file:line:col: what is wrong and what to write (check)`, exits non-zero when it finds something, and runs with the project's own Go. A line, or the line above it, with `// viva:ignore <check>` silences one finding. It also reports a `ui.C` or `ui.Box` kept past its paint (in a struct, a package variable or a goroutine), and a `ui.Memo` whose value misses a field its function reads. A `ui.Memo` whose value misses something its function reads shows stale content: `ui.MemoCheck(true)` (or `PageConfig.MemoCheck`) in a test declares every part and panics where a copy would have differed.

## The line protocol

Import `github.com/vivaui/viva/headless/auto` from a file guarded by a build tag, so release builds never contain it:

```go
//go:build vivahead

package main

import _ "github.com/vivaui/viva/headless/auto"
```

Build with `-tags vivahead` and start the program with `VIVA_HEADLESS=1`. `RunCanvas` then reads commands from standard input instead of opening a window. Controls are addressed by label rather than by coordinates:

```text
click "New Host"
type example.com
key tab
waitfor "Connected" 5000
texts
shot /tmp/frame.png
record /tmp/frames 700
quit
```

Every request gets zero or more data lines and one final `ok` or `error` line, so a script, another program or a coding agent can drive and inspect an application without touching the screen. The full command list is in the doc comment of `headless/control.go`.

`layout` reads the frame for geometry that is almost surely a mistake and prints one line each: text cut off by a clip, text that runs out of its button or field, one line drawn over another on the same layer, text that leaves the window, and focusable controls that cross without one containing the other. It never fails, so run it after every change to a screen; a few lines of text are quicker to read than a picture. It judges text by the widths the session lays out with, the platform's text engine as in a window, so a line is reported when it is cut or crosses another on that platform.

**Screenshots and recordings need the offscreen renderer.** On macOS it is linked only into builds made with `-tags vivahead`; on Windows and Linux it is always there. It draws with the window's own pipelines (Metal, Direct3D 11, Vulkan), so a capture is the window's pixels by construction. Without a renderer the protocol still operates and answers queries, but `Image`, `Screenshot` and `Record` return `ErrNoRenderer`.

`record DIR MS [FPS]` plays MS more of the animation clock and writes the frames whose pixels changed as `frame-0000.png` onwards, with a `manifest.json` that says how long each is on screen, so a transition can be reviewed as a video instead of a still. A frame that repeats is written once with a longer duration, so an application at rest records as one frame. `shot` is still the single-frame version, and plays animations to rest first.

## Platform conformance

`cmd/viva-conformance` runs the same lifecycle scenarios through the public `app` package on each platform and prints PASS, FAIL or SKIP for each: closing from another goroutine, an idle window doing no work, `Invalidate` waking exactly once, hidden windows, panels, status items, and `Quit` ending the loop whatever the windows say. Each check exists because a real platform once got it wrong. It opens real windows, so it needs a desktop session. What it cannot check, and how that is done by hand, is listed in the repository's `docs/conformance.md`.
