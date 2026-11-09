package components

import "github.com/vivaui/viva/gfx"

// ItemVariant selects the neutral surface treatment for an item row.
type ItemVariant uint8

const (
	ItemDefault ItemVariant = iota
	ItemOutline
	ItemMuted
)

// ItemSize selects the Vega spacing and typography density for an item.
type ItemSize uint8

const (
	ItemSizeDefault ItemSize = iota
	ItemSizeSmall
	ItemSizeExtraSmall
)

// ItemMediaVariant mirrors ItemMedia's icon and image variants.
type ItemMediaVariant uint8

const (
	// ItemMediaIcon is a bare 16px icon.
	ItemMediaIcon ItemMediaVariant = iota
	// ItemMediaImage is a square thumbnail or avatar: size-10, size-8 in a small
	// item and size-6 in an extra-small one.
	ItemMediaImage
)

// ItemMediaSize is the side of the media box for a variant at an item size.
func ItemMediaSize(theme gfx.Theme, size ItemSize, variant ItemMediaVariant) float32 {
	if variant == ItemMediaIcon {
		return 16
	}
	switch size {
	case ItemSizeSmall:
		return theme.Metrics.ControlHeight
	case ItemSizeExtraSmall:
		return theme.Metrics.ControlHeight - 8
	}
	return theme.Metrics.ControlHeight + 8
}

// ItemLayout is where a painted item put its parts, for the caller to fill in.
type ItemLayout struct {
	// Media is the box reserved by Media or LeadingSpace; empty when none was.
	Media gfx.Rect
	// Content is the column holding the title and description.
	Content gfx.Rect
}

// ItemGroupGap is the spacing between items of a group (ItemGroup's gap-4, or
// gap-2.5 and gap-2 for small and extra-small items).
func ItemGroupGap(size ItemSize) float32 {
	switch size {
	case ItemSizeSmall:
		return 10
	case ItemSizeExtraSmall:
		return 8
	}
	return 16
}

// ItemSeparatorHeight is the room an ItemSeparator takes: a hairline with my-2
// above and below.
const ItemSeparatorHeight = 17

// PaintItemSeparator paints an ItemSeparator across bounds' width at its top and
// returns the height it used (ItemSeparatorHeight).
func PaintItemSeparator(canvas *gfx.Canvas, theme gfx.Theme, bounds gfx.Rect) float32 {
	canvas.Fill(gfx.Rect{X: bounds.X, Y: bounds.Y + 8, Width: bounds.Width, Height: 1}, theme.Border)
	return ItemSeparatorHeight
}

// Item is one source-owned list row declaration. It keeps layout and transient
// interaction state explicit and allocates no auxiliary collections.
type Item struct {
	canvas   *gfx.Canvas
	theme    gfx.Theme
	id       gfx.ID
	bounds   gfx.Rect
	variant  ItemVariant
	size     ItemSize
	leading  float32
	disabled bool
}

// BeginItem starts an item row. ID 0 paints a preview without interaction.
func BeginItem(canvas *gfx.Canvas, theme gfx.Theme, id gfx.ID, bounds gfx.Rect) Item {
	return Item{canvas: canvas, theme: theme, id: id, bounds: bounds}
}

// Variant selects the row surface treatment.
func (item Item) Variant(variant ItemVariant) Item {
	item.variant = variant
	return item
}

// Size selects default, small, or extra-small Vega item spacing.
func (item Item) Size(size ItemSize) Item {
	item.size = size
	return item
}

// LeadingSpace reserves room plus the standard item gap for caller-painted
// media such as an icon or avatar.
func (item Item) LeadingSpace(width float32) Item {
	if width > 0 {
		item.leading = width
	}
	return item
}

// Media reserves the standard ItemMedia box of a variant for the item's size.
// Call it after Size. Paint returns the box, which the caller paints into.
func (item Item) Media(variant ItemMediaVariant) Item {
	item.leading = ItemMediaSize(item.theme, item.size, variant)
	return item
}

// Disabled renders and routes the row as inert.
func (item Item) Disabled() Item {
	item.disabled = true
	return item
}

// Paint draws the title, optional description, and optional trailing label, and
// returns the media box and text column it laid out.
func (item Item) Paint(title, description, trailing string) ItemLayout {
	metrics := item.theme.Metrics
	padX, padY, contentGap, descriptionSize := item.metrics()
	titleHeight := metrics.FontSizeM + 5
	descriptionHeight := descriptionSize + 7
	// ItemContent's gap-1, none in an extra-small item.
	textGap := float32(4)
	if item.size == ItemSizeExtraSmall {
		textGap = 0
	}
	if item.bounds.Height <= 0 {
		item.bounds.Height = padY*2 + titleHeight
		if description != "" {
			item.bounds.Height += textGap + descriptionHeight
		}
	}

	hovered := !item.disabled && !item.id.IsZero() && item.canvas.Hovered() == item.id
	pressed := !item.disabled && !item.id.IsZero() && item.canvas.Pressed() == item.id
	focused := !item.disabled && !item.id.IsZero() && item.canvas.FocusVisible() == item.id
	if focused {
		focus := item.theme.Ring
		focus.Alpha *= 0.50
		item.canvas.RoundedBorder(expand(item.bounds, 3), metrics.RadiusControl+3, 3, focus, gfx.Color{})
	}

	background := gfx.Color{}
	if item.variant == ItemMuted {
		background = item.theme.Muted
		background.Alpha *= 0.50
	}
	if hovered {
		background = item.theme.Muted
	}
	if pressed {
		background = item.theme.Accent
	}
	if item.variant == ItemOutline {
		item.canvas.RoundedBorder(item.bounds, metrics.RadiusControl, 1, item.theme.Border, background)
	} else if background.Alpha > 0 {
		item.canvas.Rounded(item.bounds, metrics.RadiusControl, background)
	}

	foreground := item.theme.Foreground
	secondary := item.theme.MutedForeground
	if item.disabled {
		foreground.Alpha *= 0.50
		secondary.Alpha *= 0.50
	}
	textX := item.bounds.X + padX
	if item.leading > 0 {
		textX += item.leading + contentGap
	}
	trailingWidth := float32(0)
	if trailing != "" {
		trailingWidth = item.canvas.TextWidthWeight(trailing, descriptionSize, gfx.WeightMedium)
		item.canvas.Label(trailing, gfx.Rect{
			X: item.bounds.X + item.bounds.Width - padX - trailingWidth,
			Y: item.bounds.Y, Width: trailingWidth, Height: item.bounds.Height,
		}, gfx.TextStyle{
			Size: descriptionSize, Weight: gfx.WeightMedium, Color: secondary,
		})
	}
	var layout ItemLayout
	if item.leading > 0 {
		// Beside a description the media is self-start (nudged 2px down); beside a
		// title alone it is centered.
		mediaY := item.bounds.Y + (item.bounds.Height-item.leading)/2
		if description != "" {
			mediaY = item.bounds.Y + padY + 2
		}
		layout.Media = gfx.Rect{X: item.bounds.X + padX, Y: mediaY, Width: item.leading, Height: item.leading}
	}
	textWidth := item.bounds.Width - (textX - item.bounds.X) - padX
	if trailingWidth > 0 {
		textWidth -= trailingWidth + contentGap
	}
	if textWidth < 0 {
		textWidth = 0
	}
	layout.Content = gfx.Rect{X: textX, Y: item.bounds.Y + padY, Width: textWidth, Height: item.bounds.Height - padY*2}
	if description == "" {
		item.canvas.Label(title, gfx.Rect{
			X: textX, Y: item.bounds.Y + padY, Width: textWidth, Height: titleHeight,
		}, gfx.TextStyle{
			Size: metrics.FontSizeM, Weight: gfx.WeightMedium, Color: foreground,
		})
	} else {
		textY := item.bounds.Y + padY
		item.canvas.Label(title, gfx.Rect{
			X: textX, Y: textY, Width: textWidth, Height: titleHeight,
		}, gfx.TextStyle{
			Size: metrics.FontSizeM, Weight: gfx.WeightMedium, Color: foreground,
		})
		availableDescriptionHeight := item.bounds.Height - padY*2 - titleHeight - textGap
		if availableDescriptionHeight < 0 {
			availableDescriptionHeight = 0
		}
		item.canvas.Label(description, gfx.Rect{
			X: textX, Y: textY + titleHeight + textGap,
			Width: textWidth, Height: availableDescriptionHeight,
		}, gfx.TextStyle{
			Size: descriptionSize, Weight: gfx.WeightRegular, Color: secondary,
		})
	}
	if !item.disabled && !item.id.IsZero() {
		item.canvas.HitWithLabel(item.id, item.bounds, title)
	}
	return layout
}

func (item Item) metrics() (padX, padY, gap, descriptionSize float32) {
	metrics := item.theme.Metrics
	switch item.size {
	case ItemSizeSmall:
		return metrics.PadControlX, metrics.PadControlX - 2, metrics.PadControlX - 2, metrics.FontSizeM
	case ItemSizeExtraSmall:
		return metrics.PadControlX - 2, metrics.GapItem, metrics.GapItem, metrics.FontSizeS
	default:
		return metrics.PadSurface, metrics.PadSurface - 2, metrics.PadSurface - 2, metrics.FontSizeM
	}
}
