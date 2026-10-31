package windows

import (
	"runtime"
	"testing"
	"unsafe"

	"github.com/vivaui/viva/internal/platform"
)

func TestBackdropWithoutCompositionReportsInitialFallback(t *testing.T) {
	state := &windowState{window: 1}
	var changes []bool
	state.config = platform.Config{
		WindowMode: platform.WindowCustomTitlebar,
		ClearColor: platform.Color{Alpha: 1},
		WindowBackdrop: func() platform.BackdropStyle {
			return platform.BackdropStyle{Kind: platform.BackdropMica}
		},
		OnBackdropChange: func(active bool) { changes = append(changes, active) },
	}
	state.applyBackdrop()
	state.applyBackdrop()
	if len(changes) != 1 || changes[0] || state.renderer.color != state.config.ClearColor {
		t.Fatalf("fallback: changes=%v, clear=%+v", changes, state.renderer.color)
	}
}

func TestBackdropAppearanceSurvivesSystemSettings(t *testing.T) {
	if !windowBackdropSupported() || !readBackdropPolicy().allows(platform.BackdropAcrylic) {
		t.Skip("system materials unavailable")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	previous := appearancePin.Swap(1) // System caption preference is light.
	defer appearancePin.Store(previous)
	state := &windowState{window: staticWindow(t)}
	choice := platform.BackdropStyle{Kind: platform.BackdropMica, Dark: true}
	state.config.WindowBackdrop = func() platform.BackdropStyle { return choice }
	state.renderer.composition = true
	state.applyBackdrop()
	state.settingsChanged()
	if !state.backdropActive || !state.titlebarDark {
		t.Fatal("system appearance replaced the material's dark appearance")
	}
	choice.Kind = platform.BackdropNone
	state.applyBackdrop()
	if state.titlebarDark {
		t.Fatal("disabling the material did not restore the caption preference")
	}
}

func TestWindowBackdropSwitchResizeAndDeviceRecovery(t *testing.T) {
	if !windowBackdropSupported() || !readBackdropPolicy().allows(platform.BackdropAcrylic) {
		t.Skip("system materials unavailable")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	state := &windowState{window: staticWindow(t), scale: 1}
	writeCompositionWindowStyle.Call(state.window, ^uintptr(19), wsExNoRedirectionBitmap)
	style := platform.BackdropStyle{}
	var changes []bool
	state.config = platform.Config{
		ClearColor:       platform.Color{Red: .2, Green: .3, Blue: .4, Alpha: 1},
		WindowBackdrop:   func() platform.BackdropStyle { return style },
		OnBackdropChange: func(active bool) { changes = append(changes, active) },
	}
	state.renderer.composition = true
	if err := state.renderer.initialize(state.window, state.config.ClearColor); err != nil {
		t.Skipf("no graphics: %v", err)
	}
	defer state.renderer.close()
	if !state.renderer.composition {
		t.Fatal("composition unexpectedly fell back")
	}
	getAttribute := dwm.NewProc("DwmGetWindowAttribute")
	for _, choice := range []platform.BackdropStyle{
		{}, {Kind: platform.BackdropMica, Dark: true},
		{Kind: platform.BackdropAcrylic, Dark: true},
		{Kind: platform.BackdropMicaAlt, Dark: true},
		{Kind: platform.BackdropMicaAlt},
		{Kind: platform.BackdropAcrylic}, {},
	} {
		style = choice
		state.applyBackdrop()
		wantActive := choice.Kind != platform.BackdropNone
		if state.backdropActive != wantActive {
			t.Fatalf("%+v: active = %v", choice, state.backdropActive)
		}
		var kind uint32
		status, _, _ := getAttribute.Call(state.window, dwmSystemBackdropType, uintptr(unsafe.Pointer(&kind)), unsafe.Sizeof(kind))
		if hresult(status) != nil || kind != uint32(choice.Kind)+1 {
			t.Fatalf("DWM kind = %d, status = %x", kind, status)
		}
		if wantActive && state.renderer.color.Alpha != 0 {
			t.Fatal("material hidden by opaque clear")
		}
		if !wantActive && state.renderer.color != state.config.ClearColor {
			t.Fatal("solid fallback lost its clear color")
		}
		if err := state.renderer.present(64, 64, 1); err != nil {
			t.Fatal(err)
		}
		if err := state.renderer.present(96, 80, 1); err != nil {
			t.Fatal(err)
		}
	}
	if len(changes) != 3 || changes[0] || !changes[1] || changes[2] {
		t.Fatalf("changes = %v", changes)
	}
	style = platform.BackdropStyle{Kind: platform.BackdropMica}
	state.applyBackdrop()
	for _, software := range []bool{false, true} {
		if err := state.renderer.recover(software); err != nil {
			t.Fatal(err)
		}
		if !state.renderer.composition {
			t.Fatal("composition lost after device recovery")
		}
		state.applyBackdrop()
		if err := state.renderer.present(64, 64, 1); err != nil {
			t.Fatal(err)
		}
	}
	// Even a remembered material must become opaque when composition fails.
	state.renderer.composition = false
	state.applyBackdrop()
	if state.backdropActive || state.renderer.color != state.config.ClearColor {
		t.Fatal("failed composition did not restore solid background")
	}
}

func TestCompositionClearIsPremultiplied(t *testing.T) {
	offscreen, err := NewOffscreen(8, 8, 1, platform.Color{Red: .8, Green: .4, Blue: .2, Alpha: .5})
	if err != nil {
		t.Skipf("no graphics: %v", err)
	}
	defer offscreen.Close()
	offscreen.renderer.composition = true
	img, err := offscreen.Image()
	if err != nil {
		t.Fatal(err)
	}
	pixel := img.RGBAAt(4, 4)
	if pixel.R < 101 || pixel.R > 103 || pixel.G < 50 || pixel.G > 52 || pixel.B < 25 || pixel.B > 27 || pixel.A < 127 || pixel.A > 129 {
		t.Fatalf("non-premultiplied clear: %v", pixel)
	}
}

func TestCompositionFallbackRestoresWindowRedirection(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	window := staticWindow(t)
	writeCompositionWindowStyle.Call(window, ^uintptr(19), wsExNoRedirectionBitmap)
	restoreWindowRedirection(window)
	style, _, _ := readCompositionWindowStyle.Call(window, ^uintptr(19))
	if style&wsExNoRedirectionBitmap != 0 {
		t.Fatal("fallback still has no redirection surface")
	}
	restoreWindowRedirection(window)
}

func TestCustomBackdropKeepsWindowControlsAndSystemMenu(t *testing.T) {
	if !windowBackdropSupported() || !readBackdropPolicy().allows(platform.BackdropAcrylic) {
		t.Skip("system materials unavailable")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	err := RunApplication(func(application platform.Application) error {
		choice := platform.BackdropStyle{Kind: platform.BackdropMica, Dark: true}
		controls := &platform.WindowActions{}
		opened, err := application.OpenWindow(platform.Config{
			Title: "Viva material titlebar test", Width: 640, Height: 400,
			WindowMode: platform.WindowCustomTitlebar, TitlebarHeight: 40,
			ClearColor: platform.Color{Alpha: 1}, Actions: controls,
			WindowBackdrop: func() platform.BackdropStyle { return choice },
			PressAt:        func(x, y float32) platform.PressKind { return platform.PressControl },
		})
		if err != nil {
			return err
		}
		defer opened.Close()
		state := opened.(*nativeWindow).state
		getStyle := user.NewProc("GetWindowLongPtrW")
		getSystemMenu := user.NewProc("GetSystemMenu")
		for _, material := range []platform.BackdropKind{platform.BackdropMica, platform.BackdropAcrylic, platform.BackdropMicaAlt, platform.BackdropNone} {
			choice.Kind = material
			state.applyBackdrop()
			if state.backdropActive != (material != platform.BackdropNone) {
				t.Errorf("material %v is not active", material)
			}
			style, _, _ := getStyle.Call(state.window, ^uintptr(15)) // GWL_STYLE = -16
			const sizingAndMenu = 0x00040000 | 0x00080000 | 0x00020000 | 0x00010000
			if style&sizingAndMenu != sizingAndMenu {
				t.Errorf("window lost sizing, minimize, maximize or menu styles: %x", style)
			}
			menu, _, _ := getSystemMenu.Call(state.window, 0)
			if menu == 0 {
				t.Error("the system menu was removed to hide the native caption")
			}
			if !state.overMaximizeButton(640-platform.WindowControlWidth*1.5, 10, 640) {
				t.Error("the custom maximize button lost its snap hit target")
			}
			if controls.Minimize == nil || controls.ToggleMaximize == nil || controls.RequestClose == nil {
				t.Error("custom window actions are missing")
			}
			var client, frame rect
			getClientRect.Call(state.window, uintptr(unsafe.Pointer(&client)))
			getWindowRect.Call(state.window, uintptr(unsafe.Pointer(&frame)))
			if client.Right != frame.Right-frame.Left || client.Bottom != frame.Bottom-frame.Top {
				t.Error("the custom frame reserved a native titlebar")
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
