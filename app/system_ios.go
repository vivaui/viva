//go:build ios

package app

import (
	"github.com/vivaui/viva/gfx"
	darwinplatform "github.com/vivaui/viva/internal/platform/darwin"
)

// What the system gives an iPhone application grows here as its backend lands; the
// rest is answered in mobile_unsupported.go.

func nativeTextEngine() gfx.TextEngine { return darwinplatform.SharedTextEngine() }
