//go:build linux && vivagtk

package linux

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/ebitengine/purego"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/dbus"
	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/internal/render"
	"github.com/vivaui/viva/internal/scene"
	"github.com/vivaui/viva/text"
)

type gtkRegionProbe struct {
	w                           *gtkWindow
	entry                       uintptr
	getFocus                    func(uintptr) uintptr
	contentClicks, titlePresses int
	edgePresses, resizeCalls    int
	entryButtons                int
	topIn, topOut               int
	areaIn, areaOut             int
	entryIn, entryOut           int
	resizeEdge                  int32
}

type gtkRegionState struct {
	mapped, childPointer, focused, nativeKeys, imeActive bool
	webFocus                                             uintptr
	gtkFocus                                             uintptr
	contentClicks, titlePresses, edgePresses             int
	entryButtons                                         int
	topIn, topOut, areaIn, areaOut                       int
	entryIn, entryOut                                    int
	resizeCalls                                          int
	resizeEdge                                           int32
	gdkPointer, gtkArea                                  uintptr
	gdkX, gdkY                                           int32
	scale                                                float32
	canvasWidth, viewportWidth, views, overlays          int
}

func gtkRegionSnapshot(ctx context.Context, app *gtkApplication, probe *gtkRegionProbe) (gtkRegionState, error) {
	answer := make(chan gtkRegionState, 1)
	app.enqueue(func() {
		w := probe.w
		state := gtkRegionState{
			mapped:        w.wl != nil && w.wl.child != 0,
			childPointer:  w.wl != nil && w.wl.s.focus == w.wl,
			focused:       w.focused,
			nativeKeys:    w.nativeKeys,
			webFocus:      w.webFocus,
			imeActive:     w.ime != nil && w.ime.active,
			contentClicks: probe.contentClicks,
			titlePresses:  probe.titlePresses,
			edgePresses:   probe.edgePresses,
			entryButtons:  probe.entryButtons,
			topIn:         probe.topIn,
			topOut:        probe.topOut,
			areaIn:        probe.areaIn,
			areaOut:       probe.areaOut,
			entryIn:       probe.entryIn,
			entryOut:      probe.entryOut,
			resizeCalls:   probe.resizeCalls,
			resizeEdge:    probe.resizeEdge,
			scale:         w.scale,
			canvasWidth:   w.scene.canvas.Width,
			viewportWidth: int(w.wl.viewportWidth),
			views:         len(w.scene.views),
		}
		device := w.g.gdkSeatPointer(w.g.gdkDisplaySeat(w.g.gdkDisplayDefault()))
		state.gdkPointer = w.g.gdkDeviceWindowAt(device, &state.gdkX, &state.gdkY)
		state.gtkArea = w.g.widgetGetWindow(w.area)
		if state.views > 0 {
			state.overlays = len(w.scene.overlays(0))
		}
		if probe.getFocus != nil {
			state.gtkFocus = probe.getFocus(w.toplevel)
		}
		answer <- state
	})
	select {
	case state := <-answer:
		return state, nil
	case <-ctx.Done():
		return gtkRegionState{}, ctx.Err()
	}
}

func gtkRegionWait(ctx context.Context, app *gtkApplication, probe *gtkRegionProbe, label string, check func(gtkRegionState) bool) error {
	deadline := time.NewTimer(2 * time.Second)
	defer deadline.Stop()
	for {
		state, err := gtkRegionSnapshot(ctx, app, probe)
		if err != nil {
			return fmt.Errorf("%s: %w", label, err)
		}
		if check(state) {
			return nil
		}
		select {
		case <-deadline.C:
			return fmt.Errorf("%s: deadline; state=%+v", label, state)
		case <-ctx.Done():
			return fmt.Errorf("%s: %w", label, ctx.Err())
		case <-time.After(10 * time.Millisecond):
		}
	}
}

func gtkRegionOnLoop(ctx context.Context, app *gtkApplication, change func()) error {
	done := make(chan struct{}, 1)
	app.enqueue(func() {
		change()
		done <- struct{}{}
	})
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func gtkRegionSelectPinyin(ctx context.Context, app *gtkApplication, probe *gtkRegionProbe) error {
	deadline := time.NewTimer(3 * time.Second)
	defer deadline.Stop()
	for {
		active, err := nativeFcitxCommand("--check", "-n")
		if err != nil {
			return err
		}
		if active != "" {
			return selectNativeFcitxEngine("pinyin")
		}
		select {
		case <-deadline.C:
			state, stateErr := gtkRegionSnapshot(ctx, app, probe)
			check, checkErr := nativeFcitxCommand("--check")
			return fmt.Errorf("Fcitx client did not register the focused native window: GTK=%+v snapshotErr=%v fcitx --check=%q checkErr=%v", state, stateErr, check, checkErr)
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(20 * time.Millisecond):
		}
	}
}

func gtkRegionFailureTrace(ctx context.Context, app *gtkApplication, probe *gtkRegionProbe, p *gtkTentativeProbe) string {
	answer := make(chan string, 1)
	app.enqueue(func() {
		w := probe.w
		signals := p.signals[max(0, len(p.signals)-12):]
		events := make([]string, 0, min(len(p.events), 12))
		for _, event := range p.events[max(0, len(p.events)-12):] {
			events = append(events, fmt.Sprintf("kind=%d phase=%d text=%q", event.Kind, event.CompositionPhase, event.Text))
		}
		ime := w.ime
		if ime == nil {
			answer <- fmt.Sprintf("signals(%d)=%q events(%d)=%v editor=%q IME unavailable closed=%t", len(p.signals), signals, len(p.events), events, p.editor.String(), w.closed)
			return
		}
		answer <- fmt.Sprintf("signals(%d)=%q events(%d)=%v editor=%q selection=%+v focused=%t nativeKeys=%t webFocus=%#x focusSerial=%d top=%d/%d area=%d/%d entry=%d/%d imeActive=%t imeFocused=%t session=%t ending=%t engineComposing=%t nativeDepth=%d",
			len(p.signals), signals, len(p.events), events, p.editor.String(), p.editor.Selection(), w.focused, w.nativeKeys, w.webFocus, w.focusSerial,
			probe.topIn, probe.topOut, probe.areaIn, probe.areaOut, probe.entryIn, probe.entryOut,
			ime.active, ime.focused, ime.session, ime.ending, ime.engineComposing, ime.nativeDepth)
	})
	select {
	case trace := <-answer:
		return trace
	case <-ctx.Done():
		return fmt.Sprintf("failure trace unavailable: %v", ctx.Err())
	}
}

func gtkRegionRun(ctx context.Context, app *gtkApplication, probe *gtkRegionProbe, p *gtkTentativeProbe, path string) (err error) {
	driver, err := newGTKSeatDriver(ctx, path)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := driver.close(); err == nil && closeErr != nil {
			err = closeErr
		}
	}()
	if err := gtkRegionWait(ctx, app, probe, "content and native-view hole mapped", func(s gtkRegionState) bool {
		return s.mapped && s.views == 1 && s.viewportWidth == 800 && s.imeActive
	}); err != nil {
		return err
	}
	for _, op := range []string{"bind", "focus", "grab"} {
		if _, err := driver.command(op, "", "wayland"); err != nil {
			return err
		}
	}
	if err := gtkRegionWait(ctx, app, probe, "GTK parent focused", func(s gtkRegionState) bool {
		return s.focused && s.imeActive
	}); err != nil {
		return err
	}
	if err := gtkRegionSelectPinyin(ctx, app, probe); err != nil {
		return fmt.Errorf("initial Viva parent engine: %w", err)
	}
	if _, err := driver.commandAt("move", 120, 60); err != nil {
		return err
	}
	if err := gtkRegionWait(ctx, app, probe, "GDK titlebar coordinate origin", func(s gtkRegionState) bool {
		return s.gdkPointer == s.gtkArea && s.gdkX >= 0 && s.gdkY >= 0
	}); err != nil {
		return err
	}
	position, err := gtkRegionSnapshot(ctx, app, probe)
	if err != nil {
		return err
	}
	originX, originY := 120-int(position.gdkX), 60-int(position.gdkY)
	moveClick := func(label string, x, y int, childPointer bool, check func(gtkRegionState) bool) error {
		if _, err := driver.commandAt("move", x+originX, y+originY); err != nil {
			return fmt.Errorf("%s: %w", label, err)
		}
		if err := gtkRegionWait(ctx, app, probe, label+" pointer target", func(s gtkRegionState) bool {
			return s.childPointer == childPointer
		}); err != nil {
			return err
		}
		if _, err := driver.commandAt("click-at", x+originX, y+originY); err != nil {
			return fmt.Errorf("%s: %w", label, err)
		}
		return gtkRegionWait(ctx, app, probe, label, check)
	}
	if err := moveClick("plain content", 250, 180, true, func(s gtkRegionState) bool {
		return s.childPointer && s.contentClicks == 1 && !s.nativeKeys && s.imeActive
	}); err != nil {
		return err
	}
	if err := moveClick("GTK titlebar strip", 120, 60, false, func(s gtkRegionState) bool {
		return !s.childPointer && s.titlePresses == 1 && s.contentClicks == 1
	}); err != nil {
		return err
	}
	if err := moveClick("GTK left resize edge", 2, 180, false, func(s gtkRegionState) bool {
		return !s.childPointer && s.edgePresses == 1 && s.resizeCalls == 1 && s.resizeEdge == 3 && s.contentClicks == 1
	}); err != nil {
		return err
	}
	if err := moveClick("GtkEntry in native-view hole", 500, 300, false, func(s gtkRegionState) bool {
		return !s.childPointer && s.webFocus == probe.entry && s.gtkFocus == probe.entry && s.nativeKeys && !s.imeActive && s.contentClicks == 1 && s.entryButtons == 1
	}); err != nil {
		return err
	}
	if err := moveClick("Viva cover inside native-view hole", 440, 240, true, func(s gtkRegionState) bool {
		return s.childPointer && s.webFocus == 0 && s.gtkFocus == 0 && !s.nativeKeys && s.imeActive && s.contentClicks == 2 && s.entryButtons == 1
	}); err != nil {
		return err
	}
	if err := gtkRegionOnLoop(ctx, app, func() {
		probe.w.wl.preferredScale(180) // a 1.5x canvas mapped onto the same 800 logical pixels
		probe.w.present(platform.Frame{
			Rectangles: []render.Rectangle{{
				Kind: scene.KindRectangle, Bounds: scene.Rect{X: 510, Y: 235, Width: 40, Height: 40},
				Color: scene.Color{Red: 1, Alpha: 1}, Layer: 2,
			}},
			Update: render.Update{Count: 1, Rebuild: true},
		}, nil)
	}); err != nil {
		return err
	}
	if err := gtkRegionWait(ctx, app, probe, "fractional canvas and overlay", func(s gtkRegionState) bool {
		return s.scale == 1.5 && s.canvasWidth == 1200 && s.viewportWidth == 800 && s.overlays > 0
	}); err != nil {
		return err
	}
	if err := moveClick("1.5x uncovered GtkEntry", 500, 300, false, func(s gtkRegionState) bool {
		return !s.childPointer && s.webFocus == probe.entry && s.gtkFocus == probe.entry && s.nativeKeys && !s.imeActive && s.contentClicks == 2 && s.entryButtons == 2
	}); err != nil {
		return err
	}
	if err := gtkRegionSelectPinyin(ctx, app, probe); err != nil {
		return fmt.Errorf("GtkEntry engine before scaled native-to-Viva click: %w", err)
	}
	entryEngine, entryEngineErr := nativeFcitxCommand("--check", "-n")
	before, err := gtkSeatState(ctx, app, p, &gtkSeatPointer{})
	if err != nil {
		return err
	}
	if _, err := driver.commandAt("move", 530+originX, 250+originY); err != nil {
		return err
	}
	if err := gtkRegionWait(ctx, app, probe, "1.5x drawn overlay pointer target", func(s gtkRegionState) bool {
		return s.childPointer
	}); err != nil {
		return err
	}
	if _, err := driver.commandAt("click-at", 530+originX, 250+originY); err != nil {
		return err
	}
	var beforeKeyEngine, beforeKeyStatus string
	if os.Getenv("VIVA_GTK_REGION_ENGINE_DIAG") == "1" {
		beforeKeyEngine, _ = nativeFcitxCommand("--check", "-n")
		beforeKeyStatus, _ = nativeFcitxCommand("--check")
	}
	for _, op := range []string{"down", "up"} {
		if _, err := driver.command(op, "n", ""); err != nil {
			return err
		}
	}
	afterKeyEngine, afterKeyEngineErr := nativeFcitxCommand("--check", "-n")
	afterKeyStatus, afterKeyStatusErr := nativeFcitxCommand("--check")
	if err := gtkRegionWait(ctx, app, probe, "1.5x drawn overlay returns pointer to Viva", func(s gtkRegionState) bool {
		return s.childPointer && s.webFocus == 0 && s.gtkFocus == 0 && !s.nativeKeys && s.imeActive && s.contentClicks == 3 && s.entryButtons == 2
	}); err != nil {
		return err
	}
	if err := gtkSeatWait(ctx, app, p, &gtkSeatPointer{}, "immediate key after 1.5x native-to-Viva click", func(s gtkSeatSnapshot) bool {
		return s.preeditSignals > before.preeditSignals && s.preeditEvents > before.preeditEvents && s.pressedKeys == 0 && s.composing
	}); err != nil {
		return fmt.Errorf("%w; Entry engine=%q (%v), before key engine/status=%q/%q, after key engine/status=%q/%q (%v/%v); %s", err,
			entryEngine, entryEngineErr, beforeKeyEngine, beforeKeyStatus, afterKeyEngine, afterKeyStatus, afterKeyEngineErr, afterKeyStatusErr,
			gtkRegionFailureTrace(ctx, app, probe, p))
	}
	if err := gtkRegionOnLoop(ctx, app, func() {}); err != nil {
		return err
	}
	after, err := gtkSeatState(ctx, app, p, &gtkSeatPointer{})
	if err != nil {
		return err
	}
	if !after.composing || !after.imeActive || after.cancelEvents != before.cancelEvents || after.preeditEvents <= before.preeditEvents {
		return fmt.Errorf("immediate preedit was lost after the focus handoff: before=%+v after=%+v", before, after)
	}
	return nil
}

// The seat enters a GDK-owned content surface, GTK titlebar, and native-view hole through real
// Weston/XTest pointer events. GtkEntry stands in for WebKit's GTK widget without loading WebKit.
func TestGTKNativeWaylandInputRegions(t *testing.T) {
	if os.Getenv("VIVA_GTK_REGION_NATIVE") != "1" || os.Getenv("VIVA_FCITX_BACKEND") != "wayland" {
		t.Skip("requires the private Wayland GTK input-region fixture")
	}
	path := os.Getenv("VIVA_GTK_SEAT_DRIVER")
	if path == "" || os.Getenv("VIVA_GTK_SEAT_NATIVE") != "1" || os.Getenv("VIVA_GTK_IME_NATIVE") != "1" || os.Getenv("VIVA_ACCESSIBILITY") == "0" {
		t.Fatal("native input-region fixture is missing its seat driver or GTK/accessibility settings")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
	connection, err := dbus.DialSession()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { connection.Close() })
	restoreA11yStatus(t, connection, "IsEnabled")
	var status dbus.Encoder
	status.String(a11yStatus)
	status.String("IsEnabled")
	status.Variant("b")
	status.Bool(true)
	if _, err := connection.Call(a11yBusName, a11yBusPath, dbusProperties, "Set", "ssv", status.Bytes()); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 35*time.Second)
	defer cancel()
	p := &gtkTentativeProbe{tentative: true}
	gtkTentativeProbeActive = p
	defer func() { gtkTentativeProbeActive = nil }()
	probe := &gtkRegionProbe{}
	result := make(chan error, 1)
	var app *gtkApplication
	var originalResize func(uintptr, int32, int32, int32, int32, uint32)
	err = RunGTKApplication(func(application platform.Application) error {
		app = application.(*gtkApplication)
		host := &platform.WebHost{}
		window, openErr := application.OpenWindow(platform.Config{
			Title: "Viva seat keyboard probe", Width: 800, Height: 600,
			WindowMode: platform.WindowCustomTitlebar, TitlebarHeight: 80,
			ClearColor: platform.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1},
			Web:        host, DynamicTextInput: true,
			NativeViews: []platform.NativeView{{
				ID: 7, Bounds: gfx.Rect{X: 420, Y: 220, Width: 160, Height: 120}, Layer: 1,
				Covers: []gfx.Rect{{X: 420, Y: 220, Width: 40, Height: 40}},
			}},
			Accessibility: []platform.AccessibilityNode{{
				ID: gfx.Item(1), Role: platform.A11yRoleTextField, Label: "Seat editor",
				X: 20, Y: 20, Width: 300, Height: 100, Focusable: true,
			}},
			OnAccessibility: func(request platform.AccessibilityRequest, _ func(platform.Frame) error) (bool, error) {
				return request.Node == gfx.Item(1) && request.Action == platform.AccessibilityFocus, nil
			},
			PressAt: func(x, y float32) platform.PressKind {
				if x >= 80 && x < 160 && y >= 50 && y < 70 {
					probe.titlePresses++
				}
				if x >= 0 && x < 6 && y >= 160 && y < 200 {
					probe.edgePresses++
				}
				if y < 80 {
					return platform.PressMove
				}
				return platform.PressClient
			},
			OnPointer: func(event platform.PointerEvent) (platform.Frame, error) {
				if event.Kind == platform.PointerDown {
					probe.contentClicks++
				}
				return platform.Frame{}, nil
			},
			OnRedraw: func() (platform.Frame, error) { return platform.Frame{}, nil },
		})
		if openErr != nil {
			return openErr
		}
		probe.w = window.(*gtkWindow)
		if typeNameOf(app.g, app.g.gdkDisplayDefault()) != "GdkWaylandDisplay" || probe.w.wl == nil || probe.w.overlay == 0 {
			probe.w.destroy()
			return fmt.Errorf("input-region test requires the Wayland subsurface and GtkOverlay")
		}
		var entryNew func() uintptr
		if !app.g.lookup(&entryNew, "gtk_entry_new") {
			probe.w.destroy()
			return fmt.Errorf("GTK input-region fixture lacks GtkEntry")
		}
		probe.entry = entryNew()
		if probe.entry == 0 {
			probe.w.destroy()
			return fmt.Errorf("GTK cannot create the input-region GtkEntry")
		}
		g := app.g
		if !g.lookup(&probe.getFocus, "gtk_window_get_focus") {
			probe.w.destroy()
			return fmt.Errorf("GTK input-region fixture lacks gtk_window_get_focus")
		}
		g.widgetSetHAlign(probe.entry, 1)
		g.widgetSetVAlign(probe.entry, 1)
		g.widgetSetMarginX(probe.entry, 420)
		g.widgetSetMarginY(probe.entry, 220)
		g.widgetSetSize(probe.entry, 160, 120)
		g.overlayAdd(probe.w.overlay, probe.entry)
		g.connect(probe.entry, "focus-in-event", callbacks().event, probe.w.id)
		g.connect(probe.entry, "focus-out-event", callbacks().event, probe.w.id)
		for _, target := range []struct {
			widget  uintptr
			in, out *int
		}{
			{probe.w.toplevel, &probe.topIn, &probe.topOut},
			{probe.w.area, &probe.areaIn, &probe.areaOut},
			{probe.entry, &probe.entryIn, &probe.entryOut},
		} {
			in, out := target.in, target.out
			g.connect(target.widget, "focus-in-event", purego.NewCallback(func(_, _, _ uintptr) uintptr {
				(*in)++
				return 0
			}), 0)
			g.connect(target.widget, "focus-out-event", purego.NewCallback(func(_, _, _ uintptr) uintptr {
				(*out)++
				return 0
			}), 0)
		}
		g.connect(probe.entry, "button-press-event", purego.NewCallback(func(_, _, _ uintptr) uintptr {
			probe.entryButtons++
			return 0
		}), 0)
		g.widgetShow(probe.entry)
		originalResize = g.windowBeginResize
		g.windowBeginResize = func(window uintptr, edge, button, rootX, rootY int32, timestamp uint32) {
			if window == probe.w.toplevel {
				probe.resizeCalls++
				probe.resizeEdge = edge
				return // keep the test window fixed while checking the native edge path
			}
			originalResize(window, edge, button, rootX, rootY, timestamp)
		}
		app.enqueue(func() {
			editor := text.NewEditor("abc")
			if !editor.EnableTentativeComposition() {
				result <- fmt.Errorf("region editor refused tentative composition")
				probe.w.destroy()
				return
			}
			p.w = probe.w
			p.install(editor)
			go func() {
				result <- gtkRegionRun(ctx, app, probe, p, path)
				app.enqueue(func() { probe.w.destroy() })
			}()
		})
		return nil
	})
	if originalResize != nil {
		app.g.windowBeginResize = originalResize
	}
	if err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-result:
		if err != nil {
			t.Fatal(err)
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
}
