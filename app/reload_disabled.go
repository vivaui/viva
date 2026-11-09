//go:build !vivadev && (darwin || windows || linux)

package app

import "github.com/vivaui/viva/gfx"

// runReloadWorkerIfEnabled is the release implementation: no worker protocol
// package is imported, so a normal build neither starts a worker nor carries
// its server and codec.
func runReloadWorkerIfEnabled(Config, *gfx.Canvas) (bool, error) { return false, nil }
