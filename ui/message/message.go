// Package message is the opt-in page control for a conversation message
// row or a group of consecutive rows. It composes the page's own primitives,
// including ui.Bubble for the body, so a page declares a message without
// rebuilding its avatar, metadata and alignment:
//
//	message.View(c, "Ari", message.New().
//		Name("Ari · Design").Time("09:42").Avatar("AR").
//		Body("Native preview ready."))
//
// The caller supplies the visible data and, through Content, any body richer
// than one bubble; the page layer owns the conversation order. End mirrors the
// row: the avatar moves to the end and the bubble, metadata and actions align
// with it.
//
// It is a separate package so an application that does not import it pays
// nothing for it, and it depends only on the public components, gfx and ui
// packages. A Row is a value built with chained options; keep no state in
// it, build it from the model in each paint, and View allocates nothing on a
// warm paint.
package message

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// Row is one row's presentation. Its zero value is a start-aligned message
// with no avatar, metadata or body; New is its readable starting point. Every
// option returns the value, so a call site chains only what it shows:
//
//	message.New().End().Name("You").Time("Delivered").Avatar("YO").
//		Surface(ui.BubblePrimary).Body("Ship the compact variant.")
type Row struct {
	name     string
	time     string
	initials string
	body     string
	image    gfx.Bitmap
	surface  ui.BubbleVariant

	end     bool
	avatar  bool
	reserve bool
	content func(ui.C)
	actions func(ui.C)
}

// New returns an empty start-aligned message whose body is a secondary bubble.
func New() Row {
	return Row{surface: ui.BubbleSecondary}
}

// Start aligns the row to the start of the conversation. It is the zero value's
// alignment and undoes End.
func (m Row) Start() Row {
	m.end = false
	return m
}

// End aligns the row to the end: the avatar is on the end side and the bubble,
// metadata and actions hug that edge.
func (m Row) End() Row {
	m.end = true
	return m
}

// Name is the sender name in the metadata row above the body.
func (m Row) Name(name string) Row {
	m.name = name
	return m
}

// Time is the timestamp in the metadata row above the body.
func (m Row) Time(value string) Row {
	m.time = value
	return m
}

// Avatar shows a round avatar with the initials, or a muted fallback circle
// when the initials are empty, at the start or end of the row.
func (m Row) Avatar(initials string) Row {
	m.initials = initials
	m.avatar = true
	return m
}

// AvatarImage shows the bitmap as the avatar instead of initials. The caller
// owns the image and keeps it in its model.
func (m Row) AvatarImage(bitmap gfx.Bitmap) Row {
	m.image = bitmap
	m.avatar = true
	return m
}

// ReserveAvatar keeps the avatar column without painting an avatar, as an empty
// avatar does: consecutive rows of one sender line up with the row that shows
// it. Avatar takes precedence.
func (m Row) ReserveAvatar() Row {
	m.reserve = true
	return m
}

// Body is the text of the bubble.
func (m Row) Body(text string) Row {
	m.body = text
	return m
}

// Surface selects the bubble's variant. New starts with BubbleSecondary;
// BubbleGhost draws no surface and no inset.
func (m Row) Surface(variant ui.BubbleVariant) Row {
	m.surface = variant
	return m
}

// Content paints the body instead of Body's bubble, for a row whose body is not
// one text: an image, a stack of attachment cards or a rich preview. The
// callback is declared in the content column, so its own alignment is the
// caller's. Body is ignored when Content is set.
func (m Row) Content(content func(ui.C)) Row {
	m.content = content
	return m
}

// Actions paints the row's actions below the body, aligned with the message.
// The callback declares the controls, so their identities and handlers are the
// caller's own.
func (m Row) Actions(actions func(ui.C)) Row {
	m.actions = actions
	return m
}

// View declares the message row at c. label names the row for assistive
// technology. Give it a width; the row is as tall as its avatar, metadata, body
// and actions.
//
// Declare the view on its own, not inside a Memo.
func View(c ui.C, label string, message Row) ui.Box {
	metrics := c.Theme().Metrics
	hasAvatar := message.avatar || message.reserve
	row := ui.Row(c).Gap(metrics.GapItem).Items(ui.Start)
	if label != "" {
		row = row.HitWith(ui.ID(c), label, gfx.HitOptions{Role: gfx.RoleGroup, SkipFocus: true})
	}
	row.Children(func() {
		if hasAvatar && !message.end {
			avatar(c, message, metrics.ControlHeight)
		}
		ui.Column(c).Gap(components.MessageSlotGap(metrics)).Grow(1).Children(func() {
			if message.name != "" || message.time != "" {
				metadata(c, message)
			}
			body(c, message)
			if message.actions != nil {
				actions(c, message)
			}
		})
		if hasAvatar && message.end {
			avatar(c, message, metrics.ControlHeight)
		}
	})
	return row
}

// Group declares consecutive message rows with the message group gap between
// them, so consecutive rows of one sender read as one turn. label names the
// group for assistive technology; give each Row its own Name, Time and
// avatar or ReserveAvatar.
//
//	message.Group(c, "Ari", first.ReserveAvatar(), last.Avatar("AR"))
func Group(c ui.C, label string, messages ...Row) ui.Box {
	column := ui.Column(c).Gap(components.MessageGroupGap(c.Theme().Metrics))
	if label != "" {
		column = column.HitWith(ui.ID(c), label, gfx.HitOptions{Role: gfx.RoleGroup, SkipFocus: true})
	}
	column.Children(func() {
		for _, row := range messages {
			View(c, "", row)
		}
	})
	return column
}

// metadata paints the sender name and timestamp above the body. The name takes
// the free room so the time sits at the far edge, as a chat header does.
func metadata(c ui.C, message Row) {
	look := c.Theme()
	row := ui.Row(c).Gap(look.Metrics.GapItem).Items(ui.Center)
	if message.end {
		row = row.Justify(ui.End)
	}
	row.Children(func() {
		if message.name != "" {
			name := ui.Text(c, message.name).FontSize(look.Metrics.FontSizeS).Bold()
			if message.time != "" {
				name.Grow(1)
			}
		}
		if message.time != "" {
			ui.Text(c, message.time).FontSize(look.Metrics.FontSizeS).Muted()
		}
	})
}

// body paints the caller's content or one bubble.
func body(c ui.C, message Row) {
	if message.content != nil {
		message.content(c)
		return
	}
	if message.body == "" {
		return
	}
	bubble := ui.Bubble(c, message.body, message.surface)
	if message.end {
		bubble.Self(ui.End)
	} else {
		bubble.Self(ui.Start)
	}
}

// actions paints the caller's action row on the message's side.
func actions(c ui.C, message Row) {
	row := ui.Row(c).Gap(c.Theme().Metrics.GapItem).Items(ui.Center)
	if message.end {
		row = row.Justify(ui.End)
	}
	row.Children(func() {
		message.actions(c)
	})
}

// avatar paints the initials or image circle, or reserves the column.
func avatar(c ui.C, message Row, side float32) {
	if message.reserve && !message.avatar {
		ui.Spacer(c).Size(side, side).Shrink(0)
		return
	}
	look := c.Theme()
	stack := ui.Stack(c).Size(side, side).Shrink(0)
	stack.Children(func() {
		if message.image.Width > 0 && message.image.Height > 0 {
			ui.Image(c, message.image).Fill()
			return
		}
		face := ui.Column(c).Fill().Radius(side / 2).Background(look.Muted).Center()
		face.Children(func() {
			if message.initials != "" {
				ui.Text(c, message.initials).FontSize(look.Metrics.FontSizeS).
					Weight(gfx.WeightSemibold).Muted()
			}
		})
	})
}
