//go:build darwin && vivahead

package darwin

/*
#cgo CFLAGS: -DVIVA_OFFSCREEN
#include <stdlib.h>
#include "native.h"
*/
import "C"

import (
	"fmt"
	"unsafe"

	"github.com/vivaui/viva/internal/render"
)

// Offscreen exists only in builds with -tags vivahead, so an application
// release links none of its native code.
//
// Offscreen renders frames to pixels with the window's own Metal pipelines and
// CoreText rasterizers but without a window, a view or a run loop, so a
// screenshot is the pixels the window would show. It is not safe for
// concurrent use.
type Offscreen struct {
	handle        unsafe.Pointer
	width, height int
	scale         float64
}

// NewOffscreen creates a renderer for a width x height logical-pixel scene at
// the given backing scale, cleared to clear before each render.
func NewOffscreen(width, height int, scale float64, clear Color) (*Offscreen, error) {
	if width < 1 || height < 1 || scale < 1 {
		return nil, fmt.Errorf("darwin: invalid offscreen size %dx%d at %gx", width, height, scale)
	}
	handle := C.viva_offscreen_create(
		(*C.uchar)(unsafe.Pointer(&sceneShaderLibrary[0])), C.size_t(len(sceneShaderLibrary)),
		C.double(width), C.double(height), C.double(scale),
		C.double(clear.Red), C.double(clear.Green), C.double(clear.Blue), C.double(clear.Alpha),
	)
	if handle == nil {
		return nil, fmt.Errorf("darwin: no Metal device for offscreen rendering")
	}
	return &Offscreen{handle: handle, width: width, height: height, scale: scale}, nil
}

// Set replaces the scene: rectangle and triangle instances in paint order,
// text labels, and the native text editor's overlay (nil for none).
func (offscreen *Offscreen) Set(rectangles []render.Rectangle, labels []TextLabel, input *TextInput) error {
	native := make([]C.viva_rectangle, len(rectangles))
	clips := make([]C.viva_clip, len(rectangles))
	for index, rectangle := range rectangles {
		native[index] = makeNativeRectangle(rectangle)
		clips[index] = makeNativeClip(rectangle)
	}
	var rectanglePointer *C.viva_rectangle
	var clipPointer *C.viva_clip
	if len(native) > 0 {
		rectanglePointer = &native[0]
		clipPointer = &clips[0]
	}
	if C.viva_offscreen_set_rectangles(offscreen.handle, rectanglePointer, clipPointer, C.size_t(len(native))) != 0 {
		return fmt.Errorf("darwin: offscreen rectangle upload failed")
	}

	if len(labels) > 0 {
		memory, strings, count, err := makeNativeTextLabels(labels)
		if err != nil {
			return err
		}
		status := C.viva_offscreen_set_labels(offscreen.handle, (*C.viva_text_label)(memory), C.size_t(count))
		C.free(memory)
		for _, value := range strings {
			C.free(value)
		}
		if status != 0 {
			return fmt.Errorf("darwin: offscreen label rasterization failed")
		}
	} else if C.viva_offscreen_set_labels(offscreen.handle, nil, 0) != 0 {
		return fmt.Errorf("darwin: offscreen label reset failed")
	}

	var config C.viva_text_input_config
	if input != nil {
		if err := validateTextInput(input); err != nil {
			return err
		}
		memory := textBytes(input.State.Value)
		if memory != nil {
			defer C.free(memory)
		}
		config = makeNativeTextInput(*input, memory)
	}
	if C.viva_offscreen_set_text_input(offscreen.handle, config) != 0 {
		return fmt.Errorf("darwin: offscreen text editor rasterization failed")
	}
	return nil
}

// Render draws the scene and returns its pixels as BGRA bytes, top row first,
// together with their dimensions in device pixels.
func (offscreen *Offscreen) Render() (pixels []byte, width, height int, err error) {
	width = int(float64(offscreen.width)*offscreen.scale + 0.999999)
	height = int(float64(offscreen.height)*offscreen.scale + 0.999999)
	pixels = make([]byte, width*height*4)
	var renderedWidth, renderedHeight C.size_t
	status := C.viva_offscreen_render(offscreen.handle, (*C.uchar)(unsafe.Pointer(&pixels[0])), C.size_t(len(pixels)), &renderedWidth, &renderedHeight)
	if status != 0 {
		return nil, 0, 0, fmt.Errorf("darwin: offscreen render failed (%dx%d device pixels)", renderedWidth, renderedHeight)
	}
	return pixels, int(renderedWidth), int(renderedHeight), nil
}

// Close releases the renderer.
func (offscreen *Offscreen) Close() {
	if offscreen.handle != nil {
		C.viva_offscreen_release(offscreen.handle)
		offscreen.handle = nil
	}
}

func (offscreen *Offscreen) LabelTextureBytes() uint64 {
	return uint64(C.viva_offscreen_label_texture_bytes(offscreen.handle))
}
