package gfx

import (
	"math"
	"time"
)

// A drag that is let go with some speed carries on: the value it moved comes to
// rest further along, at a point the application may choose (a page, a slot, an
// end of the range), by a spring that starts at the speed the pointer had. This
// file holds the pieces of that: VelocityTracker measures the speed, Project says
// where it would come to rest, RubberBand holds a value that is pulled past its
// range, Nearest picks where to settle, and Canvas.Throw hands the speed to the
// spring of Canvas.Animate.
//
//	// while dragging, on every pointer move
//	tracker.Add(event.X, canvas.Now())
//	// on release
//	rest := Nearest(Project(event.X, tracker.Velocity(), DecelerationNormal), pages...)
//	canvas.Throw(id, 0, event.X, tracker.Velocity())
//	offset := rest // painted with canvas.Animate(id, 0, rest, SpringMotion(...))

// velocitySamples is how many recent pointer positions a tracker keeps: enough
// to smooth a jittery pointer, few enough that it follows a flick.
const velocitySamples = 8

// velocityWindow is how far back a sample still counts toward the speed. A
// pointer that rested before it was let go has no speed left, however fast it
// moved earlier.
const velocityWindow = 100 * time.Millisecond

// VelocityTracker measures the speed of a value dragged along one axis from its
// recent positions. The zero value is ready; it allocates nothing.
type VelocityTracker struct {
	at    [velocitySamples]time.Duration
	value [velocitySamples]float32
	count int
	next  int
}

// Reset forgets every position, for the start of a new drag.
func (tracker *VelocityTracker) Reset() { *tracker = VelocityTracker{} }

// Add records that the dragged value was at value at time at. Times must not
// run backward; a sample at the time of the last one replaces it.
func (tracker *VelocityTracker) Add(value float32, at time.Duration) {
	if tracker.count > 0 {
		last := (tracker.next + velocitySamples - 1) % velocitySamples
		if at < tracker.at[last] {
			return
		}
		if at == tracker.at[last] {
			tracker.value[last] = value
			return
		}
	}
	tracker.at[tracker.next], tracker.value[tracker.next] = at, value
	tracker.next = (tracker.next + 1) % velocitySamples
	tracker.count = min(tracker.count+1, velocitySamples)
}

// Velocity returns the speed of the value in units per second at the last
// position added, from a least-squares line through the samples of the last
// 100ms. It is zero for fewer than two samples in that window, which is a drag
// that rested before it was let go.
func (tracker *VelocityTracker) Velocity() float32 {
	if tracker.count < 2 {
		return 0
	}
	newest := (tracker.next + velocitySamples - 1) % velocitySamples
	end := tracker.at[newest]
	var n, sumT, sumV, sumTT, sumTV float64
	for index := range tracker.count {
		slot := (tracker.next + velocitySamples - 1 - index) % velocitySamples
		age := end - tracker.at[slot]
		if age > velocityWindow {
			break
		}
		t := -age.Seconds()
		v := float64(tracker.value[slot])
		n++
		sumT, sumV, sumTT, sumTV = sumT+t, sumV+v, sumTT+t*t, sumTV+t*v
	}
	if n < 2 {
		return 0
	}
	denominator := n*sumTT - sumT*sumT
	if denominator == 0 {
		return 0
	}
	return float32((n*sumTV - sumT*sumV) / denominator)
}

// Deceleration is how fast a thrown value slows down: the fraction of its speed
// it keeps each millisecond. Higher slides further.
type Deceleration float32

const (
	// DecelerationNormal is the rate of a scroll view (0.998 per ms), which a
	// flick of a page or a list carries far.
	DecelerationNormal Deceleration = 0.998
	// DecelerationFast is a brisker stop (0.99 per ms), for a control that
	// should not slide past its neighbour.
	DecelerationFast Deceleration = 0.99
)

// Project returns where a value at position, thrown at velocity (units per
// second), would come to rest if nothing held it: the distance a speed that
// decays by deceleration each millisecond covers, the sum of the geometric
// series. It is what a snap point is chosen from, so that a flick goes on to
// the next page.
func Project(position, velocity float32, deceleration Deceleration) float32 {
	rate := float64(min(max(float32(deceleration), 0), 0.9999))
	return position + float32(float64(velocity)/1000*rate/(1-rate))
}

// Nearest returns the point of points nearest to value, or value when there are
// none.
func Nearest(value float32, points ...float32) float32 {
	if len(points) == 0 {
		return value
	}
	best := points[0]
	for _, point := range points[1:] {
		if float32(math.Abs(float64(point-value))) < float32(math.Abs(float64(best-value))) {
			best = point
		}
	}
	return best
}

// rubberBandStiffness is how soon the band resists; 0.55 is the constant of iOS
// scroll views.
const rubberBandStiffness = 0.55

// RubberBand holds a value pulled past the end of its range: an offset past the
// edge (positive) moves less and less far for the same pull, never reaching
// limit, the room the view has to give. An offset of zero or less is returned as
// it is, and the pull is continuous with it.
func RubberBand(offset, limit float32) float32 {
	if offset <= 0 || limit <= 0 {
		return offset
	}
	return (1 - 1/(offset*rubberBandStiffness/limit+1)) * limit
}

// Throw hands the speed a value was let go with to the animation of the value
// named by id and slot, so that the next Animate toward a target starts there at
// that speed instead of from rest: a card released while moving glides on to its
// snap point, and one released at rest starts slowly. value is where it is let
// go, in the units Animate returns, and velocity its speed in units per second.
// It belongs in the handler of the release, and has to name the same id and slot
// the application animates with; it keeps nothing for an element that is not
// painted afterwards.
func (canvas *Canvas) Throw(id ID, slot uint8, value, velocity float32) {
	state := &canvas.motion
	if state.clock == nil || id.IsZero() || state.reduce {
		return
	}
	now := canvas.Now()
	entry := state.slotFor(id, slot, value, now)
	*entry = motionSlot{id: id, slot: slot, seen: entry.seen, from: value, to: value, start: now, carry: float64(velocity)}
}
