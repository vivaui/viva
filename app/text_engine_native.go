//go:build !js

package app

import "github.com/vivaui/viva/ui"

func platformTextEngine() ui.TextEngine { return nativeTextEngine() }
