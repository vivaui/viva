package components

import (
	"math"
	"slices"
	"time"

	"github.com/vivaui/viva/gfx"
)

// hierarchyMaxLevels is how many panels (the list plus its fly-outs) a
// HierarchyMenu can show at once. Values nested deeper make Build fail, so no
// value becomes unreachable behind a limit.
const hierarchyMaxLevels = 8

// hierarchySwitchDelay is how long the pointer rests on another row before an
// open fly-out gives way to that row's, or closes. A fly-out opens at once when
// none is open. The rest keeps the pointer's diagonal across the rows beside the
// trigger on its way into the fly-out from switching it; the canvas has no
// pointer position to aim with instead.
const hierarchySwitchDelay = 60 * time.Millisecond

// hierarchyBarRow is the reserved row number, within a level, of that level's
// scrollbar hit.
const hierarchyBarRow = 0xFFFF

// A compact menu keeps the current level readable instead of squeezing all
// its ancestors into blank columns. Its navigation buttons use reserved levels
// above the real tree, and still travel through the caller's Choose route.
const (
	hierarchyOpenLevel = hierarchyMaxLevels
	hierarchyBackIndex = 2 * hierarchyMaxLevels << 16
	hierarchyMinColumn = 96
)

var hierarchyLeftIcon = gfx.Icon{ViewBox: 24, Stroke: 2, Cap: gfx.CapRound, Join: gfx.JoinRound, Commands: []gfx.PathCommand{gfx.MoveTo(15, 18), gfx.LineTo(9, 12), gfx.LineTo(15, 6)}}

// measureWidths visits the tree only after its values or font size change.
// Per-depth widths keep every ancestor in place when another branch opens.
func measureWidths(m *HierarchyMenuState, canvas *gfx.Canvas, size float32, parent int32, level int) {
	m.Widest.Levels = max(m.Widest.Levels, level+1)
	for n := m.Nodes[parent].First; n != 0; n = m.Nodes[n].Next {
		width := float32(math.Ceil(float64(canvas.TextWidthWeight(m.Nodes[n].Name, size, gfx.WeightRegular)))) + 47
		m.Widest.Widths[level] = max(m.Widest.Widths[level], width)
		if m.Nodes[n].Count > 0 {
			measureWidths(m, canvas, size, n, level+1)
		}
	}
}

// PaintHierarchyMenu paints the first list under anchor and a fly-out beside
// each open row, and returns the box around every panel. Its complete
// path is placed before any fly-out opens, so hovering never moves a parent out
// from under the pointer. Narrow viewports show one readable level with parent
// and child navigation buttons instead. Hovering a row in a cascade opens its
// fly-out at once; switching rows waits a short rest on the frame clock.
func PaintHierarchyMenu(canvas *gfx.Canvas, theme gfx.Theme, anchor gfx.Rect, m *HierarchyMenuState, o HierarchyMenuOptions) gfx.Rect {
	if canvas == nil || !m.OK {
		return gfx.Rect{}
	}
	floating := o.Viewport.Width > 0 && o.Viewport.Height > 0
	if o.Overlay {
		canvas.PushLayer(o.Layer)
		defer canvas.PopLayer()
		canvas.PushFocusScope(o.Scope)
		defer canvas.PopFocusScope()
		canvas.TrapFocus(o.Scope)
		canvas.SetEscapeTarget(o.DismissID)
		if floating {
			canvas.PushOverlayClip(o.Viewport)
		}
		canvas.HitWithOptions(o.DismissID, o.Viewport, o.Label, gfx.HitOptions{Role: gfx.RoleButton, SkipFocus: true, Backdrop: true, PassSecondary: true})
		if floating {
			canvas.PopOverlayClip()
		}
	}
	if floating {
		canvas.PushOverlayClip(o.Viewport)
		defer canvas.PopOverlayClip()
	}
	m.OptionIDs, m.TypeKeys = o.OptionIDs, o.Typeahead
	if m.Initial {
		if m.Value == "" && m.Level == 0 && m.Active[0] == 0 {
			for _, action := range o.Leading {
				if !action.Disabled && !action.ID.IsZero() {
					m.Action = action.ID
					break
				}
			}
			if m.Action.IsZero() && m.Nodes[0].Count == 0 {
				for _, action := range o.Trailing {
					if !action.Disabled && !action.ID.IsZero() {
						m.Action = action.ID
						break
					}
				}
			}
		}
		m.Initial = false
	}
	widths, left, reverse := panelWidths(m, canvas, theme, anchor, o)
	if m.Compact {
		m.Level = m.DepthValue
	}
	// Move the cursor before any row is painted, as the flat list does.
	hovered := canvas.Hovered()
	level, row, onRow := 0, 0, false
	if index, ok := hovered.In(o.OptionIDs); ok && index&0xFFFF != hierarchyBarRow {
		level, row, onRow = index>>16, index&0xFFFF, true
	}
	entered := m.Hover.Entered(hovered)
	onAction := false
	if !m.Compact || m.DepthValue == 0 {
		for _, actions := range [][]HierarchyMenuAction{o.Leading, o.Trailing} {
			for _, action := range actions {
				if !action.ID.IsZero() && action.ID == hovered && !action.Disabled {
					onAction = true
					if entered {
						m.Level, m.Action = 0, action.ID
						m.Typeahead.Reset()
					}
				}
			}
		}
	}
	if entered && onRow && (!m.Compact || level == m.DepthValue) {
		if n := m.RowNode(level, row); n != 0 && !m.Nodes[n].Disabled {
			m.Level, m.Active[level] = level, row
			m.Action = gfx.ID{}
			m.Typeahead.Reset()
		}
	}
	// A fly-out opens, or a deeper one closes, only after the pointer rests on a
	// row when another fly-out is open, so crossing rows on the way to one
	// changes nothing.
	n := int32(0)
	if onRow {
		n = m.RowNode(level, row)
	}
	opens := n != 0 && m.Nodes[n].Count > 0 && level+1 < hierarchyMaxLevels && !(m.DepthValue > level && m.Trail[level] == n)
	closes := (n != 0 && m.Nodes[n].Count == 0 || onAction) && m.DepthValue > level
	if !m.Compact && (opens || closes) && (entered || m.Intent.Target == hovered) {
		now, delay := time.Time{}.Add(canvas.Now()), time.Duration(0)
		if m.DepthValue > level {
			delay = hierarchySwitchDelay
		}
		if id, wait := m.Intent.Rest(hovered, now, delay); !id.IsZero() {
			m.Intent.Reset()
			m.DepthValue = level
			if opens {
				m.Trail[level], m.DepthValue = n, level+1
				m.Scrolls[level+1].Offset = 0
				m.Scrolls[level+1].Items = m.Scrolls[level+1].Items[:0]
			}
		} else if wait > 0 {
			canvas.RequestFrameIn(wait)
		}
	} else {
		m.Intent.Reset()
	}

	rows := o.MaxRows
	if rows <= 0 {
		rows = 7
	}
	var union gfx.Rect
	first := 0
	if m.Compact {
		first = m.DepthValue
		clear(m.Panels[:first])
	}
	for l := first; l <= m.DepthValue; l++ {
		parent := m.ParentOf(l)
		count := int(m.Nodes[parent].Count)
		rowCount := count
		separator := l == 0 && len(o.Trailing) > 0 && count+len(o.Leading) > 0
		if l == 0 {
			rowCount += len(o.Leading) + len(o.Trailing)
		}
		height := float32(max(1, min(rowCount, rows))*32 + 8)
		if separator {
			height += menuSeparatorHeight
		}
		back := m.Compact && l > 0
		if back {
			height += 32
		}
		popup := gfx.Rect{X: left, Y: anchor.Y + anchor.Height + 6, Width: widths[l], Height: height}
		if l == first {
			placePopupBelow(&popup, anchor, height, o.Viewport)
		} else {
			prev, trigger := m.Panels[l-1], m.Triggers[l-1]
			popup.X, popup.Y = prev.X+prev.Width-4, trigger.Y-4
			if reverse {
				popup.X = prev.X - popup.Width + 4
			}
			if o.Viewport.Height > 0 {
				top, bottom := o.Viewport.Y+1, o.Viewport.Y+o.Viewport.Height-1
				popup.Y = max(top, min(popup.Y, bottom-popup.Height))
				popup.Height = min(popup.Height, bottom-popup.Y)
			}
		}
		m.Panels[l] = popup
		union = unionRect(union, popup, l == first)

		canvas.Block(popup)
		canvas.Container(popup, gfx.RoleListBox, o.Label)
		paintPopupSurface(canvas, &theme, popup, gfx.ShadowMD)
		if m.Compact {
			canvas.Clip(popup)
		}
		scroll := &m.Scrolls[l]
		scroll.BarID = o.ScrollID
		if l > 0 {
			scroll.BarID = o.OptionIDs.At(l<<16 | hierarchyBarRow)
		}
		menu := menuSurface{canvas: canvas, theme: theme, bounds: popup, options: menuSurfaceOptions{layer: o.Layer, scope: o.Scope, scroll: scroll}, top: popup.Y + 4, bottom: popup.Y + popup.Height - 4, viewHeight: popup.Height - 8}
		if back {
			// Keep parent navigation outside the scrolling list, so it remains
			// reachable even after scrolling to the last child.
			bounds := gfx.Rect{X: popup.X + 4, Y: popup.Y + 4, Width: max(popup.Width-8, 0), Height: min(float32(32), max(popup.Height-8, 0))}
			label := m.Nodes[parent].Name
			look := menuItemRecipe(&theme, gfx.State{Hovered: canvas.Hovered() == o.OptionIDs.At(hierarchyBackIndex)})
			if look.Fill.Alpha > 0 {
				canvas.Rounded(bounds, look.Radius, look.Fill)
			}
			paintChevron(canvas, &hierarchyLeftIcon, gfx.Rect{X: bounds.X + 8, Y: bounds.Y, Width: 16, Height: bounds.Height}, look.Foreground)
			text := gfx.Rect{X: bounds.X + 32, Y: bounds.Y, Width: max(bounds.Width-40, 0), Height: bounds.Height}
			canvas.Label(canvas.FitLine(label, theme.Metrics.FontSizeM, gfx.WeightRegular, text.Width), text, gfx.TextStyle{Size: theme.Metrics.FontSizeM, Color: look.Foreground})
			canvas.HitWithOptions(o.OptionIDs.At(hierarchyBackIndex), bounds, label, gfx.HitOptions{Role: gfx.RoleButton, Description: m.Nodes[parent].Path, SkipFocus: true})
			menu.top += bounds.Height
			menu.viewHeight = max(menu.viewHeight-bounds.Height, 0)
		}
		menu.beginScroll()
		if l == 0 {
			for _, action := range o.Leading {
				paintAction(m, &menu, action)
			}
		}
		i := 0
		for node := m.Nodes[parent].First; node != 0; node, i = m.Nodes[node].Next, i+1 {
			info := m.Nodes[node]
			id := o.OptionIDs.At(l<<16 | i)
			rowBounds := menu.rowBounds(32)
			open := l < m.DepthValue && m.Trail[l] == node
			if open {
				m.Triggers[l] = rowBounds
			}
			if !menu.takeItem(rowBounds, id, info.Name, info.Disabled) {
				continue
			}
			selected := m.Equal(m.Value, info.Path)
			menu.paintRow(rowBounds, func() {
				look := menuItemRecipe(&theme, gfx.State{Hovered: open || l == m.Level && i == m.Active[l] && m.Action.IsZero(), Disabled: info.Disabled})
				if look.Fill.Alpha > 0 {
					canvas.Rounded(rowBounds, look.Radius, look.Fill)
				}
				color := look.Foreground
				reserve := float32(38)
				if selected && info.Count > 0 {
					reserve += 16
				}
				labelWidth := max(rowBounds.Width-reserve, 0)
				canvas.Label(canvas.FitLine(info.Name, theme.Metrics.FontSizeM, gfx.WeightRegular, labelWidth), gfx.Rect{X: rowBounds.X + 8, Y: rowBounds.Y, Width: labelWidth, Height: rowBounds.Height}, gfx.TextStyle{Size: theme.Metrics.FontSizeM, Color: color})
				switch {
				case info.Count > 0 && rowBounds.Width >= reserve:
					chevron := &chevronRightIcon
					if reverse {
						chevron = &hierarchyLeftIcon
					}
					paintChevron(canvas, chevron, gfx.Rect{X: rowBounds.X + rowBounds.Width - 25, Y: rowBounds.Y, Width: 16, Height: rowBounds.Height}, color)
					if selected {
						paintCheck(canvas, gfx.Rect{X: rowBounds.X + rowBounds.Width - 41, Y: rowBounds.Y, Width: 16, Height: rowBounds.Height}, 14, color)
					}
				case selected && rowBounds.Width >= reserve:
					paintCheck(canvas, gfx.Rect{X: rowBounds.X + rowBounds.Width - 25, Y: rowBounds.Y, Width: 16, Height: rowBounds.Height}, 14, color)
				}
				canvas.HitWithOptions(id, rowBounds, info.Name, gfx.HitOptions{Role: gfx.RoleOption, Value: info.Path, Selected: selected, Expanded: open, Disabled: info.Disabled, SkipFocus: true})
				if m.Compact && info.Count > 0 {
					button := gfx.Rect{X: rowBounds.X + rowBounds.Width - 30, Y: rowBounds.Y, Width: 30, Height: rowBounds.Height}
					canvas.HitWithOptions(o.OptionIDs.At((hierarchyOpenLevel+l)<<16|i), button, info.Name, gfx.HitOptions{Role: gfx.RoleButton, Value: info.Path, Expandable: true, Disabled: info.Disabled, SkipFocus: true})
				}
			})
		}
		if l == 0 {
			if separator {
				menu.addSeparator()
			}
			for _, action := range o.Trailing {
				paintAction(m, &menu, action)
			}
			if !m.Action.IsZero() && !m.ActionEnabled(m.Action) {
				m.Action = gfx.ID{}
			}
		}
		if rowCount == 0 {
			canvas.LabelCenter(o.EmptyLabel, popup, gfx.TextStyle{Size: theme.Metrics.FontSizeM, Color: theme.MutedForeground})
		}
		menu.end()
		if m.Compact {
			canvas.Unclip()
		}
	}
	if m.Reveal {
		if m.Scrolls[m.Level].Reveal(m.CurrentID()) {
			canvas.RequestFrame()
		}
		m.Reveal = false
	}
	return union
}

func paintAction(m *HierarchyMenuState, menu *menuSurface, action HierarchyMenuAction) {
	row := menu.rowBounds(32)
	if !menu.takeItem(row, action.ID, action.Label, action.Disabled) {
		return
	}
	menu.paintRow(row, func() {
		canvas, theme := menu.canvas, menu.theme
		look := menuItemRecipe(&theme, gfx.State{Hovered: m.Level == 0 && m.Action == action.ID, Disabled: action.Disabled})
		if look.Fill.Alpha > 0 {
			canvas.Rounded(row, look.Radius, look.Fill)
		}
		color := look.Foreground
		label := gfx.Rect{X: row.X + 8, Y: row.Y, Width: max(row.Width-16, 0), Height: row.Height}
		canvas.Label(canvas.FitLine(action.Label, theme.Metrics.FontSizeM, gfx.WeightRegular, label.Width), label, gfx.TextStyle{Size: theme.Metrics.FontSizeM, Color: color})
		row.Width = max(row.Width-menu.hitInset, 0)
		canvas.HitWithOptions(action.ID, row, action.Label, gfx.HitOptions{Role: gfx.RoleMenuItem, Disabled: action.Disabled, SkipFocus: true})
	})
}

// panelWidths reserves the complete cascade before the first hover. There are
// at most hierarchyMaxLevels columns. A common width cap gives long labels the
// room left by short ones. Below a readable column width it uses one panel with
// explicit navigation; shrinking eight columns to blank slivers is not useful.
func panelWidths(m *HierarchyMenuState, canvas *gfx.Canvas, theme gfx.Theme, anchor gfx.Rect, o HierarchyMenuOptions) (widths [hierarchyMaxLevels]float32, left float32, reverse bool) {
	if m.Widest.Rebuilt != m.Rebuilt || m.Widest.Size != theme.Metrics.FontSizeM {
		m.Widest.Widths, m.Widest.Levels = [hierarchyMaxLevels]float32{}, 0
		measureWidths(m, canvas, theme.Metrics.FontSizeM, 0, 0)
		m.Widest.Rebuilt, m.Widest.Size = m.Rebuilt, theme.Metrics.FontSizeM
	}
	count := max(m.Widest.Levels, 1)
	rootWidth := float32(0)
	for _, actions := range [][]HierarchyMenuAction{o.Leading, o.Trailing} {
		for _, action := range actions {
			rootWidth = max(rootWidth, float32(math.Ceil(float64(canvas.TextWidthWeight(action.Label, theme.Metrics.FontSizeM, gfx.WeightRegular))))+24)
		}
	}
	span := float32(-4 * (count - 1))
	for level := 0; level < count; level++ {
		widths[level] = max(m.Widest.Widths[level], 144)
		if level == 0 {
			widths[level] = max(widths[level], anchor.Width, rootWidth)
		}
		span += widths[level]
	}
	left = anchor.X
	m.Compact = false
	viewport := o.Viewport
	if viewport.Width <= 0 {
		return widths, left, false
	}
	start, end := viewport.X+1, viewport.X+viewport.Width-1
	rootStart := max(start, min(anchor.X, end))
	rootEnd := max(start, min(anchor.X+anchor.Width, end))
	available := max(end-rootStart, 0)
	rootMinimum := max(float32(hierarchyMinColumn), anchor.Width)
	minimum := rootMinimum + float32(hierarchyMinColumn*(count-1)-4*(count-1))
	if available >= minimum {
		left = rootStart
	} else if rootEnd-start >= minimum {
		// Keep the root at its anchor, and all fly-outs on the same side.
		// Flipping individual panels covers ancestors after the next turn.
		reverse, available = true, rootEnd-start
	} else {
		m.Compact = true
		width := min(max(slices.Max(widths[:count]), anchor.Width), max(viewport.Width-2, 0))
		for level := range widths {
			widths[level] = width
		}
		left = max(start, min(left, end-width))
		return widths, left, false
	}
	if span > available {
		ordered := widths
		budget := available + float32(4*(count-1))
		first := 0
		if rootMinimum > budget/float32(count) {
			widths[0], budget, first = rootMinimum, budget-rootMinimum, 1
		}
		slices.Sort(ordered[first:count])
		limit := budget / float32(count-first)
		for index, width := range ordered[first : count-1] {
			if width >= limit {
				break
			}
			budget -= width
			limit = budget / float32(count-first-index-1)
		}
		for level := first; level < count; level++ {
			widths[level] = min(widths[level], limit)
		}
	}
	if reverse {
		left = rootEnd - widths[0]
	}
	return widths, left, reverse
}

// placePopupBelow keeps a popup inside the viewport: above the anchor when that
// holds a usable row and below does not fit, shortened to what room is left.
func placePopupBelow(popup *gfx.Rect, anchor gfx.Rect, height float32, viewport gfx.Rect) {
	if viewport.Height > 0 {
		top, bottom := viewport.Y+1, viewport.Y+viewport.Height-1
		if popup.Y+popup.Height > bottom {
			// Prefer the room above the anchor, but only when it holds a usable
			// row; otherwise the list scrolls inside what is left below.
			if above := anchor.Y - 6 - top; above >= min(height, 40) {
				popup.Height = min(height, above)
				popup.Y = anchor.Y - popup.Height - 6
			}
		}
		popup.Y = max(top, min(popup.Y, bottom-1))
		popup.Height = max(1, min(popup.Height, bottom-popup.Y))
	}
}

// unionRect is the smallest rect holding both; the first rect starts it.
func unionRect(a, b gfx.Rect, first bool) gfx.Rect {
	if first {
		return b
	}
	x, y := min(a.X, b.X), min(a.Y, b.Y)
	return gfx.Rect{X: x, Y: y, Width: max(a.X+a.Width, b.X+b.Width) - x, Height: max(a.Y+a.Height, b.Y+b.Height) - y}
}
