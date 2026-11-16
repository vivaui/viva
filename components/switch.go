package components

import "github.com/vivaui/viva/gfx"

// SwitchSize selects the official default or small switch geometry.
type SwitchSize uint8

const (
	SwitchSizeDefault SwitchSize = iota
	SwitchSizeSmall
)

// switchSlotChecked is the animation slot holding thumb travel, 0 (off) to 1 (on).
const switchSlotChecked uint8 = 0

// Switch paints one two-state toggle with caller-owned state. When the
// runtime provides a frame clock, a change of the checked value slides the
// thumb and fades the track over the theme's Transition; the caller still
// passes only the destination state.
type Switch struct {
	canvas   *gfx.Canvas
	theme    gfx.Theme
	id       gfx.ID
	bounds   gfx.Rect
	label    string
	checked  bool
	focused  bool
	invalid  bool
	size     SwitchSize
	disabled bool
	// recipe, when set, replaces SwitchRecipe; see Button.Recipe.
	recipe func(gfx.Theme, gfx.State, float32) SwitchLook
}

// BeginSwitch starts a switch declaration. Zero-sized bounds select official
// preset geometry. ID zero is a static preview and declares no hit.
func BeginSwitch(canvas *gfx.Canvas, theme gfx.Theme, id gfx.ID, bounds gfx.Rect, checked bool) Switch {
	focused := false
	if canvas != nil && !id.IsZero() {
		focused = canvas.FocusVisible() == id
	}
	return Switch{
		canvas: canvas, theme: theme, id: id, bounds: bounds,
		label: "switch", checked: checked, focused: focused,
	}
}

// Recipe replaces the theme's switch recipe for this switch, progress being
// how far the thumb has travelled from off to on. A nil recipe keeps the
// theme's.
func (switchControl Switch) Recipe(recipe func(gfx.Theme, gfx.State, float32) SwitchLook) Switch {
	switchControl.recipe = recipe
	return switchControl
}

// Small selects the official compact switch size.
func (switchControl Switch) Small() Switch {
	switchControl.size = SwitchSizeSmall
	return switchControl
}

// Label sets the accessible name without painting text.
func (switchControl Switch) Label(label string) Switch {
	switchControl.label = label
	return switchControl
}

// Invalid applies the official destructive validation border and ring.
func (switchControl Switch) Invalid() Switch {
	switchControl.invalid = true
	return switchControl
}

// Disabled paints the official 50% opacity state and retains disabled
// accessibility semantics for nonzero IDs.
func (switchControl Switch) Disabled() Switch {
	switchControl.disabled = true
	return switchControl
}

// SwitchResult is what painting reports back; Clicked is true in the event pass of a
// click on the control (see gfx.Canvas.Clicked), so the code that declares it
// handles it.
type SwitchResult struct {
	canvas *gfx.Canvas
	id     gfx.ID
}

// Clicked reports whether this paint is delivering a click on the control.
func (result SwitchResult) Clicked() bool {
	return result.canvas != nil && result.canvas.Clicked(result.id)
}

// SwitchExtent is the width and height of a switch at the theme's density.
func SwitchExtent(theme gfx.Theme, small bool) (width, height float32) {
	scale := theme.Metrics.ControlHeight / 32
	if small {
		return 24 * scale, 14 * scale
	}
	return 32 * scale, 18.4 * scale
}

// SwitchHitExtent is the width and height of the interaction region Paint
// declares around a switch. The padding is the fixed 12 by 8 logical pixels
// Paint adds, not scaled by density.
func SwitchHitExtent(theme gfx.Theme, small bool) (width, height float32) {
	width, height = SwitchExtent(theme, small)
	return width + 24, height + 16
}

// Paint draws the switch and declares its enlarged desktop hit region.
func (switchControl Switch) Paint() SwitchResult {
	result := SwitchResult{canvas: switchControl.canvas, id: switchControl.id}
	if switchControl.canvas == nil {
		return result
	}
	scale := switchControl.theme.Metrics.ControlHeight / 32
	defaultWidth, defaultHeight := SwitchExtent(switchControl.theme, switchControl.size == SwitchSizeSmall)
	thumbSize := float32(16) * scale
	if switchControl.size == SwitchSizeSmall {
		thumbSize = 12 * scale
	}
	if switchControl.bounds.Width <= 0 {
		switchControl.bounds.Width = defaultWidth
	}
	if switchControl.bounds.Height <= 0 {
		switchControl.bounds.Height = defaultHeight
	}
	if switchControl.bounds.Height != defaultHeight {
		thumbSize = switchControl.bounds.Height - 2*scale
	}
	if thumbSize < 1 {
		thumbSize = 1
	}

	// checked is the destination; progress is how far the thumb has travelled
	// toward it. Without a frame clock progress is exactly 0 or 1.
	target := float32(0)
	if switchControl.checked {
		target = 1
	}
	progress := switchControl.canvas.Animate(switchControl.id, switchSlotChecked, target, switchControl.theme.Transition)

	look := switchRecipe(&switchControl.theme, gfx.State{
		FocusVisible: switchControl.focused, Invalid: switchControl.invalid, Disabled: switchControl.disabled,
	}, progress)
	if switchControl.recipe != nil {
		look = switchControl.recipe(switchControl.theme, gfx.State{
			FocusVisible: switchControl.focused, Invalid: switchControl.invalid, Disabled: switchControl.disabled,
		}, progress)
	}
	switchControl.canvas.Elevate(switchControl.bounds, switchControl.bounds.Height/2, look.Shadow) // shadow-xs
	if look.Bordered {
		switchControl.canvas.Rounded(expand(switchControl.bounds, look.RingWidth), switchControl.bounds.Height/2+look.RingWidth, look.Ring)
		paintBordered(switchControl.canvas, switchControl.theme, switchControl.bounds, switchControl.bounds.Height/2, look.Border, look.Track)
	} else {
		switchControl.canvas.Rounded(switchControl.bounds, switchControl.bounds.Height/2, look.Track)
	}
	thumb := look.Thumb

	// The thumb sits inside the track's 1px border: one pixel from the left edge
	// when off and one from the right when on (translate-x-[calc(100%-2px)]).
	inset := 1 * scale
	thumbX := gfx.Lerp(switchControl.bounds.X+inset, switchControl.bounds.X+switchControl.bounds.Width-thumbSize-inset, progress)
	thumbY := switchControl.bounds.Y + (switchControl.bounds.Height-thumbSize)/2
	switchControl.canvas.Rounded(gfx.Rect{
		X: thumbX, Y: thumbY, Width: thumbSize, Height: thumbSize,
	}, thumbSize/2, thumb)

	if !switchControl.id.IsZero() {
		hitBounds := gfx.Rect{
			X: switchControl.bounds.X - 12, Y: switchControl.bounds.Y - 8,
			Width: switchControl.bounds.Width + 24, Height: switchControl.bounds.Height + 16,
		}
		switchControl.canvas.HitWithOptions(switchControl.id, hitBounds, switchControl.label, gfx.HitOptions{
			Role: gfx.RoleSwitch, Disabled: switchControl.disabled,
			Checked: switchControl.checked,
		})
	}
	return result
}

// SwitchLook is the look of a switch in one state and at one point of its travel.
type SwitchLook struct {
	// Track and Thumb are the colors of the track and the knob at that point of the
	// travel; Border is the track's line, drawn only when Bordered.
	Track, Thumb, Border gfx.Color
	Bordered             bool
	// Ring is the halo of a focused or invalid switch, transparent when there is none;
	// RingWidth is how far it reaches beyond the track.
	Ring      gfx.Color
	RingWidth float32
	// Shadow is the elevation under the track.
	Shadow gfx.ShadowLevel
}

// SwitchRecipe resolves the look of a switch. progress is how far the thumb has travelled
// toward on, 0 to 1 (see gfx.Canvas.Animate), which the track and the thumb colors follow.
// FocusVisible, Invalid and Disabled of state are read; whether the switch is on is the
// progress, not state.Checked, so a switch that is changing is drawn between its looks.
func SwitchRecipe(theme gfx.Theme, state gfx.State, progress float32) SwitchLook {
	return switchRecipe(&theme, state, progress)
}

// switchRecipe is the recipe for a theme that is not to be copied: the controls paint every frame,
// and a Theme is large.
func switchRecipe(theme *gfx.Theme, state gfx.State, progress float32) SwitchLook {
	trackOff, trackOn := theme.Input, theme.Primary
	thumbOff, thumbOn := theme.Background, theme.Background
	if isDark(theme) {
		trackOff.Alpha *= 0.80
		thumbOff, thumbOn = theme.Foreground, theme.PrimaryForeground
	}
	look := SwitchLook{Track: trackOff.Mix(trackOn, progress), Thumb: thumbOff.Mix(thumbOn, progress), Shadow: gfx.ShadowXS}
	if state.FocusVisible || state.Invalid {
		look.Bordered = true
		look.Ring = scaledAlpha(theme.Ring, 0.50)
		look.Border = theme.Ring
		if state.Invalid {
			look.Ring, look.Border = invalidRing(theme), invalidBorder(theme)
		}
		look.RingWidth = 3
	}
	if state.Disabled {
		look.Track.Alpha *= 0.50
		look.Thumb.Alpha *= 0.50
		look.Border.Alpha *= 0.50
	}
	return look
}
