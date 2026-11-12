package components

import "github.com/vivaui/viva/gfx"

// AttachmentKind selects the media placeholder used until Canvas gains image
// resources.
type AttachmentKind uint8

const (
	AttachmentFile AttachmentKind = iota
	AttachmentImage
)

// AttachmentState mirrors Base UI's idle, uploading, processing, error, and
// done states. Done is the zero value so ordinary attachments need no setup.
type AttachmentState uint8

const (
	AttachmentDone AttachmentState = iota
	AttachmentIdle
	AttachmentUploading
	AttachmentProcessing
	AttachmentError
)

// AttachmentSize mirrors the official default, sm, and xs variants.
type AttachmentSize uint8

const (
	AttachmentSizeDefault AttachmentSize = iota
	AttachmentSizeSmall
	AttachmentSizeExtraSmall
)

// AttachmentOrientation mirrors the horizontal and vertical variants.
type AttachmentOrientation uint8

const (
	AttachmentHorizontal AttachmentOrientation = iota
	AttachmentVertical
)

// AttachmentData is the typed source boundary for one attachment. ResourceID
// is caller-owned and intentionally unresolved until Canvas exposes images.
type AttachmentData struct {
	Name        string
	Description string
	Kind        AttachmentKind
	ResourceID  uint64
	// Icon, when it has any commands, is painted in the media square instead of the
	// placeholder shape of Kind: a file-type or brand glyph the application owns.
	Icon        gfx.Icon
	State       AttachmentState
	Size        AttachmentSize
	Orientation AttachmentOrientation
	// Progress is the fraction, 0 to 1, of an Uploading attachment that has been sent;
	// it draws the bar along the bottom edge. The zero value draws no bar, and the
	// other states have no determinate progress to show.
	Progress float32
}

// AttachmentAction is one icon button of an attachment, such as remove or retry. It is
// the official AttachmentAction: a ghost icon-xs button that sits above the
// attachment's own trigger, so pressing it never presses the attachment.
type AttachmentAction struct {
	ID    gfx.ID
	Label string // accessible name, "Remove"
	Icon  gfx.Icon
}

// attachmentMaxActions is how many AttachmentActions one attachment holds.
const attachmentMaxActions = 3

// Attachment is one optional full-surface trigger around attachment content.
type Attachment struct {
	canvas      *gfx.Canvas
	theme       gfx.Theme
	id          gfx.ID
	bounds      gfx.Rect
	data        AttachmentData
	disabled    bool
	actions     [attachmentMaxActions]AttachmentAction
	actionCount uint8
}

// BeginAttachment starts an attachment declaration. ID 0 paints static content.
// A bounds width or height of zero means the attachment's natural size (see Size),
// at the bounds origin.
func BeginAttachment(canvas *gfx.Canvas, theme gfx.Theme, id gfx.ID, bounds gfx.Rect, data AttachmentData) Attachment {
	return Attachment{canvas: canvas, theme: theme, id: id, bounds: bounds, data: data}
}

// Disabled makes the trigger inert without changing the attachment state.
func (attachment Attachment) Disabled() Attachment {
	attachment.disabled = true
	return attachment
}

// Action adds an icon button, up to three, in the order given: at the end of a
// horizontal attachment, in the top right corner of a vertical one. The action
// has its own hit above the attachment's, so declare the attachment with an ID if
// the whole card should also be pressable.
func (attachment Attachment) Action(action AttachmentAction) Attachment {
	if int(attachment.actionCount) < len(attachment.actions) {
		attachment.actions[attachment.actionCount] = action
		attachment.actionCount++
	}
	return attachment
}

// attachmentMetrics are the sizes of one attachment at a size, orientation and density.
type attachmentMetrics struct {
	border, padX, padY, gap float32
	media                   float32 // width of the media square; vertical: set by the width
	titleSize, descSize     float32
	titleHeight, descHeight float32
	descGap                 float32
	radius, mediaRadius     float32
	action                  float32
	textInset               float32 // vertical content's px-1
	minWidth                float32
	hasText                 bool
}

// metrics resolves the Vega attachment sizes: default, sm and xs differ in media
// width, padding, gap and text size; a vertical attachment is 96 wide, or 120 with
// text, and its media fills the width as a square.
func (attachment Attachment) metrics() attachmentMetrics {
	metrics := attachment.theme.Metrics
	scale := metrics.ControlHeight / 32
	m := attachmentMetrics{
		border: 1, hasText: attachment.data.Name != "" || attachment.data.Description != "",
		titleSize: metrics.FontSizeM, descSize: metrics.FontSizeS,
		radius: radiusXL(metrics), mediaRadius: metrics.RadiusSurface,
		descGap: 2 * scale, action: metrics.ControlHeight - 8, textInset: 4 * scale,
	}
	switch attachment.data.Size {
	case AttachmentSizeSmall:
		m.media, m.padX, m.padY, m.gap = 32*scale, 8*scale, 6*scale, 10*scale
		m.titleSize = metrics.FontSizeS
	case AttachmentSizeExtraSmall:
		m.media, m.padX, m.padY, m.gap = 28*scale, 6*scale, 4*scale, 6*scale
		m.titleSize = metrics.FontSizeS
		m.radius, m.mediaRadius = metrics.RadiusSurface, metrics.RadiusControl
	default:
		m.media, m.padX, m.padY, m.gap = 40*scale, 10*scale, 8*scale, 8*scale
	}
	if !m.hasText { // media alone: p-2, p-1.5, p-1
		m.padX = m.padY
	}
	m.titleHeight, m.descHeight = m.titleSize*1.25, m.descSize*1.25
	m.minWidth = 160 * scale
	if attachment.data.Orientation == AttachmentVertical {
		m.minWidth = 96 * scale
		if m.hasText {
			m.minWidth = 120 * scale
		}
		m.media = m.minWidth - (m.border+m.padX)*2
	}
	return m
}

func (m attachmentMetrics) textHeight(description bool) float32 {
	height := m.titleHeight
	if description {
		height += m.descGap + m.descHeight
	}
	return height
}

// Size is the attachment's natural size: as wide as its text, media and actions
// need but at least the official minimum width (a vertical attachment is a fixed
// width), and as tall as its media or text.
func (attachment Attachment) Size() gfx.Size {
	m := attachment.metrics()
	hasDescription := attachment.data.Description != ""
	if attachment.data.Orientation == AttachmentVertical {
		height := (m.border+m.padY)*2 + m.media
		if m.hasText {
			height += m.gap + m.textHeight(hasDescription)
		}
		return gfx.Size{Width: m.minWidth, Height: height}
	}
	width := (m.border+m.padX)*2 + m.media
	if m.hasText {
		text := float32(0)
		if attachment.canvas != nil {
			text = attachment.canvas.TextWidthWeight(attachment.data.Name, m.titleSize, gfx.WeightMedium)
			if hasDescription {
				text = max(text, attachment.canvas.TextWidthWeight(attachment.data.Description, m.descSize, gfx.WeightRegular))
			}
		}
		width += m.gap + text + 2*m.descGap // the platform draws a line a pixel or two wider than it measures
	}
	if attachment.actionCount > 0 {
		width += m.gap + float32(attachment.actionCount)*m.action
	}
	height := max(m.media, m.textHeight(hasDescription), m.action*min(float32(attachment.actionCount), 1))
	return gfx.Size{Width: max(width, m.minWidth), Height: (m.border+m.padY)*2 + height}
}

// Paint emits state, size, orientation, media, content, and trigger commands.
// Uploading and processing show a dimmed title, and an uploading attachment its
// progress bar; neither runs a hidden animation worker.
func (attachment Attachment) Paint() {
	m := attachment.metrics()
	theme, canvas, data := attachment.theme, attachment.canvas, attachment.data
	natural := attachment.Size()
	bounds := attachment.bounds
	if bounds.Width <= 0 {
		bounds.Width = natural.Width
	}
	if bounds.Height <= 0 {
		bounds.Height = natural.Height
	}
	vertical := data.Orientation == AttachmentVertical

	interactive := !attachment.id.IsZero() && !attachment.disabled
	fill := theme.Card
	border := theme.Border
	foreground := theme.Foreground
	descriptionColor := theme.MutedForeground
	if interactive && canvas.Pressed() == attachment.id {
		fill = theme.Muted
	} else if interactive && canvas.Hovered() == attachment.id {
		// has-[>button]:hover:bg-muted/50
		fill = blend(theme.Card, theme.Muted, 0.5)
	}
	if data.State == AttachmentError {
		border = theme.Danger
		border.Alpha = 0.30
		descriptionColor = theme.Danger
		descriptionColor.Alpha = 0.80
	}
	if data.State == AttachmentUploading || data.State == AttachmentProcessing {
		foreground.Alpha *= 0.60 // stands in for the official title shimmer
	}
	if attachment.disabled {
		foreground = theme.MutedForeground
		descriptionColor = theme.MutedForeground
		foreground.Alpha *= 0.55
		descriptionColor.Alpha *= 0.55
	}
	if interactive && canvas.FocusVisible() == attachment.id {
		ring := theme.Ring
		ring.Alpha *= 0.50
		canvas.Rounded(expand(bounds, 1), m.radius+1, ring) // focus-within:ring-1
	}
	if data.State == AttachmentIdle {
		canvas.Rounded(bounds, m.radius, fill)
		attachment.paintIdleBorder(bounds, m.radius, border)
	} else {
		canvas.RoundedBorder(bounds, m.radius, 1, border, fill)
	}

	inner := gfx.Rect{X: bounds.X + m.border + m.padX, Y: bounds.Y + m.border + m.padY, Width: max(bounds.Width-(m.border+m.padX)*2, 0), Height: max(bounds.Height-(m.border+m.padY)*2, 0)}
	hasDescription := data.Description != ""
	textHeight := m.textHeight(hasDescription)
	var media, text gfx.Rect
	var actionsLeft float32
	if vertical {
		media = gfx.Rect{X: inner.X, Y: inner.Y, Width: inner.Width, Height: inner.Width}
		text = gfx.Rect{X: inner.X + m.textInset, Y: media.Y + media.Height + m.gap, Width: max(inner.Width-m.textInset*2, 0), Height: textHeight}
	} else {
		media = gfx.Rect{X: inner.X, Y: inner.Y + (inner.Height-m.media)/2, Width: m.media, Height: m.media}
		right := inner.X + inner.Width
		if attachment.actionCount > 0 {
			right -= float32(attachment.actionCount) * m.action
			actionsLeft = right
			right -= m.gap
		}
		textX := media.X + media.Width + m.gap
		text = gfx.Rect{X: textX, Y: inner.Y + (inner.Height-textHeight)/2, Width: max(right-textX, 0), Height: textHeight}
	}

	mediaFill := theme.Muted
	mediaForeground := theme.Foreground
	if data.State == AttachmentError {
		mediaFill = theme.Danger
		mediaFill.Alpha = 0.10
		mediaForeground = theme.Danger
	}
	imageDim := data.Kind == AttachmentImage && data.State != AttachmentDone && data.State != AttachmentIdle
	if imageDim {
		mediaFill.Alpha *= 0.60
		mediaForeground.Alpha *= 0.60
	}
	canvas.Rounded(media, m.mediaRadius, mediaFill)
	attachment.paintMedia(media, mediaForeground, vertical, data.Size == AttachmentSizeExtraSmall)

	if data.Name != "" {
		attachment.paintLine(data.Name, gfx.Rect{X: text.X, Y: text.Y, Width: text.Width, Height: m.titleHeight}, m.titleSize, gfx.WeightMedium, foreground)
	}
	if hasDescription {
		attachment.paintLine(data.Description, gfx.Rect{X: text.X, Y: text.Y + m.titleHeight + m.descGap, Width: text.Width, Height: m.descHeight}, m.descSize, gfx.WeightRegular, descriptionColor)
	}
	if data.State == AttachmentUploading && data.Progress > 0 {
		progress := theme.Primary
		progress.Alpha = 0.70
		fraction := min(data.Progress, 1)
		canvas.Rounded(gfx.Rect{
			X: inner.X, Y: bounds.Y + bounds.Height - m.border - 2,
			Width: inner.Width * fraction, Height: 2,
		}, 1, progress)
	}
	if interactive {
		canvas.HitWithLabel(attachment.id, bounds, data.Name)
	}

	// The actions are above the trigger: declared after its hit, they win the pointer.
	scale := theme.Metrics.ControlHeight / 32
	for index := range attachment.actionCount {
		action := gfx.Rect{Width: m.action, Height: m.action}
		if vertical {
			// absolute top-3 right-3, gap-1 between the actions
			row := float32(attachment.actionCount)*m.action + float32(attachment.actionCount-1)*4*scale
			action.X = bounds.X + bounds.Width - m.border - 12*scale - row + float32(index)*(m.action+4*scale)
			action.Y = bounds.Y + m.border + 12*scale
		} else {
			action.X = actionsLeft + float32(index)*m.action
			action.Y = bounds.Y + (bounds.Height-m.action)/2
		}
		attachment.paintAction(attachment.actions[index], action)
	}
}

// paintLine draws one truncated line of text: a title or description that is too long
// ends in an ellipsis.
func (attachment Attachment) paintLine(value string, bounds gfx.Rect, size, weight float32, color gfx.Color) {
	var scratch [1]string
	lines := attachment.canvas.WrapLines(scratch[:0], value, size, weight, bounds.Width, 1)
	if len(lines) == 0 {
		return
	}
	attachment.canvas.Label(lines[0], bounds, gfx.TextStyle{Size: size, Weight: weight, Color: color})
}

// paintAction draws one ghost icon-xs button and declares its hit.
func (attachment Attachment) paintAction(action AttachmentAction, bounds gfx.Rect) {
	theme, canvas := attachment.theme, attachment.canvas
	interactive := !action.ID.IsZero() && !attachment.disabled
	radius := theme.Metrics.RadiusControl
	if interactive {
		switch {
		case canvas.Pressed() == action.ID:
			canvas.Rounded(bounds, radius, theme.Muted)
		case canvas.Hovered() == action.ID:
			hover := theme.Muted
			if theme.Background.Red < 0.5 {
				hover.Alpha *= 0.50
			}
			canvas.Rounded(bounds, radius, hover)
		}
		if canvas.FocusVisible() == action.ID {
			ring := theme.Ring
			ring.Alpha *= 0.50
			canvas.Rounded(expand(bounds, 3), radius+3, ring)
		}
	}
	tint := theme.Foreground
	if attachment.disabled {
		tint.Alpha *= 0.50
	}
	size := 12 * theme.Metrics.ControlHeight / 32
	canvas.DrawIcon(action.Icon, gfx.Rect{X: bounds.X + (bounds.Width-size)/2, Y: bounds.Y + (bounds.Height-size)/2, Width: size, Height: size}, tint)
	if !action.ID.IsZero() {
		canvas.HitWithOptions(action.ID, bounds, action.Label, gfx.HitOptions{Role: gfx.RoleButton, Disabled: attachment.disabled})
	}
}

// paintIdleBorder draws the dashed outline of an idle attachment. Three fixed dashes
// per straight edge keep the idle/dashed distinction without a command count that
// grows with the attachment's size; the rounded corners are left open.
func (attachment Attachment) paintIdleBorder(bounds gfx.Rect, radius float32, color gfx.Color) {
	canvas := attachment.canvas
	horizontal := max(bounds.Width-radius*2, 0) / 5
	for index := 0; index < 3; index++ {
		x := bounds.X + radius + float32(index*2)*horizontal
		canvas.Fill(gfx.Rect{X: x, Y: bounds.Y, Width: horizontal, Height: 1}, color)
		canvas.Fill(gfx.Rect{X: x, Y: bounds.Y + bounds.Height - 1, Width: horizontal, Height: 1}, color)
	}
	vertical := max(bounds.Height-radius*2, 0) / 5
	for index := 0; index < 3; index++ {
		y := bounds.Y + radius + float32(index*2)*vertical
		canvas.Fill(gfx.Rect{X: bounds.X, Y: y, Width: 1, Height: vertical}, color)
		canvas.Fill(gfx.Rect{X: bounds.X + bounds.Width - 1, Y: y, Width: 1, Height: vertical}, color)
	}
}

// paintMedia draws the glyph inside the media square: the caller's icon, else the
// generic file or image glyph for the kind. The glyph is size-4 (size-6 vertical, size-3.5 xs).
func (attachment Attachment) paintMedia(media gfx.Rect, color gfx.Color, vertical, extraSmall bool) {
	scale := attachment.theme.Metrics.ControlHeight / 32
	size := 16 * scale
	switch {
	case vertical:
		size = 24 * scale
	case extraSmall:
		size = 14 * scale
	}
	glyph := gfx.Rect{X: media.X + (media.Width-size)/2, Y: media.Y + (media.Height-size)/2, Width: size, Height: size}
	icon := &attachmentFileIcon
	switch {
	case len(attachment.data.Icon.Commands) > 0 || len(attachment.data.Icon.Layers) > 0:
		icon = &attachment.data.Icon
	case attachment.data.Kind == AttachmentImage:
		icon = &attachmentImageIcon
	}
	attachment.canvas.DrawIcon(*icon, glyph, color)
}

// The media glyphs used when the caller gives no Icon: Lucide's file and image, with
// the arcs drawn as quadratic curves.
var (
	attachmentFileIcon = gfx.Icon{ViewBox: 24, Stroke: 2, Cap: gfx.CapRound, Join: gfx.JoinRound, Commands: []gfx.PathCommand{
		gfx.MoveTo(6, 22), gfx.QuadTo(4, 22, 4, 20), gfx.LineTo(4, 4), gfx.QuadTo(4, 2, 6, 2), gfx.LineTo(14, 2),
		gfx.LineTo(20, 8), gfx.LineTo(20, 20), gfx.QuadTo(20, 22, 18, 22), gfx.ClosePath(),
		gfx.MoveTo(14, 2), gfx.LineTo(14, 7), gfx.QuadTo(14, 8, 15, 8), gfx.LineTo(20, 8),
	}}
	attachmentImageIcon = gfx.Icon{ViewBox: 24, Stroke: 2, Cap: gfx.CapRound, Join: gfx.JoinRound, Commands: []gfx.PathCommand{
		gfx.MoveTo(5, 3), gfx.LineTo(19, 3), gfx.QuadTo(21, 3, 21, 5), gfx.LineTo(21, 19), gfx.QuadTo(21, 21, 19, 21),
		gfx.LineTo(5, 21), gfx.QuadTo(3, 21, 3, 19), gfx.LineTo(3, 5), gfx.QuadTo(3, 3, 5, 3), gfx.ClosePath(),
		gfx.MoveTo(9, 7), gfx.QuadTo(11, 7, 11, 9), gfx.QuadTo(11, 11, 9, 11), gfx.QuadTo(7, 11, 7, 9), gfx.QuadTo(7, 7, 9, 7), gfx.ClosePath(),
		gfx.MoveTo(21, 15), gfx.LineTo(17.5, 11.5), gfx.QuadTo(16.5, 10.5, 15.5, 11.5), gfx.LineTo(6, 21),
	}}
)

// PaintIn paints the attachment at its natural size in the next free slot of a group.
func (attachment Attachment) PaintIn(group *AttachmentGroup) {
	attachment.bounds = group.Next(attachment.Size())
	attachment.Paint()
}

// AttachmentGroup lays attachments out in a row, the official AttachmentGroup: gap-3
// between them and py-1 above and below. It only places; scrolling the row (the
// official one scrolls and snaps) is the caller's, by shifting the bounds it begins
// with inside a clip, and Extent is how far there is to scroll.
type AttachmentGroup struct {
	bounds gfx.Rect
	gap    float32
	pad    float32
	x      float32
	count  int
}

// BeginAttachmentGroup starts a row of attachments at the left of bounds.
func BeginAttachmentGroup(theme gfx.Theme, bounds gfx.Rect) AttachmentGroup {
	scale := theme.Metrics.ControlHeight / 32
	return AttachmentGroup{bounds: bounds, gap: 12 * scale, pad: 4 * scale, x: bounds.X}
}

// Fits reports whether a slot of the given size would end inside the group's width,
// for a caller that wraps or truncates the row instead of scrolling it.
func (group AttachmentGroup) Fits(size gfx.Size) bool {
	end := group.x + size.Width
	if group.count > 0 {
		end += group.gap
	}
	return end <= group.bounds.X+group.bounds.Width
}

// Next reserves a slot of the given size, vertically at the group's top padding, and
// returns its bounds.
func (group *AttachmentGroup) Next(size gfx.Size) gfx.Rect {
	if group.count > 0 {
		group.x += group.gap
	}
	slot := gfx.Rect{X: group.x, Y: group.bounds.Y + group.pad, Width: size.Width, Height: size.Height}
	group.x += size.Width
	group.count++
	return slot
}

// Extent is the width of the attachments placed so far with their gaps.
func (group AttachmentGroup) Extent() float32 { return group.x - group.bounds.X }

// Height is the group's height for attachments up to the given height: they and the
// padding above and below.
func (group AttachmentGroup) Height(attachmentHeight float32) float32 {
	return attachmentHeight + group.pad*2
}
