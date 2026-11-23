//go:build !android

package linux

import (
	"encoding/binary"
	"math"
	"os"
	"testing"
	"unsafe"

	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/internal/render"
	"github.com/vivaui/viva/internal/scene"
)

func TestNativeStructureABI(t *testing.T) {
	if unsafe.Sizeof(uintptr(0)) != 8 {
		t.Fatal("only Linux amd64 and arm64 are supported")
	}
	for _, layout := range []struct {
		name string
		size uintptr
		want uintptr
	}{
		{"VkXcbSurfaceCreateInfoKHR", unsafe.Sizeof(vkXcbSurfaceCreateInfo{}), 40},
		{"xcb_screen_t", unsafe.Sizeof(xcbScreen{}), 40},
		{"xcb_screen_iterator_t", unsafe.Sizeof(xcbScreenIterator{}), 16},
		{"xcb_setup_t", unsafe.Sizeof(xcbSetup{}), 40},
		{"xcb_generic_event_t", unsafe.Sizeof(xcbGenericEvent{}), 32},
		{"xcb_key_press_event_t", unsafe.Sizeof(xcbInputEvent{}), 32},
		{"xcb_mapping_notify_event_t", unsafe.Sizeof(xcbMappingEvent{}), 32},
		{"xcb_get_keyboard_mapping_reply_t", unsafe.Sizeof(xcbKeyboardMappingReply{}), 32},
		{"xcb_configure_notify_event_t", unsafe.Sizeof(xcbConfigureEvent{}), 28},
		{"xcb_client_message_event_t", unsafe.Sizeof(xcbClientEvent{}), 32},
		{"xcb_focus_out_event_t", unsafe.Sizeof(xcbFocusEvent{}), 32},
		{"xcb_expose_event_t", unsafe.Sizeof(xcbExposeEvent{}), 32},
		{"xcb_selection_owner_reply_t", unsafe.Sizeof(xcbSelectionOwnerReply{}), 12},
		{"xcb_get_property_reply_t", unsafe.Sizeof(xcbPropertyReply{}), 32},
		{"xcb_selection_request_event_t", unsafe.Sizeof(xcbSelectionRequestEvent{}), 28},
		{"xcb_selection_notify_event_t", unsafe.Sizeof(xcbSelectionNotifyEvent{}), 24},
		{"xcb_selection_clear_event_t", unsafe.Sizeof(xcbSelectionClearEvent{}), 16},
		{"xcb_selection_notify_wire", unsafe.Sizeof(xcbSelectionNotifyWire{}), 32},
		{"pollfd", unsafe.Sizeof(pollFD{}), 8},
		{"xcb_translate_coordinates_reply_t", unsafe.Sizeof(xcbTranslateReply{}), 16},
	} {
		if layout.size != layout.want {
			t.Errorf("%s size %d, want %d", layout.name, layout.size, layout.want)
		}
	}
}

func TestValidateConfig(t *testing.T) {
	valid := platform.Config{
		Width:      960,
		Height:     600,
		WindowMode: platform.WindowTitled,
		Rectangles: []render.Rectangle{{Color: scene.Color{Alpha: 1}}},
		OnPointer:  func(platform.PointerEvent) (platform.Frame, error) { return platform.Frame{}, nil },
		OnResize:   func(platform.Size) (platform.Frame, error) { return platform.Frame{}, nil },
		OnKey:      func(platform.KeyEvent) (platform.Frame, error) { return platform.Frame{}, nil },
		OnScroll:   func(platform.ScrollEvent) (platform.Frame, error) { return platform.Frame{}, nil },
	}
	if err := validateConfig(valid); err != nil {
		t.Fatalf("valid config: %v", err)
	}
	valid.Rectangles = []render.Rectangle{
		{Bounds: scene.Rect{Width: 24, Height: 12}, Radius: 4, Color: scene.Color{Alpha: 0.5}},
		{Kind: scene.KindTriangle, Points: [3]scene.Point{{}, {X: 10}, {Y: 10}}, EdgeMask: 7, Color: scene.Color{Alpha: 0.5}},
	}
	if err := validateConfig(valid); err != nil {
		t.Fatalf("rounded translucent scene: %v", err)
	}
	valid.TextLabels = []platform.TextLabel{{Value: "Viva 原生界面", Width: 200, Height: 28, FontSize: 16, Color: platform.Color{Alpha: 1}}}
	if err := validateConfig(valid); err != nil {
		t.Fatalf("system text label: %v", err)
	}
	valid.WindowMode = platform.WindowCustomTitlebar
	valid.TitlebarHeight = 42
	if err := validateConfig(valid); err != nil {
		t.Fatalf("custom titlebar: %v", err)
	}
	valid.WindowMode = platform.WindowTitled
	for _, config := range []platform.Config{
		{Width: 0, Height: 600, WindowMode: platform.WindowTitled},
		{Width: 960, Height: 1 << 16, WindowMode: platform.WindowTitled},
		{Width: 960, Height: 600, WindowMode: platform.WindowMode(99)},
		{Width: 960, Height: 600, WindowMode: platform.WindowTitled, TextLabels: make([]platform.TextLabel, 1)},
		{Width: 960, Height: 600, WindowMode: platform.WindowTitled, Rectangles: []render.Rectangle{{Radius: -1, Color: scene.Color{Alpha: 1}}}},
		{Width: 960, Height: 600, WindowMode: platform.WindowTitled, Rectangles: []render.Rectangle{{Kind: scene.KindTriangle, EdgeMask: 8, Color: scene.Color{Alpha: 1}}}},
		{Width: 960, Height: 600, WindowMode: platform.WindowTitled, Rectangles: []render.Rectangle{{Bounds: scene.Rect{X: float32(math.NaN())}, Color: scene.Color{Alpha: 1}}}},
	} {
		if err := validateConfig(config); err == nil {
			t.Fatalf("validateConfig(%+v) succeeded", config)
		}
	}
}

// The fields the status-area code reads sit where the X11 protocol puts them: a
// wrong offset would read the wrong byte of an event without any error.
func TestStatusAreaEventFieldOffsets(t *testing.T) {
	for _, field := range []struct {
		name      string
		got, want uintptr
	}{
		{"FocusOut detail", unsafe.Offsetof(xcbFocusEvent{}.Detail), 1},
		{"FocusOut event window", unsafe.Offsetof(xcbFocusEvent{}.Event), 4},
		{"FocusOut mode", unsafe.Offsetof(xcbFocusEvent{}.Mode), 8},
		{"Expose window", unsafe.Offsetof(xcbExposeEvent{}.Window), 4},
		{"Expose count", unsafe.Offsetof(xcbExposeEvent{}.Count), 16},
	} {
		if field.got != field.want {
			t.Errorf("%s is at offset %d, want %d", field.name, field.got, field.want)
		}
	}
}

func TestNextEventCoalescesConfigureNotifyAndInterleavedSyncAndExpose(t *testing.T) {
	type rawEvent [32]byte
	var queue []unsafe.Pointer
	var freed int
	push := func(ev rawEvent) {
		box := new(rawEvent)
		*box = ev
		queue = append(queue, unsafe.Pointer(box))
	}

	winID := uint32(77)
	wmProtocols := uint32(10)
	syncAtom := uint32(11)

	makeConfigure := func(w, h uint16) rawEvent {
		var raw rawEvent
		cfg := (*xcbConfigureEvent)(unsafe.Pointer(&raw))
		cfg.ResponseType = xcbConfigureNotify
		cfg.Window = winID
		cfg.Width = w
		cfg.Height = h
		return raw
	}
	makeExpose := func() rawEvent {
		var raw rawEvent
		exp := (*xcbExposeEvent)(unsafe.Pointer(&raw))
		exp.ResponseType = xcbExpose
		exp.Window = winID
		return raw
	}
	makeSync := func(lo, hi uint32) rawEvent {
		var raw rawEvent
		msg := (*xcbClientEvent)(unsafe.Pointer(&raw))
		msg.ResponseType = xcbClientMessage
		msg.Format = 32
		msg.Window = winID
		msg.Type = wmProtocols
		binary.LittleEndian.PutUint32(msg.Data[0:4], syncAtom)
		binary.LittleEndian.PutUint32(msg.Data[8:12], lo)
		binary.LittleEndian.PutUint32(msg.Data[12:16], hi)
		return raw
	}

	// Queue: Configure(400x300) -> Expose -> Sync(1) -> Configure(500x400) -> Sync(2) (for a future Configure not yet arrived)
	push(makeConfigure(400, 300))
	push(makeExpose())
	push(makeSync(101, 0))
	push(makeConfigure(500, 400))
	push(makeSync(202, 0))

	win := &window{
		id:              winID,
		wmProtocols:     wmProtocols,
		syncRequestAtom: syncAtom,
		syncCounter:     &syncCounter{id: 1},
		api: &xcbAPI{
			pollForEvent: func(uintptr) unsafe.Pointer {
				if len(queue) == 0 {
					return nil
				}
				ev := queue[0]
				queue = queue[1:]
				return ev
			},
			free: func(unsafe.Pointer) {
				freed++
			},
		},
	}
	state := &linuxWindowState{win: win, exposed: true}

	first := state.nextEvent()
	if first == nil {
		t.Fatal("first nextEvent returned nil")
	}
	gotCfg := (*xcbConfigureEvent)(first)
	if gotCfg.Width != 500 || gotCfg.Height != 400 {
		t.Fatalf("coalesced configure = %dx%d, want 500x400", gotCfg.Width, gotCfg.Height)
	}
	// Sync(101) belonged to Configure(500x400) and must be applied to syncPending;
	// Sync(202) arrived after Configure(500x400) with no matching Configure yet and must be held!
	if lo, _, ok := win.syncPending.take(); !ok || lo != 101 {
		t.Fatalf("syncPending.take() = (%d, %v), want (101, true)", lo, ok)
	}
	win.api.free(first)

	exposure := state.nextEvent()
	if exposure == nil || (*xcbGenericEvent)(exposure).ResponseType&0x7f != xcbExpose {
		t.Fatal("configure coalescing lost the server's exposure request")
	}
	win.api.free(exposure)

	second := state.nextEvent()
	if second == nil {
		t.Fatal("second nextEvent returned nil, want held Sync(202)")
	}
	if (*xcbGenericEvent)(second).ResponseType&0x7f != xcbClientMessage {
		t.Fatalf("second event kind = %d, want xcbClientMessage", (*xcbGenericEvent)(second).ResponseType&0x7f)
	}
	win.protocolMessage((*xcbClientEvent)(second))
	if lo, _, ok := win.syncPending.take(); !ok || lo != 202 {
		t.Fatalf("syncPending.take() after held Sync(202) = (%d, %v), want (202, true)", lo, ok)
	}
	win.api.free(second)
	if freed != 5 {
		t.Fatalf("freed events = %d, want 5", freed)
	}
}

func TestVulkanRectangleSmoke(t *testing.T) {
	if testing.Short() || os.Getenv("VIVA_LINUX_VULKAN_SMOKE") == "" {
		t.Skip("set VIVA_LINUX_VULKAN_SMOKE=1 with an X11 display and Vulkan driver")
	}
	t.Setenv("VIVA_TEST_AUTOCLOSE", "1")
	err := Run(platform.Config{
		Title:      "Viva Linux text smoke",
		Width:      160,
		Height:     100,
		WindowMode: platform.WindowTitled,
		ClearColor: platform.Color{Red: 0.05, Green: 0.05, Blue: 0.05, Alpha: 1},
		Rectangles: []render.Rectangle{
			{Bounds: scene.Rect{X: 8, Y: 8, Width: 96, Height: 56}, Radius: 12, Color: scene.Color{Red: 1, Alpha: 0.8}},
			{Bounds: scene.Rect{X: 48, Y: 32, Width: 96, Height: 56}, Radius: 18, Color: scene.Color{Blue: 1, Alpha: 0.8}},
			{Kind: scene.KindTriangle, Points: [3]scene.Point{{X: 20, Y: 90}, {X: 80, Y: 20}, {X: 140, Y: 90}}, Color: scene.Color{Green: 1, Alpha: 0.7}, EdgeMask: 7},
		},
		TextLabels: []platform.TextLabel{
			{Value: "Viva 原生界面", X: 16, Y: 10, Width: 132, Height: 24, FontSize: 16, Color: platform.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1}, Layer: 2},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestClearPixelPacksRGBChannels(t *testing.T) {
	got := clearPixel(platform.Color{Red: 0.055, Green: 0.065, Blue: 0.09, Alpha: 1})
	// 0.055*255=14 (0x0e), 0.065*255=17 (0x11), 0.09*255=23 (0x17)
	const want = uint32(14)<<16 | uint32(17)<<8 | uint32(23)
	if got != want {
		t.Fatalf("clearPixel() = %#06x, want %#06x", got, want)
	}
}
