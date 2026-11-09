package components

import (
	"github.com/vivaui/viva/gfx"
)

// TabsVariant selects a contained list or an indicator-only list.
type TabsVariant uint8

const (
	TabsDefault TabsVariant = iota
	TabsLine
)

// TabsOrientation selects a row or column list.
type TabsOrientation uint8

const (
	TabsHorizontal TabsOrientation = iota
	TabsVertical
)

// TabOption is one stable tab trigger.
type TabOption struct {
	ID       gfx.ID
	Label    string
	Disabled bool
	// Icon is an optional leading icon. Icon and label form one centered group.
	Icon gfx.Icon
	// IconOnly hides the label while keeping it as the tab's accessible name.
	// Set Label even when only the icon is painted.
	IconOnly bool
}

// Tabs paints a compact Base UI-style tab list. The active tab and associated
// panel remain application state. Canvas activation supports Enter and Space.
type Tabs struct {
	// list and labels, when set, replace options: the tabs are the items of the
	// list scope, numbered by position, and selected is an item number.
	list      gfx.Scope
	labels    []string
	canvas    *gfx.Canvas
	theme     gfx.Theme
	bounds    gfx.Rect
	options   []TabOption
	selected  gfx.ID
	variant   TabsVariant
	direction TabsOrientation
	disabled  bool
	fit       bool
	// tabStop is the one tab Tab reaches; the arrow helper moves focus among the
	// rest, which are SkipTab.
	tabStop gfx.ID
}

// Line selects the transparent list with a two-pixel active indicator.
func (tabs Tabs) Line() Tabs {
	tabs.variant = TabsLine
	return tabs
}

// Fit sizes the list to its labels like TabsList's w-fit: every trigger is as wide
// as its label plus px-2 and the transparent border, so tabs may differ in
// width, and the list is the sum of them plus its padding. bounds.Width is
// ignored (it may be zero); Paint reports the resulting bounds. Without Fit the
// triggers split bounds.Width equally.
func (tabs Tabs) Fit() Tabs {
	tabs.fit = true
	return tabs
}

// Vertical lays triggers out in a column and places the line indicator at the
// trailing edge.
func (tabs Tabs) Vertical() Tabs {
	tabs.direction = TabsVertical
	return tabs
}

// BeginTabs starts a tab-list declaration.
func BeginTabs(canvas *gfx.Canvas, theme gfx.Theme, bounds gfx.Rect, options []TabOption, selected gfx.ID) Tabs {
	tabs := Tabs{canvas: canvas, theme: theme, bounds: bounds, options: options, selected: selected}
	tabs.tabStop = tabs.rovingTabStop()
	return tabs
}

// BeginTabList starts a tab list whose tabs are the items of list, one per label
// in order, so the component numbers its own triggers and the caller keeps an
// index: selected is the number of the active tab, or -1. Paint reports the tab
// that was clicked:
//
//	if index, ok := components.BeginTabList(c, theme, bounds, tabs, labels, current).Paint().Clicked(); ok {
//		current = index
//	}
func BeginTabList(canvas *gfx.Canvas, theme gfx.Theme, bounds gfx.Rect, list gfx.Scope, labels []string, selected int) Tabs {
	tabs := Tabs{canvas: canvas, theme: theme, bounds: bounds, list: list, labels: labels, selected: list.At(selected)}
	tabs.tabStop = tabs.rovingTabStop()
	return tabs
}

// rovingTabStop keeps one tab in the Tab order: the focused one, else the
// selected one, else the first enabled one.
func (tabs Tabs) rovingTabStop() gfx.ID {
	enabled := func(option TabOption) bool {
		return !tabs.disabled && !option.Disabled && !option.ID.IsZero()
	}
	focus := gfx.ID{}
	if tabs.canvas != nil {
		focus = tabs.canvas.FocusVisible()
	}
	for index := 0; index < tabs.count(); index++ {
		if option := tabs.option(index); option.ID == focus && enabled(option) {
			return focus
		}
	}
	for index := 0; index < tabs.count(); index++ {
		if option := tabs.option(index); option.ID == tabs.selected && enabled(option) {
			return tabs.selected
		}
	}
	for index := 0; index < tabs.count(); index++ {
		if option := tabs.option(index); enabled(option) {
			return option.ID
		}
	}
	return gfx.ID{}
}

// count is how many tabs there are, and option the nth of them.
func (tabs Tabs) count() int {
	if !tabs.list.IsZero() {
		return len(tabs.labels)
	}
	return len(tabs.options)
}

func (tabs Tabs) option(index int) TabOption {
	if !tabs.list.IsZero() {
		return TabOption{ID: tabs.list.At(index), Label: tabs.labels[index]}
	}
	return tabs.options[index]
}

// TabsResult is what painting a tab list reports back.
type TabsResult struct {
	canvas *gfx.Canvas
	list   gfx.Scope
	bounds gfx.Rect
}

// Bounds is the rectangle the list occupied: bounds as given, or, with Fit, the
// width the labels asked for. It is empty when nothing was painted.
func (result TabsResult) Bounds() gfx.Rect { return result.bounds }

// Clicked returns the number of the tab clicked in this paint's event pass, for a
// list made with BeginTabList.
func (result TabsResult) Clicked() (int, bool) {
	if result.canvas == nil || result.list.IsZero() {
		return 0, false
	}
	return result.canvas.ClickedIn(result.list)
}

// Disabled renders every tab as inert.
func (tabs Tabs) Disabled() Tabs {
	tabs.disabled = true
	return tabs
}

// TabsKeyDown applies orientation-aware roving focus to current. Arrow keys on
// the tab list's layout axis wrap through enabled non-zero options; Home and
// End choose the first and last. Horizontal Left and Right reverse in RTL.
func TabsKeyDown(current gfx.ID, options []TabOption, orientation TabsOrientation, direction Direction, key gfx.Key) (gfx.ID, bool) {
	step := 0
	switch key {
	case gfx.KeyLeft:
		if orientation != TabsHorizontal {
			return current, false
		}
		step = -1
		if direction.IsRTL() {
			step = 1
		}
	case gfx.KeyRight:
		if orientation != TabsHorizontal {
			return current, false
		}
		step = 1
		if direction.IsRTL() {
			step = -1
		}
	case gfx.KeyUp:
		if orientation != TabsVertical {
			return current, false
		}
		step = -1
	case gfx.KeyDown:
		if orientation != TabsVertical {
			return current, false
		}
		step = 1
	case gfx.KeyHome:
		return firstEnabledTab(current, options, 1), true
	case gfx.KeyEnd:
		return firstEnabledTab(current, options, -1), true
	default:
		return current, false
	}
	return nextEnabledTab(current, options, step), true
}

func firstEnabledTab(current gfx.ID, options []TabOption, step int) gfx.ID {
	for index := 0; index < len(options); index++ {
		candidate := index
		if step < 0 {
			candidate = len(options) - 1 - index
		}
		option := options[candidate]
		if !option.ID.IsZero() && !option.Disabled {
			return option.ID
		}
	}
	return current
}

func nextEnabledTab(current gfx.ID, options []TabOption, step int) gfx.ID {
	if len(options) == 0 {
		return current
	}
	currentIndex := -1
	for index := range options {
		if options[index].ID == current {
			currentIndex = index
			break
		}
	}
	if currentIndex < 0 {
		return firstEnabledTab(current, options, step)
	}
	for offset := 1; offset <= len(options); offset++ {
		index := (currentIndex + step*offset) % len(options)
		if index < 0 {
			index += len(options)
		}
		option := options[index]
		if !option.ID.IsZero() && !option.Disabled {
			return option.ID
		}
	}
	return current
}

const (
	tabsListHeight    = 36
	tabsPadding       = 3  // p-[3px]
	tabsTriggerPad    = 9  // px-2 plus the 1px transparent border
	tabsLineGap       = 4  // gap-1 between line-variant triggers
	tabsIndicator     = 2  // after:h-0.5 / after:w-0.5
	tabsLineDrop      = 3  // after:bottom-[-5px] below the trigger, minus the indicator
	tabsLineRight     = 2  // after:-right-1 beyond a vertical trigger, minus the indicator
	tabsTriggerHeight = 30 // list height less its padding
)

// Paint draws the list and independently focusable tab triggers.
func (tabs Tabs) Paint() TabsResult {
	result := TabsResult{canvas: tabs.canvas, list: tabs.list}
	if tabs.canvas == nil || tabs.count() == 0 {
		return result
	}
	metrics := tabs.theme.Metrics
	weight := gfx.WeightMedium
	gap := tabs.itemGap()
	count := float32(tabs.count())
	if tabs.fit {
		var widest, total float32
		for index := range tabs.count() {
			width := tabs.contentWidth(tabs.option(index)) + tabsTriggerPad*2
			widest = max(widest, width)
			total += width
		}
		if tabs.direction == TabsVertical {
			tabs.bounds.Width = widest + tabsPadding*2
		} else {
			tabs.bounds.Width = total + gap*(count-1) + tabsPadding*2
		}
	}
	if tabs.bounds.Width <= 0 {
		return result
	}
	if tabs.bounds.Height <= 0 {
		if tabs.direction == TabsVertical {
			tabs.bounds.Height = tabsTriggerHeight*count + gap*(count-1) + tabsPadding*2
		} else {
			tabs.bounds.Height = tabsListHeight
		}
	}
	result.bounds = tabs.bounds
	radius := metrics.RadiusControl
	listFill := tabs.theme.Muted
	listRadius := radius + 2
	if tabs.variant == TabsLine {
		listFill = tabs.theme.Background
		listFill.Alpha = 0
		listRadius = 0
	}
	tabs.canvas.Rounded(tabs.bounds, listRadius, listFill)
	inner := gfx.Rect{
		X: tabs.bounds.X + tabsPadding, Y: tabs.bounds.Y + tabsPadding,
		Width: tabs.bounds.Width - tabsPadding*2, Height: tabs.bounds.Height - tabsPadding*2,
	}
	cursor := float32(0)
	for index := range tabs.count() {
		option := tabs.option(index)
		trigger := tabs.triggerBounds(inner, index, count, cursor)
		if tabs.fit && tabs.direction != TabsVertical {
			cursor += trigger.Width + gap
		}
		dimmed := tabs.disabled || option.Disabled
		state := gfx.State{Disabled: dimmed, Selected: !option.ID.IsZero() && option.ID == tabs.selected}
		if !dimmed && !option.ID.IsZero() {
			state.Hovered = tabs.canvas.Hovered() == option.ID
			state.FocusVisible = tabs.canvas.FocusVisible() == option.ID
		}
		look := tabTriggerRecipe(&tabs.theme, tabs.variant, state)
		if look.RingWidth > 0 {
			tabs.canvas.Rounded(expand(trigger, look.RingWidth), look.RingRadius, look.Ring)
		}
		if look.Bordered {
			tabs.canvas.Elevate(trigger, radius, look.Shadow) // the selected trigger's shadow-sm
			tabs.canvas.RoundedBorder(trigger, radius, 1, look.Border, look.Fill)
		} else {
			tabs.canvas.Rounded(trigger, radius, look.Fill)
		}

		indicator := look.Indicator
		indicatorBounds := gfx.Rect{X: trigger.X, Y: trigger.Y + trigger.Height + tabsLineDrop, Width: trigger.Width, Height: tabsIndicator}
		if tabs.direction == TabsVertical {
			indicatorBounds = gfx.Rect{X: trigger.X + trigger.Width + tabsLineRight, Y: trigger.Y, Width: tabsIndicator, Height: trigger.Height}
		}
		tabs.canvas.Rounded(indicatorBounds, 1, indicator)

		style := gfx.TextStyle{Size: metrics.FontSizeM, Weight: weight, Color: look.Foreground}
		tabs.paintContent(option, trigger, style)
		if !option.ID.IsZero() {
			hint := ""
			if option.IconOnly {
				hint = option.Label
			}
			tabs.canvas.HitWithOptions(option.ID, trigger, option.Label, gfx.HitOptions{
				Role: gfx.RoleTab, Value: option.Label, Hint: hint,
				Disabled: dimmed, FocusableWhenDisabled: true,
				Selected: state.Selected,
				SkipTab:  option.ID != tabs.tabStop,
			})
		}
	}
	return result
}

// triggerBounds returns the index-th trigger inside the list's padded interior.
// cursor is the distance already used along the row in Fit mode.
func (tabs Tabs) triggerBounds(inner gfx.Rect, index int, count, cursor float32) gfx.Rect {
	gap := tabs.itemGap()
	if tabs.direction == TabsVertical {
		height := (inner.Height - gap*(count-1)) / count
		return gfx.Rect{X: inner.X, Y: inner.Y + float32(index)*(height+gap), Width: inner.Width, Height: height}
	}
	if tabs.fit {
		width := tabs.contentWidth(tabs.option(index)) + tabsTriggerPad*2
		return gfx.Rect{X: inner.X + cursor, Y: inner.Y, Width: width, Height: inner.Height}
	}
	width := (inner.Width - gap*(count-1)) / count
	return gfx.Rect{X: inner.X + float32(index)*(width+gap), Y: inner.Y, Width: width, Height: inner.Height}
}

func (tabs Tabs) itemGap() float32 {
	if tabs.variant == TabsLine {
		return tabsLineGap
	}
	return 0
}

func (tabs Tabs) contentWidth(option TabOption) float32 {
	width := float32(0)
	if !option.IconOnly {
		width = tabs.canvas.TextWidthWeight(option.Label, tabs.theme.Metrics.FontSizeM, gfx.WeightMedium)
	}
	if len(option.Icon.Commands) > 0 || len(option.Icon.Layers) > 0 {
		width += tabs.theme.Metrics.FontSizeM
		if option.Label != "" && !option.IconOnly {
			width += 6
		}
	}
	return width
}

func (tabs Tabs) paintContent(option TabOption, trigger gfx.Rect, style gfx.TextStyle) {
	b := gfx.Rect{X: trigger.X + tabsTriggerPad, Y: trigger.Y, Width: max(0, trigger.Width-tabsTriggerPad*2), Height: trigger.Height}
	tabs.canvas.Clip(trigger)
	defer tabs.canvas.Unclip()
	if len(option.Icon.Commands) == 0 && len(option.Icon.Layers) == 0 {
		if tabs.direction == TabsVertical {
			tabs.canvas.Label(option.Label, b, style)
		} else {
			tabs.canvas.LabelCenter(option.Label, trigger, style)
		}
		return
	}
	size := min(tabs.theme.Metrics.FontSizeM, max(0, b.Height))
	gap := float32(0)
	if option.Label != "" && !option.IconOnly {
		gap = 6
	}
	text := option.Label
	if option.IconOnly {
		text = ""
	}
	width := min(tabs.canvas.TextWidthWeight(text, style.Size, style.Weight)+size+gap, b.Width)
	if tabs.direction != TabsVertical {
		b.X += (b.Width - width) / 2
	}
	tabs.canvas.DrawIcon(option.Icon, gfx.Rect{X: b.X, Y: b.Y + (b.Height-size)/2, Width: size, Height: size}, style.Color)
	if text == "" {
		return
	}
	b.X += size + gap
	b.Width = max(0, width-size-gap)
	tabs.canvas.Label(text, b, style)
}

// TabTriggerLook is the look of one tab of a strip in one state.
type TabTriggerLook struct {
	// Fill is the trigger's background, transparent unless the tab is the selected one of
	// the default variant. Border is its 1px line, painted only when Bordered, which is the
	// raised pill of that tab; it casts Shadow.
	Fill, Border gfx.Color
	Bordered     bool
	Shadow       gfx.ShadowLevel
	// Foreground is the color of the label and icon: dimmed while the tab is neither selected
	// nor under the pointer.
	Foreground gfx.Color
	// Indicator is the underline of the line variant's selected tab, transparent otherwise.
	Indicator gfx.Color
	// Radius is the corner radius of the trigger.
	Radius float32
	// Ring is the halo of the tab the keyboard is on, transparent with a zero RingWidth when
	// there is none; RingRadius is its outer radius.
	Ring                  gfx.Color
	RingWidth, RingRadius float32
}

// TabTriggerRecipe resolves the look of one tab. Selected, Hovered, FocusVisible and Disabled
// of state are read; a Disabled tab ignores the pointer and the keyboard and is dimmed. A pure
// function, the one definition the strip is painted from.
func TabTriggerRecipe(theme gfx.Theme, variant TabsVariant, state gfx.State) TabTriggerLook {
	return tabTriggerRecipe(&theme, variant, state)
}

// tabTriggerRecipe is the recipe for a theme that is not to be copied: the controls paint every frame,
// and a Theme is large.
func tabTriggerRecipe(theme *gfx.Theme, variant TabsVariant, state gfx.State) TabTriggerLook {
	radius := theme.Metrics.RadiusControl
	look := TabTriggerLook{Radius: radius}
	look.Fill = theme.Background
	look.Fill.Alpha = 0
	hovered := !state.Disabled && state.Hovered
	if !state.Disabled && state.FocusVisible {
		look.Ring = theme.Ring
		look.Ring.Alpha *= 0.50
		look.RingWidth, look.RingRadius = 3, radius+3
	}
	if state.Selected && variant == TabsDefault {
		fill := theme.Background
		// A transparent 1px border around the fill. In a dark theme it is dark:border-input
		// over dark:bg-input/30 on the list; otherwise it is the fill's own color, so both
		// themes lay out the same rectangles.
		border := fill
		if segmentedControlLuminance(theme.Background) < segmentedControlLuminance(theme.Foreground) {
			input := theme.Input
			border = segmentedControlBlend(theme.Muted, input, input.Alpha)
			fill = segmentedControlBlend(theme.Muted, input, input.Alpha*0.30)
		}
		if state.Disabled {
			border.Alpha *= 0.50
			fill.Alpha *= 0.50
		}
		look.Fill, look.Border, look.Bordered, look.Shadow = fill, border, true, gfx.ShadowSM
	} else if state.Disabled {
		look.Fill.Alpha *= 0.50
	}
	look.Indicator = theme.Foreground
	if !state.Selected || variant != TabsLine {
		look.Indicator.Alpha = 0
	} else if state.Disabled {
		look.Indicator.Alpha *= 0.50
	}
	look.Foreground = theme.Foreground
	if !state.Selected && !hovered {
		look.Foreground.Alpha *= 0.60
	}
	if state.Disabled {
		look.Foreground.Alpha *= 0.50
	}
	return look
}
