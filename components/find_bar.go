package components

import "github.com/vivaui/viva/ui"

// FindBarLabels are the accessible names and hints of the bar's controls, in the
// application's language: icon buttons have no text of their own to be read.
type FindBarLabels struct {
	Previous, Next, Close                 string
	MatchCase, WholeWord, Regexp          string
	ToggleReplace, ReplaceOne, ReplaceAll string
}

// FindBarSnapshot is one caller-owned presentation of a find and replace bar. The
// two fields are InputSnapshots, so the application connects them to the
// platform text service as any input; searching, stepping and replacing are the
// application's (see text/find.Session), and the bar only paints the state and
// declares the hits.
type FindBarSnapshot struct {
	// Find and Replacement are the two text fields; their IDs must be set. Their
	// Label is the accessible name and Placeholder the hint of an empty one.
	Find, Replacement InputSnapshot
	// Replacing shows the replacement row.
	Replacing bool
	// The IDs of the bar's buttons. A zero ID leaves its button out.
	PreviousID, NextID, CloseID                 ui.ID
	MatchCaseID, WholeWordID, RegexpID          ui.ID
	ToggleReplaceID, ReplaceOneID, ReplaceAllID ui.ID
	// The options as the query is read.
	MatchCase, WholeWord, Regexp bool
	// Status is the count the bar shows, such as "3 of 12" or "No results",
	// already in the application's language.
	Status string
	// NoResults paints the status in the danger color and the field invalid: the
	// query matched nothing, or is not a valid expression.
	NoResults bool
	// Disabled dims the stepping and replacing buttons, as when there are no
	// matches to go to.
	Disabled bool
	Labels   FindBarLabels
	// ReplaceOneText and ReplaceAllText are the captions of the replacement row's
	// buttons; empty ones paint their Labels.
	ReplaceOneText, ReplaceAllText string
}

// FindBarLayout is where a paint put the bar's parts.
type FindBarLayout struct {
	Bounds        ui.Rect
	Find          ui.Rect
	Replacement   ui.Rect
	Status        ui.Rect
	ToggleReplace ui.Rect
}

const (
	findBarPadding  = 6
	findBarGap      = 4
	findBarRowGap   = 4
	findBarButton   = float32(24)
	findBarMinField = float32(120)
	findBarStatus   = float32(72)
)

// FindBarHeight is how tall the bar is for the theme, with or without the
// replacement row.
func FindBarHeight(theme ui.Theme, replacing bool) float32 {
	row := findBarRow(theme)
	height := row + 2*findBarPadding
	if replacing {
		height += row + findBarRowGap
	}
	return height
}

// FindBarMinWidth is the narrowest bar that keeps its query field at a usable
// width with the buttons the snapshot declares. A narrower one still paints
// without overlap, but its field is squeezed below that.
func FindBarMinWidth(snapshot FindBarSnapshot) float32 {
	count := 0
	for _, id := range [...]ui.ID{
		snapshot.PreviousID, snapshot.NextID, snapshot.CloseID,
		snapshot.MatchCaseID, snapshot.WholeWordID, snapshot.RegexpID, snapshot.ToggleReplaceID,
	} {
		if !id.IsZero() {
			count++
		}
	}
	return 2*findBarPadding + float32(count)*(findBarButton+findBarGap) + findBarMinField - findBarGap
}

func findBarRow(theme ui.Theme) float32 {
	return max(theme.Metrics.ControlHeight-4, findBarButton)
}

// PaintFindBar paints the bar in bounds, a floating surface as a popover is, and
// returns where it put its parts. The height of bounds is ignored: the bar is
// FindBarHeight tall, from the top of bounds. The query field takes what the
// buttons leave, and the count gives way first when the bar is narrow.
func PaintFindBar(canvas *ui.Canvas, theme ui.Theme, bounds ui.Rect, snapshot FindBarSnapshot) FindBarLayout {
	bounds.Height = FindBarHeight(theme, snapshot.Replacing)
	layout := FindBarLayout{Bounds: bounds}
	if canvas == nil || bounds.Width <= 0 {
		return layout
	}
	radius := theme.Metrics.RadiusControl
	canvas.Elevate(bounds, radius, ui.ShadowMD)
	paintBordered(canvas, theme, bounds, radius, theme.Border, theme.Popover)

	row := findBarRow(theme)
	left, right := bounds.X+findBarPadding, bounds.X+bounds.Width-findBarPadding
	y := bounds.Y + findBarPadding
	muted := theme.MutedForeground

	// The right edge, back to front: close, next, previous, count, options.
	x := right
	place := func(width float32) ui.Rect {
		x -= width
		rect := ui.Rect{X: x, Y: y + (row-findBarButton)/2, Width: width, Height: findBarButton}
		x -= findBarGap
		return rect
	}
	if !snapshot.CloseID.IsZero() {
		rect := place(findBarButton)
		button := BeginButton(canvas, theme, snapshot.CloseID, rect).WithIcon(crossIcon).IconXSmall()
		button.Ghost(snapshot.Labels.Close)
	}
	for _, step := range [...]struct {
		id   ui.ID
		icon *ui.Icon
		name string
	}{{snapshot.NextID, &chevronDownIcon, snapshot.Labels.Next}, {snapshot.PreviousID, &chevronUpIcon, snapshot.Labels.Previous}} {
		if step.id.IsZero() {
			continue
		}
		rect := place(findBarButton)
		button := BeginButton(canvas, theme, step.id, rect).WithIcon(*step.icon).IconXSmall()
		if snapshot.Disabled {
			button = button.Disabled()
		}
		button.Ghost(step.name)
	}
	options := 0
	for _, id := range [...]ui.ID{snapshot.MatchCaseID, snapshot.WholeWordID, snapshot.RegexpID} {
		if !id.IsZero() {
			options++
		}
	}
	fixed := float32(options)*(findBarButton+findBarGap) + findBarMinField
	statusWidth := float32(0)
	if snapshot.Status != "" && x-left-fixed >= findBarStatus {
		statusWidth = findBarStatus
	}
	if statusWidth > 0 {
		layout.Status = ui.Rect{X: x - statusWidth + findBarGap, Y: y, Width: statusWidth - findBarGap, Height: row}
		color := muted
		if snapshot.NoResults {
			color = theme.Danger
		}
		canvas.LabelCenter(snapshot.Status, layout.Status, ui.TextStyle{Size: theme.Metrics.FontSizeS, Color: color})
		x -= statusWidth
	}
	optionList := [...]struct {
		id     ui.ID
		label  string
		name   string
		active bool
	}{
		{snapshot.RegexpID, ".*", snapshot.Labels.Regexp, snapshot.Regexp},
		{snapshot.WholeWordID, "ab", snapshot.Labels.WholeWord, snapshot.WholeWord},
		{snapshot.MatchCaseID, "Aa", snapshot.Labels.MatchCase, snapshot.MatchCase},
	}
	for _, option := range optionList {
		if option.id.IsZero() {
			continue
		}
		rect := place(findBarButton)
		toggle := BeginToggle(canvas, theme, option.id, rect, option.active).Small()
		toggle.bounds.Height = findBarButton
		paintFindBarToggle(canvas, theme, toggle, option.label, option.name)
	}
	if !snapshot.ToggleReplaceID.IsZero() {
		layout.ToggleReplace = ui.Rect{X: left, Y: y + (row-findBarButton)/2, Width: findBarButton, Height: findBarButton}
		icon := &chevronRightIcon
		if snapshot.Replacing {
			icon = &chevronDownIcon
		}
		BeginButton(canvas, theme, snapshot.ToggleReplaceID, layout.ToggleReplace).WithIcon(*icon).IconXSmall().Ghost(snapshot.Labels.ToggleReplace)
		left += findBarButton + findBarGap
	}

	layout.Find = ui.Rect{X: left, Y: y, Width: max(x-left+findBarGap, 0), Height: row}
	find := snapshot.Find
	find.Invalid = find.Invalid || snapshot.NoResults && find.Value != ""
	PaintInput(canvas, theme, layout.Find, find)

	if snapshot.Replacing {
		y += row + findBarRowGap
		right := bounds.X + bounds.Width - findBarPadding
		buttons := [...]struct {
			id   ui.ID
			text string
			name string
		}{
			{snapshot.ReplaceAllID, snapshot.ReplaceAllText, snapshot.Labels.ReplaceAll},
			{snapshot.ReplaceOneID, snapshot.ReplaceOneText, snapshot.Labels.ReplaceOne},
		}
		for _, entry := range buttons {
			if entry.id.IsZero() {
				continue
			}
			caption := entry.text
			if caption == "" {
				caption = entry.name
			}
			button := BeginButton(canvas, theme, entry.id, ui.Rect{X: right, Y: y, Height: row}).XSmall().SizeToFit(caption)
			button.bounds.X = right - button.bounds.Width
			button.bounds.Y = y + (row-button.bounds.Height)/2
			if snapshot.Disabled {
				button = button.Disabled()
			}
			button.Outline(caption)
			right -= button.bounds.Width + findBarGap
		}
		layout.Replacement = ui.Rect{X: left, Y: y, Width: max(right-left+findBarGap, 0), Height: row}
		PaintInput(canvas, theme, layout.Replacement, snapshot.Replacement)
	}
	return layout
}

// paintFindBarToggle paints one option toggle: its caption in the fixed-pitch
// font, as a code editor's find bar sets "Aa" and ".*", and the hit with the
// option's accessible name.
func paintFindBarToggle(canvas *ui.Canvas, theme ui.Theme, toggle Toggle, caption, name string) {
	bounds := toggle.bounds
	enabled := !toggle.id.IsZero()
	hovered := enabled && canvas.Hovered() == toggle.id
	focused := enabled && canvas.FocusVisible() == toggle.id
	radius := theme.Metrics.RadiusControl
	fill := theme.Background
	fill.Alpha = 0
	foreground := theme.MutedForeground
	if toggle.active {
		fill = theme.Accent
		foreground = theme.AccentForeground
	} else if hovered {
		fill = theme.Muted
		foreground = theme.Foreground
	}
	if focused {
		canvas.Rounded(expand(bounds, 2), radius+2, scaledAlpha(theme.Ring, 0.50))
	}
	canvas.Rounded(bounds, radius, fill)
	canvas.LabelCenter(caption, bounds, ui.TextStyle{Size: theme.Metrics.FontSizeS, Weight: ui.WeightMedium, Color: foreground, Mono: true})
	canvas.HitWithOptions(toggle.id, bounds, name, ui.HitOptions{Role: ui.RoleButton, Checked: toggle.active, Selected: toggle.active})
}

// FindBarCommand is a command a key asks of a find bar.
type FindBarCommand uint8

const (
	FindBarNoCommand FindBarCommand = iota
	// FindBarOpen is Command or Control+F: open the bar, with the selection as
	// the query.
	FindBarOpen
	// FindBarOpenReplace is Command+Option+F. Control+H opens it in editors on
	// Windows and Linux, but is a backspace on macOS, so FindBarCommandForKey
	// leaves that chord to the application, which knows its platform.
	FindBarOpenReplace
	// FindBarNext and FindBarPrevious are Enter and Shift+Enter in the bar's
	// field, and F3 and Shift+F3 anywhere in the editor.
	FindBarNext
	FindBarPrevious
	// FindBarCloseCommand is Escape in the bar.
	FindBarCloseCommand
)

// FindBarCommandForKey says which find command a key press asks for. inBar tells
// whether the focus is in one of the bar's fields, where Enter and Escape belong
// to the bar. Everything else of a key stays with the editor and the text system.
func FindBarCommandForKey(event ui.KeyEvent, inBar bool) (FindBarCommand, bool) {
	if event.Release {
		return FindBarNoCommand, false
	}
	shift := event.Modifiers&ui.ModifierShift != 0
	primary := event.Modifiers&(ui.ModifierControl|ui.ModifierMeta) != 0
	switch {
	case primary && !shift && event.Modifiers&ui.ModifierAlt == 0 && (event.Text == "f" || event.Text == "F"):
		return FindBarOpen, true
	case event.Modifiers&ui.ModifierAlt != 0 && event.Modifiers&ui.ModifierMeta != 0 && !shift && (event.Text == "f" || event.Text == "ƒ"):
		return FindBarOpenReplace, true
	case event.Key == ui.KeyF3 && !primary:
		if shift {
			return FindBarPrevious, true
		}
		return FindBarNext, true
	case inBar && event.Key == ui.KeyEnter && !primary:
		if shift {
			return FindBarPrevious, true
		}
		return FindBarNext, true
	case inBar && event.Key == ui.KeyEscape:
		return FindBarCloseCommand, true
	}
	return FindBarNoCommand, false
}
