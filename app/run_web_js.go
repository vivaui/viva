//go:build js && wasm

package app

import "github.com/vivaui/viva/internal/platform/web"

// runWeb is a variable so tests can replace the page runtime.
var runWeb = web.Run
