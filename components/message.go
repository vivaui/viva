package components

import "github.com/vivaui/viva/gfx"

// MessageAlignment mirrors Base UI's start/end row direction.
type MessageAlignment uint8

const (
	MessageStart MessageAlignment = iota
	MessageEnd
)

// MessageData contains visible message metadata. Bubble content remains a
// separate component, matching the official Message/MessageContent split.
type MessageData struct {
	Header     string
	Footer     string
	Initials   string
	Alignment  MessageAlignment
	ShowAvatar bool
	// ReserveAvatar keeps the avatar column without painting an avatar, the
	// official empty MessageAvatar: consecutive messages of one sender line up
	// with the one that shows the avatar. ShowAvatar takes precedence.
	ReserveAvatar bool
	// GhostContent says the content slot holds a ghost bubble, which has no
	// surface of its own, so the header and footer lose their 12px inset.
	GhostContent bool
}

// MessageRegions exposes retained slots a caller may compose into.
type MessageRegions struct {
	Avatar  gfx.Rect
	Header  gfx.Rect
	Content gfx.Rect
	Footer  gfx.Rect
}

// MessageSlotGap is the space between header, content and footer: the official
// MessageContent gap-2.5, 10px at the default density.
func MessageSlotGap(metrics gfx.Metrics) float32 { return metrics.GapItem * 1.25 }

// MessageGroupGap is the space between messages of one group, the official
// MessageGroup gap-2.
func MessageGroupGap(metrics gfx.Metrics) float32 { return metrics.GapItem }

// MessageGroup stacks the rows of consecutive messages MessageGroupGap apart. It
// only measures; each row is then handed to Message.
type MessageGroup struct {
	bounds gfx.Rect
	gap    float32
	y      float32
}

// BeginMessageGroup starts stacking message rows from the top of bounds.
func BeginMessageGroup(theme gfx.Theme, bounds gfx.Rect) MessageGroup {
	return MessageGroup{bounds: bounds, gap: MessageGroupGap(theme.Metrics), y: bounds.Y}
}

// Row reserves the next message row of the given height and returns its bounds.
func (group *MessageGroup) Row(height float32) gfx.Rect {
	row := gfx.Rect{X: group.bounds.X, Y: group.y, Width: group.bounds.Width, Height: height}
	group.y += height + group.gap
	return row
}

// Height is the stacked height of the rows reserved so far, without a trailing gap.
func (group MessageGroup) Height() float32 {
	if group.y == group.bounds.Y {
		return 0
	}
	return group.y - group.gap - group.bounds.Y
}

// Message paints optional avatar/header/footer metadata and returns the content
// slot. It does not own body text or implicitly choose a bubble variant.
func Message(canvas *gfx.Canvas, theme gfx.Theme, bounds gfx.Rect, data MessageData) MessageRegions {
	metrics := theme.Metrics
	if bounds.Height <= 0 {
		bounds.Height = metrics.ControlHeight*2 + metrics.GapItem
	}
	hasAvatar := data.ShowAvatar || data.ReserveAvatar
	avatarWidth := float32(0)
	if hasAvatar {
		avatarWidth = metrics.ControlHeight + metrics.GapItem
	}
	contentWidth := messageMax32(0, bounds.Width-avatarWidth)
	contentX := bounds.X + avatarWidth
	avatarX := bounds.X
	if data.Alignment == MessageEnd {
		contentX = bounds.X
		avatarX = bounds.X + bounds.Width - metrics.ControlHeight
	}
	gap := MessageSlotGap(metrics)
	headerHeight, headerGap := float32(0), float32(0)
	if data.Header != "" {
		headerHeight, headerGap = metrics.FontSizeS+7, gap
	}
	footerHeight, footerGap := float32(0), float32(0)
	if data.Footer != "" {
		footerHeight, footerGap = metrics.ControlHeight, gap
	}
	regions := MessageRegions{}
	regions.Header = gfx.Rect{X: contentX, Y: bounds.Y, Width: contentWidth, Height: headerHeight}
	regions.Content = gfx.Rect{X: contentX, Y: bounds.Y + headerHeight + headerGap, Width: contentWidth, Height: messageMax32(0, bounds.Height-headerHeight-headerGap-footerHeight-footerGap)}
	regions.Footer = gfx.Rect{X: contentX, Y: bounds.Y + bounds.Height - footerHeight, Width: contentWidth, Height: footerHeight}
	if hasAvatar {
		// The avatar rests on the bottom of the content, above the footer.
		avatarY := bounds.Y + bounds.Height - metrics.ControlHeight - footerHeight
		regions.Avatar = gfx.Rect{X: avatarX, Y: avatarY, Width: metrics.ControlHeight, Height: metrics.ControlHeight}
		if data.ShowAvatar {
			messageAvatar(canvas, theme, regions.Avatar, data.Initials)
		}
	}
	inset := metrics.PadControlX
	if data.GhostContent {
		inset = 0
	}
	// Header and footer sit on the side the message is aligned to: the official
	// MessageContent gives its slots self-end in an end-aligned message.
	alignment := gfx.AlignLeft
	if data.Alignment == MessageEnd {
		alignment = gfx.AlignRight
	}
	style := gfx.TextStyle{Size: metrics.FontSizeS, Weight: gfx.WeightMedium, Color: theme.MutedForeground}
	if data.Header != "" {
		headerBounds := regions.Header
		headerBounds.X += inset
		headerBounds.Width = messageMax32(0, headerBounds.Width-inset*2)
		canvas.LabelAligned(data.Header, headerBounds, style, alignment)
	}
	if data.Footer != "" {
		footerBounds := regions.Footer
		footerBounds.X += inset
		footerBounds.Width = messageMax32(0, footerBounds.Width-inset*2)
		canvas.LabelAligned(data.Footer, footerBounds, style, alignment)
	}
	return regions
}

func messageAvatar(canvas *gfx.Canvas, theme gfx.Theme, bounds gfx.Rect, initials string) {
	canvas.Rounded(bounds, bounds.Width/2, theme.Muted)
	if initials != "" {
		canvas.LabelCenter(initials, bounds, gfx.TextStyle{Size: theme.Metrics.FontSizeS, Weight: gfx.WeightSemibold, Color: theme.MutedForeground})
		return
	}
	head := bounds.Width * 0.24
	canvas.Rounded(gfx.Rect{X: bounds.X + (bounds.Width-head)/2, Y: bounds.Y + bounds.Height*0.20, Width: head, Height: head}, head/2, theme.MutedForeground)
	shouldersHeight := bounds.Height * 0.22
	canvas.Rounded(gfx.Rect{X: bounds.X + bounds.Width*0.25, Y: bounds.Y + bounds.Height*0.58, Width: bounds.Width * 0.50, Height: shouldersHeight}, shouldersHeight/2, theme.MutedForeground)
}

func messageMax32(left, right float32) float32 {
	if left > right {
		return left
	}
	return right
}
