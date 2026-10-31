package windows

import (
	"runtime"
	"testing"

	"github.com/vivaui/viva/internal/platform"
)

func TestBackdropVersionAndMaterialFallback(t *testing.T) {
	for _, test := range []struct {
		build          uint32
		system, accent bool
		want           backdropAPI
	}{
		{17134, true, true, backdropUnavailable},
		{17763, true, true, backdropAccent},
		{19045, true, true, backdropAccent},
		{22000, true, true, backdropAccent},
		{22621, true, true, backdropSystem},
		{26200, true, false, backdropSystem},
		{19045, true, false, backdropUnavailable},
		{22621, false, true, backdropUnavailable},
	} {
		api := selectBackdropAPI(test.build, test.system, test.accent)
		if api != test.want {
			t.Fatalf("build %d: API = %v, want %v", test.build, api, test.want)
		}
		for _, kind := range []platform.BackdropKind{platform.BackdropNone, platform.BackdropMica, platform.BackdropAcrylic, platform.BackdropMicaAlt, 255} {
			want := platform.BackdropNone
			if api == backdropSystem && kind <= platform.BackdropMicaAlt || api == backdropAccent && kind == platform.BackdropAcrylic {
				want = kind
			}
			if got := api.effectiveKind(kind); got != want {
				t.Errorf("build %d, kind %d: got %d, want %d", test.build, kind, got, want)
			}
		}
	}
}

func TestAcrylicAccentHasTintAndCanBeDisabled(t *testing.T) {
	light := acrylicAccentPolicy(platform.BackdropStyle{Kind: platform.BackdropAcrylic})
	dark := acrylicAccentPolicy(platform.BackdropStyle{Kind: platform.BackdropAcrylic, Dark: true})
	if light.State != 4 || dark.State != 4 || light.GradientColor>>24 == 0 || dark.GradientColor>>24 == 0 || light.GradientColor == dark.GradientColor {
		t.Fatalf("invalid acrylic tint: light=%+v dark=%+v", light, dark)
	}
	for _, kind := range []platform.BackdropKind{platform.BackdropNone, platform.BackdropMica, platform.BackdropMicaAlt, 255} {
		if policy := acrylicAccentPolicy(platform.BackdropStyle{Kind: kind}); policy != (accentPolicy{}) {
			t.Errorf("kind %d did not disable legacy acrylic: %+v", kind, policy)
		}
	}
}

func TestLegacyAcrylicSwitchResizeAndRecovery(t *testing.T) {
	if nativeBackdropAPI() != backdropAccent || !readBackdropPolicy().allows(platform.BackdropAcrylic) {
		t.Skip("requires Windows 10 1809 through Windows 11 21H2 with transparency enabled")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	err := RunApplication(func(application platform.Application) error {
		style := platform.BackdropStyle{Kind: platform.BackdropAcrylic}
		opened, err := application.OpenWindow(platform.Config{
			Title: "Viva legacy acrylic test", Width: 640, Height: 400,
			WindowMode:     platform.WindowCustomTitlebar,
			ClearColor:     platform.Color{Red: .2, Alpha: 1},
			WindowBackdrop: func() platform.BackdropStyle { return style },
		})
		if err != nil {
			return err
		}
		defer opened.Close()
		state := opened.(*nativeWindow).state
		for _, choice := range []platform.BackdropStyle{
			{Kind: platform.BackdropAcrylic}, {Kind: platform.BackdropAcrylic, Dark: true},
			{Kind: platform.BackdropMica}, {Kind: platform.BackdropMicaAlt}, {},
			{Kind: platform.BackdropAcrylic},
		} {
			style = choice
			state.applyBackdrop()
			wantActive := style.Kind == platform.BackdropAcrylic
			if state.backdropActive != wantActive {
				t.Fatalf("%+v: active = %v", style, state.backdropActive)
			}
			if !wantActive && state.renderer.color != state.config.ClearColor {
				t.Fatal("unsupported material did not restore clear color")
			}
			if err := state.renderer.present(320, 240, 1); err != nil {
				return err
			}
			if err := state.renderer.present(640, 400, 1); err != nil {
				return err
			}
		}
		if err := state.renderer.recover(true); err != nil {
			return err
		}
		state.applyBackdrop()
		if !state.backdropActive {
			t.Fatal("acrylic lost after device recovery")
		}
		return state.renderer.present(640, 400, 1)
	})
	if err != nil {
		t.Fatal(err)
	}
}
