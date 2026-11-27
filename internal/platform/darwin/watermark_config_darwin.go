//go:build darwin

package darwin

/*
#include "watermark_darwin.h"
*/
import "C"
import "github.com/vivaui/viva/internal/platform"

func nativeWatermark(value platform.Watermark, text, family *C.char) C.viva_watermark {
	return C.viva_watermark{text: text, font_family: family, red: C.double(value.Color.Red), green: C.double(value.Color.Green), blue: C.double(value.Color.Blue), alpha: C.double(value.Color.Alpha), font_size: C.float(value.FontSize), font_weight: C.float(value.FontWeight), angle: C.float(value.Angle), tile_width: C.float(value.TileWidth), tile_height: C.float(value.TileHeight), top: C.float(value.Insets.Top), right: C.float(value.Insets.Right), bottom: C.float(value.Insets.Bottom), left: C.float(value.Insets.Left)}
}
