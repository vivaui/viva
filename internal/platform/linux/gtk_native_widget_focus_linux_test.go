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
	"github.com/vivaui/viva/text"
)

type gtkNativeWidgetFocusState struct {
	windowFocused, otherFocused, nativeKeys, imeActive       bool
	windowFocusedAtOtherClick                                bool
	pointerOnEditor, pointerOnOther                          bool
	webFocus, windowFocus, afterTakeKeys                     uintptr
	focusLost, focusLostAtOtherClick, focusLostAtReturnClick int
	otherClicks, editorClicks, entryIn, entryOut             int
	eventOrder                                               []string
}

type gtkNativeWidgetTrace struct {
	entryIn, entryOut         int
	focusLostAtOtherClick     int
	windowFocusedAtOtherClick bool
	focusLostAtReturnClick    int
	afterTakeKeys             uintptr
	getFocus                  func(uintptr) uintptr
	events                    []string
}

var gtkNativeWidgetTraceActive *gtkNativeWidgetTrace

func (trace *gtkNativeWidgetTrace) note(event string) {
	if len(trace.events) == 32 {
		copy(trace.events, trace.events[1:])
		trace.events = trace.events[:31]
	}
	trace.events = append(trace.events, event)
}

func gtkNativeWidgetState(ctx context.Context, app *gtkApplication, editor, other *gtkWindow, lost, otherClicks, editorClicks *int) (gtkNativeWidgetFocusState, error) {
	answer := make(chan gtkNativeWidgetFocusState, 1)
	app.enqueue(func() {
		state := gtkNativeWidgetFocusState{
			windowFocused: editor.focused, otherFocused: other.focused,
			nativeKeys: editor.nativeKeys, webFocus: editor.webFocus,
			imeActive:       editor.ime != nil && editor.ime.active,
			pointerOnEditor: editor.wl != nil && editor.wl.s.focus == editor.wl,
			pointerOnOther:  other.wl != nil && other.wl.s.focus == other.wl,
			focusLost:       *lost, otherClicks: *otherClicks, editorClicks: *editorClicks,
		}
		if trace := gtkNativeWidgetTraceActive; trace != nil {
			state.windowFocus = trace.getFocus(editor.toplevel)
			state.afterTakeKeys = trace.afterTakeKeys
			state.entryIn, state.entryOut = trace.entryIn, trace.entryOut
			state.focusLostAtOtherClick = trace.focusLostAtOtherClick
			state.windowFocusedAtOtherClick = trace.windowFocusedAtOtherClick
			state.focusLostAtReturnClick = trace.focusLostAtReturnClick
			state.eventOrder = append([]string(nil), trace.events...)
		}
		answer <- state
	})
	select {
	case state := <-answer:
		return state, nil
	case <-ctx.Done():
		return gtkNativeWidgetFocusState{}, ctx.Err()
	}
}

func gtkNativeWidgetWait(ctx context.Context, app *gtkApplication, editor, other *gtkWindow, lost, otherClicks, editorClicks *int, label string, check func(gtkNativeWidgetFocusState) bool) error {
	deadline := time.NewTimer(2 * time.Second)
	defer deadline.Stop()
	var last gtkNativeWidgetFocusState
	for {
		state, err := gtkNativeWidgetState(ctx, app, editor, other, lost, otherClicks, editorClicks)
		if err != nil {
			return fmt.Errorf("%s: %w", label, err)
		}
		last = state
		if check(state) {
			return nil
		}
		select {
		case <-deadline.C:
			return fmt.Errorf("%s: deadline; A.focused=%t B.focused=%t B.pointer=%t nativeKeys=%t webFocus=%#x gtkFocus=%#x afterTakeKeys=%#x entryIn/out=%d/%d IME=%t OnFocusLost=%d B.pointerDown=%d A.pointerDown=%d events=%q",
				label, last.windowFocused, last.otherFocused, last.pointerOnOther, last.nativeKeys, last.webFocus,
				last.windowFocus, last.afterTakeKeys, last.entryIn, last.entryOut,
				last.imeActive, last.focusLost, last.otherClicks, last.editorClicks, last.eventOrder)
		case <-ctx.Done():
			return fmt.Errorf("%s: %w", label, ctx.Err())
		case <-time.After(10 * time.Millisecond):
		}
	}
}

func gtkNativeWidgetFocusRun(ctx context.Context, t *testing.T, app *gtkApplication, p *gtkTentativeProbe, other *gtkWindow, entry uintptr, lost, otherClicks, editorClicks *int, path string) (failures []string) {
	driver, err := newGTKSeatDriver(ctx, path)
	if err != nil {
		return []string{fmt.Sprintf("start GTK native-widget seat driver: %v", err)}
	}
	defer func() {
		if err := driver.close(); err != nil {
			failures = append(failures, err.Error())
		}
	}()
	logState := func(stage string) {
		if os.Getenv("VIVA_GTK_SEAT_TRACE") != "1" {
			return
		}
		state, err := gtkNativeWidgetState(ctx, app, p.w, other, lost, otherClicks, editorClicks)
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", stage, err))
			return
		}
		t.Logf("native-widget %s: A.focused=%t B.focused=%t nativeKeys=%t webFocus=%#x gtkFocus=%#x afterTakeKeys=%#x entryIn/out=%d/%d IME=%t lost=%d clicks(A/B)=%d/%d order=%q",
			stage, state.windowFocused, state.otherFocused, state.nativeKeys, state.webFocus,
			state.windowFocus, state.afterTakeKeys, state.entryIn, state.entryOut,
			state.imeActive, state.focusLost, state.editorClicks, state.otherClicks, state.eventOrder)
	}
	pointer := &gtkSeatPointer{}
	if err := gtkSeatWait(ctx, app, p, pointer, "A mapped editor", func(s gtkSeatSnapshot) bool { return s.mapped && s.imeActive }); err != nil {
		return []string{err.Error()}
	}
	for _, op := range []string{"bind", "focus", "grab"} {
		if _, err := driver.command(op, "", "wayland"); err != nil {
			return []string{err.Error()}
		}
	}
	if err := gtkNativeWidgetWait(ctx, app, p.w, other, lost, otherClicks, editorClicks, "A initial focus", func(s gtkNativeWidgetFocusState) bool {
		return s.windowFocused && s.imeActive
	}); err != nil {
		return []string{err.Error()}
	}
	beforeEntry, err := gtkNativeWidgetState(ctx, app, p.w, other, lost, otherClicks, editorClicks)
	if err != nil {
		return []string{err.Error()}
	}
	grabbing := make(chan struct{}, 1)
	app.enqueue(func() {
		p.w.g.widgetGrabFocus(entry)
		grabbing <- struct{}{}
	})
	select {
	case <-grabbing:
	case <-ctx.Done():
		return []string{ctx.Err().Error()}
	}
	if err := gtkNativeWidgetWait(ctx, app, p.w, other, lost, otherClicks, editorClicks, "real GtkEntry focus", func(s gtkNativeWidgetFocusState) bool {
		return s.windowFocused && s.nativeKeys && s.webFocus == entry && !s.imeActive && s.focusLost == beforeEntry.focusLost+1
	}); err != nil {
		return []string{err.Error()}
	}
	baseline, err := gtkNativeWidgetState(ctx, app, p.w, other, lost, otherClicks, editorClicks)
	if err != nil {
		return []string{err.Error()}
	}
	shown := make(chan struct{}, 1)
	app.enqueue(func() {
		other.g.widgetShowAll(other.toplevel)
		shown <- struct{}{}
	})
	select {
	case <-shown:
	case <-ctx.Done():
		return []string{ctx.Err().Error()}
	}
	if _, err := driver.command("probe-switch", "", ""); err != nil {
		return []string{err.Error()}
	}
	clickX, clickY := -1, -1
	for _, y := range []int{300, 220, 380, 140, 460, 60, 540} {
		for x := 40; x < 800; x += 80 {
			if _, err := driver.commandAt("move", x, y); err != nil {
				return []string{err.Error()}
			}
			for attempt := 0; attempt < 6; attempt++ {
				state, err := gtkNativeWidgetState(ctx, app, p.w, other, lost, otherClicks, editorClicks)
				if err != nil {
					return []string{err.Error()}
				}
				if state.pointerOnOther {
					select {
					case <-ctx.Done():
						return []string{ctx.Err().Error()}
					case <-time.After(10 * time.Millisecond):
					}
					state, err = gtkNativeWidgetState(ctx, app, p.w, other, lost, otherClicks, editorClicks)
					if err != nil {
						return []string{err.Error()}
					}
					if state.pointerOnOther {
						clickX, clickY = x, y
						break
					}
				}
				select {
				case <-ctx.Done():
					return []string{ctx.Err().Error()}
				case <-time.After(10 * time.Millisecond):
				}
			}
			if clickX >= 0 {
				break
			}
		}
		if clickX >= 0 {
			break
		}
	}
	if clickX < 0 {
		return []string{"real XTest pointer never entered B's Wayland content surface"}
	}
	if _, err := driver.commandAt("click-at", clickX, clickY); err != nil {
		return []string{err.Error()}
	}
	if err := gtkNativeWidgetWait(ctx, app, p.w, other, lost, otherClicks, editorClicks, "real B pointer-down", func(s gtkNativeWidgetFocusState) bool {
		return s.otherClicks == 1
	}); err != nil {
		return []string{err.Error()}
	}
	logState("B clicked")
	var afterSwitch gtkNativeWidgetFocusState
	if err := gtkNativeWidgetWait(ctx, app, p.w, other, lost, otherClicks, editorClicks, "external A to B focus", func(s gtkNativeWidgetFocusState) bool {
		afterSwitch = s
		return !s.windowFocused && s.otherFocused && !s.imeActive && s.focusLost >= baseline.focusLost+1
	}); err != nil {
		failures = append(failures, err.Error())
	} else {
		// B's press may precede A's focus-out, which then accounts for one more loss.
		want := afterSwitch.focusLostAtOtherClick
		if afterSwitch.windowFocusedAtOtherClick {
			want++
		}
		if afterSwitch.focusLost != want {
			failures = append(failures, fmt.Sprintf("A OnFocusLost after B.PointerDown=%d, at B.PointerDown=%d A.focused=%t, want %d; events=%q",
				afterSwitch.focusLost, afterSwitch.focusLostAtOtherClick, afterSwitch.windowFocusedAtOtherClick, want, afterSwitch.eventOrder))
		}
	}
	logState("before B hide")
	hidden := make(chan struct{}, 1)
	app.enqueue(func() {
		if trace := gtkNativeWidgetTraceActive; trace != nil {
			trace.note("B.hide begin")
		}
		other.g.widgetHide(other.toplevel)
		if trace := gtkNativeWidgetTraceActive; trace != nil {
			trace.note("B.hide end")
		}
		hidden <- struct{}{}
	})
	select {
	case <-hidden:
	case <-ctx.Done():
		return append(failures, ctx.Err().Error())
	}
	logState("after B hide, before A click")
	if _, err := driver.command("move-editor", "", ""); err != nil {
		return append(failures, err.Error())
	}
	if err := gtkNativeWidgetWait(ctx, app, p.w, other, lost, otherClicks, editorClicks, "A editor pointer entry", func(s gtkNativeWidgetFocusState) bool {
		return s.pointerOnEditor && !s.pointerOnOther
	}); err != nil {
		return append(failures, err.Error())
	}
	if _, err := driver.command("click", "", ""); err != nil {
		return append(failures, err.Error())
	}
	logState("after A click dispatch")
	if err := gtkNativeWidgetWait(ctx, app, p.w, other, lost, otherClicks, editorClicks, "A editor return click", func(s gtkNativeWidgetFocusState) bool {
		return s.editorClicks == 1 && s.windowFocused && !s.nativeKeys && s.webFocus == 0 && s.imeActive
	}); err != nil {
		logState("A return timeout")
		return append(failures, err.Error())
	}
	logState("A editor restored")
	if state, err := gtkNativeWidgetState(ctx, app, p.w, other, lost, otherClicks, editorClicks); err != nil {
		return append(failures, err.Error())
	} else if state.focusLost != state.focusLostAtReturnClick {
		failures = append(failures, fmt.Sprintf("A OnFocusLost after return=%d, at A.PointerDown=%d; A.focused=%t B.focused=%t nativeKeys=%t webFocus=%#x gtkFocus=%#x afterTakeKeys=%#x entryIn/out=%d/%d IME=%t clicks(A/B)=%d/%d events=%q",
			state.focusLost, state.focusLostAtReturnClick, state.windowFocused, state.otherFocused, state.nativeKeys,
			state.webFocus, state.windowFocus, state.afterTakeKeys, state.entryIn, state.entryOut,
			state.imeActive, state.editorClicks, state.otherClicks, state.eventOrder))
	}
	deadline := time.NewTimer(3 * time.Second)
	defer deadline.Stop()
	for {
		active, err := nativeFcitxCommand("--check", "-n")
		if err != nil {
			return append(failures, fmt.Sprintf("A Fcitx reactivation: %v", err))
		}
		if active != "" {
			break
		}
		select {
		case <-deadline.C:
			return append(failures, "A Fcitx did not reactivate")
		case <-ctx.Done():
			return append(failures, ctx.Err().Error())
		case <-time.After(20 * time.Millisecond):
		}
	}
	if err := selectNativeFcitxEngine("pinyin"); err != nil {
		return append(failures, fmt.Sprintf("A select Fcitx pinyin: %v", err))
	}
	for _, key := range []string{"n", "i"} {
		before, err := gtkSeatState(ctx, app, p, pointer)
		if err != nil {
			return append(failures, err.Error())
		}
		if _, err := driver.command("down", key, ""); err != nil {
			return append(failures, err.Error())
		}
		if _, err := driver.command("up", key, ""); err != nil {
			return append(failures, err.Error())
		}
		if err := gtkSeatWait(ctx, app, p, pointer, "A restored pinyin "+key, func(s gtkSeatSnapshot) bool {
			return s.preeditSignals > before.preeditSignals && s.preeditEvents > before.preeditEvents
		}); err != nil {
			failures = append(failures, err.Error())
		}
	}
	beforeCancel, err := gtkSeatState(ctx, app, p, pointer)
	if err != nil {
		return append(failures, err.Error())
	}
	for _, op := range []string{"down", "up"} {
		if _, err := driver.command(op, "Escape", ""); err != nil {
			return append(failures, err.Error())
		}
	}
	if err := gtkSeatWait(ctx, app, p, pointer, "A restored Escape", func(s gtkSeatSnapshot) bool {
		return s.cancelEvents == beforeCancel.cancelEvents+1 && s.value == "abc" && !s.composing && !s.canUndo && s.pressedKeys == 0
	}); err != nil {
		failures = append(failures, err.Error())
	}
	return failures
}

// GtkEntry exercises native GTK focus ownership without loading a WebKit page.
func TestGTKNativeWaylandWidgetExternalFocus(t *testing.T) {
	if os.Getenv("VIVA_GTK_NATIVE_WIDGET_FOCUS") != "1" || os.Getenv("VIVA_FCITX_BACKEND") != "wayland" {
		t.Skip("requires the private Wayland native-widget seat fixture")
	}
	path := os.Getenv("VIVA_GTK_SEAT_DRIVER")
	if path == "" || os.Getenv("VIVA_GTK_SEAT_NATIVE") != "1" || os.Getenv("VIVA_GTK_IME_NATIVE") != "1" || os.Getenv("VIVA_ACCESSIBILITY") == "0" {
		t.Fatal("native-widget focus fixture is missing its driver or GTK/accessibility settings")
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
	trace := &gtkNativeWidgetTrace{}
	gtkNativeWidgetTraceActive = trace
	defer func() { gtkNativeWidgetTraceActive = nil }()
	result := make(chan []string, 1)
	focusLost, otherClicks, editorClicks := 0, 0, 0
	err = RunGTKApplication(func(application platform.Application) error {
		app := application.(*gtkApplication)
		host := &platform.WebHost{}
		var aWindow *gtkWindow
		first, err := application.OpenWindow(platform.Config{
			Title: "Viva seat keyboard probe", Width: 800, Height: 600, WindowMode: platform.WindowFrameless,
			ClearColor: platform.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1},
			Web:        host, DynamicTextInput: true,
			Accessibility: []platform.AccessibilityNode{{
				ID: gfx.Item(1), Role: platform.A11yRoleTextField, Label: "Seat editor",
				X: 20, Y: 20, Width: 300, Height: 100, Focusable: true,
			}},
			OnAccessibility: func(request platform.AccessibilityRequest, _ func(platform.Frame) error) (bool, error) {
				return request.Node == gfx.Item(1) && request.Action == platform.AccessibilityFocus, nil
			},
			OnFocusLost: func() (platform.Frame, error) {
				focusLost++
				if aWindow != nil {
					trace.note(fmt.Sprintf("A.OnFocusLost focused=%t nativeKeys=%t webFocus=%#x serial=%d", aWindow.focused, aWindow.nativeKeys, aWindow.webFocus, aWindow.focusSerial))
				} else {
					trace.note("A.OnFocusLost")
				}
				return platform.Frame{}, nil
			},
			OnPointer: func(event platform.PointerEvent) (platform.Frame, error) {
				if event.Kind == platform.PointerDown {
					editorClicks++
					trace.focusLostAtReturnClick = focusLost
					if aWindow != nil && trace.getFocus != nil {
						trace.afterTakeKeys = trace.getFocus(aWindow.toplevel)
						trace.note(fmt.Sprintf("A.PointerDown after takeKeys gtkFocus=%#x nativeKeys=%t", trace.afterTakeKeys, aWindow.nativeKeys))
					}
				}
				return platform.Frame{}, nil
			},
			OnRedraw: func() (platform.Frame, error) { return platform.Frame{}, nil },
		})
		if err != nil {
			return err
		}
		aWindow = first.(*gtkWindow)
		p.w = aWindow
		if !aWindow.g.lookup(&trace.getFocus, "gtk_window_get_focus") {
			aWindow.destroy()
			return fmt.Errorf("GTK native-widget focus fixture lacks gtk_window_get_focus")
		}
		for _, signal := range []struct{ name, label string }{{"focus-in-event", "A.top.focus-in"}, {"focus-out-event", "A.top.focus-out"}} {
			label := signal.label
			aWindow.g.connect(aWindow.toplevel, signal.name, purego.NewCallback(func(_, _, _ uintptr) uintptr {
				trace.note(label)
				return 0
			}), 0)
		}
		// The production focus handler stops the specific signal; the generic event runs first.
		aWindow.g.connect(aWindow.area, "event", purego.NewCallback(func(_, event, _ uintptr) uintptr {
			if eventType(event) == gdkFocusChange {
				if (*gdkEventFocus)(unsafePointer(event)).In != 0 {
					trace.note("A.area.focus-in")
				} else {
					trace.note("A.area.focus-out")
				}
			}
			return 0
		}), 0)
		second, err := application.OpenWindow(platform.Config{
			Title: "Viva native focus switch", Width: 800, Height: 600, WindowMode: platform.WindowFrameless,
			ClearColor: platform.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1},
			Accessibility: []platform.AccessibilityNode{{
				ID: gfx.Item(2), Role: platform.A11yRoleButton, Label: "Switch window",
				X: 100, Y: 300, Width: 600, Height: 200,
			}},
			OnPointer: func(event platform.PointerEvent) (platform.Frame, error) {
				if event.Kind == platform.PointerDown {
					otherClicks++
					trace.focusLostAtOtherClick = focusLost
					trace.windowFocusedAtOtherClick = aWindow.focused
					trace.note(fmt.Sprintf("B.PointerDown A.focused=%t lost=%d", aWindow.focused, focusLost))
				}
				return platform.Frame{}, nil
			},
			OnRedraw: func() (platform.Frame, error) { return platform.Frame{}, nil },
		})
		if err != nil {
			aWindow.destroy()
			return err
		}
		bWindow := second.(*gtkWindow)
		bWindow.g.widgetHide(bWindow.toplevel)
		bWindow.g.windowSetTransient(bWindow.toplevel, aWindow.toplevel)
		var entryNew func() uintptr
		if aWindow.overlay == 0 || !aWindow.g.lookup(&entryNew, "gtk_entry_new") {
			bWindow.destroy()
			aWindow.destroy()
			return fmt.Errorf("GTK native-widget focus fixture lacks GtkOverlay or GtkEntry")
		}
		entry := entryNew()
		if entry == 0 {
			bWindow.destroy()
			aWindow.destroy()
			return fmt.Errorf("GTK could not create native GtkEntry")
		}
		g := aWindow.g
		g.widgetSetHAlign(entry, 1)
		g.widgetSetVAlign(entry, 1)
		g.widgetSetMarginX(entry, 500)
		g.widgetSetMarginY(entry, 400)
		g.widgetSetSize(entry, 100, 32)
		g.overlayAdd(aWindow.overlay, entry)
		g.connect(entry, "focus-in-event", callbacks().event, aWindow.id)
		g.connect(entry, "focus-out-event", callbacks().event, aWindow.id)
		g.connect(entry, "focus-in-event", purego.NewCallback(func(_, _, _ uintptr) uintptr {
			trace.entryIn++
			trace.note("GtkEntry.focus-in")
			return 0
		}), 0)
		g.connect(entry, "focus-out-event", purego.NewCallback(func(_, _, _ uintptr) uintptr {
			trace.entryOut++
			trace.note("GtkEntry.focus-out")
			return 0
		}), 0)
		g.widgetShow(entry)
		app.enqueue(func() {
			editor := text.NewEditor("abc")
			if !editor.EnableTentativeComposition() {
				result <- []string{"native-widget editor refused tentative composition"}
				bWindow.destroy()
				aWindow.destroy()
				return
			}
			p.install(editor)
			go func() {
				failures := gtkNativeWidgetFocusRun(ctx, t, app, p, bWindow, entry, &focusLost, &otherClicks, &editorClicks, path)
				result <- failures
				app.enqueue(func() {
					bWindow.destroy()
					aWindow.destroy()
				})
			}()
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case failures := <-result:
		for _, failure := range failures {
			t.Error(failure)
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
}
