//go:build js && wasm

package app

import (
	"github.com/vivaui/viva/gfx"
	webplatform "github.com/vivaui/viva/internal/platform/web"
)

func platformTextEngine() gfx.TextEngine { return webplatform.NewTextEngine() }
