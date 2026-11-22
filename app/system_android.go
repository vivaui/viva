//go:build android

package app

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/platform/android"
)

// What the system gives an Android application grows here as its backend lands; the
// rest is answered in mobile_unsupported.go.

func nativeTextEngine() gfx.TextEngine { return android.TextEngine() }
