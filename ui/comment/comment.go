// Package comment is the opt-in page layout for one comment in a discussion: the
// author's avatar, the author and the time, the text, a row of actions and the
// replies nested under it. A page declares each comment with a Spec, and the
// replies by declaring comments inside Replies:
//
//	comment.View(c, comment.Spec{
//		Avatar: &m.avatars[i], Author: post.Author, Time: post.When, Content: post.Text,
//		Actions: func() {
//			if ui.Button(c, "Reply").Ghost().Small().Clicked() {
//				m.replyTo = i
//			}
//		},
//		Replies: func() { m.replies(c, post) },
//	}).FillWidth()
//
// The control keeps no state: the actions are the page's own buttons, so it
// reads their clicks where it declares them, and the replies are comments the
// page declares in turn, so a thread of any depth is one function calling
// itself. The avatar is an avatar.State the page keeps beside the comment's
// data, configured for the author; a nil Avatar leaves the column out.
//
// It is a separate package so an application that does not import it pays
// nothing for it. A warm paint allocates nothing beyond the closures the page
// passes.
package comment

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/avatar"
)

// avatarSize is the diameter of the author's avatar.
const avatarSize = 32

// Spec declares one comment.
type Spec struct {
	// Avatar is the author's avatar; nil draws none.
	Avatar *avatar.State
	// Author and Time head the comment, the time muted after the author. Either
	// may be empty.
	Author, Time string
	// Content is the comment's text, wrapped to the room it has.
	Content string
	// Actions declares the buttons under the text, as small ghost buttons in a
	// row; nil draws none.
	Actions func()
	// Replies declares the comments nested under this one, indented to the text's
	// column; nil draws none.
	Replies func()
}

// View declares the comment at c. The comment is as wide as the room it is
// given and as tall as its content; give it FillWidth. Declare it on its own,
// not inside a Memo; key the comments of a list with c.Key so each keeps its
// own state.
func View(c ui.C, spec Spec) ui.Box {
	theme := c.Theme()
	metrics := theme.Metrics
	root := ui.Row(c).Items(ui.Start).Gap(metrics.GapItem)
	root.Children(func() {
		if spec.Avatar != nil {
			label := spec.Author
			if label == "" {
				label = "Avatar"
			}
			spec.Avatar.Diameter(avatarSize)
			spec.Avatar.View(c, label).Size(avatarSize, avatarSize).Shrink(0)
		}
		ui.Column(c).Grow(1).Basis(0).Gap(4).Children(func() {
			if spec.Author != "" || spec.Time != "" {
				ui.Row(c).Items(ui.Center).Gap(metrics.GapItem).Children(func() {
					if spec.Author != "" {
						ui.Text(c, spec.Author).Weight(gfx.WeightMedium).Ellipsis().Shrink(1)
					}
					if spec.Time != "" {
						ui.Text(c, spec.Time).FontSize(metrics.FontSizeS).Muted().Shrink(0)
					}
				})
			}
			if spec.Content != "" {
				ui.Wrap(c, spec.Content).FillWidth()
			}
			if spec.Actions != nil {
				ui.Row(c).Items(ui.Center).Gap(metrics.GapItem * 0.5).Children(spec.Actions)
			}
			if spec.Replies != nil {
				ui.Column(c).FillWidth().Gap(metrics.GapItem).MarginSides(metrics.GapItem, 0, 0, 0).Children(spec.Replies)
			}
		})
	})
	return root
}
