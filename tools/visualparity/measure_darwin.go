//go:build darwin

package main

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/platform/darwin"
	"github.com/vivaui/viva/text"
)

const platformName = "macos"

func nativeMeasurer() (text.MeasureFunc, func(), error) {
	return darwin.MeasureText, func() {}, nil
}

func nativeMono(size float32, face gfx.MonoFace) (float32, float32, error) {
	return darwin.MeasureMono(size, face)
}
