package gfx

import "time"

// Pose is where an element sits relative to its place in the layout: moved,
// scaled about a pivot and faded. The zero Pose is the element as laid out.
// Unlike a Presence, which is an overlay's entrance and exit, a Pose is the
// state of an element that is always there: a tab lifted by the pointer, a row
// pushed aside, a panel resting a few pixels off.
type Pose struct {
	// Offset moves the element, in logical pixels.
	Offset Point
	// Scale is its size as a fraction of its own; zero and one leave it alone.
	Scale float32
	// Fade is how far it is faded out, from 0, opaque, to 1, gone.
	Fade float32
}

func (pose Pose) channels() [poseChannels]float32 {
	scale := pose.Scale
	if scale == 0 {
		scale = 1
	}
	return [poseChannels]float32{pose.Offset.X, pose.Offset.Y, scale, pose.Fade}
}

const poseChannels = 4

// poseEntry is one posed element: a motionSlot per component of its pose, so
// each keeps its own velocity when the target changes on the way.
type poseEntry struct {
	id       ID
	slot     uint8
	seen     uint32
	channels [poseChannels]motionSlot
	// origin is where a glided element was painted last frame.
	origin Point
	placed bool
}

// BeginPose starts the group of primitives that make up one posed element and
// moves them to pose, about pivot, with motion. id names the element and slot
// tells apart the poses of one id. Everything the canvas paints until the
// matching EndPose belongs to the element, which is moved, scaled and faded
// after it was painted: the application paints it where the layout puts it and
// the canvas finds the way to the pose, keeping the velocity of a pose that
// changes on the way. Hits painted inside move with it, as HitPresenceRect
// reports for a native overlay anchored to them.
//
// A pose that is at rest as the zero Pose costs a stack push, as does one with
// ID zero (a static preview), without a clock, outside BeginFrame and EndFrame
// and under reduced motion, which show the pose at once. An element starts at
// its first pose, so one that appears does not move in.
func (canvas *Canvas) BeginPose(id ID, slot uint8, pose Pose, motion Motion, pivot Point) {
	state := &canvas.motion
	open := presenceOpen{
		entry:      -1,
		rectangles: len(canvas.rectangles), triangles: len(canvas.triangles), images: len(canvas.images),
		texts: len(canvas.texts), runs: len(canvas.runs), shadows: len(canvas.shadows),
		hits: len(canvas.hits), order: canvas.order,
		pose: poseTarget{pose: pose, pivot: pivot},
	}
	if state.active && !id.IsZero() {
		open.pose.entry = state.poseFor(id, slot, pose.channels(), state.frameTime()) + 1
		open.pose.motion = motion
	}
	state.open = append(state.open, open)
}

// poseTarget is what BeginPose asked for, kept until EndPose.
type poseTarget struct {
	pose   Pose
	pivot  Point
	motion Motion
	// entry is 1 + the index in motionState.poses; zero for a group not tracked.
	entry int
}

// EndPose closes the group opened by BeginPose and places it.
func (canvas *Canvas) EndPose() {
	state := &canvas.motion
	if len(state.open) == 0 {
		return
	}
	open := state.open[len(state.open)-1]
	state.open = state.open[:len(state.open)-1]
	target := open.pose
	var values [poseChannels]float32
	switch {
	case target.entry > 0 && target.entry <= len(state.poses):
		values = state.poses[target.entry-1].step(state, target.pose.channels(), target.motion)
	default:
		values = target.pose.channels()
	}
	transform := presenceTransform{
		pivot: target.pivot, offset: Point{X: values[0], Y: values[1]},
		scale: values[2], alpha: 1 - min(max(values[3], 0), 1),
	}
	fade := values[3] > 0
	if !fade && transform.scale == 1 && transform.offset == (Point{}) {
		return
	}
	transform.apply(canvas, open, fade, fade)
	state.recordHitMotions(canvas, open, transform.hitMotion())
}

// poseFor finds or creates the entry of an element, settled at its first pose.
func (state *motionState) poseFor(id ID, slot uint8, initial [poseChannels]float32, now time.Duration) int {
	index := state.poseAt
	if index >= len(state.poses) || state.poses[index].id != id || state.poses[index].slot != slot {
		index = -1
		for candidate := range state.poses {
			if state.poses[candidate].id == id && state.poses[candidate].slot == slot {
				index = candidate
				break
			}
		}
		if index < 0 {
			index = min(state.poseAt, len(state.poses))
			state.poses = append(state.poses, poseEntry{})
			copy(state.poses[index+1:], state.poses[index:])
			entry := poseEntry{id: id, slot: slot}
			for channel, value := range initial {
				entry.channels[channel] = motionSlot{from: value, to: value, start: now}
			}
			state.poses[index] = entry
		}
	}
	state.poseAt = index + 1
	state.poses[index].seen = state.frame
	return index
}

// step moves every component toward its target and returns what is shown.
func (entry *poseEntry) step(state *motionState, targets [poseChannels]float32, motion Motion) (values [poseChannels]float32) {
	now := state.frameTime()
	for channel := range entry.channels {
		slot := &entry.channels[channel]
		target := targets[channel]
		if state.reduce || motion.Duration <= 0 {
			*slot = motionSlot{from: target, to: target, start: now}
			values[channel] = target
			continue
		}
		if slot.to != target {
			slot.retarget(now, target, motion)
		}
		if !slot.settled(now) {
			state.animating = true
		}
		values[channel] = slot.value(now)
	}
	return values
}

// sweepPoses drops the poses of elements the finished paint pass did not paint.
func (state *motionState) sweepPoses() {
	kept := state.poses[:0]
	for _, entry := range state.poses {
		if entry.seen == state.frame {
			kept = append(kept, entry)
		}
	}
	clear(state.poses[len(kept):])
	state.poses = kept
	if cap(state.poses) > motionShrinkCapacity && len(state.poses)*motionShrinkFactor < cap(state.poses) {
		state.poses = append(make([]poseEntry, 0, len(state.poses)*2), state.poses...)
	}
}

// BeginGlide starts the group of primitives of an element that slides to its
// new place when the layout moves it: a tab whose neighbour closed, a row
// dropped into a list. The application paints the element where the layout puts
// it and says where that is, as place, a point measured so that what moves the
// element with everything around it, a scrolled container, is not a move: the
// element's corner less the scroll. When place differs from where the last frame painted it the
// canvas shows it where it was and glides it to where it is, keeping the
// velocity of a glide the layout interrupts. An element that is painted for the
// first time is simply there. Close the group with EndPose. id and slot name the
// element as for BeginPose, which uses the same entry: a glided element has no
// other pose.
//
// Only the position glides; the size follows the layout at once. Motion with a
// zero duration, no clock, reduced motion and ID zero place the element at once.
func (canvas *Canvas) BeginGlide(id ID, slot uint8, place Point, motion Motion) {
	state := &canvas.motion
	canvas.BeginPose(id, slot, Pose{}, motion, Point{})
	open := &state.open[len(state.open)-1]
	if open.pose.entry == 0 {
		return
	}
	entry := &state.poses[open.pose.entry-1]
	origin := place
	if entry.placed && entry.origin != origin && !state.reduce && motion.Duration > 0 {
		now := state.frameTime()
		shift := Point{X: entry.origin.X - origin.X, Y: entry.origin.Y - origin.Y}
		entry.channels[0].rebase(now, shift.X, 0, motion)
		entry.channels[1].rebase(now, shift.Y, 0, motion)
	}
	entry.origin, entry.placed = origin, true
}

// GlideFrom tells the canvas that the element named id and slot, which the
// next BeginGlide places, was last seen at from: a tab dropped where the pointer
// carried it, a row set down by a drag. The element then slides from there
// instead of from where the previous frame painted it. It has to come before the
// frame's BeginGlide of the element, and only affects the one that follows; an
// element that is not painted again, a canvas without a frame clock and reduced
// motion ignore it.
func (canvas *Canvas) GlideFrom(id ID, slot uint8, from Point) {
	state := &canvas.motion
	if !state.active || id.IsZero() || state.reduce {
		return
	}
	index := -1
	for candidate := range state.poses {
		if state.poses[candidate].id == id && state.poses[candidate].slot == slot {
			index = candidate
			break
		}
	}
	if index < 0 {
		return
	}
	entry := &state.poses[index]
	// What the eye saw is the pose shown now, on top of the place painted.
	now := state.frameTime()
	entry.origin = Point{X: from.X - entry.channels[0].value(now), Y: from.Y - entry.channels[1].value(now)}
	entry.placed = true
}
