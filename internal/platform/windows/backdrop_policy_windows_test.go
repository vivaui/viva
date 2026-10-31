package windows

import (
	"runtime"
	"testing"
	"unsafe"

	"github.com/vivaui/viva/internal/platform"
)

func TestBackdropSystemRestrictions(t *testing.T) {
	for _, kind := range []platform.BackdropKind{platform.BackdropMica, platform.BackdropAcrylic, platform.BackdropMicaAlt} {
		for _, test := range []struct {
			name   string
			policy backdropPolicy
			want   bool
		}{
			{"normal", backdropPolicy{}, true},
			{"high contrast", backdropPolicy{highContrast: true}, false},
			{"transparency disabled", backdropPolicy{transparencyDisabled: true}, false},
			{"battery saver", backdropPolicy{batterySaver: true}, kind != platform.BackdropAcrylic},
		} {
			if got := test.policy.allows(kind); got != test.want {
				t.Errorf("%s, kind %d: allowed=%v, want %v", test.name, kind, got, test.want)
			}
		}
	}
}

func TestBackdropPolicyTransitionsRestoreOpaqueClearAndNotify(t *testing.T) {
	if !windowBackdropSupported() {
		t.Skip("requires system backdrop API")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	state := &windowState{window: staticWindow(t), backdropPolicyKnown: true}
	style := platform.BackdropStyle{Kind: platform.BackdropAcrylic}
	var changes []bool
	state.config = platform.Config{
		ClearColor:       platform.Color{Red: .2, Alpha: 1},
		WindowBackdrop:   func() platform.BackdropStyle { return style },
		OnBackdropChange: func(active bool) { changes = append(changes, active) },
	}
	state.renderer.composition = true
	for _, policy := range []backdropPolicy{{}, {highContrast: true}, {}, {transparencyDisabled: true}, {}, {batterySaver: true}, {}} {
		state.backdropPolicy = policy // Simulate policy without changing the user's settings.
		state.applyBackdrop()
		want := policy.allows(style.Kind)
		if state.backdropActive != want {
			t.Fatalf("%+v: active=%v", policy, state.backdropActive)
		}
		if want && state.renderer.color.Alpha != 0 {
			t.Fatal("material is hidden")
		}
		if !want && state.renderer.color != state.config.ClearColor {
			t.Fatal("fallback lost its clear color")
		}
		state.applyBackdrop()
	}
	if len(changes) != 7 {
		t.Fatalf("notifications=%v", changes)
	}
	// Reapplying after a DWM reset is not a new activation notification.
	state.backdropApplied = false
	state.applyBackdrop()
	if len(changes) != 7 {
		t.Fatalf("duplicate activation notification: %v", changes)
	}
}

func TestBackdropSettingsRestoreDWMAttributes(t *testing.T) {
	if !windowBackdropSupported() || !readBackdropPolicy().allows(platform.BackdropMicaAlt) {
		t.Skip("system material unavailable")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	state := &windowState{window: staticWindow(t)}
	state.renderer.composition = true
	state.config.WindowBackdrop = func() platform.BackdropStyle { return platform.BackdropStyle{Kind: platform.BackdropMicaAlt} }
	state.applyBackdrop()
	disabled := uint32(1)
	dwmSetWindowAttribute.Call(state.window, dwmSystemBackdropType, uintptr(unsafe.Pointer(&disabled)), unsafe.Sizeof(disabled))
	state.settingsChanged()
	var got uint32
	status, _, _ := dwm.NewProc("DwmGetWindowAttribute").Call(state.window, dwmSystemBackdropType, uintptr(unsafe.Pointer(&got)), unsafe.Sizeof(got))
	if hresult(status) != nil || got != 4 {
		t.Fatalf("Mica Alt was not reapplied: kind=%d, status=%x", got, status)
	}
}

func TestBackdropNativeFailureFallsBackAndRetriesOnSettingsChange(t *testing.T) {
	if !windowBackdropSupported() || !readBackdropPolicy().allows(platform.BackdropMica) {
		t.Skip("system material unavailable")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	state := &windowState{window: ^uintptr(0)} // Invalid HWND forces DWM failure.
	state.renderer.composition = true
	var changes []bool
	state.config = platform.Config{
		ClearColor:       platform.Color{Blue: .4, Alpha: 1},
		WindowBackdrop:   func() platform.BackdropStyle { return platform.BackdropStyle{Kind: platform.BackdropMica} },
		OnBackdropChange: func(active bool) { changes = append(changes, active) },
	}
	state.applyBackdrop()
	if state.backdropActive || state.renderer.color != state.config.ClearColor {
		t.Fatal("failed DWM call did not restore clear color")
	}
	state.window = staticWindow(t)
	state.settingsChanged()
	if !state.backdropActive || len(changes) != 2 || changes[0] || !changes[1] {
		t.Fatalf("failed material did not recover: active=%v, changes=%v", state.backdropActive, changes)
	}
}

func TestCompositionCreationFailureUsesOpaqueSwapChain(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	window := staticWindow(t)
	writeCompositionWindowStyle.Call(window, ^uintptr(19), wsExNoRedirectionBitmap)
	previous := createCompositionDevice
	createCompositionDevice = systemDLL("dcomp.dll").NewProc("VivaTestMissingCompositionFunction")
	defer func() { createCompositionDevice = previous }()
	renderer := renderer{composition: true}
	defer renderer.close()
	if err := renderer.initialize(window, platform.Color{Alpha: 1}); err != nil {
		t.Fatal(err)
	}
	if renderer.composition || renderer.compositionDevice != 0 || renderer.compositionTarget != 0 || renderer.compositionVisual != 0 {
		t.Fatal("failed composition retained native objects")
	}
	style, _, _ := readCompositionWindowStyle.Call(window, ^uintptr(19))
	if style&wsExNoRedirectionBitmap != 0 {
		t.Fatal("fallback has no redirection bitmap")
	}
	if err := renderer.present(64, 64, 1); err != nil {
		t.Fatal(err)
	}
}

func TestBackdropPowerNotificationLifetime(t *testing.T) {
	if nativeBackdropAPI() == backdropUnavailable {
		t.Skip("no supported material")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	state := &windowState{window: staticWindow(t)}
	state.config.WindowBackdrop = func() platform.BackdropStyle { return platform.BackdropStyle{} }
	state.watchBackdropPower()
	defer state.closeBackdropPower()
	if state.backdropPowerNotification == 0 {
		t.Fatal("battery saver notification was not registered")
	}
	handle := state.backdropPowerNotification
	state.watchBackdropPower()
	if state.backdropPowerNotification != handle {
		t.Fatal("watching twice replaced the notification without releasing it")
	}
	state.closeBackdropPower()
	state.closeBackdropPower()
	if state.backdropPowerNotification != 0 {
		t.Fatal("notification handle retained after close")
	}
}
