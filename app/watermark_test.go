//go:build darwin || windows || linux

package app

import (
	"errors"
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/platform"
)

type watermarkTestControl struct {
	fakeWindow
	values []platform.Watermark
}

func (control *watermarkTestControl) SetWatermark(value platform.Watermark) error {
	control.values = append(control.values, value)
	return nil
}

func TestWatermarkDelegatesThroughResultWindowAndRejectsInvalidChanges(t *testing.T) {
	control := &watermarkTestControl{}
	window := &Window{control: resultWindow{WindowControl: control}}
	value := Watermark{Text: "测试用户 10086", Color: Color{Alpha: 0.1}, FontWeight: -0.4}
	if err := window.SetWatermark(value); err != nil {
		t.Fatal(err)
	}
	if len(control.values) != 1 || control.values[0].Text != value.Text || control.values[0].FontSize != 12 {
		t.Fatal("optional capability lost in native wrapper", control.values)
	}
	value.Insets.Left = -1
	if err := window.SetWatermark(value); err == nil || len(control.values) != 1 {
		t.Fatal("invalid change reached host")
	}
	window.closed.Store(true)
	if err := window.SetWatermark(Watermark{}); err == nil {
		t.Fatal("closed window accepted change")
	}
	unsupported := &Window{control: resultWindow{WindowControl: &fakeWindow{}}}
	if err := unsupported.SetWatermark(Watermark{}); !errors.Is(err, ErrUnsupported) {
		t.Fatal("unsupported host lost sentinel", err)
	}
}

func TestWatermarkInitialConfigNormalization(t *testing.T) {
	value, err := (Config{Watermark: Watermark{Text: "测试用户 10086", Color: Color{Alpha: 0.1}}}).normalize()
	if err != nil || value.Watermark.FontSize != 12 {
		t.Fatal("initial watermark not normalized", err, value.Watermark)
	}
	_, err = (Config{Watermark: Watermark{Text: "bad\x00value"}}).normalize()
	if err == nil {
		t.Fatal("invalid initial text accepted")
	}
}

func TestWatermarkReachesNativeWindowConfig(t *testing.T) {
	value := Watermark{Text: "测试用户 10086", Color: Color{Alpha: 0.1}, Insets: gfx.Insets{Top: 42, Left: 120}}
	normalized, err := (Config{Watermark: value}).normalize()
	if err != nil {
		t.Fatal(err)
	}
	painter, err := newPainter(&gfx.Canvas{}, &normalized)
	if err != nil {
		t.Fatal(err)
	}
	config, frames := nativeWindowConfig(&normalized, painter)
	if frames != nil {
		defer frames.stop()
	}
	if config.Watermark.Text != value.Text || config.Watermark.Insets != value.Insets || config.Watermark.TileWidth != 240 {
		t.Fatal("native initial configuration lost watermark", config.Watermark)
	}
}

func TestWatermarkTextOnlyDefaultsAndContentWindowForwarding(t *testing.T) {
	control := &watermarkTestControl{}
	content := &ContentWindow{window: &Window{control: resultWindow{WindowControl: control}}}
	if err := content.SetWatermark(Watermark{Text: "DRAFT"}); err != nil {
		t.Fatal(err)
	}
	if len(control.values) != 1 || control.values[0].Color.Alpha <= 0 {
		t.Fatal("text-only watermark is invisible", control.values)
	}
	if err := content.SetWatermark(Watermark{}); err != nil || control.values[1] != (platform.Watermark{}) {
		t.Fatal("empty text did not disable watermark", err)
	}
	var closed *ContentWindow
	if err := closed.SetWatermark(Watermark{}); err == nil {
		t.Fatal("nil content window accepted watermark")
	}
}
