package gfx

import (
	"math"
	"testing"
	"time"
)

func paintPosed(canvas *Canvas, id ID, pose Pose, motion Motion) {
	canvas.BeginPose(id, 0, pose, motion, Point{X: 50, Y: 50})
	canvas.Rounded(Rect{Width: 100, Height: 100}, 10, Color{Red: 1, Alpha: 1})
	canvas.Hit(id, Rect{Width: 100, Height: 100})
	canvas.EndPose()
}

func TestPoseStartsAtItsFirstPoseAndMovesWhenItChanges(t *testing.T) {
	var canvas Canvas
	frames := newTestFrames(&canvas)
	motion := Motion{Duration: 100 * time.Millisecond, Easing: EaseLinear}
	id := Item(1)
	bounds := func(at time.Duration, pose Pose) Rect {
		canvas.Reset()
		frames.at(at, func() { paintPosed(&canvas, id, pose, motion) })
		return canvas.Rectangles()[0].Bounds
	}
	if got := bounds(0, Pose{Offset: Point{X: 20}}); got.X != 20 || canvas.Animating() {
		t.Fatalf("first pose = %+v animating=%v, want it shown at once", got, canvas.Animating())
	}
	bounds(time.Second, Pose{Offset: Point{X: 20}})
	if got := bounds(2*time.Second, Pose{}); got.X != 20 {
		t.Fatalf("frame that changes the pose = %v, want 20", got.X)
	}
	if got := bounds(2*time.Second+50*time.Millisecond, Pose{}); math.Abs(float64(got.X-10)) > 1e-4 || !canvas.Animating() {
		t.Fatalf("halfway = %v animating=%v, want 10", got.X, canvas.Animating())
	}
	if got := bounds(3*time.Second, Pose{}); got.X != 0 || canvas.Animating() {
		t.Fatalf("rest = %v animating=%v", got.X, canvas.Animating())
	}
}

func TestPoseScalesAboutItsPivotAndFades(t *testing.T) {
	var canvas Canvas
	frames := newTestFrames(&canvas)
	canvas.Reset()
	frames.at(0, func() {
		paintPosed(&canvas, Item(1), Pose{Scale: 0.5, Fade: 0.75}, Motion{Duration: time.Second})
	})
	rectangle := canvas.Rectangles()[0]
	if rectangle.Bounds != (Rect{X: 25, Y: 25, Width: 50, Height: 50}) {
		t.Fatalf("bounds = %+v", rectangle.Bounds)
	}
	if math.Abs(float64(rectangle.Color.Alpha-0.25)) > 1e-6 {
		t.Fatalf("alpha = %v, want 0.25", rectangle.Color.Alpha)
	}
}

func TestPoseMovesHitsAlongWithTheirPaint(t *testing.T) {
	var canvas Canvas
	frames := newTestFrames(&canvas)
	canvas.Reset()
	frames.at(0, func() { paintPosed(&canvas, Item(1), Pose{Offset: Point{X: 7, Y: 3}}, DefaultMotion()) })
	got, moved := canvas.HitPresenceRect(0, Rect{Width: 100, Height: 100})
	if !moved || got != (Rect{X: 7, Y: 3, Width: 100, Height: 100}) {
		t.Fatalf("hit rect = %+v moved=%v", got, moved)
	}
}

func TestPoseRetargetKeepsVelocity(t *testing.T) {
	var canvas Canvas
	frames := newTestFrames(&canvas)
	motion := SpringMotion(400*time.Millisecond, 0)
	x := func(at time.Duration, pose Pose) float32 {
		canvas.Reset()
		frames.at(at, func() { paintPosed(&canvas, Item(1), pose, motion) })
		return canvas.Rectangles()[0].Bounds.X
	}
	x(0, Pose{})
	x(time.Second, Pose{Offset: Point{X: 100}})
	at := time.Second + 120*time.Millisecond
	a, b := x(at, Pose{Offset: Point{X: 100}}), x(at+time.Millisecond, Pose{Offset: Point{X: 100}})
	if got := x(at+time.Millisecond, Pose{}); math.Abs(float64(got-b)) > 1e-3 {
		t.Fatalf("turning around jumped from %v to %v", b, got)
	}
	if next := x(at+2*time.Millisecond, Pose{}); (next-b)/0.001 < (b-a)/0.001*0.8 {
		t.Fatalf("velocity %v after turning, was %v", (next-b)/0.001, (b-a)/0.001)
	}
}

func TestPoseIsFinalWithoutClockOrUnderReducedMotion(t *testing.T) {
	var canvas Canvas
	canvas.Reset()
	paintPosed(&canvas, Item(1), Pose{Offset: Point{X: 9}}, DefaultMotion())
	if got := canvas.Rectangles()[0].Bounds.X; got != 9 {
		t.Fatalf("no clock: x = %v, want the pose at once", got)
	}
	frames := newTestFrames(&canvas)
	canvas.SetReduceMotion(true)
	canvas.Reset()
	frames.at(0, func() { paintPosed(&canvas, Item(2), Pose{}, DefaultMotion()) })
	canvas.Reset()
	frames.at(time.Second, func() { paintPosed(&canvas, Item(2), Pose{Offset: Point{X: 9}}, DefaultMotion()) })
	if got := canvas.Rectangles()[0].Bounds.X; got != 9 || canvas.Animating() {
		t.Fatalf("reduced motion: x = %v animating=%v", got, canvas.Animating())
	}
}

func TestPoseForgetsUnpaintedElementsAndIdleAsksForNoFrame(t *testing.T) {
	var canvas Canvas
	frames := newTestFrames(&canvas)
	canvas.Reset()
	frames.at(0, func() { paintPosed(&canvas, Item(1), Pose{Offset: Point{X: 5}}, DefaultMotion()) })
	canvas.Reset()
	frames.at(time.Second, func() { paintPosed(&canvas, Item(1), Pose{Offset: Point{X: 5}}, DefaultMotion()) })
	if _, ok := canvas.NextFrame(); ok {
		t.Fatal("a pose at rest asks for a frame")
	}
	canvas.Reset()
	frames.at(2*time.Second, func() {})
	if len(canvas.motion.poses) != 0 {
		t.Fatalf("%d poses kept for an element no longer painted", len(canvas.motion.poses))
	}
}

func TestWarmPoseDoesNotAllocate(t *testing.T) {
	var canvas Canvas
	frames := newTestFrames(&canvas)
	motion := SpringMotion(300*time.Millisecond, 0.1)
	paint := func(target float32) {
		canvas.Reset()
		for id := 1; id <= 32; id++ {
			paintPosed(&canvas, Item(id), Pose{Offset: Point{X: target}}, motion)
		}
	}
	frames.at(0, func() { paint(0) })
	now := time.Duration(0)
	if allocations := testing.AllocsPerRun(100, func() {
		now += 8 * time.Millisecond
		frames.at(now, func() { paint(float32(now / (400 * time.Millisecond) % 2 * 30)) })
	}); allocations != 0 {
		t.Fatalf("warm posed frames allocate %v times", allocations)
	}
}

func paintGlided(canvas *Canvas, id ID, x float32, motion Motion) float32 {
	bounds := Rect{X: x, Width: 40, Height: 20}
	canvas.BeginGlide(id, 0, Point{X: bounds.X, Y: bounds.Y}, motion)
	canvas.Rounded(bounds, 4, Color{Alpha: 1})
	canvas.EndPose()
	return canvas.Rectangles()[len(canvas.Rectangles())-1].Bounds.X
}

func TestGlideSlidesFromWhereTheLastFramePaintedIt(t *testing.T) {
	var canvas Canvas
	frames := newTestFrames(&canvas)
	motion := Motion{Duration: 100 * time.Millisecond, Easing: EaseLinear}
	x := func(at time.Duration, layout float32) (got float32) {
		canvas.Reset()
		frames.at(at, func() { got = paintGlided(&canvas, Item(1), layout, motion) })
		return got
	}
	if got := x(0, 100); got != 100 || canvas.Animating() {
		t.Fatalf("first paint = %v animating=%v, want it simply there", got, canvas.Animating())
	}
	// The layout moves it 60 to the left: the eye still sees it at 100.
	if got := x(time.Second, 40); got != 100 {
		t.Fatalf("first frame after the move = %v, want 100", got)
	}
	if got := x(time.Second+50*time.Millisecond, 40); math.Abs(float64(got-70)) > 1e-3 || !canvas.Animating() {
		t.Fatalf("halfway = %v animating=%v, want 70", got, canvas.Animating())
	}
	if got := x(time.Second+200*time.Millisecond, 40); got != 40 || canvas.Animating() {
		t.Fatalf("arrived = %v animating=%v", got, canvas.Animating())
	}
}

func TestGlideInterruptedByAnotherMoveStaysContinuous(t *testing.T) {
	var canvas Canvas
	frames := newTestFrames(&canvas)
	motion := SpringMotion(300*time.Millisecond, 0)
	x := func(at time.Duration, layout float32) (got float32) {
		canvas.Reset()
		frames.at(at, func() { got = paintGlided(&canvas, Item(1), layout, motion) })
		return got
	}
	x(0, 100)
	x(time.Second, 0)
	at := time.Second + 100*time.Millisecond
	seen := x(at, 0)
	// A second move of the layout, by +40: the picture must not jump.
	if got := x(at, 40); math.Abs(float64(got-seen)) > 1e-3 {
		t.Fatalf("second move jumped from %v to %v", seen, got)
	}
}

func TestGlideIsInstantUnderReducedMotion(t *testing.T) {
	var canvas Canvas
	frames := newTestFrames(&canvas)
	canvas.SetReduceMotion(true)
	motion := DefaultMotion()
	canvas.Reset()
	frames.at(0, func() { paintGlided(&canvas, Item(1), 100, motion) })
	canvas.Reset()
	var got float32
	frames.at(time.Second, func() { got = paintGlided(&canvas, Item(1), 0, motion) })
	if got != 0 || canvas.Animating() {
		t.Fatalf("reduced motion = %v animating=%v", got, canvas.Animating())
	}
}

func TestGlideFromStartsWhereTheElementWasCarried(t *testing.T) {
	var canvas Canvas
	frames := newTestFrames(&canvas)
	motion := Motion{Duration: 100 * time.Millisecond, Easing: EaseLinear}
	x := func(at time.Duration, layout float32, from *Point) (got float32) {
		canvas.Reset()
		frames.at(at, func() {
			if from != nil {
				canvas.GlideFrom(Item(1), 0, *from)
			}
			got = paintGlided(&canvas, Item(1), layout, motion)
		})
		return got
	}
	x(0, 100, nil)
	if got := x(time.Second, 40, &Point{X: 300}); got != 300 {
		t.Fatalf("first frame = %v, want the point it was carried to", got)
	}
	if got := x(time.Second+50*time.Millisecond, 40, nil); math.Abs(float64(got-170)) > 1e-3 {
		t.Fatalf("halfway = %v, want 170", got)
	}
	if got := x(2*time.Second, 40, nil); got != 40 {
		t.Fatalf("arrived = %v", got)
	}
}
