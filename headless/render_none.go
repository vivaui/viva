//go:build (darwin && !vivahead) || android

package headless

import (
	"image"

	"github.com/vivaui/viva/app"
)

// Screenshots need an offscreen renderer: macOS builds have one with
// -tags vivahead (Windows and Linux builds always do). Without it, sessions
// still operate and answer queries.
const (
	rendererAvailable = false
	defaultScale      = 2
	rendersTextEditor = false
)

type renderer struct{}

func newRenderer() *renderer { return &renderer{} }

func (r *renderer) close() {}

func (r *renderer) render(app.Snapshot, float64) (*image.RGBA, error) { return nil, ErrNoRenderer }
