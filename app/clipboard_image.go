package app

import (
	"fmt"

	"github.com/vivaui/viva/internal/platform"
)

var (
	// ErrNoImage reports that the clipboard holds no image.
	ErrNoImage = platform.ErrNoClipboardImage
	// ErrTooLarge reports a clipboard image above MaxClipboardImageBytes as PNG,
	// or one with too many pixels to convert.
	ErrTooLarge = platform.ErrClipboardImageTooLarge
)

// MaxClipboardImageBytes is the largest PNG ClipboardImagePNG returns.
const MaxClipboardImageBytes = platform.MaxClipboardImageBytes

// HasClipboardImage reports whether the clipboard holds an image, without
// reading it, so a Paste button can be enabled cheaply. It never blocks on
// macOS, Windows or in a Driver run. On Linux a clipboard owned by another
// application is asked for its formats, which waits for that application for up
// to three seconds, so ask when a dialog opens or a window gains focus, not on
// every frame. The Web target cannot ask synchronously and reports false.
func HasClipboardImage() bool {
	if _, err, handled := driverClipboardImage(); handled {
		return err == nil
	}
	return nativeHasClipboardImage()
}

// ClipboardImagePNG returns the clipboard's image as PNG bytes, ErrNoImage when
// it holds none, and ErrTooLarge when the PNG would exceed
// MaxClipboardImageBytes. The platforms hand over what they have in one of
// their native formats and Viva transcodes to PNG: macOS reads public.png or
// converts public.tiff, Windows reads the registered "PNG" format or converts
// CF_DIBV5 and CF_DIB (palette, 16, 24 and 32 bit, with alpha), and Linux reads
// the X11 CLIPBOARD target image/png in a single property transfer (an owner
// that streams it with INCR is reported as unsupported). The Web target returns
// ErrAsyncRequired; use RequestClipboardImage there. The caller owns the
// returned bytes; decode them with image/png. Nothing of the image is
// retained by Viva.
func ClipboardImagePNG() ([]byte, error) {
	if png, err, handled := driverClipboardImage(); handled {
		return png, err
	}
	return nativeClipboardImagePNG()
}

// RequestClipboardImage delivers ClipboardImagePNG's answer to done, which is
// the way to read an image on the Web target, where the browser answers through
// a Promise after asking permission (call it from an input handler). Native
// platforms and driver runs call done before returning, with the same result as
// ClipboardImagePNG. done runs exactly once.
func RequestClipboardImage(done func(png []byte, err error)) {
	nativeRequestClipboardImage(done)
}

// SetClipboardImage makes png the picture the clipboard holds, as copying an
// image in a browser or a photo viewer does, so that any application's Paste
// puts it there. It replaces whatever the clipboard held, text included: one
// clipboard holds one copy. This is the write side of ClipboardImagePNG, and a
// paste of what it wrote reads back as the same picture.
//
// Each platform offers the picture in the flavours its readers ask for: macOS
// public.png beside its TIFF conversion, Windows the registered "PNG" format
// beside CF_DIB, and Linux the X11 CLIPBOARD's image/png target (the GTK shell
// serves the same target through GTK). png must be a PNG within
// MaxClipboardImageBytes and must carry a picture the platform can hold; a PNG
// that fails either check is an error and leaves the clipboard as it was. The
// Web target writes through a Promise and returns ErrAsyncRequired;
// SetClipboardImageAsync writes there.
func SetClipboardImage(png []byte) error {
	if len(png) == 0 {
		return fmt.Errorf("app: no image to put on the clipboard")
	}
	if err := platform.CheckClipboardPNG(png); err != nil {
		return err
	}
	if err, handled := driverSetClipboardImage(png); handled {
		return err
	}
	return nativeSetClipboardImage(png)
}
