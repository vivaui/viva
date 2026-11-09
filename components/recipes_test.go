package components

import (
	"reflect"
	"testing"

	"github.com/vivaui/viva/gfx"
)

func recipeThemes() map[string]gfx.Theme {
	translucent := gfx.LightTheme()
	translucent.Background.Alpha = .1
	return map[string]gfx.Theme{"light": gfx.LightTheme(), "dark": gfx.DarkTheme(), "translucent": translucent}
}

// A recipe is the one definition of a look: painting the shapes it describes with the
// canvas calls a custom control makes gives what the component paints. This is what lets
// an application draw its own checkbox without copying the control's rules.
func TestCheckboxRecipePaintsAsTheComponentDoes(t *testing.T) {
	states := map[string]gfx.State{
		"off": {}, "on": {Checked: true}, "mixed": {Mixed: true}, "mixed and checked": {Mixed: true, Checked: true},
		"focus": {FocusVisible: true}, "focus on": {FocusVisible: true, Checked: true},
		"invalid": {Invalid: true}, "invalid on": {Invalid: true, Checked: true},
		"disabled": {Disabled: true}, "disabled on": {Disabled: true, Checked: true},
	}
	id := gfx.Item(1)
	for themeName, theme := range recipeThemes() {
		for name, state := range states {
			t.Run(themeName+"/"+name, func(t *testing.T) {
				bounds := gfx.Rect{X: 5, Y: 5, Width: CheckboxExtent(theme), Height: CheckboxExtent(theme)}
				var want gfx.Canvas
				box := BeginCheckbox(&want, theme, gfx.ID{}, bounds, state.Checked)
				if state.Mixed {
					box = box.Indeterminate()
				}
				if state.Invalid {
					box = box.Invalid()
				}
				if state.Disabled {
					box = box.Disabled()
				}
				if state.FocusVisible {
					box.focused = true
				}
				box.Paint()

				var got gfx.Canvas
				look := CheckboxRecipe(theme, state)
				got.Elevate(bounds, look.Radius, look.Shadow)
				if look.RingWidth > 0 {
					got.Rounded(expand(bounds, look.RingWidth), look.RingRadius, look.Ring)
				}
				paintBordered(&got, theme, bounds, look.Radius, look.Border, look.Fill)
				PaintCheckboxMark(&got, bounds, state, look.Mark)
				_ = id
				if !reflect.DeepEqual(want.Rectangles(), got.Rectangles()) || !reflect.DeepEqual(want.Shadows(), got.Shadows()) ||
					len(want.Triangles()) != len(got.Triangles()) {
					t.Errorf("recipe and component differ:\nwant %+v\n got %+v", want.Rectangles(), got.Rectangles())
				}
			})
		}
	}
}

// The recipe has no side effects and the same inputs give the same look.
func TestRecipesArePure(t *testing.T) {
	theme := gfx.DarkTheme()
	state := gfx.State{Checked: true, FocusVisible: true, Invalid: true}
	if CheckboxRecipe(theme, state) != CheckboxRecipe(theme, state) {
		t.Error("CheckboxRecipe is not deterministic")
	}
	if SwitchRecipe(theme, state, .5) != SwitchRecipe(theme, state, .5) {
		t.Error("SwitchRecipe is not deterministic")
	}
	if TextEntryRecipe(theme, state) != TextEntryRecipe(theme, state) {
		t.Error("TextEntryRecipe is not deterministic")
	}
}

// A checked box wins over the focus and invalid borders, as in the component; the halo still shows.
func TestCheckboxRecipeCheckedKeepsPrimaryBorderAndShowsTheHalo(t *testing.T) {
	theme := gfx.LightTheme()
	look := CheckboxRecipe(theme, gfx.State{Checked: true, FocusVisible: true, Invalid: true})
	if look.Border != theme.Primary || look.Fill != theme.Primary {
		t.Errorf("checked look = %+v, want the primary color", look)
	}
	if look.RingWidth <= 0 || look.Ring.Alpha <= 0 {
		t.Errorf("no halo on a focused invalid box: %+v", look)
	}
}

func TestSwitchRecipeFollowsTheTravelAndTheState(t *testing.T) {
	theme := gfx.LightTheme()
	off, on := SwitchRecipe(theme, gfx.State{}, 0), SwitchRecipe(theme, gfx.State{}, 1)
	if off.Track == on.Track || off.Bordered || on.RingWidth != 0 {
		t.Errorf("off %+v on %+v", off, on)
	}
	if got := SwitchRecipe(theme, gfx.State{Invalid: true}, 0); !got.Bordered || got.Border != invalidBorder(&theme) || got.RingWidth <= 0 {
		t.Errorf("invalid look = %+v", got)
	}
	if got := SwitchRecipe(theme, gfx.State{Disabled: true}, 1); got.Track.Alpha >= on.Track.Alpha {
		t.Errorf("a disabled track is not dimmed: %+v", got)
	}
}

func TestTextEntryRecipeStatesAndFlattening(t *testing.T) {
	light := gfx.LightTheme()
	rest, focus, invalid := TextEntryRecipe(light, gfx.State{}), TextEntryRecipe(light, gfx.State{Focused: true}), TextEntryRecipe(light, gfx.State{Invalid: true, Focused: true})
	if focus.Border == rest.Border || invalid.Border == focus.Border {
		t.Errorf("rest %+v focus %+v invalid %+v: the border does not follow the state", rest.Border, focus.Border, invalid.Border)
	}
	if rest.Fill.Alpha != 1 || rest.Border.Alpha != 1 {
		t.Errorf("an opaque theme's look is not flattened: %+v", rest)
	}
	// Over a translucent theme the fill keeps its transparency: flattened onto the page
	// background it would become an opaque slab that hides the material behind the control.
	if got := TextEntryRecipe(recipeThemes()["translucent"], gfx.State{}); got.Fill.Alpha >= 1 {
		t.Errorf("a translucent theme's fill was flattened: %+v", got)
	}
}

func BenchmarkRecipes(b *testing.B) {
	theme := gfx.LightTheme()
	state := gfx.State{Checked: true, FocusVisible: true}
	b.ReportAllocs()
	for range b.N {
		_ = CheckboxRecipe(theme, state)
		_ = SwitchRecipe(theme, state, .5)
		_ = TextEntryRecipe(theme, state)
	}
}

// A button of the application's own, painted from the recipe with the canvas calls a custom
// control makes, is the button the component paints: for every variant and state the surface
// is the same shapes in the same colors. This is what ButtonBase plus ButtonRecipe relies on.
func TestButtonRecipePaintsTheSurfaceAsTheComponentDoes(t *testing.T) {
	variants := map[string]ButtonVariant{
		"default": ButtonDefault, "destructive": ButtonDestructive, "outline": ButtonOutline, "secondary": ButtonSecondary,
		"ghost": ButtonGhost, "link": ButtonLink, "destructive outline": ButtonDestructiveOutline,
	}
	states := map[string]gfx.State{
		"rest": {}, "hover": {Hovered: true}, "pressed": {Hovered: true, Pressed: true},
		"focus": {FocusVisible: true}, "invalid": {Invalid: true}, "disabled": {Disabled: true},
		"disabled hover": {Disabled: true, Hovered: true},
	}
	for themeName, theme := range recipeThemes() {
		for variantName, variant := range variants {
			for stateName, state := range states {
				t.Run(themeName+"/"+variantName+"/"+stateName, func(t *testing.T) {
					bounds := gfx.Rect{X: 4, Y: 4, Width: 90, Height: theme.Metrics.ControlHeight}
					var want gfx.Canvas
					button := BeginButton(&want, theme, gfx.ID{}, bounds).WithState(ButtonState{
						Hovered: state.Hovered, Pressed: state.Pressed, Focused: state.FocusVisible, Invalid: state.Invalid,
					})
					if state.Disabled {
						button = button.Disabled()
					}
					button.paint("", variant)

					var got gfx.Canvas
					look := ButtonRecipe(theme, variant, state)
					if look.Shadow != gfx.ShadowNone {
						got.Elevate(bounds, look.Radius, look.Shadow)
					}
					if look.RingWidth > 0 {
						if look.RingOutline {
							got.RoundedOutline(expand(bounds, look.RingWidth), look.RingRadius, 2, look.Ring)
						} else {
							got.Rounded(expand(bounds, look.RingWidth), look.RingRadius, look.Ring)
						}
					}
					if look.Bordered && look.Filled {
						paintBordered(&got, theme, bounds, look.Radius, look.Border, look.Background)
					} else if look.Filled {
						got.Rounded(bounds, look.Radius, look.Background)
					}
					// The underline of a hovered link belongs to the label, which this test has none of.
					var surface []gfx.Rectangle
					for _, r := range want.Rectangles() {
						if r.Bounds.Width > 0 {
							surface = append(surface, r)
						}
					}
					if len(surface) == 0 && len(got.Rectangles()) == 0 {
						surface = got.Rectangles()
					}
					if !reflect.DeepEqual(surface, got.Rectangles()) || !reflect.DeepEqual(want.Shadows(), got.Shadows()) ||
						len(want.Triangles()) != len(got.Triangles()) {
						t.Errorf("recipe and component differ:\nwant %+v\n got %+v", surface, got.Rectangles())
					}
				})
			}
		}
	}
}

func TestButtonRecipeDisabledIgnoresThePointerAndDims(t *testing.T) {
	theme := gfx.LightTheme()
	rest, hovered := ButtonRecipe(theme, ButtonDefault, gfx.State{Disabled: true}), ButtonRecipe(theme, ButtonDefault, gfx.State{Disabled: true, Hovered: true, Pressed: true})
	if rest != hovered {
		t.Errorf("a disabled button reacts to the pointer: %+v vs %+v", rest, hovered)
	}
	if on := ButtonRecipe(theme, ButtonDefault, gfx.State{}); rest.Background.Alpha >= on.Background.Alpha || rest.Foreground.Alpha >= on.Foreground.Alpha {
		t.Errorf("a disabled button is not dimmed: %+v vs %+v", rest, on)
	}
}

func TestPopupRecipeIsTheSurfaceOfTheFloatingPanels(t *testing.T) {
	for name, theme := range recipeThemes() {
		look := PopupRecipe(theme)
		if look.Fill != theme.Popover || look.Border != theme.Border || look.Radius != theme.Metrics.RadiusPopup() || look.Shadow != gfx.ShadowMD {
			t.Errorf("%s: popup look = %+v", name, look)
		}
		// Painting through the recipe is what the components do: one panel, with its shadow.
		var canvas gfx.Canvas
		paintPopupSurface(&canvas, &theme, gfx.Rect{Width: 100, Height: 60}, gfx.ShadowMD)
		var by gfx.Canvas
		by.Elevate(gfx.Rect{Width: 100, Height: 60}, look.Radius, look.Shadow)
		by.RoundedBorder(gfx.Rect{Width: 100, Height: 60}, look.Radius, 1, look.Border, look.Fill)
		if !reflect.DeepEqual(canvas.Rectangles(), by.Rectangles()) || !reflect.DeepEqual(canvas.Shadows(), by.Shadows()) {
			t.Errorf("%s: paintPopupSurface differs from the recipe's shapes", name)
		}
	}
	var none gfx.Canvas
	light := gfx.LightTheme()
	paintPopupSurface(&none, &light, gfx.Rect{Width: 10, Height: 10}, gfx.ShadowNone)
	if len(none.Shadows()) != 0 {
		t.Errorf("a panel with no elevation cast a shadow: %+v", none.Shadows())
	}
}

func TestMenuItemRecipeHighlightsWhatIsActiveAndNeverWhatIsDisabled(t *testing.T) {
	theme := gfx.LightTheme()
	for name, state := range map[string]gfx.State{
		"hover": {Hovered: true}, "pressed": {Pressed: true}, "keyboard": {FocusVisible: true},
	} {
		if got := MenuItemRecipe(theme, state); got.Fill != theme.Accent || got.Foreground != theme.PopoverForeground {
			t.Errorf("%s: look = %+v, want the accent highlight", name, got)
		}
	}
	// Focus by a click is not a keyboard highlight, and a row at rest has none.
	if got := MenuItemRecipe(theme, gfx.State{Focused: true}); got.Fill.Alpha != 0 {
		t.Errorf("a pointer-focused row is highlighted: %+v", got)
	}
	if got := MenuItemRecipe(theme, gfx.State{}); got.Fill.Alpha != 0 || got.Radius != theme.Metrics.RadiusItem() {
		t.Errorf("a row at rest = %+v", got)
	}
	off := MenuItemRecipe(theme, gfx.State{Disabled: true, Hovered: true, FocusVisible: true})
	if off.Fill.Alpha != 0 || off.Foreground.Alpha >= theme.PopoverForeground.Alpha {
		t.Errorf("a disabled row is highlighted or not dimmed: %+v", off)
	}
}

func TestSelectRecipeFollowsTheTriggerState(t *testing.T) {
	theme := gfx.LightTheme()
	rest, focus := SelectRecipe(theme, gfx.State{}, false), SelectRecipe(theme, gfx.State{Focused: true}, false)
	if rest.Surface.Border == focus.Surface.Border {
		t.Error("the trigger's border does not follow focus")
	}
	if rest.Foreground != theme.Foreground || SelectRecipe(theme, gfx.State{}, true).Foreground != theme.MutedForeground {
		t.Error("the placeholder is not muted, or the value is")
	}
	if off := SelectRecipe(theme, gfx.State{Disabled: true}, false); off.Foreground.Alpha >= rest.Foreground.Alpha || off.Chevron.Alpha >= rest.Chevron.Alpha {
		t.Errorf("a disabled trigger is not dimmed: %+v", off)
	}
}

// An unpainted select is the same select to the pointer, the keyboard and assistive
// technology: it declares the very hits, the focus scope and the escape target, in the same
// places, and paints nothing. It runs the same code, so the two cannot drift apart.
func TestUnpaintedSelectDeclaresWhatThePaintedOneDoesAndPaintsNothing(t *testing.T) {
	theme := gfx.LightTheme()
	run := func(unpainted bool) *gfx.Canvas {
		var canvas gfx.Canvas
		options := SelectOptions{
			TriggerID: gfx.Item(1), Label: "Density", Value: "Comfortable", Open: true,
			Content: gfx.Rect{X: 10, Y: 48, Width: 120, Height: 160}, Layer: 9, Scope: gfx.Item(31), DismissID: gfx.Item(7),
			Viewport: gfx.Rect{Width: 400, Height: 300}, Unpainted: unpainted,
		}
		control := BeginSelect(&canvas, theme, gfx.Rect{X: 10, Y: 12, Width: 160}, options)
		control.AddLabel("Density")
		control.AddItem(SelectItem{ID: gfx.Item(2), Label: "Comfortable", Value: "comfortable", Selected: true})
		control.AddSeparator()
		control.AddItem(SelectItem{ID: gfx.Item(3), Label: "Compact", Value: "compact", Disabled: true})
		control.AddScrollDown(gfx.Item(4), false)
		control.End()
		return &canvas
	}
	painted, bare := run(false), run(true)
	if len(bare.Rectangles()) != 0 || len(bare.Texts()) != 0 || len(bare.Triangles()) != 0 || len(bare.Shadows()) != 0 {
		t.Fatalf("an unpainted select painted: %d rectangles, %d texts, %d triangles, %d shadows",
			len(bare.Rectangles()), len(bare.Texts()), len(bare.Triangles()), len(bare.Shadows()))
	}
	if len(painted.Rectangles()) == 0 {
		t.Fatal("the painted select painted nothing: the comparison means nothing")
	}
	if !reflect.DeepEqual(painted.Hits(), bare.Hits()) {
		t.Errorf("hits differ:\npainted %+v\nbare    %+v", painted.Hits(), bare.Hits())
	}
	if painted.TrappedFocusScope() != bare.TrappedFocusScope() || painted.EscapeTarget() != bare.EscapeTarget() {
		t.Errorf("focus scope %v/%v, escape %v/%v", painted.TrappedFocusScope(), bare.TrappedFocusScope(), painted.EscapeTarget(), bare.EscapeTarget())
	}
}

func TestTabTriggerRecipeSelectedDefaultIsARaisedPillAndTheLineVariantIsAnUnderline(t *testing.T) {
	for name, theme := range recipeThemes() {
		pill := TabTriggerRecipe(theme, TabsDefault, gfx.State{Selected: true})
		if !pill.Bordered || pill.Shadow != gfx.ShadowSM || pill.Indicator.Alpha != 0 {
			t.Errorf("%s: selected default tab = %+v, want a raised pill with no underline", name, pill)
		}
		rest := TabTriggerRecipe(theme, TabsDefault, gfx.State{})
		if rest.Bordered || rest.Shadow != gfx.ShadowNone || rest.Fill.Alpha != 0 {
			t.Errorf("%s: resting default tab = %+v, want a transparent trigger", name, rest)
		}
		line := TabTriggerRecipe(theme, TabsLine, gfx.State{Selected: true})
		if line.Bordered || line.Indicator.Alpha != theme.Foreground.Alpha {
			t.Errorf("%s: selected line tab = %+v, want an underline and no pill", name, line)
		}
		if rest.Foreground.Alpha >= theme.Foreground.Alpha || TabTriggerRecipe(theme, TabsDefault, gfx.State{Hovered: true}).Foreground.Alpha != theme.Foreground.Alpha {
			t.Errorf("%s: an unselected tab is not muted until it is hovered", name)
		}
	}
}

func TestTabTriggerRecipeDisabledIgnoresThePointerAndTheKeyboardAndDims(t *testing.T) {
	theme := gfx.LightTheme()
	off := TabTriggerRecipe(theme, TabsDefault, gfx.State{Disabled: true, Hovered: true, FocusVisible: true, Selected: true})
	on := TabTriggerRecipe(theme, TabsDefault, gfx.State{Selected: true})
	if off.RingWidth != 0 || off.Foreground.Alpha >= on.Foreground.Alpha || off.Fill.Alpha >= on.Fill.Alpha {
		t.Errorf("a disabled tab = %+v, want no halo and a dimmed pill", off)
	}
	if got := TabTriggerRecipe(theme, TabsDefault, gfx.State{FocusVisible: true}); got.RingWidth <= 0 || got.RingRadius <= got.Radius {
		t.Errorf("a keyboard-focused tab has no halo: %+v", got)
	}
}

func TestSegmentRecipeRaisesTheSelectedChoiceAndRingsTheFocusedOne(t *testing.T) {
	for name, theme := range recipeThemes() {
		rest := SegmentRecipe(theme, gfx.State{}, false)
		selected := SegmentRecipe(theme, gfx.State{Selected: true}, false)
		if selected.Fill == rest.Fill && theme.Background.Alpha >= 1 {
			t.Errorf("%s: the selected choice is not told apart from a resting one", name)
		}
		if rest.Foreground != theme.MutedForeground || selected.Foreground != theme.Foreground {
			t.Errorf("%s: labels are not muted until the choice is selected: rest %+v selected %+v", name, rest.Foreground, selected.Foreground)
		}
		if SegmentRecipe(theme, gfx.State{Hovered: true}, false).Foreground != theme.Foreground {
			t.Errorf("%s: a hovered choice's label is not raised", name)
		}
		focus := SegmentRecipe(theme, gfx.State{FocusVisible: true}, false)
		if focus.RingWidth <= 0 || focus.Border != theme.Ring || focus.RingOutline != (theme.Background.Alpha < 1) {
			t.Errorf("%s: focused choice = %+v", name, focus)
		}
		off := SegmentRecipe(theme, gfx.State{Disabled: true, FocusVisible: true, Selected: true}, false)
		if off.RingWidth != 0 || off.Fill != theme.Muted || off.Foreground != theme.MutedForeground {
			t.Errorf("%s: a disabled selected choice = %+v", name, off)
		}
	}
	// A disabled control rests its choices on a different track.
	if on, off := SegmentRecipe(gfx.LightTheme(), gfx.State{}, false), SegmentRecipe(gfx.LightTheme(), gfx.State{}, true); on.Fill == off.Fill {
		t.Error("a disabled control's track is the same as an enabled one's")
	}
}

// The panels differ in their corners and their shadow, by design, and each has its own recipe:
// a dialog stands on a scrim and casts none, a popover and a list float and do.
func TestPanelRecipesDifferInCornersAndShadowAndShareTheFill(t *testing.T) {
	for name, theme := range recipeThemes() {
		dialog, popover, popup := DialogRecipe(theme), PopoverRecipe(theme), PopupRecipe(theme)
		for _, look := range []PopupLook{dialog, popover, popup} {
			if look.Fill != theme.Popover || look.Border != theme.Border {
				t.Errorf("%s: panel colors = %+v, want the popover fill and the border", name, look)
			}
		}
		if dialog.Radius != theme.Metrics.RadiusSurface || dialog.Shadow != gfx.ShadowNone {
			t.Errorf("%s: dialog = %+v, want the surface radius and no shadow", name, dialog)
		}
		if popover.Radius != theme.Metrics.RadiusControl || popover.Shadow != gfx.ShadowMD {
			t.Errorf("%s: popover = %+v, want the control radius and shadow-md", name, popover)
		}
		if popup.Radius != theme.Metrics.RadiusPopup() || popup.Shadow != gfx.ShadowMD {
			t.Errorf("%s: popup = %+v, want the popup radius and shadow-md", name, popup)
		}
	}
}

// A dialog is painted from its recipe: one solid panel and no shadow. A popover casts its shadow.
func TestDialogAndPopoverPaintTheirPanelsFromTheirRecipes(t *testing.T) {
	theme := gfx.LightTheme()
	bounds := gfx.Rect{X: 20, Y: 20, Width: 200, Height: 120}
	var dialog, popover gfx.Canvas
	BeginDialog(&dialog, theme, gfx.Rect{Width: 400, Height: 300}, DialogOptions{DismissID: gfx.Item(1), CloseID: gfx.Item(2), Scope: gfx.Item(3), ContentSize: gfx.Size{Width: 300, Height: 200}}).Paint("Title", "Body")
	BeginPopover(&popover, theme, gfx.Rect{Width: 400, Height: 300}, bounds, PopoverOptions{DismissID: gfx.Item(1), Scope: gfx.Item(3), ContentSize: gfx.Size{Width: 200, Height: 100}}).Paint("Title", "Body")
	if len(dialog.Shadows()) != 0 {
		t.Errorf("a dialog cast a shadow: %+v", dialog.Shadows())
	}
	if len(popover.Shadows()) == 0 {
		t.Error("a popover cast no shadow")
	}
}
