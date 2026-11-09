package components

import "github.com/vivaui/viva/gfx"

// SidebarSide selects the desktop edge carrying the separator.
type SidebarSide uint8

const (
	SidebarLeft SidebarSide = iota
	SidebarRight
)

// SidebarVariant mirrors the official sidebar, floating, and inset surfaces.
type SidebarVariant uint8

const (
	SidebarStandard SidebarVariant = iota
	SidebarFloating
	SidebarInset
)

// SidebarCollapsible mirrors off-canvas, icon-only, and fixed behavior.
type SidebarCollapsible uint8

const (
	SidebarOffcanvas SidebarCollapsible = iota
	SidebarIcon
	SidebarFixed
)

// SidebarOptions contains caller-owned desktop state. Mobile sheet behavior is
// intentionally outside this native desktop recipe.
type SidebarOptions struct {
	Side        SidebarSide
	Variant     SidebarVariant
	Collapsible SidebarCollapsible
	Collapsed   bool
}

// SidebarMenuVariant selects the default or outlined menu-button recipe.
type SidebarMenuVariant uint8

const (
	SidebarMenuDefault SidebarMenuVariant = iota
	SidebarMenuOutline
)

// SidebarMenuSize mirrors official sm/default/lg button sizes.
type SidebarMenuSize uint8

const (
	SidebarMenuSizeDefault SidebarMenuSize = iota
	SidebarMenuSizeSmall
	SidebarMenuSizeLarge
)

// SidebarMenuAction is the secondary button of a menu item (SidebarMenuAction):
// a 20px square at the item's trailing edge, usually a "..." that opens a menu.
// It has an ID of its own; ShowOnHover shows it only while the item or the
// action is hovered or focused.
type SidebarMenuAction struct {
	ID          gfx.ID
	Icon        gfx.Icon
	Label       string
	ShowOnHover bool
}

// SidebarMenuItem is one explicit menu-button declaration. Trailing is the
// SidebarMenuBadge: a small medium-weight count at the trailing edge, which
// takes the item's foreground color while it is active or hovered.
type SidebarMenuItem struct {
	ID         gfx.ID
	Label      string
	Icon       string
	VectorIcon gfx.Icon
	Trailing   string
	Action     SidebarMenuAction
	Active     bool
	Disabled   bool
	Variant    SidebarMenuVariant
	Size       SidebarMenuSize
}

// SidebarSubSize mirrors SidebarMenuSubButton's md/sm sizes.
type SidebarSubSize uint8

const (
	SidebarSubSizeMedium SidebarSubSize = iota
	SidebarSubSizeSmall
)

// SidebarSubItem is one button of a SidebarMenuSub.
type SidebarSubItem struct {
	ID       gfx.ID
	Label    string
	Active   bool
	Disabled bool
	Size     SidebarSubSize
}

// Sidebar incrementally paints a compact application navigation rail. It owns
// only paint-time insertion coordinates; selection remains caller-owned.
//
// Content flows down from the top like SidebarContent: AddHeader/title, then
// groups (AddGroup) of menu items (AddMenuItem, with AddSubMenu under an item).
// Footers are reserved from the bottom edge and shrink the room above them.
type Sidebar struct {
	canvas    *gfx.Canvas
	theme     gfx.Theme
	bounds    gfx.Rect
	y         float32
	footerTop float32 // top of the topmost footer, or 0 when there is none
	hidden    bool
	icon      bool
	started   bool // a group or item has been added
}

// BeginSidebar paints the neutral sidebar surface and optional product title.
func BeginSidebar(canvas *gfx.Canvas, theme gfx.Theme, bounds gfx.Rect, title string) Sidebar {
	return BeginSidebarWithOptions(canvas, theme, bounds, title, SidebarOptions{Collapsible: SidebarFixed})
}

// BeginSidebarWithOptions paints the selected desktop surface and collapse
// state. Off-canvas collapsed sidebars emit no commands; icon collapse keeps a
// narrow rail and suppresses labels.
//
// Floating is a rounded, ringed card inside a p-2 margin. Inset is the same
// margin around a plain surface: the card in that variant is the page content,
// which PaintSidebarInset paints.
func BeginSidebarWithOptions(canvas *gfx.Canvas, theme gfx.Theme, bounds gfx.Rect, title string, options SidebarOptions) Sidebar {
	if options.Collapsed && options.Collapsible == SidebarOffcanvas {
		return Sidebar{canvas: canvas, theme: theme, bounds: bounds, hidden: true}
	}
	icon := options.Collapsed && options.Collapsible == SidebarIcon
	if icon {
		bounds.Width = theme.Metrics.ControlHeight + theme.Metrics.GapItem*2
	}
	switch options.Variant {
	case SidebarFloating:
		bounds = bounds.Inset(gfx.All(theme.Metrics.GapItem))
		canvas.Elevate(bounds, theme.Metrics.RadiusPopup(), gfx.ShadowSM) // the floating variant's shadow-sm
		canvas.RoundedBorder(bounds, theme.Metrics.RadiusPopup(), 1, theme.Border, theme.CardElevated)
	case SidebarInset:
		canvas.Fill(bounds, theme.CardElevated)
		bounds = bounds.Inset(gfx.All(theme.Metrics.GapItem))
	default:
		canvas.Fill(bounds, theme.CardElevated)
		if bounds.Width > 0 {
			x := bounds.X + bounds.Width - 1
			if options.Side == SidebarRight {
				x = bounds.X
			}
			canvas.Fill(gfx.Rect{X: x, Y: bounds.Y, Width: 1, Height: bounds.Height}, theme.Border)
		}
	}
	sidebar := Sidebar{canvas: canvas, theme: theme, bounds: bounds, y: bounds.Y, icon: icon}
	if title != "" && !icon {
		header := sidebar.AddHeader(theme.Metrics.ControlHeight)
		canvas.Label(title, gfx.Rect{X: header.X + theme.Metrics.GapItem, Y: header.Y, Width: sidebarMax32(0, header.Width-theme.Metrics.GapItem*2), Height: header.Height},
			gfx.TextStyle{Size: theme.Metrics.FontSizeM, Weight: gfx.WeightSemibold, Color: theme.Foreground})
	} else {
		// SidebarGroup's p-2 above the first group.
		sidebar.y += theme.Metrics.GapItem
	}
	return sidebar
}

// PaintSidebarInset paints the page surface that goes with the Inset variant: the
// region beside the sidebar takes the sidebar color and the content is a
// rounded background-colored card inside an 8px margin, flush with the sidebar
// edge (m-2 ml-0). It returns the card's bounds for the caller's content.
func PaintSidebarInset(canvas *gfx.Canvas, theme gfx.Theme, bounds gfx.Rect, side SidebarSide) gfx.Rect {
	margin := theme.Metrics.GapItem
	canvas.Fill(bounds, theme.CardElevated)
	card := bounds.Inset(gfx.All(margin))
	if side == SidebarRight {
		card.Width += margin
	} else {
		card.X -= margin
		card.Width += margin
	}
	if card.Width <= 0 || card.Height <= 0 {
		return gfx.Rect{}
	}
	canvas.Rounded(card, theme.Metrics.RadiusSurface+4, theme.Background)
	return card
}

// AddHeader reserves SidebarHeader: a p-2 padded block of the given content
// height at the top. It returns the content rectangle for the caller to fill
// (a team switcher, a logo) and moves the insertion point below it.
func (sidebar *Sidebar) AddHeader(height float32) gfx.Rect {
	if sidebar.hidden {
		return gfx.Rect{}
	}
	metrics := sidebar.theme.Metrics
	rect := gfx.Rect{
		X: sidebar.bounds.X + metrics.GapItem, Y: sidebar.y + metrics.GapItem,
		Width: sidebarMax32(0, sidebar.bounds.Width-metrics.GapItem*2-1), Height: height,
	}
	// p-2 below the header plus SidebarGroup's p-2 above the first group.
	sidebar.y = rect.Y + height + metrics.GapItem*2
	return rect
}

// AddFooter reserves SidebarFooter: a p-2 padded block of the given content
// height against the bottom edge, above any footer added before it. It returns
// the content rectangle and leaves less room for the content above.
func (sidebar *Sidebar) AddFooter(height float32) gfx.Rect {
	if sidebar.hidden {
		return gfx.Rect{}
	}
	metrics := sidebar.theme.Metrics
	bottom := sidebar.bounds.Y + sidebar.bounds.Height - metrics.GapItem
	if sidebar.footerTop > 0 {
		bottom = sidebar.footerTop - metrics.GapItem/2
	}
	rect := gfx.Rect{
		X: sidebar.bounds.X + metrics.GapItem, Y: bottom - height,
		Width: sidebarMax32(0, sidebar.bounds.Width-metrics.GapItem*2-1), Height: height,
	}
	sidebar.footerTop = rect.Y
	return rect
}

// AddGroup paints a SidebarGroupLabel: a 32px, medium-weight caption at 70%
// foreground, without transforming caller text, preserving localization and
// acronyms exactly. A group that follows other content is separated from it by
// the groups' p-2 and the content's gap-2.
func (sidebar *Sidebar) AddGroup(label string) {
	if sidebar.hidden || sidebar.icon {
		return
	}
	metrics := sidebar.theme.Metrics
	if sidebar.started {
		sidebar.y += metrics.GapItem*2 + metrics.GapItem/2
	}
	height := metrics.ControlHeight
	if !sidebar.hasRoom(height) {
		return
	}
	color := sidebar.theme.Foreground
	color.Alpha *= 0.70
	sidebar.canvas.Label(label, gfx.Rect{
		X:      sidebar.bounds.X + metrics.GapItem*2,
		Y:      sidebar.y,
		Width:  sidebarMax32(0, sidebar.bounds.Width-metrics.GapItem*4-1),
		Height: height,
	}, gfx.TextStyle{Size: metrics.FontSizeS, Weight: gfx.WeightMedium, Color: color})
	sidebar.y += height
	sidebar.started = true
}

// AddItem paints one navigation destination. Selected items use a restrained
// blue accent; disabled items remain visible but emit no hit region.
func (sidebar *Sidebar) AddItem(id gfx.ID, label, trailing string, selected, disabled bool) gfx.Rect {
	return sidebar.AddMenuItem(SidebarMenuItem{ID: id, Label: label, Trailing: trailing, Active: selected, Disabled: disabled})
}

// sidebarActionSize is SidebarMenuAction's w-5 square.
const sidebarActionSize = 20

// AddMenuItem paints full Base UI menu-button state and size variants.
func (sidebar *Sidebar) AddMenuItem(item SidebarMenuItem) gfx.Rect {
	if sidebar.hidden {
		return gfx.Rect{}
	}
	metrics := sidebar.theme.Metrics
	height := sidebar.itemHeight(item.Size)
	if !sidebar.hasRoom(height) {
		return gfx.Rect{}
	}
	sidebar.started = true
	itemBounds := gfx.Rect{
		X:      sidebar.bounds.X + metrics.GapItem,
		Y:      sidebar.y,
		Width:  sidebarMax32(0, sidebar.bounds.Width-metrics.GapItem*2-1),
		Height: height,
	}
	if sidebar.icon {
		itemBounds.Width = height
	}
	foreground := sidebar.theme.Foreground
	background := sidebar.theme.CardElevated
	paintBackground := false
	interactive := !item.ID.IsZero() && !item.Disabled
	// Active, pressed and hovered buttons all take sidebar-accent; only the font
	// weight tells the active one apart.
	hovered := interactive && sidebar.canvas.Hovered() == item.ID
	if item.Active || interactive && sidebar.canvas.Pressed() == item.ID || hovered {
		background = sidebar.theme.Accent
		foreground = sidebar.theme.AccentForeground
		paintBackground = true
	}
	if item.Disabled {
		foreground = sidebar.theme.MutedForeground
	}
	radius := metrics.RadiusItem()
	if interactive && sidebar.canvas.FocusVisible() == item.ID {
		ring := sidebar.theme.Primary
		ring.Alpha = 0.28
		sidebar.canvas.Rounded(gfx.Rect{X: itemBounds.X - 1.5, Y: itemBounds.Y - 1.5, Width: itemBounds.Width + 3, Height: itemBounds.Height + 3}, radius+1.5, ring)
		paintBackground = true
	}
	if item.Variant == SidebarMenuOutline {
		// bg-background with a 1px sidebar-border ring that turns sidebar-accent on hover.
		fill := sidebar.theme.Background
		border := sidebar.theme.Border
		if paintBackground {
			fill = background
		}
		if hovered {
			border = sidebar.theme.Accent
		}
		sidebar.canvas.RoundedBorder(itemBounds, radius, 1, border, fill)
	} else if paintBackground {
		sidebar.canvas.Rounded(itemBounds, radius, background)
	}
	weight := gfx.WeightRegular
	if item.Active {
		weight = gfx.WeightMedium
	}
	if sidebar.icon {
		if item.VectorIcon.ViewBox > 0 {
			sidebar.canvas.DrawIcon(item.VectorIcon, gfx.Rect{X: itemBounds.X + (itemBounds.Width-16)/2, Y: itemBounds.Y + (itemBounds.Height-16)/2, Width: 16, Height: 16}, foreground)
		} else {
			icon := item.Icon
			if icon == "" {
				icon = "•"
			}
			sidebar.canvas.LabelCenter(icon, itemBounds, gfx.TextStyle{Size: metrics.FontSizeM, Weight: weight, Color: foreground})
		}
		if interactive {
			sidebar.canvas.HitWithOptions(item.ID, itemBounds, item.Label, gfx.HitOptions{Role: gfx.RoleMenuItem, Selected: item.Active})
		}
		sidebar.y += height + metrics.GapItem/2
		return itemBounds
	}
	action := item.Action
	hasAction := !action.ID.IsZero()
	// p-2 each side; an action widens the trailing padding to pr-8.
	textX := itemBounds.X + metrics.GapItem
	labelWidth := itemBounds.Width - metrics.GapItem*2
	if hasAction {
		labelWidth -= metrics.GapItem * 3
	}
	if item.Trailing != "" {
		// The badge is right-1 with px-1: its text ends 8px from the edge.
		trailingWidth := sidebar.canvas.TextWidthWeight(item.Trailing, metrics.FontSizeS, gfx.WeightMedium)
		sidebar.canvas.Label(item.Trailing, gfx.Rect{
			X: itemBounds.X + itemBounds.Width - metrics.GapItem - trailingWidth,
			Y: itemBounds.Y, Width: trailingWidth, Height: itemBounds.Height,
		}, gfx.TextStyle{Size: metrics.FontSizeS, Weight: gfx.WeightMedium, Color: foreground})
		labelWidth -= trailingWidth + metrics.GapItem
	}
	if item.VectorIcon.ViewBox > 0 {
		sidebar.canvas.DrawIcon(item.VectorIcon, gfx.Rect{X: textX, Y: itemBounds.Y + (itemBounds.Height-16)/2, Width: 16, Height: 16}, foreground)
		textX += 16 + metrics.GapItem
		labelWidth -= 16 + metrics.GapItem
	} else if item.Icon != "" {
		sidebar.canvas.LabelCenter(item.Icon, gfx.Rect{X: textX, Y: itemBounds.Y, Width: 16, Height: itemBounds.Height}, gfx.TextStyle{Size: metrics.FontSizeM, Weight: gfx.WeightRegular, Color: foreground})
		textX += 16 + metrics.GapItem
		labelWidth -= 16 + metrics.GapItem
	}
	fontSize := metrics.FontSizeM
	if item.Size == SidebarMenuSizeSmall {
		fontSize = metrics.FontSizeS
	}
	sidebar.canvas.Label(item.Label, gfx.Rect{
		X: textX, Y: itemBounds.Y,
		Width: sidebarMax32(0, labelWidth), Height: itemBounds.Height,
	}, gfx.TextStyle{Size: fontSize, Weight: weight, Color: foreground})
	if interactive {
		// The action sits on top of the button; hit regions of one scope may not
		// overlap, so the button's own ends where the action starts.
		hit := itemBounds
		if hasAction {
			hit.Width = sidebar.actionBounds(itemBounds, item.Size).X - hit.X
		}
		sidebar.canvas.HitWithOptions(item.ID, hit, item.Label, gfx.HitOptions{Role: gfx.RoleMenuItem, Selected: item.Active})
	}
	if hasAction {
		sidebar.paintMenuAction(itemBounds, item, hovered)
	}
	sidebar.y += height + metrics.GapItem/2
	return itemBounds
}

// paintMenuAction paints and declares an item's secondary button.
func (sidebar *Sidebar) paintMenuAction(itemBounds gfx.Rect, item SidebarMenuItem, itemHovered bool) {
	action := item.Action
	metrics := sidebar.theme.Metrics
	hovered := sidebar.canvas.Hovered() == action.ID
	focused := sidebar.canvas.FocusVisible() == action.ID
	if action.ShowOnHover && !itemHovered && !hovered && !focused {
		// Hidden, but still reachable by keyboard like an opacity-0 button.
		sidebar.canvas.HitWithOptions(action.ID, sidebar.actionBounds(itemBounds, item.Size), action.Label, gfx.HitOptions{Role: gfx.RoleButton, Disabled: item.Disabled})
		return
	}
	bounds := sidebar.actionBounds(itemBounds, item.Size)
	color := sidebar.theme.Foreground
	if hovered || sidebar.canvas.Pressed() == action.ID {
		sidebar.canvas.Rounded(bounds, metrics.RadiusItem(), sidebar.theme.Accent)
		color = sidebar.theme.AccentForeground
	} else if itemHovered || item.Active {
		color = sidebar.theme.AccentForeground
	}
	if focused {
		ring := sidebar.theme.Primary
		ring.Alpha = 0.28
		sidebar.canvas.Rounded(expand(bounds, 1.5), metrics.RadiusItem()+1.5, ring)
	}
	if action.Icon.ViewBox > 0 {
		sidebar.canvas.DrawIcon(action.Icon, gfx.Rect{X: bounds.X + (bounds.Width-16)/2, Y: bounds.Y + (bounds.Height-16)/2, Width: 16, Height: 16}, color)
	}
	sidebar.canvas.HitWithOptions(action.ID, bounds, action.Label, gfx.HitOptions{Role: gfx.RoleButton, Disabled: item.Disabled})
}

// actionBounds is right-1 with the action's top at top-1.5/top-1/top-2.5 for the
// default, small and large buttons.
func (sidebar *Sidebar) actionBounds(itemBounds gfx.Rect, size SidebarMenuSize) gfx.Rect {
	top := (itemBounds.Height - sidebarActionSize) / 2
	if size == SidebarMenuSizeLarge {
		top = 10
	}
	return gfx.Rect{
		X: itemBounds.X + itemBounds.Width - sidebar.theme.Metrics.GapItem/2 - sidebarActionSize, Y: itemBounds.Y + top,
		Width: sidebarActionSize, Height: sidebarActionSize,
	}
}

// AddSubMenu paints SidebarMenuSub under the item just added: a one pixel rule
// indented from the menu with 28px buttons beside it. It returns the rectangle
// the sub menu occupies. Icon-collapsed sidebars hide sub menus.
func (sidebar *Sidebar) AddSubMenu(items []SidebarSubItem) gfx.Rect {
	if sidebar.hidden || sidebar.icon || len(items) == 0 {
		return gfx.Rect{}
	}
	metrics := sidebar.theme.Metrics
	buttonHeight := metrics.ControlHeight - 4
	gap := metrics.GapItem / 2
	pad := metrics.GapItem / 4 // py-0.5
	height := pad*2 + buttonHeight*float32(len(items)) + gap*float32(len(items)-1)
	if !sidebar.hasRoom(height) {
		return gfx.Rect{}
	}
	margin := metrics.GapItem * 1.75 // mx-3.5
	inset := metrics.GapItem * 1.25  // px-2.5
	rule := gfx.Rect{X: sidebar.bounds.X + metrics.GapItem + margin + 1, Y: sidebar.y, Width: 1, Height: height}
	right := sidebar.bounds.X + sidebar.bounds.Width - 1 - metrics.GapItem - margin
	sidebar.canvas.Fill(rule, sidebar.theme.Border)
	x := rule.X + 1 + inset
	width := sidebarMax32(0, right-inset-x)
	y := sidebar.y + pad
	for _, item := range items {
		bounds := gfx.Rect{X: x, Y: y, Width: width, Height: buttonHeight}
		interactive := !item.ID.IsZero() && !item.Disabled
		foreground := sidebar.theme.Foreground
		hovered := interactive && sidebar.canvas.Hovered() == item.ID
		if item.Active || hovered || interactive && sidebar.canvas.Pressed() == item.ID {
			sidebar.canvas.Rounded(bounds, metrics.RadiusItem(), sidebar.theme.Accent)
			foreground = sidebar.theme.AccentForeground
		}
		if item.Disabled {
			foreground = sidebar.theme.MutedForeground
		}
		if interactive && sidebar.canvas.FocusVisible() == item.ID {
			ring := sidebar.theme.Primary
			ring.Alpha = 0.28
			sidebar.canvas.Rounded(expand(bounds, 1.5), metrics.RadiusItem()+1.5, ring)
		}
		size := metrics.FontSizeM
		if item.Size == SidebarSubSizeSmall {
			size = metrics.FontSizeS
		}
		sidebar.canvas.Label(item.Label, gfx.Rect{X: bounds.X + metrics.GapItem, Y: bounds.Y, Width: sidebarMax32(0, bounds.Width-metrics.GapItem*2), Height: bounds.Height},
			gfx.TextStyle{Size: size, Color: foreground})
		if interactive {
			sidebar.canvas.HitWithOptions(item.ID, bounds, item.Label, gfx.HitOptions{Role: gfx.RoleMenuItem, Selected: item.Active})
		}
		y += buttonHeight + gap
	}
	sidebar.y += height + gap
	return gfx.Rect{X: rule.X, Y: rule.Y, Width: right - rule.X, Height: height}
}

// AddFooterMenuItem paints one menu item as a footer, against the bottom edge
// or above the footers added before it, without changing the insertion point
// for ordinary menu content.
func (sidebar *Sidebar) AddFooterMenuItem(item SidebarMenuItem) gfx.Rect {
	if sidebar.hidden {
		return gfx.Rect{}
	}
	height := sidebar.itemHeight(item.Size)
	footer := sidebar.AddFooter(height)
	reserved := sidebar.footerTop
	previous, started := sidebar.y, sidebar.started
	// The item is painted inside the room it just reserved, so that room must not
	// count against it.
	sidebar.footerTop = 0
	sidebar.y = footer.Y
	bounds := sidebar.AddMenuItem(item)
	sidebar.y, sidebar.started, sidebar.footerTop = previous, started, reserved
	return bounds
}

// AddSeparator inserts a SidebarSeparator: a divider inset by mx-2.
func (sidebar *Sidebar) AddSeparator() {
	if sidebar.hidden {
		return
	}
	metrics := sidebar.theme.Metrics
	height := metrics.GapItem + 1
	if !sidebar.hasRoom(height) {
		return
	}
	sidebar.canvas.Fill(gfx.Rect{
		X: sidebar.bounds.X + metrics.GapItem, Y: sidebar.y + metrics.GapItem/2,
		Width: sidebarMax32(0, sidebar.bounds.Width-metrics.GapItem*2-1), Height: 1,
	}, sidebar.theme.Border)
	sidebar.y += height
}

// limit is the lowest y the content may reach: the bottom edge, or the top of
// the footers with the content's gap-2 above them.
func (sidebar Sidebar) limit() float32 {
	if sidebar.footerTop > 0 {
		return sidebar.footerTop - sidebar.theme.Metrics.GapItem
	}
	return sidebar.bounds.Y + sidebar.bounds.Height
}

func (sidebar Sidebar) hasRoom(height float32) bool {
	return height > 0 && sidebar.y+height <= sidebar.limit()
}

func (sidebar Sidebar) itemHeight(size SidebarMenuSize) float32 {
	height := sidebar.theme.Metrics.ControlHeight
	switch size {
	case SidebarMenuSizeSmall:
		height -= 4
	case SidebarMenuSizeLarge:
		height *= 1.5 // h-12 beside h-8
	}
	return height
}

func sidebarMax32(left, right float32) float32 {
	if left > right {
		return left
	}
	return right
}
