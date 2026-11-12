package componentdemo

import (
	"fmt"
	"testing"
	"time"
	"unsafe"

	"github.com/vivaui/viva/gfx"
)

func TestPaintAllocations(t *testing.T) {
	for _, component := range Catalog() {
		for _, example := range component.Examples {
			t.Run(component.ID+"/"+example.ID, func(t *testing.T) {
				state := newTestState(t, component.ID, example.ID)
				var canvas gfx.Canvas
				paint := func() { paintFrame(&canvas, state, testSize, time.Second, true) }
				paint()
				if allocations := testing.AllocsPerRun(100, paint); allocations != 0 {
					t.Fatalf("static warm paint allocated %v times", allocations)
				}
			})
		}
	}
	if allocations := testing.AllocsPerRun(100, func() { Catalog() }); allocations != 0 {
		t.Fatalf("catalog allocated %v times", allocations)
	}
}

func TestPlatformInputDeterminismAndSnapshotAllocations(t *testing.T) {
	for _, example := range inputComponent.Examples {
		for _, focused := range []bool{false, true} {
			for _, dark := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/focused=%t/dark=%t", example.ID, focused, dark), func(t *testing.T) {
					state := newTestState(t, "input", example.ID)
					state.EnableTextEditing()
					state.SetDark(dark)
					var canvas gfx.Canvas
					if focused {
						canvas.SetInteraction(gfx.ID{}, gfx.ID{}, inputEditable, true)
					}
					paint := func() { paintFrame(&canvas, state, testSize, time.Second, true) }
					paint()
					expected := snapshot(&canvas)
					paint()
					assertSamePaint(t, expected, &canvas)
					if canvas.Animating() {
						t.Fatal("native input requested an idle frame")
					}
					// text.Editor caches its string between edits, so even the
					// examples that read the field's value every frame allocate
					// nothing.
					if allocations := testing.AllocsPerRun(100, paint); allocations != 0 {
						t.Fatalf("native snapshot allocations = %v, want 0", allocations)
					}
				})
			}
		}
	}
}

func TestControlledInteractionAllocations(t *testing.T) {
	for _, componentID := range []string{"checkbox", "switch", "dialog"} {
		state := newTestState(t, componentID, "default")
		var canvas gfx.Canvas
		interact := func() {
			switch componentID {
			case "checkbox":
				click(&canvas, state, checkboxBox.At(0))
			case "switch":
				click(&canvas, state, switchToggle.At(0))
			case "dialog":
				if state.dialog.open {
					click(&canvas, state, dialogCancel)
				} else {
					click(&canvas, state, dialogOpen)
				}
			}
			paintFrame(&canvas, state, testSize, time.Second, true)
		}
		interact()
		interact()
		if allocations := testing.AllocsPerRun(100, interact); allocations != 0 {
			t.Fatalf("%s warm interaction allocated %v times", componentID, allocations)
		}
	}
}

func BenchmarkPaint(b *testing.B) {
	for _, component := range Catalog() {
		for _, example := range component.Examples {
			for _, width := range []float32{320, 760} {
				b.Run(fmt.Sprintf("%s/%s/%.0f", component.ID, example.ID, width), func(b *testing.B) {
					state := newTestState(b, component.ID, example.ID)
					var canvas gfx.Canvas
					size := gfx.Size{Width: width, Height: 340}
					paintFrame(&canvas, state, size, time.Second, true)
					b.ReportAllocs()
					b.ResetTimer()
					for range b.N {
						paintFrame(&canvas, state, size, time.Second, true)
					}
				})
			}
		}
	}
}

func BenchmarkNativeInputPaint(b *testing.B) {
	for _, example := range inputComponent.Examples {
		for _, focused := range []bool{false, true} {
			b.Run(fmt.Sprintf("%s/focused=%t", example.ID, focused), func(b *testing.B) {
				state := newTestState(b, "input", example.ID)
				state.EnableTextEditing()
				var canvas gfx.Canvas
				if focused {
					canvas.SetInteraction(gfx.ID{}, gfx.ID{}, inputEditable, true)
				}
				paintFrame(&canvas, state, testSize, time.Second, true)
				b.ReportAllocs()
				b.ResetTimer()
				for range b.N {
					paintFrame(&canvas, state, testSize, time.Second, true)
				}
			})
		}
	}
}

func BenchmarkClickAndPaint(b *testing.B) {
	for _, componentID := range []string{"button", "checkbox", "switch", "dialog"} {
		b.Run(componentID, func(b *testing.B) {
			state := newTestState(b, componentID, "default")
			var canvas gfx.Canvas
			interact := func() {
				switch componentID {
				case "button":
					click(&canvas, state, buttonPress.At(0))
				case "checkbox":
					click(&canvas, state, checkboxBox.At(0))
				case "switch":
					click(&canvas, state, switchToggle.At(0))
				case "dialog":
					click(&canvas, state, dialogOpen)
				}
				paintFrame(&canvas, state, testSize, time.Second, true)
				if componentID == "dialog" {
					click(&canvas, state, dialogCancel)
				}
			}
			interact()
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				interact()
			}
		})
	}
}

func BenchmarkSwitchAnimatedFrame(b *testing.B) {
	state := newTestState(b, "switch", "default")
	var canvas gfx.Canvas
	paintFrame(&canvas, state, testSize, 0, false)
	b.ReportAllocs()
	b.ResetTimer()
	now := time.Duration(0)
	for iteration := 0; iteration < b.N; iteration++ {
		now += 8 * time.Millisecond
		if iteration%20 == 0 {
			clickAt(&canvas, state, switchToggle.At(0), now, false)
		}
		paintFrame(&canvas, state, testSize, now, false)
	}
}

var benchmarkState *State

func BenchmarkNew(b *testing.B) {
	for _, component := range Catalog() {
		b.Run(component.ID, func(b *testing.B) {
			b.ReportAllocs()
			b.ReportMetric(float64(unsafe.Sizeof(State{})), "state-B")
			for range b.N {
				var err error
				benchmarkState, err = New(component.ID, "")
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
	b.Run("platform-input", func(b *testing.B) {
		b.ReportAllocs()
		for range b.N {
			var err error
			benchmarkState, err = New("input", "default")
			if err != nil {
				b.Fatal(err)
			}
			benchmarkState.EnableTextEditing()
		}
	})
}
