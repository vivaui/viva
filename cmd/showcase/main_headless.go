//go:build vivahead && (darwin || windows || linux)

package main

// Linked only into -tags vivahead builds: with VIVA_HEADLESS=1 the gallery serves
// the headless line protocol instead of opening a window (see package headless).
import _ "github.com/vivaui/viva/headless/auto"
