//go:build !js

package app

import "github.com/vivaui/viva/gfx"

func platformTextEngine() gfx.TextEngine { return nativeTextEngine() }
