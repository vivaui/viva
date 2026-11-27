package app

import (
	"fmt"
	"sync"

	"github.com/vivaui/viva/gfx"

	"github.com/vivaui/viva/internal/diag"
	"github.com/vivaui/viva/internal/platform"
)

// Watermark is a retained, input-transparent single-line text pattern above canvas and native
// views. Text is centered in each tile using font metrics, then the pattern is
// rotated by Angle degrees in the content's top-left, Y-down coordinate system.
// Zero sizes use 12pt text and a 240 by 180 logical-pixel tile. A zero Color uses
// neutral gray at 12% opacity; other colors are linear RGBA, as Config.ClearColor.
// FontWeight uses gfx.WeightLight through gfx.WeightBold. FontFamily optionally
// names an installed family, with native fallback; empty uses the system font.
// Typography is explicit and independent of the canvas's interface font setting.
// Text is limited to 16 KiB, font size to 4096 logical pixels and tile sides to
// 8192 logical pixels; native backing tiles are additionally limited to 16 Mpixels.
// Empty Text disables the overlay. Use Supports(CapabilityWindowWatermark) to
// check native support; headless hosts may record the configuration for tests.
type Watermark struct {
	Text, FontFamily            string
	Color                       Color
	FontSize, FontWeight, Angle float32
	TileWidth, TileHeight       float32
	Insets                      gfx.Insets
}

func (w Watermark) platform() platform.Watermark {
	return platform.Watermark{Text: w.Text, FontFamily: w.FontFamily, Color: platform.Color(w.Color), FontSize: w.FontSize, FontWeight: w.FontWeight, Angle: w.Angle, TileWidth: w.TileWidth, TileHeight: w.TileHeight, Insets: w.Insets}
}

func (w Watermark) normalized() (Watermark, error) {
	value, err := platform.NormalizeWatermark(w.platform())
	if err != nil {
		return Watermark{}, err
	}
	return Watermark{Text: value.Text, FontFamily: value.FontFamily, Color: Color(value.Color), FontSize: value.FontSize, FontWeight: value.FontWeight, Angle: value.Angle, TileWidth: value.TileWidth, TileHeight: value.TileHeight, Insets: value.Insets}, nil
}

// Hosts implement this optional capability using the public value; application
// hosts outside Viva cannot import internal/platform.
type watermarkControl interface{ SetWatermark(Watermark) error }
type nativeWatermarkControl interface {
	SetWatermark(platform.Watermark) error
}

// SetWatermark replaces the retained overlay. Calls are safe from any goroutine;
// native changes run on the window thread, and Windows queues the latest request.
// A successful return accepts the request, not completion of native rendering.
// Invalid requests change nothing; asynchronous native failures use the diagnostic
// sink. Unsupported hosts return an error wrapping ErrUnsupported.
func (window *Window) SetWatermark(value Watermark) error {
	if window == nil || window.closed.Load() {
		return fmt.Errorf("app: window is closed")
	}
	value, err := value.normalized()
	if err != nil {
		return err
	}
	control, ok := window.control.(watermarkControl)
	if !ok {
		return fmt.Errorf("%w: window watermarks", ErrUnsupported)
	}
	return control.SetWatermark(value)
}

var unsupportedWatermarkNotice sync.Once

func reportUnsupportedWatermark() {
	unsupportedWatermarkNotice.Do(func() {
		diag.Report(diag.Debug, "window", "watermark-unsupported", "native window watermarks are unsupported by this host")
	})
}
