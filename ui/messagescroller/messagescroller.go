// Package messagescroller is the opt-in page control for a controlled
// scrolling stream of messages. It declares the page's own ScrollAt area, so
// the wheel, the scrollbars, clipping and layout are the engine's, and adds the
// earlier/later jump controls:
//
//	stream := &messagescroller.State{
//		Offset:       m.offset,
//		EarlierLabel: "Earlier messages",
//		LaterLabel:   "Later messages",
//		Content: func(c ui.C) {
//			for _, message := range m.messages {
//				ui.Row(c).Height(62).Children(message)
//			}
//		},
//	}
//	stream.OnScroll = func(offset float32) { m.offset = offset }
//	messagescroller.View(c, "Conversation", stream).FillWidth().Height(280)
//
// The caller owns the offset and the content; the view owns no timer, follows
// no content by itself and schedules no frame. The scroll area is declared with
// ScrollAt, so the page does not handle its wheel: give Config.OnScroll the
// view's Wheel, and call Key for keyboard scrolling when the stream should take
// focus. OnScroll reports every offset the reader moves to, including a
// scrollbar drag, so the caller can store it; OnJump reports the target of an
// earlier/later button.
//
// It is a separate package so an application that does not import it pays
// nothing for it. Keep a State in the model with its callbacks set once; the
// view's handlers are the caller's own, and a warm paint allocates nothing.
package messagescroller

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// Direction names the end a jump control goes to.
type Direction uint8

const (
	// Earlier jumps to the start of the content.
	Earlier Direction = iota
	// Later jumps to the end of the content.
	Later
)

// Geometry is the scroll view's complete layout as of the last paint of View.
// Read it after View, in the same paint, beside the other controls that read
// their result there.
type Geometry struct {
	// Bounds is the scroll area's rectangle; the toolbar, when shown, is above it.
	Bounds gfx.Rect
	// Viewport is the visible part of Bounds, less its padding.
	Viewport gfx.Rect
	// Content is the caller's content translated by Offset.
	Content gfx.Rect
	// Offset and MaxOffset are the settled offset and the greatest offset.
	Offset, MaxOffset float32
	// ContentExtent and ViewportExtent are the content's and the viewport's
	// heights before translation.
	ContentExtent, ViewportExtent float32
	// CanEarlier and CanLater report whether the jump control for that end does
	// anything at this offset.
	CanEarlier, CanLater bool
}

// State is one message stream's page control. Fill the fields from the
// model every paint; callbacks stay as they are and are read when they are
// needed.
type State struct {
	// Offset is the caller-owned vertical offset painted this frame. The page's
	// ScrollAt area is declared here, so the caller is the source of truth:
	// store the offsets OnScroll and OnJump report and write them back.
	Offset float32

	// EarlierLabel and LaterLabel name the two jump controls; an empty label
	// omits that control, and both empty omit the toolbar.
	EarlierLabel, LaterLabel string
	// EarlierID and LaterID name the jump controls for input. A zero ID keeps
	// the page's derived identity.
	EarlierID, LaterID gfx.ID
	// ScrollbarID names the scrollbar for input; zero keeps the derived name.
	ScrollbarID gfx.ID
	// ScrollbarLabel and ScrollbarValue name the scrollbar for assistive
	// technology and give its value, as the component's thin scrollbar does.
	ScrollbarLabel, ScrollbarValue string

	// Content declares the caller-owned rows inside the scroll area. The area
	// lays them out as a column.
	Content func(ui.C)

	// OnScroll receives the clamped offset after the reader scrolls: a wheel
	// given to Wheel, a key given to Key, or a drag of the area's scrollbar.
	OnScroll func(offset float32)
	// OnJump receives an earlier/later button's target offset. Without it the
	// view writes Offset itself.
	OnJump func(direction Direction, offset float32)

	area      ui.Box
	state     *ui.ScrollState
	stateID   gfx.ID
	requested float32
	settled   bool
}

// View declares the scrolling message stream at c. label names the control for
// assistive technology. Give it a height: the area takes the room the toolbar
// leaves and clips what does not fit.
//
// Declare the view on its own, not inside a Memo.
func View(c ui.C, label string, scroller *State) ui.Box {
	if scroller == nil {
		panic("messagescroller: View needs its State")
	}
	offset := scroller.Offset
	if scroller.state != nil && scroller.state.ID == scroller.stateID &&
		scroller.settled && scroller.state.Offset != scroller.requested {
		// The reader moved the area since the last declaration, by dragging its
		// scrollbar or by a command that did not go through Wheel. Adopt the
		// settled offset so it does not snap back, and report it.
		offset = scroller.state.Offset
		scroller.reportOffset(offset)
	}

	metrics := c.Theme().Metrics
	column := ui.Column(c).Gap(metrics.GapItem)
	if label != "" {
		column = column.HitWith(ui.ID(c), label, gfx.HitOptions{Role: gfx.RoleGroup, SkipFocus: true})
	}
	column.Children(func() {
		if scroller.EarlierLabel != "" || scroller.LaterLabel != "" {
			scroller.toolbar(c)
		}
		area := ui.ScrollAt(c, offset).Grow(1)
		if scroller.ScrollbarLabel != "" || scroller.ScrollbarValue != "" {
			area = area.ScrollbarLabels(scroller.ScrollbarLabel, "").
				ScrollbarValues(scroller.ScrollbarValue, "")
		}
		if !scroller.ScrollbarID.IsZero() {
			area = area.ScrollbarIDs(scroller.ScrollbarID, gfx.ID{})
		}
		area = area.AlwaysScrollbars()
		area.Children(func() {
			if scroller.Content != nil {
				scroller.Content(c)
			}
		})
		scroller.area = area
		scroller.state = area.ScrollState()
		scroller.stateID = scroller.state.ID
		scroller.requested = offset
		scroller.settled = true
	})
	return column
}

// Geometry is the layout of the stream painted last. It is valid after View in
// the same paint, beside the other controls that read their result there.
func (s *State) Geometry() Geometry {
	state := s.state
	if state == nil || state.ID != s.stateID || !s.area.Valid() {
		return Geometry{}
	}
	bounds := s.area.Rect()
	return Geometry{
		Bounds:         bounds,
		Viewport:       gfx.Rect{X: bounds.X, Y: bounds.Y, Width: bounds.Width, Height: state.Viewport},
		Content:        gfx.Rect{X: bounds.X, Y: bounds.Y - state.Offset, Width: bounds.Width, Height: state.Content},
		Offset:         state.Offset,
		MaxOffset:      state.Max(),
		ContentExtent:  state.Content,
		ViewportExtent: state.Viewport,
		CanEarlier:     s.canEarlier(),
		CanLater:       s.canLater(),
	}
}

// SettledOffset is the settled offset of the last paint, after clamping.
func (s *State) SettledOffset() float32 {
	if s.state == nil || s.state.ID != s.stateID {
		return s.Offset
	}
	return s.state.Offset
}

// ContentHeight is the content height the area measured at the last paint.
func (s *State) ContentHeight() float32 {
	if s.state == nil || s.state.ID != s.stateID {
		return 0
	}
	return s.state.Content
}

// ViewportHeight is the visible height the area measured at the last paint.
func (s *State) ViewportHeight() float32 {
	if s.state == nil || s.state.ID != s.stateID {
		return 0
	}
	return s.state.Viewport
}

// CanEarlier reports whether the earlier jump control does anything at the
// offset painted last.
func (s *State) CanEarlier() bool { return s.canEarlier() }

// CanLater reports whether the later jump control does anything at the offset
// painted last.
func (s *State) CanLater() bool { return s.canLater() }

// Contains reports whether a point is inside the scroll area painted last, for
// a page that gives the wheel to the stream only when it is over it.
func (s *State) Contains(x, y float32) bool {
	if !s.area.Valid() {
		return false
	}
	bounds := s.area.Rect()
	return x >= bounds.X && x < bounds.X+bounds.Width &&
		y >= bounds.Y && y < bounds.Y+bounds.Height
}

// Wheel applies a wheel event to the stream: positive reveals content below,
// a mouse notch moves at least a line and a trackpad exactly what it reports.
// Give it to the page's Config.OnScroll; the area is declared with ScrollAt, so
// the page itself leaves the wheel to the application. The settled offset is
// reported to OnScroll, or written to Offset when OnScroll is nil.
func (s *State) Wheel(event gfx.ScrollEvent) gfx.EventResult {
	return s.apply(s.state, func(state *ui.ScrollState) gfx.EventResult {
		return state.Wheel(event)
	})
}

// Key applies the keyboard's scrolling to the stream, given to a page that
// gives the stream the focus. A step of zero or less is one line, 32 pixels;
// PageUp and PageDown move by a viewport, Home and End to the ends.
func (s *State) Key(event gfx.KeyEvent, step float32) gfx.EventResult {
	return s.apply(s.state, func(state *ui.ScrollState) gfx.EventResult {
		return state.Key(event, step)
	})
}

// apply runs one ScrollState input call and reports the offset it settled on.
func (s *State) apply(state *ui.ScrollState, input func(*ui.ScrollState) gfx.EventResult) gfx.EventResult {
	if state == nil || state.ID != s.stateID {
		return gfx.EventResult{}
	}
	result := input(state)
	if result.Handled {
		s.requested = state.Offset
		s.reportOffset(state.Offset)
	}
	return result
}

// toolbar declares the two jump controls. They are present but inert at the
// ends they cannot leave, so the stream does not change height as it scrolls.
func (s *State) toolbar(c ui.C) {
	row := ui.Row(c).Gap(c.Theme().Metrics.GapItem)
	row.Children(func() {
		if s.EarlierLabel != "" {
			button := ui.Button(c, s.EarlierLabel).Small()
			if !s.EarlierID.IsZero() {
				button = button.Hit(s.EarlierID, s.EarlierLabel)
			}
			if !s.canEarlier() {
				button = button.Disabled(true)
			}
			if button.Clicked() {
				s.jump(Earlier)
			}
		}
		if s.LaterLabel != "" {
			button := ui.Button(c, s.LaterLabel).Small()
			if !s.LaterID.IsZero() {
				button = button.Hit(s.LaterID, s.LaterLabel)
			}
			if !s.canLater() {
				button = button.Disabled(true)
			}
			if button.Clicked() {
				s.jump(Later)
			}
		}
	})
}

// jump reports the offset a jump control goes to: the start or the end of the
// content as of the last paint.
func (s *State) jump(direction Direction) {
	target := float32(0)
	if direction == Later && s.state != nil && s.state.ID == s.stateID {
		target = s.state.Max()
	}
	if s.OnJump != nil {
		s.OnJump(direction, target)
		return
	}
	s.Offset = target
}

// reportOffset hands the reader's offset to the model, or keeps it in Offset
// when the caller did not ask to observe it.
func (s *State) reportOffset(offset float32) {
	if s.OnScroll != nil {
		s.OnScroll(offset)
		return
	}
	s.Offset = offset
}

func (s *State) canEarlier() bool {
	state := s.state
	return state != nil && state.ID == s.stateID &&
		state.Offset > components.MessageScrollerEdgeThreshold
}

func (s *State) canLater() bool {
	state := s.state
	return state != nil && state.ID == s.stateID &&
		state.Offset < state.Max()-components.MessageScrollerEdgeThreshold
}
