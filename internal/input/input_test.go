package input

import "github.com/vivaui/viva/gfx"

import "testing"

func TestHitTestUsesFrontToBackOrderAndSkipsDisabled(t *testing.T) {
	var router Router
	router.SetRegions([]Region{
		{Target: gfx.Item(1), Bounds: Rect{Width: 100, Height: 100}},
		{Target: gfx.Item(2), Bounds: Rect{X: 20, Y: 20, Width: 60, Height: 60}, Disabled: true},
		{Target: gfx.Item(3), Bounds: Rect{X: 30, Y: 30, Width: 40, Height: 40}},
	})

	if target, ok := router.HitTest(50, 50); !ok || target != gfx.Item(3) {
		t.Fatalf("overlap hit = %d, %v; want 3", target, ok)
	}
	if target, ok := router.HitTest(25, 25); !ok || target != gfx.Item(1) {
		t.Fatalf("disabled overlap hit = %d, %v; want 1", target, ok)
	}
}

func TestPointerDownFocusesFocusableTarget(t *testing.T) {
	var router Router
	router.SetRegions([]Region{
		{Target: gfx.Item(1), Bounds: Rect{Width: 50, Height: 50}},
		{Target: gfx.Item(2), Bounds: Rect{X: 50, Width: 50, Height: 50}, Focusable: true},
	})

	result := router.PointerDown(75, 25)
	if result.Target != gfx.Item(2) || !result.FocusChanged {
		t.Fatalf("pointer result = %+v", result)
	}
	if focused, ok := router.Focused(); !ok || focused != gfx.Item(2) {
		t.Fatalf("focus = %d, %v", focused, ok)
	}
	result = router.PointerDown(25, 25)
	if result.Target != gfx.Item(1) || result.FocusChanged {
		t.Fatalf("non-focusable pointer result = %+v", result)
	}
}

func TestTabTraversalWrapsAndSkipsUnavailableTargets(t *testing.T) {
	var router Router
	router.SetRegions([]Region{
		{Target: gfx.Item(1), Focusable: true},
		{Target: gfx.Item(2), Focusable: true, Disabled: true},
		{Target: gfx.Item(3)},
		{Target: gfx.Item(4), Focusable: true},
	})

	if result := router.KeyDown(KeyEvent{Key: KeyTab}); result.Target != gfx.Item(1) || !result.FocusChanged {
		t.Fatalf("first tab = %+v", result)
	}
	if result := router.KeyDown(KeyEvent{Key: KeyTab}); result.Target != gfx.Item(4) || !result.FocusChanged {
		t.Fatalf("second tab = %+v", result)
	}
	if result := router.KeyDown(KeyEvent{Key: KeyTab}); result.Target != gfx.Item(1) || !result.FocusChanged {
		t.Fatalf("wrapped tab = %+v", result)
	}
	if result := router.KeyDown(KeyEvent{Key: KeyTab, Modifiers: ModifierShift}); result.Target != gfx.Item(4) {
		t.Fatalf("reverse tab = %+v", result)
	}
}

func TestActivationUsesFocusedTarget(t *testing.T) {
	var router Router
	router.SetRegions([]Region{{Target: gfx.Item(9), Focusable: true}})
	router.Focus(gfx.Item(9))
	for _, key := range []Key{KeyEnter, KeySpace} {
		result := router.KeyDown(KeyEvent{Key: key})
		if !result.Activated || result.Target != gfx.Item(9) {
			t.Fatalf("activation for %d = %+v", key, result)
		}
	}
}

func TestActivationIgnoresKeyRepeat(t *testing.T) {
	var router Router
	router.SetRegions([]Region{{Target: gfx.Item(7), Bounds: Rect{Width: 20, Height: 20}, Focusable: true}})
	router.Focus(gfx.Item(7))
	for _, key := range []Key{KeyEnter, KeySpace} {
		if result := router.KeyDown(KeyEvent{Key: key, Repeat: true}); result.Activated {
			t.Fatalf("repeat key %d activated target", key)
		}
	}
}

func TestRegionReplacementPreservesOrClearsFocus(t *testing.T) {
	var router Router
	router.SetRegions([]Region{{Target: gfx.Item(1), Focusable: true}, {Target: gfx.Item(2), Focusable: true}})
	router.Focus(gfx.Item(2))
	router.SetRegions([]Region{{Target: gfx.Item(2), Focusable: true}, {Target: gfx.Item(3), Focusable: true}})
	if focused, ok := router.Focused(); !ok || focused != gfx.Item(2) {
		t.Fatalf("preserved focus = %d, %v", focused, ok)
	}
	router.SetRegions([]Region{{Target: gfx.Item(2), Focusable: true, Disabled: true}})
	if _, ok := router.Focused(); ok {
		t.Fatal("disabled target retained focus")
	}
}

func TestPassesSecondaryOnlyOnTheFrontMostPassingBackdrop(t *testing.T) {
	var router Router
	router.SetRegions([]Region{
		{Target: gfx.Item(1), Bounds: Rect{Width: 100, Height: 100}},
		{Target: gfx.Item(2), Bounds: Rect{Width: 100, Height: 100}, Backdrop: true, PassSecondary: true},
		{Target: gfx.Item(3), Bounds: Rect{Width: 20, Height: 20}},
		{Target: gfx.Item(4), Bounds: Rect{X: 50, Width: 50, Height: 20}, Backdrop: true},
	})
	if !router.PassesSecondary(30, 50) {
		t.Fatal("a passing backdrop over a page control does not pass a secondary press")
	}
	if router.PassesSecondary(10, 10) {
		t.Fatal("a control in front of the backdrop passes")
	}
	if router.PassesSecondary(70, 10) {
		t.Fatal("a backdrop that does not pass, in front of one that does, passes")
	}
	if router.PassesSecondary(200, 200) {
		t.Fatal("empty space passes")
	}
}

func BenchmarkPassesSecondary10K(b *testing.B) {
	regions := make([]Region, 10_000)
	for index := range regions {
		regions[index] = Region{Target: gfx.Item(index + 1), Bounds: Rect{X: float32(index), Width: 1, Height: 1}}
	}
	regions[len(regions)-1] = Region{Target: gfx.Item(1_000_000), Bounds: Rect{X: -10, Y: -10, Width: 20, Height: 20}, Backdrop: true, PassSecondary: true}
	var router Router
	router.SetRegions(regions)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		router.PassesSecondary(-1, -1)
	}
}

func BenchmarkHitTest10KMiss(b *testing.B) {
	regions := make([]Region, 10_000)
	for index := range regions {
		regions[index] = Region{
			Target: gfx.Item(int(index + 1)),
			Bounds: Rect{X: float32(index), Width: 1, Height: 1},
		}
	}
	var router Router
	router.SetRegions(regions)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		router.HitTest(-1, -1)
	}
}

func BenchmarkFocusNext10K(b *testing.B) {
	regions := make([]Region, 10_000)
	for index := range regions {
		regions[index] = Region{Target: gfx.Item(int(index + 1)), Focusable: true}
	}
	var router Router
	router.SetRegions(regions)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		router.KeyDown(KeyEvent{Key: KeyTab})
	}
}

func TestFocusScopeFiltersInteractionAndRestoresFocus(t *testing.T) {
	var router Router
	router.SetRegions([]Region{
		{Target: gfx.Item(1), Bounds: Rect{Width: 20, Height: 20}, Focusable: true},
		{Target: gfx.Item(2), Bounds: Rect{X: 30, Width: 20, Height: 20}, Scope: gfx.Item(9), Focusable: true},
		{Target: gfx.Item(3), Bounds: Rect{X: 60, Width: 20, Height: 20}, Scope: gfx.Item(9), Focusable: true},
	})
	router.Focus(gfx.Item(1))
	if !router.SetActiveScope(gfx.Item(9), gfx.ID{}) {
		t.Fatal("opening scope did not clear outer focus")
	}
	if _, ok := router.HitTest(10, 10); ok {
		t.Fatal("outer hit remained active inside focus trap")
	}
	if target, ok := router.HitTest(35, 10); !ok || target != gfx.Item(2) {
		t.Fatalf("scoped hit = %d, %v", target, ok)
	}
	if result := router.KeyDown(KeyEvent{Key: KeyTab}); result.Target != gfx.Item(2) {
		t.Fatalf("first scoped tab = %+v", result)
	}
	if result := router.KeyDown(KeyEvent{Key: KeyTab}); result.Target != gfx.Item(3) {
		t.Fatalf("second scoped tab = %+v", result)
	}

	router.SetRegions([]Region{{Target: gfx.Item(1), Bounds: Rect{Width: 20, Height: 20}, Focusable: true}})
	if !router.SetActiveScope(gfx.ID{}, gfx.ID{}) {
		t.Fatal("closing scope did not restore outer focus")
	}
	if focused, ok := router.Focused(); !ok || focused != gfx.Item(1) {
		t.Fatalf("restored focus = %d, %v", focused, ok)
	}
}

func TestADialogThatReplacesAnotherInTheSameScopeTakesFocus(t *testing.T) {
	var router Router
	router.SetRegions([]Region{
		{Target: gfx.Item(2), Bounds: Rect{X: 30, Width: 20, Height: 20}, Scope: gfx.Item(9), Focusable: true},
		{Target: gfx.Item(3), Bounds: Rect{X: 60, Width: 20, Height: 20}, Scope: gfx.Item(9), Focusable: true},
	})
	router.SetActiveScope(gfx.Item(9), gfx.Item(2))
	router.SetRegions([]Region{
		{Target: gfx.Item(4), Bounds: Rect{X: 30, Width: 20, Height: 20}, Scope: gfx.Item(9), Focusable: true},
		{Target: gfx.Item(5), Bounds: Rect{X: 60, Width: 20, Height: 20}, Scope: gfx.Item(9), Focusable: true},
	})
	if !router.SetActiveScope(gfx.Item(9), gfx.Item(4)) {
		t.Fatal("the replacing dialog's initial target did not take focus")
	}
	if focused, _ := router.Focused(); focused != gfx.Item(4) {
		t.Fatalf("focus = %d, want 4", focused)
	}
	router.Focus(gfx.Item(5))
	if router.SetActiveScope(gfx.Item(9), gfx.Item(4)) {
		t.Fatal("announcing the same dialog again moved focus")
	}
	if focused, _ := router.Focused(); focused != gfx.Item(5) {
		t.Fatalf("focus = %d, want the user's 5", focused)
	}
}

func TestFocusScopeOpensOnInitialTargetWithoutStealingLater(t *testing.T) {
	var router Router
	regions := []Region{
		{Target: gfx.Item(1), Bounds: Rect{Width: 20, Height: 20}, Focusable: true},
		{Target: gfx.Item(2), Bounds: Rect{X: 30, Width: 20, Height: 20}, Scope: gfx.Item(9), Focusable: true},
		{Target: gfx.Item(3), Bounds: Rect{X: 60, Width: 20, Height: 20}, Scope: gfx.Item(9), Focusable: true},
	}
	router.SetRegions(regions)
	router.Focus(gfx.Item(1))

	if !router.SetActiveScope(gfx.Item(9), gfx.Item(3)) {
		t.Fatal("opening a scope with an initial target did not move focus")
	}
	if focused, _ := router.Focused(); focused != gfx.Item(3) {
		t.Fatalf("focus after open = %d, want the initial target 3", focused)
	}

	// The scope stays open and is announced again every frame; the user has
	// since moved focus and must not be pulled back.
	router.Focus(gfx.Item(2))
	if router.SetActiveScope(gfx.Item(9), gfx.Item(3)) {
		t.Fatal("re-announcing the open scope moved focus")
	}
	if focused, _ := router.Focused(); focused != gfx.Item(2) {
		t.Fatalf("focus after re-announce = %d, want 2", focused)
	}

	// Closing restores the control that opened the scope.
	if !router.SetActiveScope(gfx.ID{}, gfx.ID{}) {
		t.Fatal("closing the scope did not restore focus")
	}
	if focused, _ := router.Focused(); focused != gfx.Item(1) {
		t.Fatalf("restored focus = %d, want 1", focused)
	}
}

func TestFocusScopeIgnoresUnavailableInitialTarget(t *testing.T) {
	for name, initial := range map[string]Target{
		"outside the scope": gfx.Item(1),
		"not focusable":     gfx.Item(4),
		"disabled":          gfx.Item(5),
		"absent":            gfx.Item(99),
	} {
		t.Run(name, func(t *testing.T) {
			var router Router
			router.SetRegions([]Region{
				{Target: gfx.Item(1), Bounds: Rect{Width: 20, Height: 20}, Focusable: true},
				{Target: gfx.Item(2), Bounds: Rect{X: 30, Width: 20, Height: 20}, Scope: gfx.Item(9), Focusable: true},
				{Target: gfx.Item(4), Bounds: Rect{X: 60, Width: 20, Height: 20}, Scope: gfx.Item(9)},
				{Target: gfx.Item(5), Bounds: Rect{X: 90, Width: 20, Height: 20}, Scope: gfx.Item(9), Focusable: true, Disabled: true},
			})
			router.Focus(gfx.Item(1))
			router.SetActiveScope(gfx.Item(9), initial)
			if focused, ok := router.Focused(); ok {
				t.Fatalf("an unavailable initial target left focus on %d, want none", focused)
			}
		})
	}
}

func TestBlockingRegionStopsPressesReachingTargetsDeclaredBeforeIt(t *testing.T) {
	var router Router
	router.SetRegions([]Region{
		{Target: gfx.Item(1), Bounds: Rect{Width: 100, Height: 100}, Scope: gfx.Item(9)},                              // full-window dismiss
		{Bounds: Rect{X: 20, Y: 20, Width: 60, Height: 60}, Scope: gfx.Item(9), Blocks: true},                         // the panel
		{Target: gfx.Item(2), Bounds: Rect{X: 30, Y: 30, Width: 20, Height: 20}, Scope: gfx.Item(9), Focusable: true}, // a button on the panel
	})
	router.SetActiveScope(gfx.Item(9), gfx.ID{})

	if target, ok := router.HitTest(5, 5); !ok || target != gfx.Item(1) {
		t.Fatalf("outside the panel = %d, %v; want the dismiss target", target, ok)
	}
	if target, ok := router.HitTest(70, 70); ok {
		t.Fatalf("panel padding = %d, %v; the press must reach nothing", target, ok)
	}
	if target, ok := router.HitTest(40, 40); !ok || target != gfx.Item(2) {
		t.Fatalf("button on the panel = %d, %v; later hits still win over the block", target, ok)
	}
	if result := router.PointerDown(70, 70); !result.Target.IsZero() || result.FocusChanged {
		t.Fatalf("pressing the panel = %+v, want no target and no focus change", result)
	}
	// A block is never a focus stop.
	if result := router.KeyDown(KeyEvent{Key: KeyTab}); result.Target != gfx.Item(2) {
		t.Fatalf("Tab landed on %d, want the button", result.Target)
	}
}

func TestBlockOutsideTheActiveScopeIsIgnored(t *testing.T) {
	var router Router
	router.SetRegions([]Region{
		{Target: gfx.Item(1), Bounds: Rect{Width: 100, Height: 100}},
		{Bounds: Rect{X: 20, Y: 20, Width: 60, Height: 60}, Scope: gfx.Item(9), Blocks: true},
	})
	router.SetActiveScope(gfx.ID{}, gfx.ID{})
	if target, ok := router.HitTest(50, 50); ok && target != gfx.Item(1) {
		t.Fatalf("hit %d", target)
	}
	router.SetActiveScope(gfx.Item(5), gfx.ID{}) // another trap is active: scope 9 is not in force
	if _, ok := router.HitTest(50, 50); ok {
		t.Fatal("a block from a scope that is not active swallowed a press")
	}
}

func TestPressingALabelFocusesItsControlAndIsNoTabStop(t *testing.T) {
	field, other := gfx.Item(1), gfx.Item(2)
	var router Router
	router.SetRegions([]Region{
		{Target: field, Bounds: Rect{X: 100, Y: 0, Width: 80, Height: 20}, Focusable: true},
		{Target: field, Bounds: Rect{X: 0, Y: 0, Width: 60, Height: 20}, Label: true},
		{Target: other, Bounds: Rect{X: 0, Y: 40, Width: 80, Height: 20}, Focusable: true},
	})
	result := router.PointerDown(10, 10)
	if result.Target != field || !result.FocusChanged {
		t.Fatalf("pressing the label: %+v, want focus on the field", result)
	}
	if focused, _ := router.Focused(); focused != field {
		t.Fatalf("focused %v, want the field", focused)
	}
	if next := router.KeyDown(KeyEvent{Key: KeyTab}); next.Target != other {
		t.Fatalf("Tab from the field reached %v, want the next control, not its label", next.Target)
	}
	router.Blur()
	router.SetRegions([]Region{
		{Target: field, Bounds: Rect{X: 100, Y: 0, Width: 80, Height: 20}, Focusable: true, Disabled: true},
		{Target: field, Bounds: Rect{X: 0, Y: 0, Width: 60, Height: 20}, Label: true, Disabled: true},
	})
	if result := router.PointerDown(10, 10); result.FocusChanged {
		t.Fatalf("pressing the label of a disabled control: %+v", result)
	}
}

// Tab skips a roving group's non-selected items, but an arrow key can still
// focus one explicitly; that is what lets a tab list have one tab stop.
func TestTabSkipsRovingItemsThatStayFocusable(t *testing.T) {
	var router Router
	router.SetRegions([]Region{
		{Target: gfx.Item(1), Focusable: true},
		{Target: gfx.Item(2), Focusable: true, SkipTab: true},
		{Target: gfx.Item(3), Focusable: true},
	})
	if result := router.KeyDown(KeyEvent{Key: KeyTab}); result.Target != gfx.Item(1) {
		t.Fatalf("first tab = %+v", result)
	}
	if result := router.KeyDown(KeyEvent{Key: KeyTab}); result.Target != gfx.Item(3) {
		t.Fatalf("Tab reached a roving item: %+v", result)
	}
	if !router.Focus(gfx.Item(2)) {
		t.Fatal("a roving item is not focusable by arrow key or explicit focus")
	}
	if result := router.KeyDown(KeyEvent{Key: KeyTab}); result.Target != gfx.Item(3) {
		t.Fatalf("Tab from a roving item = %+v", result)
	}
}

// Enter activates buttons, tabs, menu items and options, but not checkboxes,
// radios, switches or sliders; Space activates the checkable controls.
func TestActivationFollowsTheSemanticRole(t *testing.T) {
	for _, test := range []struct {
		role         gfx.Role
		enter, space bool
	}{
		{gfx.RoleButton, true, true},
		{gfx.RoleTab, true, true},
		{gfx.RoleMenuItem, true, true},
		{gfx.RoleCheckbox, false, true},
		{gfx.RoleRadio, false, true},
		{gfx.RoleSwitch, false, true},
		{gfx.RoleSlider, false, false},
		{gfx.RoleGroup, false, false},
	} {
		var router Router
		router.SetRegions([]Region{{Target: gfx.Item(1), Focusable: true, Role: test.role}})
		router.Focus(gfx.Item(1))
		if got := router.KeyDown(KeyEvent{Key: KeyEnter}).Activated; got != test.enter {
			t.Errorf("role %v Enter activated=%v, want %v", test.role, got, test.enter)
		}
		if got := router.KeyDown(KeyEvent{Key: KeySpace}).Activated; got != test.space {
			t.Errorf("role %v Space activated=%v, want %v", test.role, got, test.space)
		}
	}
}

// A focusable disabled control takes focus but never activates.
func TestDisabledFocusableDoesNotActivate(t *testing.T) {
	var router Router
	router.SetRegions([]Region{{Target: gfx.Item(1), Focusable: true, Disabled: true, FocusableDisabled: true, Role: gfx.RoleButton}})
	if !router.Focus(gfx.Item(1)) {
		t.Fatal("a focusable disabled control could not take focus")
	}
	if result := router.KeyDown(KeyEvent{Key: KeyEnter}); result.Activated {
		t.Fatalf("Enter activated a disabled control: %+v", result)
	}
}

func TestPressAtWalksFromTheFrontAndFindsTheWindowMoveRegion(t *testing.T) {
	const modal = 9
	region := Region{Bounds: Rect{Width: 400, Height: 40}, MovesWindow: true}
	control := Region{Target: gfx.Item(1), Bounds: Rect{X: 10, Width: 40, Height: 40}}
	backdrop := Region{Target: gfx.Item(2), Bounds: Rect{Width: 400, Height: 300}, Backdrop: true, Scope: gfx.Item(modal)}
	panel := Region{Bounds: Rect{X: 100, Width: 100, Height: 300}, Blocks: true, Scope: gfx.Item(modal)}
	for _, test := range []struct {
		name    string
		regions []Region
		scope   gfx.ID
		x, y    float32
		want    PressKind
	}{
		{"no move region is the application's", []Region{control}, gfx.ID{}, 300, 20, PressClient},
		{"empty move region", []Region{region}, gfx.ID{}, 300, 20, PressMove},
		{"a control over the region", []Region{region, control}, gfx.ID{}, 20, 20, PressControl},
		{"outside the region", []Region{region}, gfx.ID{}, 300, 100, PressClient},
		{"a backdrop over the region", []Region{region, backdrop}, gfx.Item(modal), 300, 20, PressMoveOnDrag},
		{"a backdrop where no region lies", []Region{region, backdrop}, gfx.Item(modal), 300, 100, PressClient},
		{"a control under a modal is out of reach, the region is not", []Region{region, control, backdrop}, gfx.Item(modal), 20, 20, PressMoveOnDrag},
		{"the modal's own surface", []Region{region, backdrop, panel}, gfx.Item(modal), 150, 20, PressClient},
		{"a surface of a scope that is not active is skipped", []Region{region, Region{Bounds: Rect{Width: 400, Height: 40}, Blocks: true, Scope: gfx.Item(7)}}, gfx.Item(modal), 20, 20, PressMove},
	} {
		var router Router
		router.SetRegions(test.regions)
		router.SetActiveScope(test.scope, gfx.ID{})
		if got := router.PressAt(test.x, test.y); got != test.want {
			t.Errorf("%s: PressAt = %d, want %d", test.name, got, test.want)
		}
	}
}
