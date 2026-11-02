package app

import (
	"github.com/vivaui/viva/internal/platform"
	"testing"
)

func TestWindowMaterialsBridge(t *testing.T) {
	if (Config{}).windowMaterials().WindowBackdrop != nil {
		t.Fatal("zero config opted into composition")
	}
	style := BackdropStyle{}
	notified := false
	config := Config{
		WindowBackdrop:   func() BackdropStyle { return style },
		OnBackdropChange: func(active bool) { notified = active },
	}
	materials := config.windowMaterials()
	for _, pair := range []struct {
		public BackdropKind
		native platform.BackdropKind
	}{
		{BackdropNone, platform.BackdropNone}, {BackdropMica, platform.BackdropMica},
		{BackdropAcrylic, platform.BackdropAcrylic}, {BackdropMicaAlt, platform.BackdropMicaAlt},
		{255, 255},
	} {
		style = BackdropStyle{Kind: pair.public, Dark: true}
		if got := materials.WindowBackdrop(); got.Kind != pair.native || !got.Dark {
			t.Fatalf("live style lost at platform boundary: %+v", got)
		}
	}
	materials.OnBackdropChange(true)
	if !notified {
		t.Fatal("activation callback not forwarded")
	}
	if got := testing.AllocsPerRun(100, func() { materials.WindowBackdrop() }); got != 0 {
		t.Fatalf("material polling allocated %g times", got)
	}
}
