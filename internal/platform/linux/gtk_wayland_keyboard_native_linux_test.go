//go:build linux && vivagtk

package linux

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/dbus"
	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/text"
)

type gtkSeatSnapshot struct {
	mapped, focused, imeActive, composing, canUndo, canRedo bool
	pointers, signalCount, eventCount                       int
	preeditSignals, preeditEvents, cancelEvents, commits    int
	pressedKeys, clickCount                                 int
	value                                                   string
	lastSignal                                              string
	lastEvent                                               platform.TextEvent
	lastEvents                                              []string
	selection, mirroredSelection, composition               text.Range
	compositionActive                                       bool
	editorIdentity, mirrorIdentity, imeIdentity             text.CompositionIdentity
}

type gtkSeatPointer struct {
	downs, clickCount int
}

type gtkSeatSurfaceState struct {
	child                     uintptr
	oldRegistered, registered bool
	display                   uintptr
	closed                    bool
}

type gtkSeatDriver struct {
	cmd    *exec.Cmd
	input  io.WriteCloser
	output *json.Decoder
}

type gtkSeatReply struct {
	OK     bool   `json:"ok"`
	Error  string `json:"error"`
	Code   uint16 `json:"code"`
	Window uint64 `json:"window"`
}

type gtkSeatRequest struct {
	Op      string `json:"op"`
	Key     string `json:"key,omitempty"`
	Backend string `json:"backend,omitempty"`
	PID     int    `json:"pid,omitempty"`
	X       int    `json:"x,omitempty"`
	Y       int    `json:"y,omitempty"`
}

func newGTKSeatDriver(ctx context.Context, path string) (*gtkSeatDriver, error) {
	cmd := exec.CommandContext(ctx, "/usr/bin/python3", path)
	input, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	output, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	driver := &gtkSeatDriver{cmd: cmd, input: input, output: json.NewDecoder(output)}
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return driver, nil
}

func (d *gtkSeatDriver) command(op, key, backend string) (gtkSeatReply, error) {
	return d.send(gtkSeatRequest{Op: op, Key: key, Backend: backend, PID: os.Getpid()})
}

func (d *gtkSeatDriver) commandAt(op string, x, y int) (gtkSeatReply, error) {
	return d.send(gtkSeatRequest{Op: op, PID: os.Getpid(), X: x, Y: y})
}

func (d *gtkSeatDriver) send(request gtkSeatRequest) (gtkSeatReply, error) {
	if err := json.NewEncoder(d.input).Encode(request); err != nil {
		return gtkSeatReply{}, fmt.Errorf("seat %s command: %w", request.Op, err)
	}
	var response gtkSeatReply
	if err := d.output.Decode(&response); err != nil {
		return response, fmt.Errorf("seat %s response: %w (see driver stderr)", request.Op, err)
	}
	if !response.OK {
		return response, fmt.Errorf("seat %s: %s", request.Op, response.Error)
	}
	return response, nil
}

func (d *gtkSeatDriver) close() error {
	_, _ = d.command("quit", "", "")
	_ = d.input.Close()
	if err := d.cmd.Wait(); err != nil {
		return fmt.Errorf("seat driver exited: %w (see driver stderr)", err)
	}
	return nil
}

func gtkSeatState(ctx context.Context, a *gtkApplication, p *gtkTentativeProbe, pointers *gtkSeatPointer) (gtkSeatSnapshot, error) {
	answer := make(chan gtkSeatSnapshot, 1)
	a.enqueue(func() {
		w := p.w
		state := gtkSeatSnapshot{
			focused: w.focused, pointers: pointers.downs, clickCount: pointers.clickCount,
			value: p.editor.String(), canUndo: p.editor.CanUndo(), canRedo: p.editor.CanRedo(),
			signalCount: len(p.signals), eventCount: len(p.events),
			composing: w.text.state.Composing, pressedKeys: len(w.keys.down),
			selection: p.editor.Selection(), mirroredSelection: w.text.state.Selection,
			editorIdentity: p.editor.TentativeCompositionIdentity(), mirrorIdentity: w.text.state.CompositionIdentity,
		}
		state.composition, state.compositionActive = p.editor.Composition()
		for _, signal := range p.signals {
			if strings.HasPrefix(signal, "preedit:") && len(signal) > len("preedit:") {
				state.preeditSignals++
			}
			if strings.HasPrefix(signal, "commit:") {
				state.commits++
			}
		}
		if len(p.signals) > 0 {
			state.lastSignal = p.signals[len(p.signals)-1]
		}
		for _, event := range p.events {
			if event.Kind == platform.TextSetMarked && event.CompositionPhase == text.CompositionUpdate && event.Text != "" {
				state.preeditEvents++
			}
			if event.CompositionPhase == text.CompositionCancel {
				state.cancelEvents++
			}
		}
		if len(p.events) > 0 {
			state.lastEvent = p.events[len(p.events)-1]
		}
		for _, event := range p.events[max(0, len(p.events)-4):] {
			state.lastEvents = append(state.lastEvents, fmt.Sprintf("%d/%d:%.24q", event.Kind, event.CompositionPhase, event.Text))
		}
		if w.ime != nil {
			state.imeActive = w.ime.ctx != 0 && w.ime.active
			state.imeIdentity = w.ime.owner
		}
		state.mapped = w.wl == nil || w.wl.child != 0
		answer <- state
	})
	select {
	case state := <-answer:
		return state, nil
	case <-ctx.Done():
		return gtkSeatSnapshot{}, ctx.Err()
	}
}

func gtkSeatSurface(ctx context.Context, a *gtkApplication, p *gtkTentativeProbe, old uintptr) (gtkSeatSurfaceState, error) {
	answer := make(chan gtkSeatSurfaceState, 1)
	a.enqueue(func() {
		w := p.w
		state := gtkSeatSurfaceState{child: w.wl.child, display: w.wl.s.display, closed: w.closed}
		state.oldRegistered = w.wl.s.surfaces[old] == w.wl
		state.registered = state.child != 0 && w.wl.s.surfaces[state.child] == w.wl
		answer <- state
	})
	select {
	case state := <-answer:
		return state, nil
	case <-ctx.Done():
		return gtkSeatSurfaceState{}, ctx.Err()
	}
}

func gtkSeatWait(ctx context.Context, a *gtkApplication, p *gtkTentativeProbe, pointers *gtkSeatPointer, label string, check func(gtkSeatSnapshot) bool) error {
	deadline := time.NewTimer(2 * time.Second)
	defer deadline.Stop()
	var last gtkSeatSnapshot
	for {
		state, err := gtkSeatState(ctx, a, p, pointers)
		if err != nil {
			return fmt.Errorf("%s: %w", label, err)
		}
		last = state
		if check(state) {
			return nil
		}
		select {
		case <-deadline.C:
			return fmt.Errorf("%s: deadline; focused=%t imeActive=%t mapped=%t pointerDown=%d preedit=%d/%d cancel=%d keysDown=%d composing=%t undo=%t textLen=%d signals=%d events=%d lastSignal=%.80q lastEvent=%d/%d:%.80q",
				label, last.focused, last.imeActive, last.mapped, last.pointers, last.preeditSignals, last.preeditEvents, last.cancelEvents, last.pressedKeys,
				last.composing, last.canUndo, len(last.value), last.signalCount, last.eventCount,
				last.lastSignal, last.lastEvent.Kind, last.lastEvent.CompositionPhase, last.lastEvent.Text)
		case <-ctx.Done():
			return fmt.Errorf("%s: %w", label, ctx.Err())
		case <-time.After(10 * time.Millisecond):
		}
	}
}

func gtkSeatRun(ctx context.Context, t *testing.T, a *gtkApplication, p *gtkTentativeProbe, pointers *gtkSeatPointer, backend, path string, remap bool) (failures []string) {
	driver, err := newGTKSeatDriver(ctx, path)
	if err != nil {
		return []string{fmt.Sprintf("start XTest/AT-SPI driver: %v", err)}
	}
	defer func() {
		if err := driver.close(); err != nil {
			failures = append(failures, err.Error())
		}
	}()
	if err := gtkSeatWait(ctx, a, p, pointers, "mapped native window", func(s gtkSeatSnapshot) bool { return s.mapped && s.imeActive }); err != nil {
		return []string{err.Error()}
	}
	if backend == "wayland" && os.Getenv("VIVA_GTK_SEAT_TRACE") == "1" {
		surfaces := make(chan [2]uintptr, 1)
		a.enqueue(func() {
			w := p.w
			surfaces <- [2]uintptr{
				w.wl.s.surfaceOf(w.g.widgetGetWindow(w.toplevel)),
				w.wl.s.surfaceOf(w.g.widgetGetWindow(w.area)),
			}
		})
		select {
		case value := <-surfaces:
			t.Logf("GDK area Wayland surface: top=%#x area=%#x nonzero=%t distinct=%t", value[0], value[1], value[1] != 0, value[1] != 0 && value[1] != value[0])
		case <-ctx.Done():
			return []string{ctx.Err().Error()}
		}
	}
	for _, operation := range []string{"bind", "focus", "grab"} {
		if _, err := driver.command(operation, "", backend); err != nil {
			return []string{err.Error()}
		}
	}
	if err := gtkSeatWait(ctx, a, p, pointers, "parent GTK focus", func(s gtkSeatSnapshot) bool { return s.focused && s.imeActive }); err != nil {
		return []string{err.Error()}
	}
	selectPinyin := func() error {
		deadline := time.NewTimer(3 * time.Second)
		defer deadline.Stop()
		for {
			active, err := nativeFcitxCommand("--check", "-n")
			if err != nil {
				return fmt.Errorf("Fcitx client registration: %w", err)
			}
			if active != "" {
				break
			}
			select {
			case <-deadline.C:
				return fmt.Errorf("Fcitx client did not register the focused native window")
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(20 * time.Millisecond):
			}
		}
		return selectNativeFcitxEngine("pinyin")
	}
	if err := selectPinyin(); err != nil {
		return []string{fmt.Sprintf("select Fcitx pinyin: %v", err)}
	}
	trace := func(stage string, state gtkSeatSnapshot) {
		if !remap || os.Getenv("VIVA_GTK_SEAT_TRACE") != "1" || !strings.HasPrefix(stage, "remap ") {
			return
		}
		t.Logf("seat %s value=%.48q selection=%d:%d mirrorSelection=%d:%d composition=%d:%d active=%t owner=%v/%v/%v undo=%t redo=%t focused=%t ime=%t pointerDown=%d clickCount=%d signals=%d events=%d lastEvents=%q",
			stage, state.value, state.selection.Start, state.selection.End,
			state.mirroredSelection.Start, state.mirroredSelection.End,
			state.composition.Start, state.composition.End, state.compositionActive,
			state.editorIdentity, state.mirrorIdentity, state.imeIdentity,
			state.canUndo, state.canRedo, state.focused, state.imeActive,
			state.pointers, state.clickCount, state.signalCount, state.eventCount, state.lastEvents)
	}
	typeKey := func(phase, symbol string, preedit bool) {
		before, err := gtkSeatState(ctx, a, p, pointers)
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s before %s: %v", phase, symbol, err))
			return
		}
		trace(phase+" before "+symbol, before)
		pressed, err := driver.command("down", symbol, "")
		if err != nil {
			failures = append(failures, err.Error())
			return
		}
		released, err := driver.command("up", symbol, "")
		if err != nil {
			failures = append(failures, err.Error())
			return
		}
		if pressed.Code == 0 || pressed.Code != released.Code {
			failures = append(failures, fmt.Sprintf("%s XTest keycode mismatch for %s: %d/%d", phase, symbol, pressed.Code, released.Code))
			return
		}
		if preedit {
			if err := gtkSeatWait(ctx, a, p, pointers, phase+" GTK/Fcitx preedit "+symbol, func(s gtkSeatSnapshot) bool {
				return s.preeditSignals > before.preeditSignals && s.preeditEvents > before.preeditEvents
			}); err != nil {
				failures = append(failures, err.Error())
			}
		}
		if after, err := gtkSeatState(ctx, a, p, pointers); err == nil {
			trace(phase+" after "+symbol, after)
		} else {
			failures = append(failures, fmt.Sprintf("%s after %s: %v", phase, symbol, err))
		}
	}
	trial := func(phase string) {
		typeKey(phase, "n", true)
		typeKey(phase, "i", true)
		before, err := gtkSeatState(ctx, a, p, pointers)
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s before Escape: %v", phase, err))
			return
		}
		typeKey(phase, "Escape", false)
		if err := gtkSeatWait(ctx, a, p, pointers, phase+" Escape cancel", func(s gtkSeatSnapshot) bool {
			return s.cancelEvents == before.cancelEvents+1 && s.value == "abc" && !s.composing && !s.canUndo && s.pressedKeys == 0
		}); err != nil {
			failures = append(failures, err.Error())
		}
		select {
		case <-ctx.Done():
			failures = append(failures, fmt.Sprintf("%s native tail: %v", phase, ctx.Err()))
			return
		case <-time.After(50 * time.Millisecond):
		}
		after, err := gtkSeatState(ctx, a, p, pointers)
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s native tail: %v", phase, err))
		} else if after.cancelEvents != before.cancelEvents+1 || after.pressedKeys != 0 {
			failures = append(failures, fmt.Sprintf("%s native tail: cancellations=%d, want=%d; keysDown=%d", phase, after.cancelEvents, before.cancelEvents+1, after.pressedKeys))
		} else if strings.Contains(phase, "content") && after.pointers != 1 {
			failures = append(failures, fmt.Sprintf("%s native tail: pointerDown=%d, want=1", phase, after.pointers))
		}
		if err == nil {
			trace(phase+" settled", after)
		}
	}
	reset := func() error {
		ready := make(chan struct{}, 1)
		a.enqueue(func() {
			p.signals, p.events = nil, nil
			pointers.downs, pointers.clickCount = 0, 0
			ready <- struct{}{}
		})
		select {
		case <-ready:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	trial("parent AX focus")
	if err := reset(); err != nil {
		return append(failures, err.Error())
	}
	if _, err := driver.command("click", "", ""); err != nil {
		return append(failures, err.Error())
	}
	if err := gtkSeatWait(ctx, a, p, pointers, "content pointer-down", func(s gtkSeatSnapshot) bool { return s.pointers == 1 }); err != nil {
		return append(failures, err.Error())
	}
	trial("content click")
	if len(failures) != 0 {
		return failures
	}
	for cycle := 1; remap && cycle <= 2; cycle++ {
		label := fmt.Sprintf("remap %d", cycle)
		before, err := gtkSeatSurface(ctx, a, p, 0)
		if err != nil || before.child == 0 || !before.registered || before.display == 0 || before.closed {
			return append(failures, fmt.Sprintf("%s initial child: state=%+v error=%v", label, before, err))
		}
		hidden := make(chan struct{}, 1)
		a.enqueue(func() {
			p.w.g.widgetHide(p.w.toplevel)
			hidden <- struct{}{}
		})
		select {
		case <-hidden:
		case <-ctx.Done():
			return append(failures, fmt.Sprintf("%s hide: %v", label, ctx.Err()))
		}
		if err := gtkSeatWait(ctx, a, p, pointers, label+" hidden child", func(s gtkSeatSnapshot) bool { return !s.mapped }); err != nil {
			return append(failures, err.Error())
		}
		old, err := gtkSeatSurface(ctx, a, p, before.child)
		if err != nil || old.child != 0 || old.oldRegistered || old.display == 0 || old.closed {
			return append(failures, fmt.Sprintf("%s old child retained: state=%+v error=%v", label, old, err))
		}
		shown := make(chan struct{}, 1)
		a.enqueue(func() {
			p.w.g.widgetShowAll(p.w.toplevel)
			shown <- struct{}{}
		})
		select {
		case <-shown:
		case <-ctx.Done():
			return append(failures, fmt.Sprintf("%s show: %v", label, ctx.Err()))
		}
		if err := gtkSeatWait(ctx, a, p, pointers, label+" new child", func(s gtkSeatSnapshot) bool { return s.mapped }); err != nil {
			return append(failures, err.Error())
		}
		current, err := gtkSeatSurface(ctx, a, p, 0)
		if err != nil || current.child == 0 || !current.registered || current.display == 0 || current.closed {
			return append(failures, fmt.Sprintf("%s new child absent: state=%+v error=%v", label, current, err))
		}
		for _, operation := range []string{"bind", "focus", "grab"} {
			if _, err := driver.command(operation, "", backend); err != nil {
				return append(failures, fmt.Sprintf("%s: %v", label, err))
			}
		}
		if err := gtkSeatWait(ctx, a, p, pointers, label+" parent focus", func(s gtkSeatSnapshot) bool { return s.focused && s.imeActive }); err != nil {
			return append(failures, err.Error())
		}
		if err := selectPinyin(); err != nil {
			return append(failures, fmt.Sprintf("%s select Fcitx pinyin: %v", label, err))
		}
		if err := reset(); err != nil {
			return append(failures, err.Error())
		}
		trial(label + " parent AX focus")
		if len(failures) != 0 {
			return failures
		}
		if err := reset(); err != nil {
			return append(failures, err.Error())
		}
		if state, err := gtkSeatState(ctx, a, p, pointers); err == nil {
			trace(label+" before click", state)
		} else {
			return append(failures, fmt.Sprintf("%s before click: %v", label, err))
		}
		if _, err := driver.command("click", "", ""); err != nil {
			return append(failures, fmt.Sprintf("%s: %v", label, err))
		}
		if err := gtkSeatWait(ctx, a, p, pointers, label+" content pointer-down", func(s gtkSeatSnapshot) bool { return s.pointers == 1 }); err != nil {
			return append(failures, err.Error())
		}
		if state, err := gtkSeatState(ctx, a, p, pointers); err == nil {
			trace(label+" after click", state)
			if state.clickCount != 1 {
				failures = append(failures, fmt.Sprintf("%s first content click count=%d, want=1", label, state.clickCount))
			}
		} else {
			return append(failures, fmt.Sprintf("%s after click: %v", label, err))
		}
		trial(label + " content click")
		if len(failures) != 0 {
			return failures
		}
	}
	before, err := gtkSeatState(ctx, a, p, pointers)
	if err != nil {
		return append(failures, fmt.Sprintf("content commit baseline: %v", err))
	}
	typeKey("content commit", "n", true)
	typeKey("content commit", "i", true)
	typeKey("content commit", "space", false)
	if len(failures) != 0 {
		return failures
	}
	if err := gtkSeatWait(ctx, a, p, pointers, "content commit and caret", func(s gtkSeatSnapshot) bool {
		return s.commits == before.commits+1 && s.lastEvent.Kind == platform.TextInsert &&
			s.lastEvent.CompositionPhase == text.CompositionResult && strings.IndexFunc(s.value, func(r rune) bool { return r > 127 }) >= 0 &&
			!s.composing && !s.compositionActive && s.canUndo &&
			s.selection.Start == s.selection.End && s.selection.End > before.selection.End && s.pressedKeys == 0
	}); err != nil {
		return append(failures, err.Error())
	}
	undone := make(chan gtkSeatSnapshot, 1)
	a.enqueue(func() {
		if !p.editor.Undo() {
			undone <- gtkSeatSnapshot{}
			return
		}
		undone <- gtkSeatSnapshot{value: p.editor.String(), canUndo: p.editor.CanUndo(), canRedo: p.editor.CanRedo()}
	})
	select {
	case state := <-undone:
		if state.value != "abc" || state.canUndo || !state.canRedo {
			failures = append(failures, fmt.Sprintf("content commit Undo: text=%q undo=%t redo=%t, want abc/false/true", state.value, state.canUndo, state.canRedo))
		}
	case <-ctx.Done():
		failures = append(failures, fmt.Sprintf("content commit Undo: %v", ctx.Err()))
	}
	return failures
}

func TestGTKNativeSeatKeyboardAfterContentClick(t *testing.T) {
	testGTKNativeSeatKeyboard(t, false)
}

func TestGTKNativeWaylandSeatHideShow(t *testing.T) {
	if os.Getenv("VIVA_GTK_SEAT_REMAP") != "1" || os.Getenv("VIVA_FCITX_BACKEND") != "wayland" {
		t.Skip("requires the private Wayland seat remap fixture")
	}
	testGTKNativeSeatKeyboard(t, true)
}

func testGTKNativeSeatKeyboard(t *testing.T, remap bool) {
	t.Helper()
	if os.Getenv("VIVA_GTK_SEAT_NATIVE") != "1" {
		t.Skip("requires the private Xvfb/DBus/Fcitx seat fixture")
	}
	backend, path := os.Getenv("VIVA_FCITX_BACKEND"), os.Getenv("VIVA_GTK_SEAT_DRIVER")
	if (backend != "x11" && backend != "wayland") || path == "" || os.Getenv("VIVA_GTK_IME_NATIVE") != "1" || os.Getenv("VIVA_ACCESSIBILITY") == "0" {
		t.Fatal("native seat fixture is missing its backend, driver or GTK/accessibility settings")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("native seat driver: %v", err)
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
	result := make(chan []string, 1)
	pointers := gtkSeatPointer{}
	err = RunGTKApplication(func(application platform.Application) error {
		a := application.(*gtkApplication)
		window, openErr := application.OpenWindow(platform.Config{
			Title: "Viva seat keyboard probe", Width: 800, Height: 600, WindowMode: platform.WindowFrameless,
			ClearColor:       platform.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1},
			DynamicTextInput: true,
			Accessibility: []platform.AccessibilityNode{{
				ID: gfx.Item(1), Role: platform.A11yRoleTextField, Label: "Seat editor",
				X: 20, Y: 20, Width: 300, Height: 100, Focusable: true,
			}},
			OnAccessibility: func(request platform.AccessibilityRequest, _ func(platform.Frame) error) (bool, error) {
				return request.Node == gfx.Item(1) && request.Action == platform.AccessibilityFocus, nil
			},
			OnPointer: func(event platform.PointerEvent) (platform.Frame, error) {
				if event.Kind == platform.PointerDown {
					pointers.downs++
					pointers.clickCount = event.ClickCount
				}
				return platform.Frame{}, nil
			},
			OnRedraw: func() (platform.Frame, error) { return platform.Frame{}, nil },
		})
		if openErr != nil {
			return openErr
		}
		p.w = window.(*gtkWindow)
		actual := typeNameOf(a.g, a.g.gdkDisplayDefault())
		want := "GdkX11Display"
		if backend == "wayland" {
			want = "GdkWaylandDisplay"
		}
		if actual != want {
			p.w.destroy()
			return fmt.Errorf("GTK native seat backend=%s, want %s", actual, want)
		}
		if backend == "wayland" && p.w.wl == nil || backend == "x11" && p.w.wl != nil {
			p.w.destroy()
			return fmt.Errorf("native content surface does not match %s", backend)
		}
		a.enqueue(func() {
			editor := text.NewEditor("abc")
			if !editor.EnableTentativeComposition() {
				result <- []string{"editor refused tentative composition"}
				p.w.destroy()
				return
			}
			p.install(editor)
			go func() {
				failures := gtkSeatRun(ctx, t, a, p, &pointers, backend, path, remap)
				result <- failures
				a.enqueue(func() { p.w.destroy() })
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
