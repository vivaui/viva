package input

import "testing"

func TestModifierTapReportsAKeyPressedAndReleasedAlone(t *testing.T) {
	var tap ModifierTap
	tap.Press(KeyShiftLeft, 50, ModifierShift, false)
	key, ok := tap.Release(50)
	if !ok || key != KeyShiftLeft {
		t.Fatalf("release = %v, %v", key, ok)
	}
	if _, ok := tap.Release(50); ok {
		t.Fatal("one tap was reported twice")
	}
}

func TestModifierTapEndsWhenAnythingElseHappens(t *testing.T) {
	cases := map[string]func(*ModifierTap){
		"cancel (a key, text, pointer or focus loss)": func(tap *ModifierTap) { tap.Cancel() },
		"a second modifier":                           func(tap *ModifierTap) { tap.Press(KeyControlLeft, 51, ModifierShift|ModifierControl, false) },
		"another modifier of the same kind":           func(tap *ModifierTap) { tap.Press(KeyShiftRight, 52, ModifierShift, false) },
	}
	for name, interrupt := range cases {
		var tap ModifierTap
		tap.Press(KeyShiftLeft, 50, ModifierShift, false)
		interrupt(&tap)
		if _, ok := tap.Release(50); ok {
			t.Errorf("%s: still reported as a tap", name)
		}
	}
}

func TestModifierTapIgnoresAModifierPressedWhileAnotherIsHeld(t *testing.T) {
	var tap ModifierTap
	tap.Press(KeyShiftLeft, 50, ModifierShift|ModifierControl, false)
	if _, ok := tap.Release(50); ok {
		t.Fatal("Shift with Control already down is a chord, not a tap")
	}
}

func TestModifierTapIgnoresAutoRepeatOfTheHeldKey(t *testing.T) {
	var tap ModifierTap
	tap.Press(KeyShiftLeft, 50, ModifierShift, false)
	tap.Press(KeyShiftLeft, 50, ModifierShift, true)
	tap.Press(KeyShiftLeft, 50, ModifierShift, true)
	if _, ok := tap.Release(50); !ok {
		t.Fatal("holding the key made the repeats cancel the tap")
	}
}

func TestModifierTapNeedsACodeToPairThePressWithItsRelease(t *testing.T) {
	var tap ModifierTap
	tap.Press(KeyShiftLeft, 0, ModifierShift, false)
	if _, ok := tap.Release(0); ok {
		t.Fatal("a key with no code was reported")
	}
}

func TestModifierTapIgnoresOtherKeysReleases(t *testing.T) {
	var tap ModifierTap
	tap.Press(KeyShiftLeft, 50, ModifierShift, false)
	if _, ok := tap.Release(99); ok {
		t.Fatal("another key's release ended the tap")
	}
	if _, ok := tap.Release(50); !ok {
		t.Fatal("the tap was lost")
	}
}

func TestModifierTapDoesNotAllocate(t *testing.T) {
	var tap ModifierTap
	if allocations := testing.AllocsPerRun(100, func() {
		tap.Press(KeyShiftLeft, 50, ModifierShift, false)
		tap.Release(50)
		tap.Cancel()
	}); allocations != 0 {
		t.Fatalf("%v allocations", allocations)
	}
}
