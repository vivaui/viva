// Package attachment is the opt-in page control for attachment cards and
// attachment groups of package components. A page declares a card with typed
// configuration methods and views it as a ui.Control:
//
//	card := attachment.New()
//	...
//	card.Reset().ID(m.fileID).Title("product-demo.mov").Meta(states[m.state]).
//		File().Uploading(0.64)
//	card.OnClick(m.nextState)
//	card.View(c, "product-demo.mov").FillWidth()
//	if card.Clicked() {
//		m.state = (m.state + 1) % len(states)
//	}
//
//	group := attachment.NewGroup()
//	group.Reset()
//	group.Add().ID(m.removeIDs[0]).Title("release-notes.md").Meta("18 KB").
//		Small().Action(m.removeID, "Remove release-notes.md", icon, m.remove)
//	group.View(c, "Attachments")
//
// The caller owns the title, meta line, kind, icon, state, progress, action
// identities and callbacks; the wrapper owns the control's handlers and draw
// function. A card or action with a zero identity stays inert, as in the
// component. Clicked and ClickedAction report a click after View, and OnClick
// and an action's callback run for it, so a page can either read the click or
// be called back.
//
// It is a separate package so an application that does not import it pays
// nothing for it. Keep an Attachment or Group in the model beside its data: its
// handlers and draw function are made once, and Reset keeps the action storage,
// so a warm paint allocates nothing. Do not copy one after it has been painted.
package attachment

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// Kind, State, Size and Orientation are the component's typed variants, named
// here so a page that declares an attachment reads through this package alone.
type (
	Kind        = components.AttachmentKind
	State       = components.AttachmentState
	Size        = components.AttachmentSize
	Orientation = components.AttachmentOrientation
)

const (
	File  Kind = components.AttachmentFile
	Image Kind = components.AttachmentImage

	Done       State = components.AttachmentDone
	Idle       State = components.AttachmentIdle
	Uploading  State = components.AttachmentUploading
	Processing State = components.AttachmentProcessing
	Error      State = components.AttachmentError

	SizeDefault    Size = components.AttachmentSizeDefault
	SizeSmall      Size = components.AttachmentSizeSmall
	SizeExtraSmall Size = components.AttachmentSizeExtraSmall

	Horizontal Orientation = components.AttachmentHorizontal
	Vertical   Orientation = components.AttachmentVertical
)

// maxActions is how many actions one attachment paints, the component's limit.
const maxActions = 3

// Action is one icon button of an attachment. An action with a zero ID is
// painted but inert, as in the component.
type Action struct {
	ID      gfx.ID
	Label   string
	Icon    gfx.Icon
	OnClick func()
}

// Attachment is one attachment's caller-owned data and its page control.
// Configure it with the chainable methods. Reset clears the values and actions
// so a page that reconfigures the card every paint starts from a clean card;
// the action storage is kept.
type Attachment struct {
	id          gfx.ID
	title       string
	meta        string
	kind        Kind
	icon        gfx.Icon
	state       State
	progress    float32
	size        Size
	orientation Orientation
	disabled    bool
	actions     []Action
	onClick     func()

	handlers ui.Handlers
	drawFn   func(*ui.DrawContext)
	prepared bool
	clicked  gfx.ID
	hadClick bool
}

// New returns an attachment with the default size, done state and no content.
func New() *Attachment { return new(Attachment) }

// Reset clears the identity, content, state, progress, size, orientation and
// actions, returning the card to the default horizontal, done, full-size card.
// The action storage is kept so a reset followed by the same actions does not
// allocate.
func (a *Attachment) Reset() *Attachment {
	a.resetValues()
	return a
}

func (a *Attachment) resetValues() {
	a.id = gfx.ID{}
	a.title = ""
	a.meta = ""
	a.kind = File
	a.icon = gfx.Icon{}
	a.state = Done
	a.progress = 0
	a.size = SizeDefault
	a.orientation = Horizontal
	a.disabled = false
	a.actions = a.actions[:0]
	a.onClick = nil
}

// ID is the caller's identity for the card's trigger. A zero ID paints static
// content; give the card an ID if it should be clickable or read with Clicked.
func (a *Attachment) ID(id gfx.ID) *Attachment { a.id = id; return a }

// Title is the attachment's first line, truncated with an ellipsis when needed.
func (a *Attachment) Title(value string) *Attachment { a.title = value; return a }

// Meta is the attachment's muted second line, such as a size or state summary.
func (a *Attachment) Meta(value string) *Attachment { a.meta = value; return a }

// File selects the generic file media glyph.
func (a *Attachment) File() *Attachment { a.kind = File; return a }

// Image selects the image media glyph.
func (a *Attachment) Image() *Attachment { a.kind = Image; return a }

// Icon sets a caller-owned glyph painted instead of the kind's placeholder.
func (a *Attachment) Icon(icon gfx.Icon) *Attachment { a.icon = icon; return a }

// State sets the typed transfer state.
func (a *Attachment) State(state State) *Attachment { a.state = state; return a }

// Progress sets the fraction, 0 to 1, shown along an Uploading card's bottom
// edge. The zero value draws no bar.
func (a *Attachment) Progress(progress float32) *Attachment { a.progress = progress; return a }

// Done marks the attachment complete.
func (a *Attachment) Done() *Attachment { a.state, a.progress = Done, 0; return a }

// Idle marks the attachment idle, with its dashed outline.
func (a *Attachment) Idle() *Attachment { a.state, a.progress = Idle, 0; return a }

// Uploading marks the attachment as uploading and sets its progress fraction.
func (a *Attachment) Uploading(progress float32) *Attachment {
	a.state, a.progress = Uploading, progress
	return a
}

// Processing marks the attachment as processing, with no determinate progress.
func (a *Attachment) Processing() *Attachment { a.state, a.progress = Processing, 0; return a }

// Failed marks the attachment as failed.
func (a *Attachment) Failed() *Attachment { a.state, a.progress = Error, 0; return a }

// Small selects the small attachment size.
func (a *Attachment) Small() *Attachment { a.size = SizeSmall; return a }

// ExtraSmall selects the extra-small attachment size.
func (a *Attachment) ExtraSmall() *Attachment { a.size = SizeExtraSmall; return a }

// Vertical stacks the media above the text. The default is horizontal.
func (a *Attachment) Vertical() *Attachment { a.orientation = Vertical; return a }

// Disabled paints the card inert without changing its transfer state.
func (a *Attachment) Disabled() *Attachment { a.disabled = true; return a }

// Action adds an icon button. The component paints at most three per card; a
// zero id is painted but inert. The callback runs after the click is read.
func (a *Attachment) Action(id gfx.ID, label string, icon gfx.Icon, onClick func()) *Attachment {
	a.actions = append(a.actions, Action{ID: id, Label: label, Icon: icon, OnClick: onClick})
	return a
}

// OnClick sets the callback run when the card's own trigger is clicked.
func (a *Attachment) OnClick(fn func()) *Attachment { a.onClick = fn; return a }

// View declares the attachment at c. label names the card for assistive
// technology. The box is the card's natural size; give it a wider box to let
// the card fill it.
//
// Declare the view on its own, not inside a Memo.
func (a *Attachment) View(c ui.C, label string) ui.Box {
	if a == nil {
		panic("attachment: View needs its Attachment")
	}
	a.prepare()
	size := a.naturalSize(c.Canvas(), *c.Theme())
	return ui.Control(c, label, &a.handlers).Size(size.Width, size.Height).Draw(a.drawFn)
}

// Clicked reports whether the paint just made delivered a click on the card's
// trigger. It is false in ordinary paints. Ask it after View.
func (a *Attachment) Clicked() bool {
	return a.hadClick && !a.id.IsZero() && a.clicked == a.id
}

// ClickedAction reports whether the paint just made delivered a click on the
// action with id. It is false in ordinary paints or when id is zero.
func (a *Attachment) ClickedAction(id gfx.ID) bool {
	return a.hadClick && !id.IsZero() && a.clicked == id
}

func (a *Attachment) prepare() {
	if a.prepared {
		return
	}
	a.prepared = true
	a.handlers.Role = gfx.RoleGroup
	a.drawFn = a.draw
}

func (a *Attachment) draw(d *ui.DrawContext) {
	a.clicked, a.hadClick = d.Canvas.ClickTarget()
	a.component(d.Canvas, *d.Theme, a.id, d.Rect).Paint()
	a.notify(a.clicked, a.hadClick)
}

// naturalSize measures the card the component will paint, including its text,
// media and actions.
func (a *Attachment) naturalSize(canvas *gfx.Canvas, theme gfx.Theme) gfx.Size {
	return a.component(canvas, theme, gfx.ID{}, gfx.Rect{}).Size()
}

// component builds the component's declaration from the typed configuration.
func (a *Attachment) component(canvas *gfx.Canvas, theme gfx.Theme, id gfx.ID, bounds gfx.Rect) components.Attachment {
	card := components.BeginAttachment(canvas, theme, id, bounds, components.AttachmentData{
		Name: a.title, Description: a.meta, Kind: a.kind, Icon: a.icon,
		State: a.state, Size: a.size, Orientation: a.orientation, Progress: a.progress,
	})
	if a.disabled {
		card = card.Disabled()
	}
	for index := 0; index < len(a.actions) && index < maxActions; index++ {
		action := a.actions[index]
		card = card.Action(components.AttachmentAction{ID: action.ID, Label: action.Label, Icon: action.Icon})
	}
	return card
}

// notify runs the callback of the card or action that was clicked.
func (a *Attachment) notify(clicked gfx.ID, hadClick bool) {
	if !hadClick || clicked.IsZero() {
		return
	}
	if clicked == a.id {
		if a.onClick != nil {
			a.onClick()
		}
		return
	}
	for index := range a.actions {
		if a.actions[index].ID == clicked {
			if a.actions[index].OnClick != nil {
				a.actions[index].OnClick()
			}
			return
		}
	}
}

// Group is one row of attachments and its page control. Configure the gap,
// Reset it, then Add the cards that apply now.
type Group struct {
	gap      float32
	gapSet   bool
	items    []Attachment
	clicked  gfx.ID
	hadClick bool

	handlers ui.Handlers
	drawFn   func(*ui.DrawContext)
	prepared bool
}

// NewGroup returns an empty attachment group with the component's default gap.
func NewGroup() *Group { return new(Group) }

// Reset removes every card, keeping the layout configuration, the card storage
// and each card's action storage so a warm re-declaration allocates nothing.
func (g *Group) Reset() *Group { g.items = g.items[:0]; return g }

// Gap sets the spacing between cards in pixels. The default is the component's
// scaled 12 pixels.
func (g *Group) Gap(pixels float32) *Group {
	if pixels < 0 {
		pixels = 0
	}
	g.gap, g.gapSet = pixels, true
	return g
}

// Add appends a card and returns it for configuration. A reused card is reset;
// a new card starts as an empty default card.
func (g *Group) Add() *Attachment {
	if len(g.items) == cap(g.items) {
		g.items = append(g.items, Attachment{})
	} else {
		g.items = g.items[:len(g.items)+1]
		g.items[len(g.items)-1].resetValues()
	}
	return &g.items[len(g.items)-1]
}

// View declares the group at c. label names the row for assistive technology.
// The box is as wide as the cards and gaps, and as tall as the tallest card
// plus the component's vertical padding. Cards that do not fit the box are not
// painted, as in the component's group; wrap the group or drop cards instead of
// letting them overflow.
//
// Declare the view on its own, not inside a Memo.
func (g *Group) View(c ui.C, label string) ui.Box {
	if g == nil {
		panic("attachment: View needs its Group")
	}
	g.prepare()
	width, height := g.naturalSize(c.Canvas(), *c.Theme())
	return ui.Control(c, label, &g.handlers).Size(width, height).Draw(g.drawFn)
}

// Clicked reports whether the paint just made delivered a click on the card or
// action with id. It is false in ordinary paints or when id is zero.
func (g *Group) Clicked(id gfx.ID) bool {
	return g.hadClick && !id.IsZero() && g.clicked == id
}

func (g *Group) prepare() {
	if g.prepared {
		return
	}
	g.prepared = true
	g.handlers.Role = gfx.RoleGroup
	g.drawFn = g.draw
}

func (g *Group) draw(d *ui.DrawContext) {
	g.clicked, g.hadClick = d.Canvas.ClickTarget()
	theme := *d.Theme
	group := components.BeginAttachmentGroup(theme, d.Rect)
	for index := range g.items {
		item := &g.items[index]
		card := item.component(d.Canvas, theme, item.id, gfx.Rect{})
		if !group.Fits(card.Size()) {
			break
		}
		card.PaintIn(&group)
	}
	if g.hadClick && !g.clicked.IsZero() {
		for index := range g.items {
			g.items[index].notify(g.clicked, true)
		}
	}
}

func (g *Group) naturalSize(canvas *gfx.Canvas, theme gfx.Theme) (width, height float32) {
	if len(g.items) == 0 {
		return 0, 0
	}
	gap := g.gapFor(theme)
	pad := 4 * theme.Metrics.ControlHeight / 32
	for index := range g.items {
		size := g.items[index].naturalSize(canvas, theme)
		if index > 0 {
			width += gap
		}
		width += size.Width
		height = max(height, size.Height)
	}
	return width, height + pad*2
}

func (g *Group) gapFor(theme gfx.Theme) float32 {
	if g.gapSet {
		return g.gap
	}
	return 12 * theme.Metrics.ControlHeight / 32
}
