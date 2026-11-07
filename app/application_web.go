//go:build js && wasm

package app

import "fmt"

// RunApplication reports that a browser page hosts one window. Applications
// that need several windows must use the native backends; a web build can use
// Run or RunCanvas for its primary window.
func RunApplication(start func(*Application) error) error {
	return fmt.Errorf("%w: a browser page hosts one window; use Run or RunCanvas", ErrUnsupported)
}
