package linux

import (
	"encoding/binary"
	"errors"
	"reflect"
	"testing"
	"time"
	"unsafe"

	"github.com/vivaui/viva/internal/input"
	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/internal/render"
	"github.com/vivaui/viva/internal/scene"
	"github.com/vivaui/viva/ui"
)

func TestNormalizedKeysym(t *testing.T) {
	for _, test := range []struct {
		keysym uint32
		want   input.Key
	}{
		{keysymTab, input.KeyTab},
		{keysymISOTab, input.KeyTab},
		{keysymEnter, input.KeyEnter},
		{keysymKPEnter, input.KeyEnter},
		{' ', input.KeySpace},
		{keysymEscape, input.KeyEscape},
		{keysymLeft, input.KeyLeft},
		{keysymKPRight, input.KeyRight},
		{keysymUp, input.KeyUp},
		{keysymKPDown, input.KeyDown},
		{keysymHome, input.KeyHome},
		{keysymKPEnd, input.KeyEnd},
		{keysymBackspace, input.KeyBackspace},
		{keysymKPDelete, input.KeyDelete},
		{'A', input.KeyUnknown},
	} {
		if got := normalizedKeysym(test.keysym); got != test.want {
			t.Errorf("normalizedKeysym(%#x) = %d, want %d", test.keysym, got, test.want)
		}
	}
}

func TestKeymapUsesServerKeysyms(t *testing.T) {
	keymap := xcbKeymap{
		first:      8,
		last:       10,
		perKeycode: 2,
		keysyms: []uint32{
			0, 0,
			keysymTab, keysymISOTab,
			'A', 'a',
		},
	}
	if got := keymap.key(9); got != input.KeyTab {
		t.Fatalf("key 9 = %d, want Tab", got)
	}
	for _, keycode := range []uint8{7, 8, 10, 11} {
		if got := keymap.key(keycode); got != input.KeyUnknown {
			t.Errorf("key %d = %d, want unknown", keycode, got)
		}
	}
}

func TestKeymapProducesUTF8TextWithShiftAndCapsLock(t *testing.T) {
	keymap := xcbKeymap{first: 8, last: 9, perKeycode: 2, keysyms: []uint32{'a', 'A', 0x01004e2d, 0}}
	for _, test := range []struct {
		keycode uint8
		state   uint16
		want    string
	}{
		{8, 0, "a"},
		{8, xcbModShift, "A"},
		{8, xcbModLock, "A"},
		{8, xcbModShift | xcbModLock, "a"},
		{9, 0, "中"},
		{7, 0, ""},
	} {
		if got := keymap.text(test.keycode, test.state); got != test.want {
			t.Errorf("text(%d, %#x) = %q, want %q", test.keycode, test.state, got, test.want)
		}
	}
}

func TestXCBModifiers(t *testing.T) {
	state := uint16(xcbModShift | xcbModControl | xcbMod1 | xcbMod4 | 1<<8)
	want := input.ModifierShift | input.ModifierControl | input.ModifierAlt | input.ModifierMeta
	if got := xcbModifiers(state); got != want {
		t.Fatalf("modifiers = %08b, want %08b", got, want)
	}
}

func TestXCBPointerButtons(t *testing.T) {
	for _, testCase := range []struct {
		button uint8
		want   ui.PointerButton
	}{
		{xcbButtonLeft, ui.PointerButtonPrimary},
		{xcbButtonRight, ui.PointerButtonSecondary},
		{xcbButtonMiddle, ui.PointerButtonMiddle},
		{xcbButtonBack, ui.PointerButtonBack},
		{xcbButtonForward, ui.PointerButtonForward},
		{xcbButtonScrollUp, ui.PointerButtonNone},
	} {
		if got := xcbPointerButton(testCase.button); got != testCase.want {
			t.Fatalf("button %d = %d, want %d", testCase.button, got, testCase.want)
		}
	}
}

func TestXCBScrollEvent(t *testing.T) {
	for _, test := range []struct {
		button uint8
		want   platform.ScrollEvent
		ok     bool
	}{
		{xcbButtonScrollUp, platform.ScrollEvent{DeltaY: -48}, true},
		{xcbButtonScrollDown, platform.ScrollEvent{DeltaY: 48}, true},
		{xcbButtonScrollLeft, platform.ScrollEvent{DeltaX: -48}, true},
		{xcbButtonScrollRight, platform.ScrollEvent{DeltaX: 48}, true},
		{xcbButtonLeft, platform.ScrollEvent{}, false},
	} {
		got, ok := xcbScrollEvent(test.button)
		if got != test.want || ok != test.ok {
			t.Errorf("scroll button %d = %+v, %v; want %+v, %v", test.button, got, ok, test.want, test.ok)
		}
	}
}

func TestEventRouterRoutesNativeInputAndFrames(t *testing.T) {
	frame := platform.Frame{Update: render.Update{Rebuild: true}}
	var pointers []platform.PointerEvent
	var keys []platform.KeyEvent
	var scrolls []platform.ScrollEvent
	var frames []platform.Frame
	router := eventRouter{
		config: platform.Config{
			OnPointer: func(event platform.PointerEvent) (platform.Frame, error) {
				pointers = append(pointers, event)
				return frame, nil
			},
			OnKey: func(event platform.KeyEvent) (platform.Frame, error) {
				keys = append(keys, event)
				return frame, nil
			},
			OnScroll: func(event platform.ScrollEvent) (platform.Frame, error) {
				scrolls = append(scrolls, event)
				return frame, nil
			},
		},
		present: func(got platform.Frame) error {
			frames = append(frames, got)
			return nil
		},
	}
	native := xcbInputEvent{Detail: xcbButtonLeft, EventX: 17, EventY: 23, State: xcbModShift}
	if err := router.pointer(platform.PointerDown, &native); err != nil {
		t.Fatal(err)
	}
	if err := router.pointerExit(); err != nil {
		t.Fatal(err)
	}
	keymap := xcbKeymap{first: 9, last: 9, perKeycode: 1, keysyms: []uint32{keysymTab}}
	native.Detail = 9
	if err := router.key(&keymap, &native, true); err != nil {
		t.Fatal(err)
	}
	native.Detail = xcbButtonScrollDown
	if handled, err := router.scroll(&native); err != nil || !handled {
		t.Fatalf("scroll handled=%v err=%v", handled, err)
	}
	if len(pointers) != 2 || pointers[0] != (platform.PointerEvent{Kind: platform.PointerDown, Button: ui.PointerButtonPrimary, Modifiers: input.ModifierShift, X: 17, Y: 23, ClickCount: 1}) || pointers[1].Kind != platform.PointerExit {
		t.Fatalf("pointer events = %+v", pointers)
	}
	if len(keys) != 1 || keys[0] != (platform.KeyEvent{Key: input.KeyTab, Modifiers: input.ModifierShift, Repeat: true}) {
		t.Fatalf("key events = %+v", keys)
	}
	if len(scrolls) != 1 || scrolls[0] != (platform.ScrollEvent{Modifiers: input.ModifierShift, X: 17, Y: 23, DeltaY: 48}) {
		t.Fatalf("scroll events = %+v", scrolls)
	}
	if len(frames) != 4 {
		t.Fatalf("presented %d frames, want 4", len(frames))
	}
}

func TestEventRouterSkipsUnknownKeysAndPropagatesErrors(t *testing.T) {
	called := false
	want := errors.New("pointer failed")
	router := eventRouter{
		config: platform.Config{
			OnPointer: func(platform.PointerEvent) (platform.Frame, error) {
				return platform.Frame{}, want
			},
			OnKey: func(platform.KeyEvent) (platform.Frame, error) {
				called = true
				return platform.Frame{}, nil
			},
		},
		present: func(platform.Frame) error {
			called = true
			return nil
		},
	}
	if err := router.pointer(platform.PointerMove, &xcbInputEvent{}); !errors.Is(err, want) {
		t.Fatalf("pointer error = %v", err)
	}
	if err := router.key(&xcbKeymap{}, &xcbInputEvent{Detail: 99}, false); err != nil {
		t.Fatal(err)
	}
	if called {
		t.Fatal("unknown key reached handler or presenter")
	}
}

func TestRendererApplyReRecordsOnlyChangedFrames(t *testing.T) {
	waits, resets, begins, ends := 0, 0, 0, 0
	storage := make([]byte, 64*vkInstanceBytes)
	renderer := vulkanRenderer{
		api: &vulkanAPI{
			waitForFences: func(uintptr, uint32, *uintptr, uint32, uint64) int32 {
				waits++
				return vkSuccess
			},
			resetCommandPool: func(uintptr, uintptr, uint32) int32 {
				resets++
				return vkSuccess
			},
			beginCommandBuffer: func(uintptr, *vkCommandBufferBeginInfo) int32 {
				begins++
				return vkSuccess
			},
			cmdBeginRenderPass:   func(uintptr, *vkRenderPassBeginInfo, uint32) {},
			cmdClearAttachments:  func(uintptr, uint32, *vkClearAttachment, uint32, *vkClearRect) {},
			cmdBindPipeline:      func(uintptr, uint32, uintptr) {},
			cmdSetViewport:       func(uintptr, uint32, uint32, *vkViewport) {},
			cmdSetScissor:        func(uintptr, uint32, uint32, *vkRect2D) {},
			cmdPushConstants:     func(uintptr, uintptr, uint32, uint32, uint32, unsafe.Pointer) {},
			cmdBindVertexBuffers: func(uintptr, uint32, uint32, *uintptr, *uint64) {},
			cmdDraw:              func(uintptr, uint32, uint32, uint32, uint32) {},
			cmdEndRenderPass:     func(uintptr) {},
			endCommandBuffer: func(uintptr) int32 {
				ends++
				return vkSuccess
			},
		},
		device:       1,
		commandPool:  2,
		fence:        3,
		width:        100,
		height:       80,
		commands:     []uintptr{4, 5},
		framebuffers: []uintptr{6, 7},
		scene:        vulkanScene{capacity: 64, mapped: unsafe.Pointer(&storage[0])},
	}
	if changed, err := renderer.apply(platform.Frame{}); err != nil || changed {
		t.Fatalf("empty apply changed=%v err=%v", changed, err)
	}
	if waits != 0 || resets != 0 {
		t.Fatalf("empty apply waited=%d reset=%d", waits, resets)
	}
	rectangles := []render.Rectangle{{Bounds: scene.Rect{Width: 10, Height: 10}, Color: scene.Color{Alpha: 1}}}
	changed, err := renderer.apply(platform.Frame{Update: render.Update{Rebuild: true, Count: 1}, Rectangles: rectangles})
	if err != nil || !changed {
		t.Fatalf("changed apply changed=%v err=%v", changed, err)
	}
	if waits != 1 || resets != 1 || begins != 2 || ends != 2 {
		t.Fatalf("command lifecycle waits=%d resets=%d begins=%d ends=%d", waits, resets, begins, ends)
	}
	rectangles[0].Bounds.Width = 99
	if renderer.rectangles[0].Bounds.Width != 10 {
		t.Fatal("renderer retained caller-owned rectangle storage")
	}
}

func TestEventRouterNumbersConsecutivePressesAsClicks(t *testing.T) {
	var got []platform.PointerEvent
	router := eventRouter{
		config: platform.Config{OnPointer: func(event platform.PointerEvent) (platform.Frame, error) {
			got = append(got, event)
			return platform.Frame{}, nil
		}},
		present: func(platform.Frame) error { return nil },
		clicks:  &input.ClickCounter{Interval: 500 * time.Millisecond, Distance: 4},
	}
	at := func(milliseconds uint32, x int16) *xcbInputEvent {
		return &xcbInputEvent{Detail: 1, Time: milliseconds, EventX: x, EventY: 10}
	}
	steps := []struct {
		kind  platform.PointerEventKind
		event *xcbInputEvent
		want  int
	}{
		{platform.PointerDown, at(1000, 10), 1},
		{platform.PointerUp, at(1050, 10), 1},
		{platform.PointerDown, at(1200, 11), 2},
		{platform.PointerUp, at(1250, 11), 2},
		{platform.PointerMove, at(1260, 40), 0},
		{platform.PointerDown, at(1300, 40), 1},
		{platform.PointerDown, at(2000, 40), 1},
	}
	for index, step := range steps {
		if err := router.pointer(step.kind, step.event); err != nil {
			t.Fatal(err)
		}
		if got[index].ClickCount != step.want {
			t.Errorf("step %d (kind %d): click count %d, want %d", index, step.kind, got[index].ClickCount, step.want)
		}
	}

	// Without a counter every press is a lone click.
	got = got[:0]
	router.clicks = nil
	router.pointer(platform.PointerDown, at(0, 1))
	router.pointer(platform.PointerDown, at(10, 1))
	if got[0].ClickCount != 1 || got[1].ClickCount != 1 {
		t.Errorf("uncounted presses = %+v", got)
	}
}

func TestCustomTitlebarMovesAndResizesTheWindowOnceThePointerDrags(t *testing.T) {
	var ungrabbed uint32
	var target, mask uint32
	var message xcbClientEvent
	var sent int
	win := &window{
		connection: 7, id: 8, moveResizeAtom: 9, screen: &xcbScreen{Root: 10}, width: 800, height: 600,
		api: &xcbAPI{
			ungrabPointer: func(_ uintptr, timestamp uint32) xcbVoidCookie { ungrabbed = timestamp; return xcbVoidCookie{} },
			sendEvent: func(_ uintptr, _ uint8, destination, eventMask uint32, data *byte) xcbVoidCookie {
				sent++
				target, mask = destination, eventMask
				message = *(*xcbClientEvent)(unsafe.Pointer(data))
				return xcbVoidCookie{}
			},
			flush: func(uintptr) int32 { return 1 },
		},
	}
	config := platform.Config{WindowMode: platform.WindowCustomTitlebar, TitlebarHeight: 40, PressAt: func(x, y float32) platform.PressKind {
		switch {
		case y >= 40:
			return platform.PressClient
		case x >= 400 && x < 700:
			return platform.PressMoveOnDrag
		case x >= 100 && x < 400:
			return platform.PressControl
		}
		return platform.PressMove
	}}
	motion := func(event *xcbInputEvent) bool {
		consumed, cancelled := win.dragMotion(event)
		if cancelled {
			t.Fatal("a press the application never saw was reported as cancelled")
		}
		return consumed
	}
	direction := func() uint32 { return binary.LittleEndian.Uint32(message.Data[8:12]) }

	// A press on the empty titlebar is taken, but the window manager hears
	// nothing until the pointer drags.
	if !win.beginWindowMove(config, &xcbInputEvent{Detail: xcbButtonLeft, Time: 11, RootX: 320, RootY: 240, EventX: 50, EventY: 20}) {
		t.Fatal("empty titlebar did not take the press")
	}
	if sent != 0 {
		t.Fatal("a press alone asked the window manager to move the window")
	}
	if !motion(&xcbInputEvent{State: xcbButton1Mask, Time: 12, RootX: 322, RootY: 241}) || sent != 0 {
		t.Fatal("a motion within the slop started the move")
	}
	if !motion(&xcbInputEvent{State: xcbButton1Mask, Time: 13, RootX: 340, RootY: 250}) {
		t.Fatal("a drag was not taken")
	}
	if ungrabbed != 13 || target != 10 || mask != xcbEventSubstructureRedirect|xcbEventSubstructureNotify || message.Type != 9 {
		t.Fatalf("move request = ungrab %d target %d mask %#x message %+v", ungrabbed, target, mask, message)
	}
	if direction() != moveResizeMove || binary.LittleEndian.Uint32(message.Data[0:4]) != 340 || binary.LittleEndian.Uint32(message.Data[4:8]) != 250 {
		t.Fatalf("move request data = %v", message.Data)
	}
	if motion(&xcbInputEvent{State: xcbButton1Mask, RootX: 400, RootY: 300}) || sent != 1 {
		t.Fatal("the drag was asked for more than once")
	}

	// A press that is released without dragging asks for nothing.
	win.beginWindowMove(config, &xcbInputEvent{Detail: xcbButtonLeft, RootX: 320, RootY: 240, EventX: 50, EventY: 20})
	win.endPress()
	if motion(&xcbInputEvent{State: xcbButton1Mask, RootX: 400, RootY: 300}) || sent != 1 {
		t.Fatal("a released press still started a move")
	}

	// A press on an edge resizes from that edge.
	win.beginWindowMove(config, &xcbInputEvent{Detail: xcbButtonLeft, RootX: 900, RootY: 700, EventX: 799, EventY: 300})
	motion(&xcbInputEvent{State: xcbButton1Mask, RootX: 920, RootY: 700})
	if sent != 2 || direction() != moveResizeDirection(platform.EdgeRight) {
		t.Fatalf("edge press: %d requests, direction %d", sent, direction())
	}

	if win.beginWindowMove(config, &xcbInputEvent{Detail: xcbButtonLeft, EventX: 120, EventY: 20}) {
		t.Fatal("interactive titlebar control took the press")
	}
	if win.beginWindowMove(config, &xcbInputEvent{Detail: xcbButtonLeft, EventX: 50, EventY: 50}) {
		t.Fatal("content below titlebar took the press")
	}

	// A maximized window cannot be resized from its edges, and a drag from it
	// is still a move.
	win.frame.maximized = true
	win.beginWindowMove(config, &xcbInputEvent{Detail: xcbButtonLeft, RootX: 5, RootY: 20, EventX: 2, EventY: 20})
	motion(&xcbInputEvent{State: xcbButton1Mask, RootX: 60, RootY: 80})
	if sent != 3 || direction() != moveResizeMove {
		t.Fatalf("maximized press: %d requests, direction %d", sent, direction())
	}
	win.frame.maximized = false

	// A press on a backdrop in the strip is the application's too, to dismiss an
	// overlay on a click. Motion within the slop is still its own; a drag moves
	// the window and tells the application its press ended.
	if win.beginWindowMove(config, &xcbInputEvent{Detail: xcbButtonLeft, RootX: 500, RootY: 20, EventX: 500, EventY: 20}) {
		t.Fatal("a backdrop in the strip did not leave the press to the application")
	}
	if consumed, cancelled := win.dragMotion(&xcbInputEvent{State: xcbButton1Mask, RootX: 502, RootY: 21}); consumed || cancelled || sent != 3 {
		t.Fatal("a motion within the slop was taken from the application")
	}
	if consumed, cancelled := win.dragMotion(&xcbInputEvent{State: xcbButton1Mask, RootX: 520, RootY: 21}); !consumed || !cancelled || sent != 4 || direction() != moveResizeMove {
		t.Fatalf("a drag from a backdrop: consumed %v cancelled %v, %d requests", consumed, cancelled, sent)
	}
	win.beginWindowMove(config, &xcbInputEvent{Detail: xcbButtonLeft, RootX: 500, RootY: 20, EventX: 500, EventY: 20})
	win.endPress()
	if consumed, cancelled := win.dragMotion(&xcbInputEvent{State: xcbButton1Mask, RootX: 520, RootY: 21}); consumed || cancelled || sent != 4 {
		t.Fatal("a click on a backdrop still started a move")
	}
}

func TestTerminalThatAsksForReleasesGetsPairedKeysAndModifiers(t *testing.T) {
	var keys []platform.KeyEvent
	router := eventRouter{
		config: platform.Config{OnKey: func(event platform.KeyEvent) (platform.Frame, error) {
			keys = append(keys, event)
			return platform.Frame{}, nil
		}},
		text:    &linuxTextInput{enabled: true, input: platform.TextInput{Terminal: true, KeyReleases: true}},
		present: func(platform.Frame) error { return nil },
	}
	// keycode 9 is a, keycode 10 is Shift_L.
	keymap := xcbKeymap{first: 9, last: 10, perKeycode: 2, keysyms: []uint32{'a', 'A', keysymShiftL, 0}}
	shift := xcbInputEvent{Detail: 10}
	letter := xcbInputEvent{Detail: 9, State: xcbModShift}
	if err := router.key(&keymap, &shift, false); err != nil {
		t.Fatal(err)
	}
	if err := router.key(&keymap, &letter, false); err != nil {
		t.Fatal(err)
	}
	shift.State = xcbModShift
	if err := router.release(&keymap, &shift); err != nil {
		t.Fatal(err)
	}
	letter.State = 0
	if err := router.release(&keymap, &letter); err != nil {
		t.Fatal(err)
	}
	want := []platform.KeyEvent{
		{Key: input.KeyShiftLeft, Modifiers: input.ModifierShift, Code: 10},
		{Modifiers: input.ModifierShift, Text: "a", Typed: "A", Code: 9}, // the base character is the identity, the capital what it types
		{Key: input.KeyShiftLeft, Release: true, Code: 10},
		{Release: true, Code: 9},
	}
	if !reflect.DeepEqual(keys, want) {
		t.Fatalf("keys =\n%+v\nwant\n%+v", keys, want)
	}
}

func TestModifierKeysAreNotDeliveredToTerminalsThatDidNotAsk(t *testing.T) {
	called := false
	router := eventRouter{
		config:  platform.Config{OnKey: func(platform.KeyEvent) (platform.Frame, error) { called = true; return platform.Frame{}, nil }},
		present: func(platform.Frame) error { return nil },
	}
	keymap := xcbKeymap{first: 9, last: 9, perKeycode: 1, keysyms: []uint32{keysymShiftL}}
	_ = router.key(&keymap, &xcbInputEvent{Detail: 9}, false)
	_ = router.release(&keymap, &xcbInputEvent{Detail: 9})
	if called {
		t.Fatal("a modifier key or a release reached a handler that never asked")
	}
}

// X reports device pixels; the application is given logical ones, and a screen
// position in the same units as the window's Frame.
func TestEventRouterGivesLogicalCoordinatesAtADisplayScale(t *testing.T) {
	var pointers []platform.PointerEvent
	var scrolls []platform.ScrollEvent
	router := eventRouter{
		config: platform.Config{
			OnPointer: func(event platform.PointerEvent) (platform.Frame, error) {
				pointers = append(pointers, event)
				return platform.Frame{}, nil
			},
			OnScroll: func(event platform.ScrollEvent) (platform.Frame, error) {
				scrolls = append(scrolls, event)
				return platform.Frame{}, nil
			},
		},
		present: func(platform.Frame) error { return nil },
		scale:   func() float32 { return 1.5 },
	}
	native := xcbInputEvent{Detail: xcbButtonLeft, EventX: 30, EventY: 45, RootX: 330, RootY: 645}
	if err := router.pointer(platform.PointerMove, &native); err != nil {
		t.Fatal(err)
	}
	native.Detail = xcbButtonScrollUp
	if handled, err := router.scroll(&native); err != nil || !handled {
		t.Fatalf("scroll handled=%v err=%v", handled, err)
	}
	if len(pointers) != 1 || pointers[0].X != 20 || pointers[0].Y != 30 || pointers[0].ScreenX != 220 || pointers[0].ScreenY != 430 {
		t.Fatalf("pointer = %+v, want 20,30 in the window and 220,430 on the screen", pointers)
	}
	if len(scrolls) != 1 || scrolls[0].X != 20 || scrolls[0].Y != 30 {
		t.Fatalf("scroll = %+v, want 20,30", scrolls)
	}
}

func TestEventRouterOffersAShortcutItsLatinLetterOnAnyLayout(t *testing.T) {
	var shortcuts []platform.KeyEvent
	router := eventRouter{
		config: platform.Config{OnShortcut: func(event platform.KeyEvent) (platform.Frame, bool, error) {
			shortcuts = append(shortcuts, event)
			return platform.Frame{}, true, nil
		}},
		present: func(platform.Frame) error { return nil },
	}
	// US then Russian: the key that types c on the first layout types с on the second.
	keymap := xcbKeymap{first: 54, last: 54, perKeycode: 4, keysyms: []uint32{'c', 'C', 0x6d3, 0x6f3}}
	event := xcbInputEvent{Detail: 54, State: xcbModControl | 1<<xcbGroupShift}
	if err := router.key(&keymap, &event, false); err != nil {
		t.Fatal(err)
	}
	if len(shortcuts) != 1 || shortcuts[0].Text != "c" || shortcuts[0].Modifiers != input.ModifierControl {
		t.Fatalf("shortcut = %+v", shortcuts)
	}
}

func TestEventRouterComposesDeadKeysIntoTheTextTyped(t *testing.T) {
	var typed []platform.KeyEvent
	router := eventRouter{
		config: platform.Config{OnKey: func(event platform.KeyEvent) (platform.Frame, error) {
			typed = append(typed, event)
			return platform.Frame{}, nil
		}},
		present: func(platform.Frame) error { return nil },
		text:    &linuxTextInput{},
	}
	keymap := xcbKeymap{first: 20, last: 22, perKeycode: 2, keysyms: []uint32{0xfe51, 0xfe51, 'e', 'E', 'x', 'X'}} // dead_acute, e, x
	press := func(keycode uint8) {
		t.Helper()
		if err := router.key(&keymap, &xcbInputEvent{Detail: keycode}, false); err != nil {
			t.Fatal(err)
		}
	}
	press(20)
	press(21)
	press(20)
	press(22)
	if len(typed) != 2 || typed[0].Text != "é" || typed[1].Text != "´x" {
		t.Fatalf("typed = %+v", typed)
	}
}

func TestEventRouterUsesTheDesktopsDoubleClickTime(t *testing.T) {
	var got []platform.PointerEvent
	settings := xsettings{DoubleClickTime: 100, hasDoubleClick: true}
	router := eventRouter{
		config: platform.Config{OnPointer: func(event platform.PointerEvent) (platform.Frame, error) {
			got = append(got, event)
			return platform.Frame{}, nil
		}},
		present:  func(platform.Frame) error { return nil },
		clicks:   &input.ClickCounter{Interval: defaultDoubleClick, Distance: defaultDoubleClickDistance},
		settings: func() xsettings { return settings },
	}
	press := func(milliseconds uint32) {
		if err := router.pointer(platform.PointerDown, &xcbInputEvent{Detail: 1, Time: milliseconds, EventX: 10, EventY: 10}); err != nil {
			t.Fatal(err)
		}
	}
	press(1000)
	press(1200) // beyond the desktop's 100 ms, inside the default 500
	settings.DoubleClickTime = 400
	press(1400)
	if got[1].ClickCount != 1 || got[2].ClickCount != 2 {
		t.Fatalf("click counts = %d, %d, %d", got[0].ClickCount, got[1].ClickCount, got[2].ClickCount)
	}
}

func TestX11ModifierReleaseKeepsOtherSideHeld(t *testing.T) {
	var keys []platform.KeyEvent
	router := eventRouter{
		config: platform.Config{OnKey: func(event platform.KeyEvent) (platform.Frame, error) {
			keys = append(keys, event)
			return platform.Frame{}, nil
		}},
		text:         &linuxTextInput{enabled: true, input: platform.TextInput{Terminal: true, KeyReleases: true}},
		present:      func(platform.Frame) error { return nil },
		modifierKeys: newModifierTracker(),
	}
	// keycode 10 is Shift_L, 11 is Shift_R.
	keymap := xcbKeymap{first: 10, last: 11, perKeycode: 1, keysyms: []uint32{keysymShiftL, keysymShiftR}}
	if err := router.key(&keymap, &xcbInputEvent{Detail: 10}, false); err != nil {
		t.Fatal(err)
	}
	if err := router.key(&keymap, &xcbInputEvent{Detail: 11}, false); err != nil {
		t.Fatal(err)
	}
	if err := router.release(&keymap, &xcbInputEvent{Detail: 11, State: xcbModShift}); err != nil {
		t.Fatal(err)
	}
	if err := router.release(&keymap, &xcbInputEvent{Detail: 10}); err != nil {
		t.Fatal(err)
	}
	if len(keys) != 4 {
		t.Fatalf("got %d key events, want two presses and two releases", len(keys))
	}
	if keys[2].Modifiers&input.ModifierShift == 0 {
		t.Fatalf("Shift_R release dropped Shift while Shift_L is held: %+v", keys[2])
	}
	if keys[3].Modifiers&input.ModifierShift != 0 {
		t.Fatalf("last Shift release still reports Shift: %+v", keys[3])
	}
}

func TestShortcutTextFallsBackToShiftedOnlyLevel(t *testing.T) {
	// The layout gives the key only its shifted symbol, as Wayland's level scan
	// allows.
	keymap := xcbKeymap{first: 9, last: 9, perKeycode: 2, keysyms: []uint32{0, 'c'}}
	if got := keymap.shortcutText(9, 0); got != "c" {
		t.Fatalf("shortcut text = %q, want c", got)
	}
}

func TestX11FocusInForgetsHeldKeys(t *testing.T) {
	tracker := newModifierTracker()
	tracker.press(50, keysymShiftL, input.KeyShiftLeft)
	state := &linuxWindowState{
		win:         &window{},
		application: &linuxApplication{},
		router:      eventRouter{modifierKeys: tracker},
	}
	previous := activeWindow.Load()
	defer activeWindow.Store(previous)

	// Mode 0 is NotifyNormal. Detail 5 (NotifyPointer) accompanies a focus
	// change elsewhere and must not forget keys held here.
	pointer := xcbFocusEvent{ResponseType: xcbFocusIn, Detail: 5}
	if _, err := state.handleEvent(unsafe.Pointer(&pointer)); err != nil {
		t.Fatal(err)
	}
	if len(tracker.down) != 1 {
		t.Fatalf("a pointer-detail FocusIn forgot held keys: %v", tracker.down)
	}

	// Detail 0 is NotifyAncestor: this window gained the keyboard.
	event := xcbFocusEvent{ResponseType: xcbFocusIn}
	if _, err := state.handleEvent(unsafe.Pointer(&event)); err != nil {
		t.Fatal(err)
	}
	if len(tracker.down) != 0 {
		t.Fatalf("held keys survived a real focus gain: %v", tracker.down)
	}
}

func TestModifierTrackerRetainsKeysStillDown(t *testing.T) {
	tracker := newModifierTracker()
	tracker.press(10, 0, input.KeyUnknown)
	tracker.press(20, 0, input.KeyUnknown)
	var keys [32]byte
	keys[20/8] = 1 << (20 % 8)

	tracker.retain(keys)

	if _, ok := tracker.down[10]; ok {
		t.Fatal("a key the server says is up survived")
	}
	if _, ok := tracker.down[20]; !ok {
		t.Fatal("a key still held across the focus change was forgotten")
	}
}

func TestX11KeymapReloadReclassifiesHeldKeys(t *testing.T) {
	state := &linuxWindowState{win: &window{}}
	state.win.keymap = xcbKeymap{first: 100, last: 100, perKeycode: 2, keysyms: []uint32{keysymAltR, 0}}
	tracker := newModifierTracker()
	tracker.press(100, keysymLevel3, input.KeyAltRight) // held as AltGr
	state.router = eventRouter{modifierKeys: tracker}

	state.refreshModifierKeys()

	if got := tracker.down[100]; got != input.ModifierAlt {
		t.Fatalf("held key bit after reload = %#x, want Alt", got)
	}
}

func TestModifierTrackerSeedsHeldModifiers(t *testing.T) {
	tracker := newModifierTracker()
	tracker.press(30, 0, input.KeyUnknown) // released while unfocused
	var keys [32]byte
	keys[10/8] = 1 << (10 % 8) // Shift_L is down on the server

	tracker.seed(keys, func(scancode uint32) input.Modifiers {
		if scancode == 10 {
			return input.ModifierShift
		}
		return 0
	})

	if _, ok := tracker.down[30]; ok {
		t.Fatal("a key the server says is up survived")
	}
	if tracker.down[10] != input.ModifierShift {
		t.Fatalf("a held modifier was not seeded: %v", tracker.down)
	}
}

func TestShortcutTextFallbackStripsAltGr(t *testing.T) {
	// The key types nothing at levels 0/1 and '@' at the AltGr level.
	keymap := xcbKeymap{first: 9, last: 9, perKeycode: 6, keysyms: []uint32{0, 0, 0, 0, '@', 0}}
	if got := keymap.shortcutText(9, xcbMod5); got != "" {
		t.Fatalf("shortcut text = %q, want the AltGr level ignored", got)
	}
}
