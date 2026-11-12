package componentdemo

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// caption declares the muted explanatory line under a specimen; it wraps instead of
// clipping, so it stays readable at the narrowest stage.
func caption(c ui.C, value string) {
	ui.Wrap(c, value).FontSize(c.Theme().Metrics.FontSizeS).Muted()
}

// drawStage lays content over the whole stage, with no page title: for a demo
// whose body is the stage itself, such as the calendar or the sidebar. It shares
// the State's Page with drawPage, so identities, theme and page memory follow
// the same rules; the caller declares the root container that fills the stage.
func (state *State) drawStage(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size, content func(c ui.C)) {
	state.pageFor(theme).Draw(canvas, gfx.Rect{Width: size.Width, Height: size.Height}, content)
}

// listIndex returns the item number of id in the list scope, or -1: the way a key
// handler reads which control of a list has the focus.
func listIndex(id gfx.ID, list gfx.Scope) int {
	if index, ok := id.In(list); ok {
		return index
	}
	return -1
}

// widgetBag is the memory a pointer-driven demo keeps beside its values. A pointer
// event arrives without a canvas, so the demo remembers where its control was placed
// by the last paint (the ui page decides that, not the demo), and the range slider
// keeps its two thumbs. It is allocated once, by the demo's initializer, and held in
// State.slider, so a State stays comparable and painting leaves the State unchanged.
type widgetBag struct {
	// placed is where the demo's pointer-driven control was in the last paint.
	placed gfx.Rect
	thumbs [2]components.SliderThumbValue
}

// sliderDemoState is what State.slider holds: the bag of the demos that need one.
type sliderDemoState = *widgetBag

// bag returns the demo's widgetBag, allocating it on first use.
func (state *State) bag() *widgetBag {
	if state.slider == nil {
		state.slider = new(widgetBag)
	}
	return state.slider
}
