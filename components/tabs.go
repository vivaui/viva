package components

import (
	"github.com/vivaui/viva/base"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/style"
)

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
	style     TabsStyle
	// unpaintedSurface is set when the caller's engine paints the strip itself,
	// as a page's does; BeginTabs users leave it off and get the strip painted.
	unpaintedSurface bool
	// tabStop is the one tab Tab reaches; the arrow helper moves focus among the
	// rest, which are SkipTab.
	tabStop gfx.ID
	// The strip's effective style, filled from TabStyle's zero fields at Paint.
	paint tabsPaint
}

// TabsStyle overrides what a tab strip paints and how its triggers are laid
// out. Every field is optional: a zero value keeps the size's default, so this
// is how the page passes Pad, Gap, Radius, FontSize, Weight, Color, Background
// and Border of the element down to the component.
type TabsStyle struct {
	// ListHeight and ListPadding size the strip; TriggerHeight and TriggerPadding
	// size one trigger inside it. LineGap is the gap between line-variant triggers;
	// Gap is used for both variants when set. ListPadding is the room on each of the
	// strip's four sides; HasListPadding distinguishes an explicit zero from the
	// theme default.
	ListHeight     float32
	ListPadding    gfx.Insets
	HasListPadding bool
	TriggerHeight  float32
	TriggerPadding float32
	LineGap, Gap   float32
	// TriggerRadius is a trigger's corner radius; the strip's own radius is the
	// element's Radius, painted by the engine.
	TriggerRadius float32
	// FontSize and Weight style a trigger's label and icon.
	FontSize, Weight float32
	// Fill paints the list's surface when HasFill is set; Border draws its line
	// when HasBorder is set. Foreground replaces the label color when HasForeground
	// is set, keeping the selected, hovered and disabled dimming.
	Fill, Border, Foreground gfx.Color
	HasFill, HasBorder       bool
	HasForeground            bool
}

// tabsPaint is TabsStyle resolved against the theme at Paint.
type tabsPaint struct {
	listHeight, triggerHeight, triggerPadding, lineGap, gap float32
	listPadding                                             gfx.Insets
	listRadius, triggerRadius, fontSize, weight             float32
	foreground                                              gfx.Color
	hasForeground                                           bool
}

// WithStyle overrides the strip's measurements, colors and label style; zero
// fields keep the size's defaults.
func (tabs Tabs) WithStyle(style TabsStyle) Tabs {
	tabs.style = style
	return tabs
}

// UnpaintedSurface declares that the caller paints the strip's own surface
// behind the triggers, as the page's engine does; Paint then draws only the
// triggers and their indicator.
func (tabs Tabs) UnpaintedSurface() Tabs {
	tabs.unpaintedSurface = true
	return tabs
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

// resolvedPaint is the strip's style with the theme defaults filled in and the
// caller's overrides applied.
func (tabs Tabs) resolvedPaint() tabsPaint {
	metrics := tabs.theme.Metrics
	defHeight, defPadding, defTriggerHeight, defTriggerPad, defLineGap, defFont, defWeight := style.TabsGeometry(metrics)
	p := tabsPaint{
		listHeight: defHeight, listPadding: gfx.All(defPadding),
		triggerHeight: defTriggerHeight, triggerPadding: defTriggerPad,
		lineGap: defLineGap, gap: 0,
		listRadius: style.TabsListRadius(metrics), triggerRadius: style.TabsTriggerRadius(metrics),
		fontSize: defFont, weight: defWeight,
	}
	override := tabs.style
	if override.ListHeight > 0 {
		p.listHeight = override.ListHeight
	}
	if override.HasListPadding {
		p.listPadding = override.ListPadding
	}
	if override.TriggerHeight > 0 {
		p.triggerHeight = override.TriggerHeight
	}
	if override.TriggerPadding > 0 {
		p.triggerPadding = override.TriggerPadding
	}
	if override.LineGap > 0 {
		p.lineGap = override.LineGap
	}
	if override.Gap > 0 {
		p.gap = override.Gap
	}
	if override.TriggerRadius > 0 {
		p.triggerRadius = override.TriggerRadius
	}
	if override.FontSize > 0 {
		p.fontSize = override.FontSize
	}
	if override.Weight > 0 {
		p.weight = override.Weight
	}
	if override.HasForeground {
		p.foreground, p.hasForeground = override.Foreground, true
	}
	return p
}

// Paint draws the list and independently focusable tab triggers.
func (tabs Tabs) Paint() TabsResult {
	result := TabsResult{canvas: tabs.canvas, list: tabs.list}
	if tabs.canvas == nil || tabs.count() == 0 {
		return result
	}
	tabs.paint = tabs.resolvedPaint()
	weight := tabs.paint.weight
	gap := tabs.itemGap()
	count := float32(tabs.count())
	if tabs.fit {
		var widest, total float32
		for index := range tabs.count() {
			width := tabs.contentWidth(tabs.option(index)) + tabs.paint.triggerPadding*2
			widest = max(widest, width)
			total += width
		}
		pad := tabs.paint.listPadding
		if tabs.direction == TabsVertical {
			tabs.bounds.Width = widest + pad.Left + pad.Right
		} else {
			tabs.bounds.Width = total + gap*(count-1) + pad.Left + pad.Right
		}
	}
	if tabs.bounds.Width <= 0 {
		return result
	}
	if tabs.bounds.Height <= 0 {
		pad := tabs.paint.listPadding
		if tabs.direction == TabsVertical {
			tabs.bounds.Height = tabs.paint.triggerHeight*count + gap*(count-1) + pad.Top + pad.Bottom
		} else if tabs.paint.listHeight > 0 {
			tabs.bounds.Height = tabs.paint.listHeight
		} else {
			tabs.bounds.Height = tabs.paint.triggerHeight + pad.Top + pad.Bottom
		}
	}
	result.bounds = tabs.bounds
	triggerRadius := tabs.paint.triggerRadius
	if !tabs.unpaintedSurface {
		listFill := tabs.theme.Muted
		listRadius := tabs.paint.listRadius
		if tabs.variant == TabsLine {
			listFill = tabs.theme.Background
			listFill.Alpha = 0
			listRadius = 0
		}
		tabs.canvas.Rounded(tabs.bounds, listRadius, listFill)
	}
	pad := tabs.paint.listPadding
	inner := gfx.Rect{
		X: tabs.bounds.X + pad.Left, Y: tabs.bounds.Y + pad.Top,
		Width: max(tabs.bounds.Width-pad.Left-pad.Right, 0), Height: max(tabs.bounds.Height-pad.Top-pad.Bottom, 0),
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
		if tabs.paint.hasForeground {
			foreground := tabs.paint.foreground
			hovered := !state.Disabled && state.Hovered
			if !state.Selected && !hovered {
				foreground.Alpha *= 0.60
			}
			if state.Disabled {
				foreground.Alpha *= 0.50
			}
			look.Foreground = foreground
			if look.Indicator.Alpha > 0 {
				indicator := tabs.paint.foreground
				if state.Disabled {
					indicator.Alpha *= 0.50
				}
				look.Indicator = indicator
			}
		}
		if look.RingWidth > 0 {
			tabs.canvas.Rounded(expand(trigger, look.RingWidth), look.RingRadius, look.Ring)
		}
		if look.Bordered {
			tabs.canvas.Elevate(trigger, triggerRadius, look.Shadow) // the selected trigger's shadow-sm
			tabs.canvas.RoundedBorder(trigger, triggerRadius, 1, look.Border, look.Fill)
		} else {
			tabs.canvas.Rounded(trigger, triggerRadius, look.Fill)
		}

		indicator := look.Indicator
		indicatorBounds := gfx.Rect{X: trigger.X, Y: trigger.Y + trigger.Height + tabsLineDrop, Width: trigger.Width, Height: tabsIndicator}
		if tabs.direction == TabsVertical {
			indicatorBounds = gfx.Rect{X: trigger.X + trigger.Width + tabsLineRight, Y: trigger.Y, Width: tabsIndicator, Height: trigger.Height}
		}
		tabs.canvas.Rounded(indicatorBounds, 1, indicator)

		textStyle := gfx.TextStyle{Size: tabs.paint.fontSize, Weight: weight, Color: look.Foreground}
		tabs.paintContent(option, trigger, textStyle)
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
	if tabs.paint.fontSize == 0 {
		tabs.paint = tabs.resolvedPaint()
	}
	gap := tabs.itemGap()
	if tabs.direction == TabsVertical {
		height := (inner.Height - gap*(count-1)) / count
		return gfx.Rect{X: inner.X, Y: inner.Y + float32(index)*(height+gap), Width: inner.Width, Height: height}
	}
	if tabs.fit {
		width := tabs.contentWidth(tabs.option(index)) + tabs.paint.triggerPadding*2
		return gfx.Rect{X: inner.X + cursor, Y: inner.Y, Width: width, Height: inner.Height}
	}
	width := (inner.Width - gap*(count-1)) / count
	return gfx.Rect{X: inner.X + float32(index)*(width+gap), Y: inner.Y, Width: width, Height: inner.Height}
}

func (tabs Tabs) itemGap() float32 {
	if tabs.paint.fontSize == 0 {
		tabs.paint = tabs.resolvedPaint()
	}
	if tabs.paint.gap > 0 {
		return tabs.paint.gap
	}
	if tabs.variant == TabsLine {
		return tabs.paint.lineGap
	}
	return 0
}

func (tabs Tabs) contentWidth(option TabOption) float32 {
	if tabs.paint.fontSize == 0 {
		tabs.paint = tabs.resolvedPaint()
	}
	width := float32(0)
	if !option.IconOnly {
		width = tabs.canvas.TextWidthWeight(option.Label, tabs.paint.fontSize, tabs.paint.weight)
	}
	if len(option.Icon.Commands) > 0 || len(option.Icon.Layers) > 0 {
		width += tabs.paint.fontSize
		if option.Label != "" && !option.IconOnly {
			width += 6
		}
	}
	return width
}

func (tabs Tabs) paintContent(option TabOption, trigger gfx.Rect, style gfx.TextStyle) {
	if tabs.paint.fontSize == 0 {
		tabs.paint = tabs.resolvedPaint()
	}
	b := gfx.Rect{X: trigger.X + tabs.paint.triggerPadding, Y: trigger.Y, Width: max(0, trigger.Width-tabs.paint.triggerPadding*2), Height: trigger.Height}
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
	size := min(tabs.paint.fontSize, max(0, b.Height))
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

// TabsListRecipe resolves the look of the strip behind a tab list: its fill and
// its corner radius. The line variant draws no strip, so it is not filled.
func TabsListRecipe(theme gfx.Theme, variant TabsVariant) style.ButtonLook {
	if variant == TabsLine {
		return style.ButtonLook{}
	}
	return style.ButtonLook{
		Background: theme.Muted, Filled: true,
		Radius: style.TabsListRadius(theme.Metrics),
	}
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

type TabOption = base.TabOption

type TabsOrientation = base.TabsOrientation

type TabsVariant = base.TabsVariant

func TabsKeyDown(current gfx.ID, options []TabOption, orientation TabsOrientation, direction Direction, key gfx.Key) (gfx.ID, bool) {
	return base.TabsKeyDown(current, options, orientation, base.WritingDirection(direction), key)
}

const (
	TabsHorizontal = base.TabsHorizontal
)
const (
	TabsVertical = base.TabsVertical
	TabsDefault  = base.TabsDefault
	TabsLine     = base.TabsLine
)
