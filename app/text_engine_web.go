//go:build js && wasm

package app

import (
	webplatform "github.com/vivaui/viva/internal/platform/web"
	"github.com/vivaui/viva/ui"
)

func platformTextEngine() ui.TextEngine { return webplatform.NewTextEngine() }
