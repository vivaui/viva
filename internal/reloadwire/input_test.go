package reloadwire

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/vivaui/viva/ui"
)

func TestInputEventCodecs(t *testing.T) {
	pointer := ui.PointerEvent{
		Kind: ui.PointerMove, Button: ui.PointerButtonSecondary,
		Modifiers: ui.ModifierShift | ui.ModifierMeta,
		X:         12.5, Y: -8.25, Viewport: ui.Size{Width: 760, Height: 380}, Target: ui.Item(19), Captured: true,
	}
	pointerData := encodePointerEvent(pointer)
	if got, err := decodePointerEvent(pointerData); err != nil || got != pointer {
		t.Fatalf("pointer = %+v, want %+v, err = %v", got, pointer, err)
	}

	for _, button := range []ui.PointerButton{ui.PointerButtonBack, ui.PointerButtonForward} {
		side := ui.PointerEvent{Kind: ui.PointerDown, Button: button, X: 1, Y: 2}
		if got, err := decodePointerEvent(encodePointerEvent(side)); err != nil || got != side {
			t.Fatalf("side button %d = %+v, err = %v", button, got, err)
		}
	}
	if _, err := decodePointerEvent(encodePointerEvent(ui.PointerEvent{Kind: ui.PointerDown, Button: ui.PointerButtonForward + 1})); err == nil {
		t.Fatal("a pointer button beyond Forward was accepted")
	}

	key := ui.KeyEvent{Key: ui.KeyEnd, Modifiers: ui.ModifierControl | ui.ModifierAlt, Target: ui.Item(23), Repeat: true}
	keyData := encodeKeyEvent(key)
	if got, err := decodeKeyEvent(keyData); err != nil || got != key {
		t.Fatalf("key = %+v, want %+v, err = %v", got, key, err)
	}
	chord := ui.KeyEvent{Modifiers: ui.ModifierControl, Target: ui.Item(3), Text: "c"}
	if got, err := decodeKeyEvent(encodeKeyEvent(chord)); err != nil || got != chord {
		t.Fatalf("chord = %+v, want %+v, err = %v", got, chord, err)
	}
	function := ui.KeyEvent{Key: ui.KeyF12, Target: ui.Item(3)}
	if got, err := decodeKeyEvent(encodeKeyEvent(function)); err != nil || got != function {
		t.Fatalf("function key = %+v, want %+v, err = %v", got, function, err)
	}
	if _, err := decodeKeyEvent(encodeKeyEvent(ui.KeyEvent{Key: ui.KeyF12 + 1})); err == nil {
		t.Fatal("a key beyond F12 was accepted")
	}

	scroll := ui.ScrollEvent{
		Modifiers: ui.ModifierShift | ui.ModifierControl,
		X:         12.5, Y: 18.75, DeltaX: 3.5, DeltaY: -17.25,
		Viewport: ui.Size{Width: 800, Height: 450}, Target: ui.Item(29), Precise: true,
	}
	scrollData := encodeScrollEvent(scroll)
	if got, err := decodeScrollEvent(scrollData); err != nil || got != scroll {
		t.Fatalf("scroll = %+v, want %+v, err = %v", got, scroll, err)
	}

	result := ui.EventResult{Focus: ui.Item(31), Handled: true, Repaint: true, Blur: true, FocusVisible: true, ClipboardText: "copied"}
	resultData := encodeEventResult(result)
	if got, err := decodeEventResult(resultData); err != nil || got != result {
		t.Fatalf("event result = %+v, want %+v, err = %v", got, result, err)
	}
}

func TestInputEventCodecsRejectInvalidPayloads(t *testing.T) {
	pointer := encodePointerEvent(ui.PointerEvent{Kind: ui.PointerDown})
	key := encodeKeyEvent(ui.KeyEvent{Key: ui.KeyEnter})
	result := encodeEventResult(ui.EventResult{})
	for _, testCase := range []struct {
		name   string
		decode func([]byte) error
		data   []byte
	}{
		{name: "pointer kind", decode: decodePointerError, data: replaceByte(pointer, 0, 0xff)},
		{name: "pointer button", decode: decodePointerError, data: replaceByte(pointer, 1, 0xff)},
		{name: "pointer modifiers", decode: decodePointerError, data: replaceByte(pointer, 2, 0x80)},
		{name: "pointer trailing", decode: decodePointerError, data: append(pointer, 0)},
		{name: "key identity", decode: decodeKeyError, data: replaceByte(key, 1, 0xff)},
		{name: "key modifiers", decode: decodeKeyError, data: replaceByte(key, 2, 0x80)},
		{name: "key truncated", decode: decodeKeyError, data: key[:len(key)-1]},
		{name: "scroll modifiers", decode: decodeScrollError, data: replaceByte(encodeScrollEvent(ui.ScrollEvent{}), 0, 0x80)},
		{name: "result boolean", decode: decodeResultError, data: replaceByte(result, byteThatFlips(t, encodeEventResult(ui.EventResult{}), encodeEventResult(ui.EventResult{Handled: true})), 2)},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			if err := testCase.decode(testCase.data); err == nil {
				t.Fatalf("invalid payload was accepted: %x", testCase.data)
			}
		})
	}
}

func TestWorkerProtocolForwardsInputEventsAndResults(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	t.Setenv(AddressEnvironment, listener.Addr().String())
	t.Setenv(TokenEnvironment, "input-events")

	wantPointer := ui.PointerEvent{Kind: ui.PointerUp, Button: ui.PointerButtonPrimary, X: 14, Y: 18, Target: ui.Item(7), Captured: true}
	wantKey := ui.KeyEvent{Key: ui.KeySpace, Modifiers: ui.ModifierShift, Target: ui.Item(11), Repeat: true}
	wantShortcut := ui.KeyEvent{Modifiers: ui.ModifierControl, Target: ui.Item(11), Text: "k"}
	wantScroll := ui.ScrollEvent{
		Modifiers: ui.ModifierMeta, X: 16, Y: 24, DeltaX: 2, DeltaY: -6,
		Viewport: ui.Size{Width: 320, Height: 200}, Target: ui.Item(13), Precise: true,
	}
	pointerResult := ui.EventResult{Focus: ui.Item(7), Handled: true, Repaint: true}
	keyResult := ui.EventResult{Focus: ui.Item(11), Handled: true, FocusVisible: true}
	shortcutResult := ui.EventResult{Handled: true, Repaint: true}
	scrollResult := ui.EventResult{Blur: true, Repaint: true}
	wantTap := ui.ModifierTapEvent{Key: ui.KeyShiftRight, Target: ui.Item(11)}
	tapResult := ui.EventResult{Handled: true, Repaint: true}
	var gotTap ui.ModifierTapEvent
	var gotPointer ui.PointerEvent
	var gotKey ui.KeyEvent
	var gotShortcut ui.KeyEvent
	var gotScroll ui.ScrollEvent
	serveDone := make(chan error, 1)
	go func() {
		serveDone <- Serve(WorkerConfig{
			Window:      WindowConfig{Title: "input", Width: 320, Height: 200, HasPointer: true, HasKey: true, HasShortcut: true, HasScroll: true, HasModifierTap: true},
			Pointer:     func(event ui.PointerEvent) ui.EventResult { gotPointer = event; return pointerResult },
			Key:         func(event ui.KeyEvent) ui.EventResult { gotKey = event; return keyResult },
			Shortcut:    func(event ui.KeyEvent) ui.EventResult { gotShortcut = event; return shortcutResult },
			Scroll:      func(event ui.ScrollEvent) ui.EventResult { gotScroll = event; return scrollResult },
			ModifierTap: func(event ui.ModifierTapEvent) ui.EventResult { gotTap = event; return tapResult },
		})
	}()
	connection, err := listener.Accept()
	if err != nil {
		t.Fatal(err)
	}
	_, client, err := Accept(connection, "input-events")
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if got, err := client.Pointer(ctx, wantPointer); err != nil || got != pointerResult {
		t.Fatalf("pointer result = %+v, want %+v, err = %v", got, pointerResult, err)
	}
	if got, err := client.Key(ctx, wantKey); err != nil || got != keyResult {
		t.Fatalf("key result = %+v, want %+v, err = %v", got, keyResult, err)
	}
	if got, err := client.Shortcut(ctx, wantShortcut); err != nil || got != shortcutResult {
		t.Fatalf("shortcut result = %+v, want %+v, err = %v", got, shortcutResult, err)
	}
	if got, err := client.Scroll(ctx, wantScroll); err != nil || got != scrollResult {
		t.Fatalf("scroll result = %+v, want %+v, err = %v", got, scrollResult, err)
	}
	if got, err := client.ModifierTap(ctx, wantTap); err != nil || got != tapResult || gotTap != wantTap {
		t.Fatalf("modifier tap = %+v / %+v, want %+v / %+v, err = %v", got, gotTap, tapResult, wantTap, err)
	}
	if gotPointer != wantPointer || gotKey != wantKey || gotShortcut != wantShortcut || gotScroll != wantScroll {
		t.Fatalf("worker events = (%+v, %+v, %+v, %+v)", gotPointer, gotKey, gotShortcut, gotScroll)
	}
	if err := client.Shutdown(ctx); err != nil {
		t.Fatal(err)
	}
	if err := <-serveDone; err != nil {
		t.Fatal(err)
	}
}

func replaceByte(data []byte, index int, value byte) []byte {
	result := append([]byte(nil), data...)
	result[index] = value
	return result
}

func decodePointerError(data []byte) error { _, err := decodePointerEvent(data); return err }
func decodeKeyError(data []byte) error     { _, err := decodeKeyEvent(data); return err }
func decodeScrollError(data []byte) error  { _, err := decodeScrollEvent(data); return err }
func decodeResultError(data []byte) error  { _, err := decodeEventResult(data); return err }

func TestWindowConfigCodecPreservesInputCapabilities(t *testing.T) {
	want := Hello{Token: "capabilities", Config: WindowConfig{
		Title: "input", Width: 640, Height: 480, WindowMode: 2, TitlebarHeight: 36,
		MinWidth: 480, MinHeight: 320,
		HasClick: true, HasPointer: true, HasKey: true, HasShortcut: true, HasScroll: true, HasModifierTap: true,
		HasClose: true, HasSnapshot: true, HasRestore: true,
	}}
	data, err := encodeHello(want)
	if err != nil {
		t.Fatal(err)
	}
	got, err := decodeHello(data)
	if err != nil || got != want {
		t.Fatalf("hello = %+v, want %+v, err = %v", got, want, err)
	}
}

// byteThatFlips finds the one byte at which two encodings of values that differ in a
// single flag disagree, so a test can corrupt that flag without knowing the layout.
func byteThatFlips(t *testing.T, a, b []byte) int {
	t.Helper()
	if len(a) != len(b) {
		t.Fatalf("encodings differ in length: %d and %d", len(a), len(b))
	}
	at := -1
	for i := range a {
		if a[i] != b[i] {
			if at >= 0 {
				t.Fatal("the two encodings differ in more than one byte")
			}
			at = i
		}
	}
	if at < 0 {
		t.Fatal("the two encodings are identical")
	}
	return at
}

func TestModifierTapCodecRejectsAKeyThatIsNotAModifier(t *testing.T) {
	good := encodeModifierTapEvent(ui.ModifierTapEvent{Key: ui.KeyShiftLeft, Target: ui.Item(2)})
	if _, err := decodeModifierTapEvent(good); err != nil {
		t.Fatal(err)
	}
	if _, err := decodeModifierTapEvent(replaceByte(good, 0, byte(ui.KeyEnter))); err == nil {
		t.Fatal("a tap of Enter decoded")
	}
	if _, err := decodeModifierTapEvent(good[:len(good)-1]); err == nil {
		t.Fatal("a short tap decoded")
	}
}
