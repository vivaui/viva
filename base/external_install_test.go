package base_test

import "github.com/vivaui/viva/ui"

// The engine's tests exercise the styled layer's painting too: installing the
// painters gives this test binary what a real application gets from the ui
// constructors it calls.
func init() { ui.InstallPainters() }
