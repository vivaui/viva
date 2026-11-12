package componentdemo

import (
	"testing"

	"github.com/vivaui/viva/gfx"
)

func TestInputOTPDefaultExampleIsACodeStillBeingTyped(t *testing.T) {
	state, err := New("input-otp", "default")
	if err != nil {
		t.Fatal(err)
	}
	var canvas gfx.Canvas
	state.Paint(&canvas, gfx.Size{Width: 460, Height: 340})
	digits := 0
	for _, text := range canvas.Texts() {
		switch text.Value {
		case "4", "8", "2":
			digits++
		case "9", "0", "1":
			t.Fatalf("the digit %q was drawn in a slot the code has not reached", text.Value)
		}
	}
	if digits != 3 {
		t.Fatalf("%d digits painted, want the 3 typed so far", digits)
	}
	caret := 0
	for _, rectangle := range canvas.Rectangles() {
		if rectangle.Bounds.Width == 1 && rectangle.Bounds.Height == 16 {
			caret++
		}
	}
	if caret != 1 {
		t.Fatalf("%d carets painted, want exactly the one in the empty active slot", caret)
	}
}

func TestInputOTPCompleteCodeShowsNoCaret(t *testing.T) {
	state, err := New("input-otp", "states")
	if err != nil {
		t.Fatal(err)
	}
	var canvas gfx.Canvas
	state.Paint(&canvas, gfx.Size{Width: 460, Height: 340})
	for _, rectangle := range canvas.Rectangles() {
		if rectangle.Bounds.Width == 1 && rectangle.Bounds.Height == 16 {
			t.Fatal("a caret was drawn through a digit of a complete code")
		}
	}
}
