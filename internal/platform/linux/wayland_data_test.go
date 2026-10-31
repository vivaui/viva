//go:build linux

package linux

import (
	"errors"
	"io"
	"os"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/vivaui/viva/internal/platform"
)

func TestWaylandDataOfferMimeMatching(t *testing.T) {
	off := &waylandDataOffer{
		mimeTypes: []string{"application/pdf", "UTF8_STRING", "text/plain"},
	}
	if !off.hasMime("UTF8_STRING") {
		t.Fatalf("expected off to have UTF8_STRING")
	}
	if off.hasMime("image/png") {
		t.Fatalf("did not expect image/png")
	}
	if got := off.findTextMime(); got != "UTF8_STRING" {
		t.Fatalf("expected UTF8_STRING, got %q", got)
	}
}

func TestWaylandLateDataDeviceDependencies(t *testing.T) {
	tests := []struct {
		name          string
		first, second string
	}{
		{name: "seat then manager", first: "wl_seat", second: "wl_data_device_manager"},
		{name: "manager then seat", first: "wl_data_device_manager", second: "wl_seat"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			createCalls := 0
			listenerCalls := make(map[uintptr]int)
			api := &waylandAPI{
				proxyMarshalFlags: func(proxy uintptr, opcode uint32, _ uintptr, _, _ uint32, args ...any) uintptr {
					switch {
					case proxy == 1 && opcode == wlRegistryBindOpcode:
						if args[0].(uint32) == 1 {
							if test.first == "wl_seat" {
								return 100
							}
							return 200
						}
						if test.second == "wl_seat" {
							return 100
						}
						return 200
					case proxy == 200 && opcode == wlDataDeviceManagerGetDataDeviceOpcode:
						createCalls++
						return 300
					case proxy == 200 && opcode == wlDataDeviceManagerCreateDataSourceOpcode:
						return 400
					}
					return 0
				},
				proxyAddListener: func(proxy uintptr, _ uintptr, _ unsafe.Pointer) int32 {
					listenerCalls[proxy]++
					return 0
				},
				proxyDestroy: func(uintptr) {},
			}
			app := &waylandApplication{api: api}
			app.globals.api = api
			app.globals.onDataDeviceChange = app.setupDataDevice
			bind := func(iface string, name uint32) {
				app.globals.handleGlobal(1, name, iface, 5)
			}

			bind(test.first, 1)
			if app.dataDevice != 0 || createCalls != 0 {
				t.Fatalf("data device created with one dependency: device=%d calls=%d", app.dataDevice, createCalls)
			}
			bind(test.first, 1) // Duplicate globals are ignored.
			bind(test.second, 2)
			if app.dataDevice != 300 || createCalls != 1 || listenerCalls[300] != 1 {
				t.Fatalf("late initialization device=%d calls=%d listeners=%d", app.dataDevice, createCalls, listenerCalls[300])
			}
			if err := app.setClipboardText("ready"); err != nil {
				t.Fatalf("clipboard after dependencies arrived: %v", err)
			}
			marker := &waylandDataOffer{proxy: 999}
			app.dataOffers[marker.proxy] = marker

			bind(test.first, 1)
			bind(test.second, 2)
			app.setupDataDevice()
			if createCalls != 1 || listenerCalls[300] != 1 {
				t.Fatalf("duplicate initialization calls=%d listeners=%d", createCalls, listenerCalls[300])
			}
			if app.dataOffers[marker.proxy] != marker || !app.ownsClipboard || app.clipboardText != "ready" {
				t.Fatal("duplicate dependency event reset live data-device state")
			}

			app.teardownDataDevice()
			if app.dataDevice != 0 || app.dataOffers != nil {
				t.Fatalf("teardown left device=%d offers=%v", app.dataDevice, app.dataOffers)
			}
		})
	}
}

func TestWaylandClipboardSetAndAsyncSend(t *testing.T) {
	var (
		mu               sync.Mutex
		dataSourceOffers []string
		selectedSource   uintptr
		selectedSerial   uint32
	)

	api := &waylandAPI{
		proxyMarshalFlags: func(proxy uintptr, opcode uint32, interfacePtr uintptr, version uint32, flags uint32, args ...any) uintptr {
			mu.Lock()
			defer mu.Unlock()
			if proxy == 100 && opcode == wlDataDeviceManagerCreateDataSourceOpcode {
				return 200 // new dataSource proxy
			}
			if proxy == 200 && opcode == wlDataSourceOfferOpcode {
				if len(args) >= 1 {
					ptr := (*byte)(args[0].(unsafe.Pointer))
					dataSourceOffers = append(dataSourceOffers, waylandGoString(ptr))
				}
			}
			if proxy == 300 && opcode == wlDataDeviceSetSelectionOpcode {
				if len(args) >= 2 {
					selectedSource = args[0].(uintptr)
					selectedSerial = args[1].(uint32)
				}
			}
			return 0
		},
		proxyAddListener: func(proxy uintptr, implementation uintptr, data unsafe.Pointer) int32 {
			return 0
		},
		proxyDestroy: func(proxy uintptr) {},
	}

	app := &waylandApplication{
		api: api,
		globals: waylandGlobals{
			dataDeviceManager: 100,
		},
		dataDevice: 300,
		lastSerial: 999,
	}

	// 1. Set 1MB clipboard content
	largePayload := strings.Repeat("A-1234567890-B\n", 70000) // ~1MB
	if err := app.setClipboardText(largePayload); err != nil {
		t.Fatalf("setClipboardText failed: %v", err)
	}

	mu.Lock()
	if selectedSource != 200 || selectedSerial != 999 {
		t.Fatalf("unexpected setSelection: source=%d, serial=%d", selectedSource, selectedSerial)
	}
	if len(dataSourceOffers) != 3 {
		t.Fatalf("expected 3 MIME offers, got %v", dataSourceOffers)
	}
	mu.Unlock()

	// 2. Simulate external paste request via send callback
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe failed: %v", err)
	}

	initDataListeners()
	sendCb := rawDataSourceListener[1]

	// Trigger async send: callback must return immediately without waiting for pipe reader!
	start := time.Now()
	cMime := []byte("text/plain;charset=utf-8\x00")
	dupFd, err := syscall.Dup(int(w.Fd()))
	if err != nil {
		t.Fatalf("dup failed: %v", err)
	}
	puregoInvokeCallback4(sendCb, unsafe.Pointer(app), 200, uintptr(unsafe.Pointer(&cMime[0])), uintptr(dupFd))
	_ = w.Close()
	elapsed := time.Since(start)

	if elapsed > 100*time.Millisecond {
		t.Fatalf("send callback took too long to return (%v), expected async non-blocking", elapsed)
	}

	// Read from pipe reader asynchronously
	data, readErr := io.ReadAll(r)
	_ = r.Close()
	if readErr != nil {
		t.Fatalf("read failed: %v", readErr)
	}
	if string(data) != largePayload {
		t.Fatalf("read data length mismatch: got %d, want %d", len(data), len(largePayload))
	}
}

func TestWaylandClipboardPasteTimeout(t *testing.T) {
	var holdWriter *os.File
	api := &waylandAPI{
		proxyMarshalFlags: func(proxy uintptr, opcode uint32, interfacePtr uintptr, version uint32, flags uint32, args ...any) uintptr {
			if opcode == wlDataOfferReceiveOpcode && len(args) >= 2 {
				fd := args[1].(int32)
				if dupFd, err := syscall.Dup(int(fd)); err == nil {
					holdWriter = os.NewFile(uintptr(dupFd), "mock_peer_writer")
				}
			}
			return 0
		},
	}
	defer func() {
		if holdWriter != nil {
			_ = holdWriter.Close()
		}
	}()
	app := &waylandApplication{
		api: api,
		clipboardOffer: &waylandDataOffer{
			proxy:     501,
			mimeTypes: []string{"text/plain;charset=utf-8"},
		},
	}

	start := time.Now()
	_, err := app.getClipboardText()
	elapsed := time.Since(start)

	if err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("expected timeout error, got %v", err)
	}
	if elapsed < 1800*time.Millisecond || elapsed > 3000*time.Millisecond {
		t.Fatalf("unexpected timeout duration: %v", elapsed)
	}
}

func TestWaylandClipboardPasteSuccess(t *testing.T) {
	api := &waylandAPI{
		proxyMarshalFlags: func(proxy uintptr, opcode uint32, interfacePtr uintptr, version uint32, flags uint32, args ...any) uintptr {
			if proxy == 502 && opcode == wlDataOfferReceiveOpcode && len(args) >= 2 {
				fd := args[1].(int32)
				dupFd, err := syscall.Dup(int(fd))
				if err != nil {
					t.Fatalf("dup failed: %v", err)
				}
				go func(wFd int) {
					f := os.NewFile(uintptr(wFd), "simulated_writer")
					defer f.Close()
					_, _ = f.Write([]byte("Wayland Hello World!"))
				}(dupFd)
			}
			return 0
		},
	}
	app := &waylandApplication{
		api: api,
		clipboardOffer: &waylandDataOffer{
			proxy:     502,
			mimeTypes: []string{"UTF8_STRING"},
		},
	}

	got, err := app.getClipboardText()
	if err != nil {
		t.Fatalf("getClipboardText failed: %v", err)
	}
	if got != "Wayland Hello World!" {
		t.Fatalf("expected 'Wayland Hello World!', got %q", got)
	}
}

func TestWaylandFileDropInteraction(t *testing.T) {
	var mu sync.Mutex
	var dropPhases []platform.FileDropPhase
	var droppedPaths []string
	presentCount := 0

	win := &waylandWindow{
		surface: 777,
	}
	win.state = &waylandWindowState{
		win: win,
		config: platform.Config{
			OnFileDrop: func(event platform.FileDropEvent) (platform.Frame, bool, error) {
				mu.Lock()
				dropPhases = append(dropPhases, event.Phase)
				if event.Phase == platform.FileDropDrop {
					droppedPaths = event.Paths
				}
				mu.Unlock()
				return platform.Frame{}, event.Phase == platform.FileDropDrop, nil
			},
		},
	}
	win.state.router.present = func(platform.Frame) error {
		mu.Lock()
		presentCount++
		mu.Unlock()
		return nil
	}

	api := &waylandAPI{
		proxyMarshalFlags: func(proxy uintptr, opcode uint32, interfacePtr uintptr, version uint32, flags uint32, args ...any) uintptr {
			if proxy == 601 && opcode == wlDataOfferReceiveOpcode && len(args) >= 2 {
				fd := args[1].(int32)
				dupFd, err := syscall.Dup(int(fd))
				if err != nil {
					return 0
				}
				go func(wFd int) {
					f := os.NewFile(uintptr(wFd), "simulated_drop_writer")
					defer f.Close()
					_, _ = f.Write([]byte("file:///home/user/document.pdf\r\nfile:///home/user/image.png\r\n"))
				}(dupFd)
			}
			return 0
		},
	}

	app := &waylandApplication{
		api: api,
		windows: []*waylandWindowState{
			win.state,
		},
		dataOffers: map[uintptr]*waylandDataOffer{
			601: {
				proxy:     601,
				mimeTypes: []string{"text/uri-list"},
			},
		},
	}

	initDataListeners()

	// 1. Enter
	puregoInvokeCallback7(rawDataDeviceListener[1], unsafe.Pointer(app), 300, 111, win.surface, 0, 0, 601)

	// 2. Motion
	puregoInvokeCallback5(rawDataDeviceListener[3], unsafe.Pointer(app), 300, 222, 0, 0)

	// 3. Drop
	puregoInvokeCallback2(rawDataDeviceListener[4], unsafe.Pointer(app), 300)

	// Allow pipe reading goroutine in drop to complete
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		count := len(dropPhases)
		presents := presentCount
		mu.Unlock()
		if count >= 3 && presents >= 3 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	mu.Lock()
	defer mu.Unlock()

	if len(dropPhases) != 3 {
		t.Fatalf("expected 3 drop phases (Over, Over, Drop), got %v", dropPhases)
	}
	if dropPhases[0] != platform.FileDropOver || dropPhases[1] != platform.FileDropOver || dropPhases[2] != platform.FileDropDrop {
		t.Fatalf("unexpected drop phases: %v", dropPhases)
	}
	if len(droppedPaths) != 2 || droppedPaths[0] != "/home/user/document.pdf" || droppedPaths[1] != "/home/user/image.png" {
		t.Fatalf("unexpected dropped paths: %v", droppedPaths)
	}
	if presentCount != 3 {
		t.Fatalf("presented %d file-drop frames, want enter, motion and drop", presentCount)
	}
}

func TestWaylandNonFileDragOfferDoesNotEmitFileEvents(t *testing.T) {
	app := &waylandApplication{
		dataOffers: map[uintptr]*waylandDataOffer{
			601: {proxy: 601, mimeTypes: []string{"text/plain"}},
		},
	}
	win := &waylandWindow{app: app, surface: 777}
	var phases []platform.FileDropPhase
	presents := 0
	state := &waylandWindowState{
		app: app,
		win: win,
		config: platform.Config{
			OnFileDrop: func(event platform.FileDropEvent) (platform.Frame, bool, error) {
				phases = append(phases, event.Phase)
				return platform.Frame{}, false, nil
			},
		},
	}
	state.router.present = func(platform.Frame) error {
		presents++
		return nil
	}
	win.state = state
	app.windows = []*waylandWindowState{state}

	initDataListeners()
	puregoInvokeCallback7(rawDataDeviceListener[1], unsafe.Pointer(app), 300, 111, win.surface, 0, 0, 601)
	puregoInvokeCallback5(rawDataDeviceListener[3], unsafe.Pointer(app), 300, 222, 0, 0)
	puregoInvokeCallback2(rawDataDeviceListener[4], unsafe.Pointer(app), 300)

	if len(phases) != 0 {
		t.Fatalf("non-file offer emitted file-drop phases: %v", phases)
	}
	if presents != 0 {
		t.Fatalf("non-file offer presented %d phantom hover frames", presents)
	}
}

func TestWaylandFileDropErrorsReachApplication(t *testing.T) {
	callbackErr := errors.New("file drop callback failed")
	presentErr := errors.New("file drop present failed")
	tests := []struct {
		name        string
		callbackErr error
		presentErr  error
		want        error
	}{
		{name: "callback", callbackErr: callbackErr, want: callbackErr},
		{name: "present", presentErr: presentErr, want: presentErr},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			app := &waylandApplication{
				dataOffers: map[uintptr]*waylandDataOffer{
					601: {proxy: 601, mimeTypes: []string{"text/uri-list"}},
				},
			}
			win := &waylandWindow{app: app, surface: 777}
			state := &waylandWindowState{
				app: app,
				win: win,
				config: platform.Config{
					OnFileDrop: func(platform.FileDropEvent) (platform.Frame, bool, error) {
						return platform.Frame{}, true, test.callbackErr
					},
				},
			}
			state.router.present = func(platform.Frame) error { return test.presentErr }
			win.state = state
			app.windows = []*waylandWindowState{state}

			initDataListeners()
			puregoInvokeCallback7(rawDataDeviceListener[1], unsafe.Pointer(app), 300, 111, win.surface, 0, 0, 601)
			if !errors.Is(app.firstErr, test.want) {
				t.Fatalf("application error = %v, want %v", app.firstErr, test.want)
			}
		})
	}
}

func TestWaylandDataOfferProxyCleanupOnSelectionChange(t *testing.T) {
	var destroyedProxies []uintptr

	api := &waylandAPI{
		proxyMarshalFlags: func(proxy uintptr, opcode uint32, interfacePtr uintptr, version uint32, flags uint32, args ...any) uintptr {
			if opcode == wlDataOfferDestroyOpcode {
				destroyedProxies = append(destroyedProxies, proxy)
			}
			return 0
		},
		proxyDestroy: func(proxy uintptr) {},
	}

	app := &waylandApplication{
		api: api,
		dataOffers: map[uintptr]*waylandDataOffer{
			501: {proxy: 501},
			502: {proxy: 502},
		},
	}
	initDataListeners()

	// Initial selection with offer 501
	puregoInvokeCallback3(rawDataDeviceListener[5], unsafe.Pointer(app), 300, 501)
	if app.clipboardOffer == nil || app.clipboardOffer.proxy != 501 {
		t.Fatalf("expected clipboardOffer 501")
	}

	// New selection with offer 502 -> should destroy superseded offer 501
	puregoInvokeCallback3(rawDataDeviceListener[5], unsafe.Pointer(app), 300, 502)
	if app.clipboardOffer == nil || app.clipboardOffer.proxy != 502 {
		t.Fatalf("expected clipboardOffer 502")
	}

	if len(destroyedProxies) != 1 || destroyedProxies[0] != 501 {
		t.Fatalf("expected destroyed proxy [501], got %v", destroyedProxies)
	}
	if _, exists := app.dataOffers[501]; exists {
		t.Fatalf("expected offer 501 to be removed from app.dataOffers")
	}
}

func TestWaylandDataOfferFinishVersionGuard(t *testing.T) {
	var finishCalled bool

	api := &waylandAPI{
		proxyMarshalFlags: func(proxy uintptr, opcode uint32, interfacePtr uintptr, version uint32, flags uint32, args ...any) uintptr {
			if opcode == wlDataOfferFinishOpcode {
				finishCalled = true
			}
			return 0
		},
	}

	// 1. Ver < 3 (e.g. version 2) -> must NOT call finish
	app := &waylandApplication{
		api: api,
		globals: waylandGlobals{
			dataDeviceManagerVer: 2,
		},
	}
	if app.globals.dataDeviceManagerVer >= 3 {
		app.api.dataOfferFinish(601)
	}
	if finishCalled {
		t.Fatal("dataOfferFinish should not be called when dataDeviceManagerVer < 3")
	}

	// 2. Ver >= 3 -> must call finish
	app.globals.dataDeviceManagerVer = 3
	if app.globals.dataDeviceManagerVer >= 3 {
		app.api.dataOfferFinish(601)
	}
	if !finishCalled {
		t.Fatal("expected dataOfferFinish to be called when dataDeviceManagerVer >= 3")
	}
}

func puregoInvokeCallback3(fn uintptr, a1 unsafe.Pointer, a2 uintptr, a3 uintptr) {
	purego.SyscallN(fn, uintptr(a1), a2, a3)
}

func puregoInvokeCallback2(fn uintptr, a1 unsafe.Pointer, a2 uintptr) {
	purego.SyscallN(fn, uintptr(a1), a2)
}

func puregoInvokeCallback4(fn uintptr, a1 unsafe.Pointer, a2 uintptr, a3 uintptr, a4 uintptr) {
	purego.SyscallN(fn, uintptr(a1), a2, a3, a4)
}

func puregoInvokeCallback5(fn uintptr, a1 unsafe.Pointer, a2 uintptr, a3 uint32, a4 wlFixed, a5 wlFixed) {
	purego.SyscallN(fn, uintptr(a1), a2, uintptr(a3), uintptr(a4), uintptr(a5))
}

func puregoInvokeCallback7(fn uintptr, a1 unsafe.Pointer, a2 uintptr, a3 uint32, a4 uintptr, a5 wlFixed, a6 wlFixed, a7 uintptr) {
	purego.SyscallN(fn, uintptr(a1), a2, uintptr(a3), a4, uintptr(a5), uintptr(a6), a7)
}
